package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimiterAllowsThenBlocks(t *testing.T) {
	rl := newRateLimiter(0, 2) // 0/s refill, burst of 2
	defer rl.Stop()

	mw := rl.middleware("test")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(next)

	call := func() int {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "1.2.3.4:5555"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	if got := call(); got != http.StatusOK {
		t.Fatalf("first call: got %d want 200", got)
	}
	if got := call(); got != http.StatusOK {
		t.Fatalf("second call: got %d want 200", got)
	}
	if got := call(); got != http.StatusTooManyRequests {
		t.Fatalf("third call: got %d want 429", got)
	}
}

func TestRateLimiterIsolatesByIP(t *testing.T) {
	rl := newRateLimiter(0, 1)
	defer rl.Stop()

	mw := rl.middleware("test")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	handler := mw(next)

	call := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = ip + ":5555"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	if got := call("1.1.1.1"); got != http.StatusOK {
		t.Fatalf("ip1 first: got %d want 200", got)
	}
	// Different IP must have its own bucket.
	if got := call("2.2.2.2"); got != http.StatusOK {
		t.Fatalf("ip2 first: got %d want 200", got)
	}
	// ip1 second call exhausts its burst.
	if got := call("1.1.1.1"); got != http.StatusTooManyRequests {
		t.Fatalf("ip1 second: got %d want 429", got)
	}
}

func TestClientIPHonoursXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "10.0.0.1:5555"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	if got := clientIP(req); got != "203.0.113.7" {
		t.Fatalf("clientIP: got %q want 203.0.113.7", got)
	}
}

func TestRateLimiterSweepEvictsIdle(t *testing.T) {
	rl := newRateLimiter(1, 1)
	defer rl.Stop()

	rl.allow("default|9.9.9.9")
	if len(rl.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(rl.entries))
	}
	// Force the entry to look idle and sweep.
	rl.entries["default|9.9.9.9"].lastSeen = rl.entries["default|9.9.9.9"].lastSeen.Add(-2 * rateLimiterTTL)
	rl.sweep()
	if len(rl.entries) != 0 {
		t.Fatalf("expected 0 entries after sweep, got %d", len(rl.entries))
	}
}
