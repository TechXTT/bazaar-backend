package users

import (
	"net/http"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/jwt"
	"github.com/TechXTT/bazaar-backend/services/middleware"
	"github.com/TechXTT/bazaar-backend/services/web"
	"github.com/gorilla/mux"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

type (
	Credentials struct {
		Email    string
		Password string
	}

	// Service is the users service interface
	Service interface {
		// CreateUser creates a new user
		CreateUser(u *Users) error

		// UpdateUser updates a user
		UpdateUser(id string, u *Users) error

		// DeleteUser deletes a user
		DeleteUser(id string) error

		// GetMe returns the current user using JWKS token
		GetMe(id string) (*Users, error)

		// LoginUser logs in a user
		LoginUser(email string, password string) (string, error)

		// VerifyUser verifies a user
		VerifyUser(token string) error
	}

	// Handler provides the users handlers
	Handler interface {

		// Create handles a request to create a new user
		Create(w http.ResponseWriter, r *http.Request)

		// Update handles a request to update a user
		Update(w http.ResponseWriter, r *http.Request)

		// Delete handles a request to delete a user
		Delete(w http.ResponseWriter, r *http.Request)

		// Me handles a request to get the current user
		Me(w http.ResponseWriter, r *http.Request)

		// Login handles a request to login a user
		Login(w http.ResponseWriter, r *http.Request)

		// Verify handles a request to verify a user
		Verify(w http.ResponseWriter, r *http.Request)
	}

	usersService struct {
		db   db.DB
		jwks jwt.Jwks
		cfg  config.Config
	}

	usersHandler struct {
		svc Service
	}
)

func init() {

	// Provide dependencies during app boot
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewUsersService)
		do.Provide(e.Msg, NewUsersHandler)
	})

	// Register routes during router build
	web.HookBuildRouter.Listen(func(e hooks.Event[*mux.Router]) {
		h := do.MustInvoke[Handler](do.DefaultInjector)

		middleware := do.MustInvoke[middleware.Middleware](do.DefaultInjector)
		authenticatedHandler := e.Msg.NewRoute().Subrouter()
		authenticatedHandler.Use(middleware.AuthMiddleware)

		authenticatedHandler.HandleFunc("/users", h.Update).Methods(http.MethodPut)
		authenticatedHandler.HandleFunc("/users", h.Delete).Methods(http.MethodDelete)
		authenticatedHandler.HandleFunc("/users/me", h.Me).Methods(http.MethodGet)

		e.Msg.HandleFunc("/users", h.Create).Methods(http.MethodPost)
		e.Msg.HandleFunc("/users/login", h.Login).Methods(http.MethodPost)
		e.Msg.HandleFunc("/users/verify-email", h.Verify).Methods(http.MethodGet)
	})
}
