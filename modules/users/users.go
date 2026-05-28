package users

import (
	"net/http"
	"sync"
	"time"

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

type nonceEntry struct {
	nonce     string
	expiresAt time.Time
}

type (
	// Service is the users service interface
	Service interface {
		// GetNonce returns a one-time nonce for the given wallet address
		GetNonce(walletAddress string) (string, error)

		// VerifySIWE verifies a SIWE message+signature and returns a JWT
		VerifySIWE(message string, signature string) (string, *Users, error)

		// UpdateUser updates display name fields for the authenticated wallet
		UpdateUser(walletAddress string, u *Users) error

		// DeleteUser removes the user identified by walletAddress
		DeleteUser(walletAddress string) error

		// GetMe returns the user for the given wallet address (from JWT subject)
		GetMe(walletAddress string) (*Users, error)

		// RefreshToken issues a fresh JWT for an already-authenticated wallet
		RefreshToken(walletAddress string) (string, error)
	}

	// Handler provides the users HTTP handlers
	Handler interface {
		// Nonce handles POST /api/auth/nonce
		Nonce(w http.ResponseWriter, r *http.Request)

		// Verify handles POST /api/auth/verify
		Verify(w http.ResponseWriter, r *http.Request)

		// Update handles PUT /api/users
		Update(w http.ResponseWriter, r *http.Request)

		// Delete handles DELETE /api/users
		Delete(w http.ResponseWriter, r *http.Request)

		// Me handles GET /api/users/me
		Me(w http.ResponseWriter, r *http.Request)

		// Refresh handles POST /api/users/refresh
		Refresh(w http.ResponseWriter, r *http.Request)
	}

	usersService struct {
		db     db.DB
		jwks   jwt.Jwks
		cfg    config.Config
		nonces sync.Map // walletAddress -> nonceEntry
	}

	usersHandler struct {
		svc Service
	}
)

func init() {
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewUsersService)
		do.Provide(e.Msg, NewUsersHandler)
	})

	web.HookBuildRouter.Listen(func(e hooks.Event[*mux.Router]) {
		h := do.MustInvoke[Handler](do.DefaultInjector)
		mw := do.MustInvoke[middleware.Middleware](do.DefaultInjector)

		authenticatedHandler := e.Msg.NewRoute().Subrouter()
		authenticatedHandler.Use(mw.AuthMiddleware)

		authenticatedHandler.HandleFunc("/users", h.Update).Methods(http.MethodPut)
		authenticatedHandler.HandleFunc("/users", h.Delete).Methods(http.MethodDelete)
		authenticatedHandler.HandleFunc("/users/me", h.Me).Methods(http.MethodGet)
		authenticatedHandler.HandleFunc("/users/refresh", h.Refresh).Methods(http.MethodPost)

		// Public SIWE endpoints
		e.Msg.HandleFunc("/auth/nonce", h.Nonce).Methods(http.MethodPost)
		e.Msg.HandleFunc("/auth/verify", h.Verify).Methods(http.MethodPost)
	})
}
