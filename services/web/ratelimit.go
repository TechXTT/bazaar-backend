package web

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/TechXTT/bazaar-backend/pkg/httpjson"
	"golang.org/x/time/rate"
)

// BE-6: IP+route rate limiting.
//
// We key a token-bucket limiter on (clientIP, routeBucket) so that a flood of
// requests from a single source on a single class of route is throttled without
// starving other clients/routes. Entries are swept on a ticker so the in-memory
// map cannot grow unbounded (the same DoS surface BE-6 calls out for the nonce
// store).

const (
	// rateLimiterTTL is how long an idle limiter is retained before the sweeper
	// evicts it.
	rateLimiterTTL = 10 * time.Minute
	// rateLimiterSweepInterval is how often the sweeper runs.
	rateLimiterSweepInterval = 5 * time.Minute
)

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// rateLimiter is a per-(ip,bucket) token-bucket limiter store with a background
// sweeper.
type rateLimiter struct {
	mu       sync.Mutex
	entries  map[string]*limiterEntry
	r        rate.Limit
	burst    int
	stopOnce sync.Once
	stop     chan struct{}
}

func newRateLimiter(perSecond float64, burst int) *rateLimiter {
	rl := &rateLimiter{
		entries: make(map[string]*limiterEntry),
		r:       rate.Limit(perSecond),
		burst:   burst,
		stop:    make(chan struct{}),
	}
	go rl.sweepLoop()
	return rl
}

func (rl *rateLimiter) sweepLoop() {
	ticker := time.NewTicker(rateLimiterSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case <-ticker.C:
			rl.sweep()
		}
	}
}

func (rl *rateLimiter) sweep() {
	cutoff := time.Now().Add(-rateLimiterTTL)
	rl.mu.Lock()
	for k, e := range rl.entries {
		if e.lastSeen.Before(cutoff) {
			delete(rl.entries, k)
		}
	}
	rl.mu.Unlock()
}

// Stop terminates the background sweeper.
func (rl *rateLimiter) Stop() {
	rl.stopOnce.Do(func() { close(rl.stop) })
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	e, ok := rl.entries[key]
	if !ok {
		e = &limiterEntry{limiter: rate.NewLimiter(rl.r, rl.burst)}
		rl.entries[key] = e
	}
	e.lastSeen = time.Now()
	limiter := e.limiter
	rl.mu.Unlock()
	return limiter.Allow()
}

// clientIP extracts a best-effort client IP, honouring a single hop of
// X-Forwarded-For (the platform proxy) before falling back to RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Left-most entry is the original client.
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// middleware returns an http middleware that throttles requests keyed on
// clientIP + bucket. bucket lets callers separate limiter pools (e.g. a strict
// "auth" pool vs a looser "default" pool) so one route class cannot exhaust
// another's budget.
func (rl *rateLimiter) middleware(bucket string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := bucket + "|" + clientIP(r)
			if !rl.allow(key) {
				w.Header().Set("Retry-After", "1")
				httpjson.WriteError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
