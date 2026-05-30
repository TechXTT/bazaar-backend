package disputes

import (
	"net/http"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/middleware"
	"github.com/TechXTT/bazaar-backend/services/web"
	"github.com/gorilla/mux"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

type (
	// Service exposes read-only views over on-chain dispute data indexed by the observer.
	Service interface {
		// ListDisputes returns all disputes where the caller (identified by user UUID)
		// is the order's buyer or owns the store the ordered product belongs to.
		ListDisputes(userID string) ([]Disputes, error)

		// GetDispute returns a single dispute with evidence for the given orderId.
		GetDispute(walletAddress string, orderId string) (*Disputes, error)

		// GetEvidence returns evidence items for the given orderId.
		GetEvidence(orderId string) ([]DisputeEvidence, error)
	}

	// Handler provides HTTP handlers for dispute read endpoints.
	Handler interface {
		GetDisputes(w http.ResponseWriter, r *http.Request)
		GetDispute(w http.ResponseWriter, r *http.Request)
		GetEvidence(w http.ResponseWriter, r *http.Request)
	}

	disputesService struct {
		db db.DB
	}

	disputesHandler struct {
		svc Service
	}
)

func init() {
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewDisputesService)
		do.Provide(e.Msg, NewDisputesHandler)
	})

	web.HookBuildRouter.Listen(func(e hooks.Event[*mux.Router]) {
		h := do.MustInvoke[Handler](do.DefaultInjector)
		mw := do.MustInvoke[middleware.Middleware](do.DefaultInjector)

		authenticatedHandler := e.Msg.NewRoute().Subrouter()
		authenticatedHandler.Use(mw.AuthMiddleware)

		authenticatedHandler.HandleFunc("/disputes", h.GetDisputes).Methods(http.MethodGet)
		authenticatedHandler.HandleFunc("/disputes/{orderId}", h.GetDispute).Methods(http.MethodGet)
		authenticatedHandler.HandleFunc("/orders/{orderId}/evidence", h.GetEvidence).Methods(http.MethodGet)
	})
}
