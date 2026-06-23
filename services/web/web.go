package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/gorilla/mux"
	"github.com/mikestefanello/hooks"
	"github.com/rs/cors"
	"github.com/samber/do"
)

type (
	Web interface {
		// Start serves until ctx is cancelled, then gracefully drains in-flight
		// requests via srv.Shutdown before returning (BE-3).
		Start(ctx context.Context) error
	}

	web struct {
		handler   *mux.Router
		cfg       config.Config
		db        db.DB
		defaultRL *rateLimiter
		authRL    *rateLimiter
	}
)

var HookBuildRouter = hooks.NewHook[*mux.Router]("router.build")

// authRateLimiter holds the strict per-IP limiter used for auth endpoints. It is
// initialised in NewWeb and exposed via AuthRateLimit so modules registering
// auth routes (e.g. users) can opt into tighter throttling than the global
// default.
var authRateLimiter *rateLimiter

// AuthRateLimit returns a middleware that applies the strict auth rate limit.
// Safe to call from router-build hooks; if the limiter is not yet initialised it
// returns a pass-through so route registration never panics.
func AuthRateLimit() func(http.Handler) http.Handler {
	if authRateLimiter == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return authRateLimiter.middleware("auth")
}

func init() {
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewWeb)
	})
}

func NewWeb(i *do.Injector) (Web, error) {
	w := &web{
		handler: mux.NewRouter().PathPrefix("/api").Subrouter(),
		cfg:     do.MustInvoke[config.Config](i),
		db:      do.MustInvoke[db.DB](i),
		// BE-6: default limiter applied to every /api route (generous), plus a
		// strict limiter for auth endpoints to blunt nonce/verify floods.
		defaultRL: newRateLimiter(20, 40),
		authRL:    newRateLimiter(1, 5),
	}
	authRateLimiter = w.authRL
	w.buildRouter()

	return w, nil
}

func (w *web) buildRouter() {
	// BE-15: structured request logging on every /api route, applied before the
	// rate limiter so even throttled requests are observed.
	w.handler.Use(requestLogger)
	// BE-15: record Prometheus request metrics (counter + latency histogram) for
	// every /api route, before rate limiting so throttled (429) responses are
	// still counted.
	w.handler.Use(requestMetrics)
	// BE-6: apply the default per-IP rate limiter to every /api route. Modules
	// that register auth routes additionally opt into AuthRateLimit().
	w.handler.Use(w.defaultRL.middleware("default"))

	uploadDir := w.cfg.GetS3Spaces().LocalUploadDir
	if uploadDir != "" {
		w.handler.PathPrefix("/uploads/").Handler(
			http.StripPrefix("/api/uploads/", http.FileServer(http.Dir(uploadDir))),
		).Methods(http.MethodGet)
	}

	// /health is a cheap liveness probe (process is up).
	w.handler.HandleFunc("/health", func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("OK"))
	}).Methods(http.MethodGet)

	// BE-15: /readyz is a readiness probe — it verifies the process can actually
	// serve traffic by pinging the database. A failing DB returns 503 so load
	// balancers stop routing to this instance.
	w.handler.HandleFunc("/readyz", w.readyz).Methods(http.MethodGet)

	// BE-15: Prometheus metrics. Unauthenticated by design (scrapers don't carry
	// app auth) — MUST be network-restricted in production (internal interface /
	// ingress allowlist) so traffic and error-rate data is not public.
	w.handler.Handle("/metrics", metricsHandler()).Methods(http.MethodGet)

	HookBuildRouter.Dispatch(w.handler)
}

func (w *web) readyz(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	sqlDB, err := w.db.DB().DB()
	if err == nil {
		err = sqlDB.PingContext(ctx)
	}
	if err != nil {
		slog.Error("readyz: database not ready", "error", err)
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusServiceUnavailable)
		rw.Write([]byte(`{"status":"unavailable","db":"down"}`))
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte(`{"status":"ready","db":"up"}`))
}

func (w *web) Start(ctx context.Context) error {
	httpCfg := w.cfg.GetHTTP()

	c := cors.New(cors.Options{
		AllowedOrigins:   nonEmptyList(w.cfg.GetHTTP().AllowedOrigins),
		AllowedMethods:   w.cfg.GetHTTP().AllowedMethods,
		AllowedHeaders:   w.cfg.GetHTTP().AllowedHeaders,
		AllowCredentials: w.cfg.GetHTTP().AllowCredentials,
		ExposedHeaders:   w.cfg.GetHTTP().ExposedHeaders,
	})

	handler := c.Handler(w.handler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", httpCfg.Port),
		Handler:      handler,
		ReadTimeout:  httpCfg.ReadTimeout,
		WriteTimeout: httpCfg.WriteTimeout,
		IdleTimeout:  httpCfg.IdleTimeout,
	}

	// Stop the rate-limiter sweepers once we return.
	defer w.defaultRL.Stop()
	defer w.authRL.Stop()

	serveErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		// BE-3: graceful shutdown — give in-flight requests a bounded window to
		// drain before forcing the listener closed.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-serveErr
	}
}

func nonEmptyList(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
