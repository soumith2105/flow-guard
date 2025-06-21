package metrics

import (
	"net/http"
	"time"

	"flowguard/internal/limiter"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for FlowGuard
type Metrics struct {
	requestsTotal     *prometheus.CounterVec
	requestsDropped   *prometheus.CounterVec
	tokensUsed        *prometheus.CounterVec
	tokensRemaining   *prometheus.GaugeVec
	requestDuration   *prometheus.HistogramVec
	bucketsRemaining  *prometheus.GaugeVec
	rateLimiter       *limiter.Manager
}

// NewMetrics creates and registers Prometheus metrics
func NewMetrics(rateLimiter *limiter.Manager) *Metrics {
	m := &Metrics{
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "flowguard_requests_total",
				Help: "Total number of requests processed by FlowGuard",
			},
			[]string{"client_id", "status"},
		),
		requestsDropped: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "flowguard_requests_dropped_total",
				Help: "Total number of requests dropped due to rate limiting",
			},
			[]string{"client_id", "reason"},
		),
		tokensUsed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "flowguard_tokens_used_total",
				Help: "Total number of tokens consumed by clients",
			},
			[]string{"client_id"},
		),
		tokensRemaining: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "flowguard_tokens_remaining",
				Help: "Current number of tokens remaining in client buckets",
			},
			[]string{"client_id", "bucket_type"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "flowguard_request_duration_milliseconds",
				Help:    "Request latency in milliseconds",
				Buckets: prometheus.ExponentialBuckets(1, 2, 10), // 1ms to ~1s
			},
			[]string{"client_id"},
		),
		bucketsRemaining: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "flowguard_rate_limit_remaining",
				Help: "Current rate limit remaining for each client and type",
			},
			[]string{"client_id", "limit_type"},
		),
		rateLimiter: rateLimiter,
	}

	// Register metrics with Prometheus
	prometheus.MustRegister(
		m.requestsTotal,
		m.requestsDropped,
		m.tokensUsed,
		m.tokensRemaining,
		m.requestDuration,
		m.bucketsRemaining,
	)

	return m
}

// UpdateMetrics updates Prometheus metrics with current data from rate limiter
func (m *Metrics) UpdateMetrics() {
	stats := m.rateLimiter.GetAllStats()
	configs := m.rateLimiter.GetAllClients()

	for clientID, stat := range stats {
		// Update request metrics
		m.requestsTotal.WithLabelValues(clientID, "success").Add(float64(stat.SuccessRequests))
		m.requestsTotal.WithLabelValues(clientID, "total").Add(float64(stat.TotalRequests))

		// Update dropped request metrics
		m.requestsDropped.WithLabelValues(clientID, "rpm").Add(float64(stat.RPMDropped))
		m.requestsDropped.WithLabelValues(clientID, "tpm").Add(float64(stat.TPMDropped))

		// Update token metrics
		m.tokensUsed.WithLabelValues(clientID).Add(float64(stat.TokensUsed))

		// Update remaining token gauges
		m.tokensRemaining.WithLabelValues(clientID, "rpm").Set(float64(stat.RPMRemaining))
		m.tokensRemaining.WithLabelValues(clientID, "tpm").Set(float64(stat.TPMRemaining))

		// Update latency histogram
		if stat.AvgLatencyMs > 0 {
			m.requestDuration.WithLabelValues(clientID).Observe(stat.AvgLatencyMs)
		}

		// Update rate limit remaining gauges
		if config, exists := configs[clientID]; exists {
			if config.RPM != nil {
				m.bucketsRemaining.WithLabelValues(clientID, "rpm").Set(float64(stat.RPMRemaining))
			}
			if config.TPM != nil {
				m.bucketsRemaining.WithLabelValues(clientID, "tpm").Set(float64(stat.TPMRemaining))
			}
		}
	}
}

// RecordRequest records a successful request
func (m *Metrics) RecordRequest(clientID string, duration time.Duration) {
	m.requestsTotal.WithLabelValues(clientID, "success").Inc()
	m.requestDuration.WithLabelValues(clientID).Observe(float64(duration.Milliseconds()))
}

// RecordDroppedRequest records a dropped request
func (m *Metrics) RecordDroppedRequest(clientID, reason string) {
	m.requestsDropped.WithLabelValues(clientID, reason).Inc()
}

// RecordTokensUsed records token consumption
func (m *Metrics) RecordTokensUsed(clientID string, tokens int64) {
	m.tokensUsed.WithLabelValues(clientID).Add(float64(tokens))
}

// StartMetricsUpdater starts a goroutine that periodically updates metrics
func (m *Metrics) StartMetricsUpdater(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			m.UpdateMetrics()
		}
	}()
}

// Handler returns the Prometheus metrics HTTP handler
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
} 