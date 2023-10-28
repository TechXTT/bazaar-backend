package web

import (
	"fmt"
	"net/http"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/gorilla/mux"
	"github.com/mikestefanello/hooks"
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
		handler: mux.NewRouter(),
		cfg:     do.MustInvoke[config.Config](i),
	}
	w.buildRouter()

	return w, nil
}

func (w *web) buildRouter() {
	w.handler.Use(mux.CORSMethodMiddleware(w.handler))

	w.handler.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods(http.MethodGet)

	HookBuildRouter.Dispatch(w.handler)
}

func (w *web) Start() error {
	httpCfg := w.cfg.GetHTTP()

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", httpCfg.Hostname, httpCfg.Port),
		Handler:      w.handler,
		ReadTimeout:  httpCfg.ReadTimeout,
		WriteTimeout: httpCfg.WriteTimeout,
		IdleTimeout:  httpCfg.IdleTimeout,
	}

	return srv.ListenAndServe()
}
