package monitoring

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-sasl"
	"github.com/google/uuid"
	"github.com/patrick246/mail-e2e/internal/config"
	"github.com/patrick246/mail-e2e/internal/logging"
	"github.com/patrick246/mail-e2e/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"net/smtp"
	"text/template"
	"time"
)

var ErrShutdownTimeout = errors.New("shutdown timeout")

var mailTemplate = template.Must(template.New("testmail").Parse(`To: {{ .To }}
From: {{ .From }}
Subject: {{ .Subject }}
X-Mail-E2E-ID: {{ .ID }}

This is a mail for end-to-end monitoring. 
`))

type TargetMonitor struct {
	target config.Target
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	inflightMails map[string]MailMetadata
}

type MailMetadata struct {
}

func NewTargetMonitor(target config.Target) *TargetMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &TargetMonitor{
		target: target,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
}

func (t *TargetMonitor) Start() {
	log := logging.CreateLogger("target").With("target", t.target.Name)
	for {
		select {
		case <-t.ctx.Done():
			close(t.done)
			return
		case <-time.After(30 * time.Second):
		}

		mailId := uuid.New().String()
		log.Debugw("starting e2e check", "id", mailId, "to", t.target.SMTP.To)

		timer := prometheus.NewTimer(metrics.MailDelay.WithLabelValues(t.target.Name))

		err := t.sendSingleMail(mailId, log)
		metrics.MailSent.WithLabelValues(t.target.Name).Inc()
		if err != nil {
			metrics.MailSentError.WithLabelValues(t.target.Name).Inc()
			continue
		}

		err = t.receiveMail(mailId, log)
		metrics.MailReceived.WithLabelValues(t.target.Name).Inc()
		if err != nil {
			metrics.MailReceivedError.WithLabelValues(t.target.Name).Inc()
		}

		log.Debugw("e2e check done", "id", mailId)
		timer.ObserveDuration()
	}
}

func (t *TargetMonitor) sendSingleMail(mailId string, log *zap.SugaredLogger) error {
	mailBody := bytes.Buffer{}
	err := mailTemplate.Execute(&mailBody, map[string]string{
		"To":      t.target.SMTP.To,
		"From":    t.target.SMTP.From,
		"Subject": "",
		"ID":      mailId,
	})
	if err != nil {
		log.Errorw("mail template error", "error", err)
		return err
	}

	var auth smtp.Auth
	if t.target.SMTP.Username != "" && t.target.SMTP.Password != "" {
		auth = smtp.PlainAuth("", t.target.SMTP.Username, t.target.SMTP.Password, t.target.SMTP.Hostname)
	}

	err = smtp.SendMail(
		fmt.Sprintf("%s:%d", t.target.SMTP.Hostname, t.target.SMTP.Port),
		auth,
		t.target.SMTP.From,
		[]string{t.target.SMTP.To},
		mailBody.Bytes(),
	)
	if err != nil {
		log.Warnw("mail send error", "id", mailId, "to", t.target.SMTP.To, "error", err)
		return err
	}
	return nil
}

func (t *TargetMonitor) receiveMail(mailId string, log *zap.SugaredLogger) (err error) {
	c, err := client.DialTLS(fmt.Sprintf("%s:%d", t.target.IMAP.Hostname, t.target.IMAP.Port), &tls.Config{
		InsecureSkipVerify: t.target.IMAP.InsecureSkipVerify,
	})
	if err != nil {
		log.Errorw("connection error", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port, "insecureSkipVerify", t.target.IMAP.InsecureSkipVerify)
		return err
	}
	defer func() {
		closeErr := c.Logout()
		if err == nil && closeErr != nil {
			log.Errorw("connection close error", "error", err)
			err = closeErr
		}
	}()

	err = c.Authenticate(sasl.NewPlainClient("", t.target.IMAP.Username, t.target.IMAP.Password))
	if err != nil {
		log.Errorw("login error", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port, "username", t.target.IMAP.Username)
		return err
	}

	inbox, err := c.Select(imap.InboxName, false)
	if err != nil {
		log.Errorw("select error", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port, "username", t.target.IMAP.Username, "mailbox", imap.InboxName)
		return err
	}

	log.Infow("inbox state", "messages", inbox.Messages)

	criteria := imap.NewSearchCriteria()
	criteria.Header.Set("X-Mail-E2E-ID", mailId)

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		log.Infow("searching mail", "deadline", deadline.Format(time.RFC3339), "remaining", deadline.Sub(time.Now()).String())

		seqNums, err := c.Search(criteria)
		if err != nil {
			log.Errorw("search error", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port, "id", mailId, "error", err)
			return err
		}

		if len(seqNums) == 0 {
			time.Sleep(1 * time.Second)
			continue
		}
		if len(seqNums) > 1 {
			log.Errorw("duplicate mail id", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port)
			return errors.New("duplicate mail id")
		}

		seqSet := imap.SeqSet{}
		seqSet.AddNum(seqNums...)
		err = c.Store(&seqSet, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil)
		if err != nil {
			log.Errorw("mail delete error", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port, "error", err)
			return err
		}

		err = c.Expunge(nil)
		if err != nil {
			log.Errorw("expunge error", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port, "error", err)
			return err
		}

		err = t.cleanMailbox(c)
		if err != nil {
			log.Errorw("mailbox clean error", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port, "error", err)
		}
		return nil
	}
	return errors.New("deadline exceeded")
}

func (t *TargetMonitor) cleanMailbox(c *client.Client) error {
	criteria := imap.NewSearchCriteria()
	criteria.SentBefore = time.Now().Add(-5 * time.Minute)
	seqNums, err := c.Search(criteria)
	if err != nil {
		return err
	}

	if len(seqNums) == 0 {
		return nil
	}

	seqSet := &imap.SeqSet{}
	seqSet.AddNum(seqNums...)

	err = c.Store(seqSet, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil)
	if err != nil {
		return err
	}

	err = c.Expunge(nil)
	if err != nil {
		return err
	}
	return nil
}

func (t *TargetMonitor) Shutdown(ctx context.Context) error {
	t.cancel()

	select {
	case <-ctx.Done():
		return ErrShutdownTimeout
	case <-t.done:
		return nil
	}
}
