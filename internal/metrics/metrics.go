package metrics

import "github.com/prometheus/client_golang/prometheus"

const namespace = "maile2e"

var MailSent = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: namespace,
	Subsystem: "smtp",
	Name:      "mail_sent_total",
}, []string{"target"})

var MailSentError = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: namespace,
	Subsystem: "smtp",
	Name:      "mail_sent_error_total",
}, []string{"target"})

var MailReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: namespace,
	Subsystem: "imap",
	Name:      "mail_received_total",
}, []string{"target"})

var MailReceivedError = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: namespace,
	Subsystem: "imap",
	Name:      "mail_received_error_total",
}, []string{"target"})

var MailDelay = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: namespace,
	Subsystem: "",
	Name:      "delivery_delay",
	Buckets:   prometheus.LinearBuckets(1, 2, 15),
}, []string{"target"})

func init() {
	prometheus.MustRegister(
		MailSent,
		MailSentError,
		MailReceived,
		MailReceivedError,
		MailDelay,
	)
}
