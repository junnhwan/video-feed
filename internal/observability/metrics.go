package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests grouped by method/path/status.",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency (seconds).",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	CacheHitTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hit_total",
			Help: "Cache hit counter by component and layer.",
		},
		[]string{"component", "layer"},
	)
	CacheMissTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_miss_total",
			Help: "Cache miss counter by component and layer.",
		},
		[]string{"component", "layer"},
	)

	MQPublishTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mq_publish_total",
			Help: "MQ publish counter by exchange/routing_key/result.",
		},
		[]string{"exchange", "routing_key", "result"},
	)
	MQConsumeTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mq_consume_total",
			Help: "MQ consume counter by queue/result.",
		},
		[]string{"queue", "result"},
	)

	RateLimitRejectTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limit_reject_total",
			Help: "Rate limit rejection counter by key prefix.",
		},
		[]string{"key_prefix"},
	)

	OutboxPendingGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "outbox_pending_messages",
			Help: "Outbox pending messages waiting to be published.",
		},
	)
)

func GinMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}
		status := strconv.Itoa(c.Writer.Status())
		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
