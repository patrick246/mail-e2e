package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

const (
	namespace   = "maile2e"
	bucketStart = 1
	bucketWidth = 2
	bucketCount = 15
)

type Metrics struct {
	mailSent          *prometheus.CounterVec
	mailSentError     *prometheus.CounterVec
	mailReceived      *prometheus.CounterVec
	mailReceivedError *prometheus.CounterVec
	mailDelay         *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		mailSent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "smtp",
			Name:      "mail_sent_total",
			Help:      "Counter of mails that have been sent via SMTP",
		}, []string{"target"}),
		mailSentError: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "smtp",
			Name:      "mail_sent_error_total",
			Help:      "Counter of mails that have been sent via SMTP, but resulted in an error",
		}, []string{"target"}),
		mailReceived: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "imap",
			Name:      "mail_received_total",
			Help:      "Counter of mails received via IMAP",
		}, []string{"target"}),
		mailReceivedError: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "imap",
			Name:      "mail_received_error_total",
			Help:      "Counter of mails received via IMAP, but resulted in an error while fetching",
		}, []string{"target"}),
		mailDelay: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "",
			Name:      "delivery_delay",
			Buckets:   prometheus.LinearBuckets(bucketStart, bucketWidth, bucketCount),
			Help:      "Time between sending an email and receiving it again.",
		}, []string{"target"}),
	}
}

func (m *Metrics) InitTarget(target string) {
	m.mailSent.WithLabelValues(target).Add(0)
	m.mailSentError.WithLabelValues(target).Add(0)
	m.mailReceived.WithLabelValues(target).Add(0)
	m.mailReceivedError.WithLabelValues(target).Add(0)
	m.mailDelay.WithLabelValues(target)
}

func (m *Metrics) IncMailSent(target string) {
	m.mailSent.WithLabelValues(target).Inc()
}

func (m *Metrics) IncMailSentError(target string) {
	m.mailSentError.WithLabelValues(target).Inc()
}

func (m *Metrics) IncMailReceived(target string) {
	m.mailReceived.WithLabelValues(target).Inc()
}

func (m *Metrics) IncMailReceivedError(target string) {
	m.mailReceivedError.WithLabelValues(target).Inc()
}

func (m *Metrics) ObserveMailDelay(target string, delay time.Duration) {
	m.mailDelay.WithLabelValues(target).Observe(delay.Seconds())
}

func (m *Metrics) Registry() (*prometheus.Registry, error) {
	reg := prometheus.NewRegistry()

	for _, metric := range []prometheus.Collector{
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{
			ReportErrors: false,
		}),
		collectors.NewBuildInfoCollector(),
		m.mailSent,
		m.mailSentError,
		m.mailReceived,
		m.mailReceivedError,
		m.mailDelay,
	} {
		err := reg.Register(metric)
		if err != nil {
			return nil, err
		}
	}

	return reg, nil
}
