package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRequestLogger covers BE-15: the access-log middleware captures the
// downstream status and passes the response through unchanged.
func TestRequestLogger(t *testing.T) {
	handler := requestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusTeapot)
	}
	if rec.Body.String() != "hello" {
		t.Fatalf("body: got %q", rec.Body.String())
	}
}

// TestStatusRecorderDefaultsToOK verifies that a handler that writes a body
// without an explicit WriteHeader is recorded as 200.
func TestStatusRecorderDefaultsToOK(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec}
	if _, err := sr.Write([]byte("ok")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if sr.status != http.StatusOK {
		t.Fatalf("status: got %d want 200", sr.status)
	}
	if sr.bytes != 2 {
		t.Fatalf("bytes: got %d want 2", sr.bytes)
	}
}
