package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/gorilla/mux"
	"github.com/mikestefanello/hooks"
	"github.com/rs/cors"
	"github.com/samber/do"
)

type (
	Web interface {
		Start() error
	}

	web struct {
		handler   *mux.Router
		cfg       config.Config
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
	// BE-6: apply the default per-IP rate limiter to every /api route. Modules
	// that register auth routes additionally opt into AuthRateLimit().
	w.handler.Use(w.defaultRL.middleware("default"))

	uploadDir := w.cfg.GetS3Spaces().LocalUploadDir
	if uploadDir != "" {
		w.handler.PathPrefix("/uploads/").Handler(
			http.StripPrefix("/api/uploads/", http.FileServer(http.Dir(uploadDir))),
		).Methods(http.MethodGet)
	}

	w.handler.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods(http.MethodGet)

	HookBuildRouter.Dispatch(w.handler)
}

func (w *web) Start() error {
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

	return srv.ListenAndServe()
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
