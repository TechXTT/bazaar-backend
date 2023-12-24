package web

import (
	"fmt"
	"net/http"

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
		handler *mux.Router
		cfg     config.Config
	}
)

var HookBuildRouter = hooks.NewHook[*mux.Router]("router.build")

func init() {
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewWeb)
	})
}

func NewWeb(i *do.Injector) (Web, error) {
	w := &web{
		handler: mux.NewRouter().PathPrefix("/api").Subrouter(),
		cfg:     do.MustInvoke[config.Config](i),
	}
	w.buildRouter()

	return w, nil
}

func (w *web) buildRouter() {

	w.handler.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods(http.MethodGet)

	HookBuildRouter.Dispatch(w.handler)
}

func (w *web) Start() error {
	httpCfg := w.cfg.GetHTTP()

	c := cors.New(cors.Options{
		AllowedOrigins:   w.cfg.GetHTTP().AllowedOrigins,
		AllowedMethods:   w.cfg.GetHTTP().AllowedMethods,
		AllowedHeaders:   w.cfg.GetHTTP().AllowedHeaders,
		AllowCredentials: w.cfg.GetHTTP().AllowCredentials,
		ExposedHeaders:   w.cfg.GetHTTP().ExposedHeaders,
	})

	handler := c.Handler(w.handler)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", httpCfg.Hostname, httpCfg.Port),
		Handler:      handler,
		ReadTimeout:  httpCfg.ReadTimeout,
		WriteTimeout: httpCfg.WriteTimeout,
		IdleTimeout:  httpCfg.IdleTimeout,
	}

	return srv.ListenAndServe()
}
