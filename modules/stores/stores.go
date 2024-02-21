package stores

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
	// Service is the stores service interface
	Service interface {
		// GetStores returns all stores
		GetStores() ([]Stores, error)

		// GetStore returns a store by id
		GetStore(id string) (*Stores, error)

		// CreateStore creates a new store
		CreateStore(userId string, s *Stores) error

		// UpdateStore updates a store
		UpdateStore(userId string, id string, s *Stores) error

		// DeleteStore deletes a store
		DeleteStore(userId string, id string) error

		// GetUserStores returns all stores for a user
		GetUserStores(userId string) ([]Stores, error)

		// TODO: Add methods for products and categories
	}

	// Handler provides the stores handlers
	Handler interface {

		// Gets handles a request to get all stores
		Gets(w http.ResponseWriter, r *http.Request)

		// Get handles a request to get a store
		Get(w http.ResponseWriter, r *http.Request)

		// Create handles a request to create a new store
		Create(w http.ResponseWriter, r *http.Request)

		// Update handles a request to update a store
		Update(w http.ResponseWriter, r *http.Request)

		// Delete handles a request to delete a store
		Delete(w http.ResponseWriter, r *http.Request)

		// GetUser handles a request to get a user's stores
		GetUser(w http.ResponseWriter, r *http.Request)
	}

	storesService struct {
		db db.DB
	}

	storesHandler struct {
		svc Service
	}
)

func init() {

	// Provide dependencies during app boot
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewStoresService)
		do.Provide(e.Msg, NewStoresHandler)
	})

	web.HookBuildRouter.Listen(func(e hooks.Event[*mux.Router]) {
		h := do.MustInvoke[Handler](do.DefaultInjector)

		middleware := do.MustInvoke[middleware.Middleware](do.DefaultInjector)
		authenticatedHandler := e.Msg.NewRoute().Subrouter()
		authenticatedHandler.Use(middleware.AuthMiddleware)

		authenticatedHandler.HandleFunc("/stores/user", h.GetUser).Methods("GET")

		e.Msg.HandleFunc("/stores/{id}", h.Get).Methods("GET")
		e.Msg.HandleFunc("/stores", h.Gets).Methods("GET")

		authenticatedHandler.HandleFunc("/stores", h.Create).Methods("POST")
		authenticatedHandler.HandleFunc("/stores/{id}", h.Update).Methods("PUT")
		authenticatedHandler.HandleFunc("/stores/{id}", h.Delete).Methods("DELETE")
	})
}
