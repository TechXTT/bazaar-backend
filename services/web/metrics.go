package web

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// BE-15: Prometheus instrumentation. The metrics defined here are registered on
// the default registry at package init so they are exposed by the standard
// promhttp handler at /metrics. The /metrics endpoint is intentionally
// unauthenticated (Prometheus scrapers rarely carry app credentials); in
// production it MUST be network-restricted (bind to an internal interface, or
// gate it behind the ingress / a metrics-only allowlist) so request-pattern and
// error-rate data is not exposed publicly.
var (
	// httpRequestsTotal counts HTTP requests partitioned by route, method and
	// status code, so error rates and traffic per endpoint can be alerted on.
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bazaar_http_requests_total",
			Help: "Total number of HTTP requests, by route, method and status code.",
		},
		[]string{"route", "method", "status"},
	)

	// httpRequestDuration observes request latency (seconds) by route+method so
	// p50/p95/p99 can be computed via histogram_quantile.
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "bazaar_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds, by route and method.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"route", "method"},
	)
)

// metricsHandler returns the standard Prometheus exposition handler.
func metricsHandler() http.Handler {
	return promhttp.Handler()
}

// requestMetrics is mux middleware that records a counter and latency histogram
// for every request. The route is taken from the matched mux route template
// (not the raw path) so high-cardinality path params do not explode the metric
// label space (BE-15).
func requestMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		route := routeLabel(r)
		httpRequestsTotal.WithLabelValues(route, r.Method, strconv.Itoa(rec.status)).Inc()
		httpRequestDuration.WithLabelValues(route, r.Method).Observe(time.Since(start).Seconds())
	})
}

// routeLabel returns a low-cardinality label for the request: the matched mux
// route template when available, otherwise "unmatched".
func routeLabel(r *http.Request) string {
	if route := mux.CurrentRoute(r); route != nil {
		if tmpl, err := route.GetPathTemplate(); err == nil {
			return tmpl
		}
	}
	return "unmatched"
}
