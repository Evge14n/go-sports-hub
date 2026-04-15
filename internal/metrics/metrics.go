package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_total", Help: "Total HTTP requests"},
		[]string{"method", "path", "status"},
	)
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP request duration",
			Buckets: prometheus.DefBuckets},
		[]string{"method", "path"},
	)
	ActiveWSConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "ws_active_connections", Help: "Active WebSocket connections"},
	)
	EventsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "events_processed_total", Help: "Events processed"},
		[]string{"sport", "status"},
	)
	CacheHits = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "cache_hits_total", Help: "Cache hits"},
	)
	CacheMisses = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "cache_misses_total", Help: "Cache misses"},
	)
)

func init() {
	prometheus.MustRegister(HTTPRequestsTotal, HTTPRequestDuration, ActiveWSConnections, EventsProcessed, CacheHits, CacheMisses)
}
