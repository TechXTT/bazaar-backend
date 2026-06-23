package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestRequestMetricsRecordsCounter covers BE-15: the request-metrics middleware
// increments the request counter labelled by the matched route template, method
// and status, and the response passes through unchanged.
func TestRequestMetricsRecordsCounter(t *testing.T) {
	r := mux.NewRouter()
	r.Use(requestMetrics)
	r.HandleFunc("/widgets/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("ok"))
	}).Methods(http.MethodGet)

	before := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("/widgets/{id}", http.MethodGet, "201"))

	req := httptest.NewRequest(http.MethodGet, "/widgets/abc123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusCreated)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body: got %q", rec.Body.String())
	}

	after := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("/widgets/{id}", http.MethodGet, "201"))
	if after-before != 1 {
		t.Fatalf("counter delta: got %v want 1 (route template should be the label, not the raw path)", after-before)
	}
}

// TestRouteLabelUnmatched verifies that a request with no matched mux route
// gets the low-cardinality "unmatched" label rather than the raw path.
func TestRouteLabelUnmatched(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/no/such/route", nil)
	if got := routeLabel(req); got != "unmatched" {
		t.Fatalf("routeLabel: got %q want %q", got, "unmatched")
	}
}

// TestMetricsHandlerExposesCollectors covers BE-15: the /metrics endpoint
// serves the Prometheus exposition format including our custom collectors.
func TestMetricsHandlerExposesCollectors(t *testing.T) {
	// Touch a collector so it appears in the output.
	httpRequestsTotal.WithLabelValues("/metrics", http.MethodGet, "200").Inc()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	metricsHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "bazaar_http_requests_total") {
		t.Fatalf("metrics output missing bazaar_http_requests_total:\n%s", string(body))
	}
}
