package monitoring

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/smtp"
	"text/template"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-sasl"
	"github.com/google/uuid"

	"github.com/patrick246/mail-e2e/internal/config"
	"github.com/patrick246/mail-e2e/internal/logging"
	"github.com/patrick246/mail-e2e/internal/metrics"
)

var (
	ErrShutdownTimeout  = errors.New("shutdown timeout")
	ErrDuplicateMail    = errors.New("duplicate mail id")
	ErrDeadlineExceeded = errors.New("deadline exceeded")
)

//nolint:gochecknoglobals // compile once, is constant
var mailTemplate = template.Must(template.New("testmail").Parse(`To: {{ .To }}
From: {{ .From }}
Subject: {{ .Subject }}
X-Mail-E2E-ID: {{ .ID }}

This is a mail for end-to-end monitoring. 
`))

const defaultInterval = 30 * time.Second

type TargetMonitor struct {
	target  *config.Target
	cancel  context.CancelFunc
	done    chan struct{}
	metrics *metrics.Metrics
}

func NewTargetMonitor(target *config.Target, metrics *metrics.Metrics) *TargetMonitor {
	return &TargetMonitor{
		target:  target,
		done:    make(chan struct{}),
		metrics: metrics,
	}
}

func (t *TargetMonitor) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	t.cancel = cancel

	log := logging.CreateLogger("target").With("target", t.target.Name)

	interval := t.target.Interval

	if interval == 0 {
		interval = defaultInterval
	}

	for {
		select {
		case <-ctx.Done():
			close(t.done)
			return
		case <-time.After(interval):
		}

		mailID := uuid.New().String()
		log.Debug("starting e2e check", "id", mailID, "to", t.target.SMTP.To)

		startTime := time.Now()

		err := t.sendSingleMail(mailID, log)
		t.metrics.IncMailSent(t.target.Name)

		if err != nil {
			t.metrics.IncMailSentError(t.target.Name)
			continue
		}

		err = t.receiveMail(mailID, log)
		t.metrics.IncMailReceived(t.target.Name)

		if err != nil {
			t.metrics.IncMailReceivedError(t.target.Name)
		}

		log.Debug("e2e check done", "id", mailID)
		t.metrics.ObserveMailDelay(t.target.Name, time.Since(startTime))
	}
}

func (t *TargetMonitor) sendSingleMail(mailID string, log *slog.Logger) error {
	mailBody := bytes.Buffer{}

	err := mailTemplate.Execute(&mailBody, map[string]string{
		"To":      t.target.SMTP.To,
		"From":    t.target.SMTP.From,
		"Subject": "",
		"ID":      mailID,
	})
	if err != nil {
		log.Error("mail template error", "error", err)
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
		log.Warn("mail send error", "id", mailID, "to", t.target.SMTP.To, "error", err)
		return err
	}

	return nil
}

func (t *TargetMonitor) receiveMail(mailID string, log *slog.Logger) (err error) {
	c, err := client.DialTLS(fmt.Sprintf("%s:%d", t.target.IMAP.Hostname, t.target.IMAP.Port), &tls.Config{
		InsecureSkipVerify: t.target.IMAP.InsecureSkipVerify, //nolint:gosec // Yes, this might be insecure. It has insecure in the name.
	})
	if err != nil {
		log.Error("connection error",
			"host", t.target.IMAP.Hostname,
			"port", t.target.IMAP.Port,
			"insecureSkipVerify", t.target.IMAP.InsecureSkipVerify,
		)

		return err
	}

	defer func() {
		closeErr := c.Logout()
		if err == nil && closeErr != nil {
			log.Error("connection close error", "error", err)
			err = closeErr
		}
	}()

	err = c.Authenticate(sasl.NewPlainClient("", t.target.IMAP.Username, t.target.IMAP.Password))
	if err != nil {
		log.Error("login error", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port, "username", t.target.IMAP.Username)
		return err
	}

	inbox, err := c.Select(imap.InboxName, false)
	if err != nil {
		log.Error(
			"select error",
			"host", t.target.IMAP.Hostname,
			"port", t.target.IMAP.Port,
			"username", t.target.IMAP.Username,
			"mailbox", imap.InboxName,
		)

		return err
	}

	log.Info("inbox state", "messages", inbox.Messages)

	interval := t.target.Interval
	if interval == 0 {
		interval = defaultInterval
	}

	deadline := time.Now().Add(interval)
	for time.Now().Before(deadline) {
		found, err := t.searchMail(c, log, deadline, mailID)
		if err != nil {
			log.Error("mailbox search error", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port)
		}

		if !found {
			time.Sleep(1 * time.Second)

			continue
		}

		err = t.cleanMailbox(c)
		if err != nil {
			log.Error("mailbox clean error", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port, "error", err)
		}

		return nil
	}

	return ErrDeadlineExceeded
}

func (t *TargetMonitor) searchMail(
	c *client.Client, log *slog.Logger, deadline time.Time, mailID string,
) (bool, error) {
	log.Info("searching mail",
		"deadline", deadline.Format(time.RFC3339),
		"remaining", time.Until(deadline).String(),
	)

	criteria := imap.NewSearchCriteria()
	criteria.Header.Set("X-Mail-E2E-ID", mailID)

	seqNums, err := c.Search(criteria)
	if err != nil {
		log.Error("search error", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port, "id", mailID, "error", err)
		return false, err
	}

	if len(seqNums) == 0 {
		return false, nil
	}

	if len(seqNums) > 1 {
		log.Error("duplicate mail id", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port)
		return false, ErrDuplicateMail
	}

	seqSet := imap.SeqSet{}
	seqSet.AddNum(seqNums...)

	err = c.Store(&seqSet, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil)
	if err != nil {
		log.Error("mail delete error", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port, "error", err)
		return false, err
	}

	err = c.Expunge(nil)
	if err != nil {
		log.Error("expunge error", "host", t.target.IMAP.Hostname, "port", t.target.IMAP.Port, "error", err)
		return false, err
	}

	return true, nil
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
