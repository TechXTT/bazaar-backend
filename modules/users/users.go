package users

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/jwt"
	"github.com/TechXTT/bazaar-backend/services/middleware"
	"github.com/TechXTT/bazaar-backend/services/refreshtoken"
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

		// VerifySIWE verifies a SIWE message+signature and returns a short-lived
		// access JWT, an opaque refresh token, and the user (BE-16).
		VerifySIWE(message string, signature string) (token string, refresh string, u *Users, err error)

		// UpdateUser updates display name fields for the user identified by UUID
		UpdateUser(userID string, u *Users) error

		// DeleteUser removes the user identified by UUID
		DeleteUser(userID string) error

		// GetMe returns the user for the given UUID (the JWT subject)
		GetMe(userID string) (*Users, error)

		// RefreshToken rotates an opaque refresh token (single-use): it validates
		// the supplied refresh token, invalidates it, and returns a fresh access
		// JWT plus a new refresh token (BE-16).
		RefreshToken(refreshToken string) (token string, newRefresh string, err error)

		// Logout denylists the supplied refresh token so it can no longer be
		// rotated (BE-16).
		Logout(refreshToken string) error
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

		// Refresh handles POST /api/auth/refresh
		Refresh(w http.ResponseWriter, r *http.Request)

		// Logout handles POST /api/auth/logout
		Logout(w http.ResponseWriter, r *http.Request)
	}

	usersService struct {
		db         db.DB
		jwks       jwt.Jwks
		refresh    refreshtoken.Service
		cfg        config.Config
		nonces     sync.Map     // walletAddress -> nonceEntry
		nonceCount atomic.Int64 // BE-6: bounds the nonce store
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

		// Public SIWE + token endpoints. BE-6: subject them to the strict auth
		// rate limiter so the unbounded nonce store and signature verification
		// cannot be flooded from a single source. BE-16: refresh and logout take
		// an opaque refresh token in the body (not the access JWT) so they live
		// here, not behind AuthMiddleware — the access token may legitimately be
		// expired when the client refreshes.
		authHandler := e.Msg.NewRoute().Subrouter()
		authHandler.Use(web.AuthRateLimit())
		authHandler.HandleFunc("/auth/nonce", h.Nonce).Methods(http.MethodPost)
		authHandler.HandleFunc("/auth/verify", h.Verify).Methods(http.MethodPost)
		authHandler.HandleFunc("/auth/refresh", h.Refresh).Methods(http.MethodPost)
		authHandler.HandleFunc("/auth/logout", h.Logout).Methods(http.MethodPost)
	})
}
