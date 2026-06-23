package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/pkg/httpjson"
	"github.com/TechXTT/bazaar-backend/services/jwt"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

// contextKey is an unexported type for request-context keys so they cannot
// collide with keys from other packages.
type contextKey string

// userIDKey is the context key under which the authenticated user's UUID is
// stored by AuthMiddleware (BE-17).
const userIDKey contextKey = "user_id"

// UserID returns the authenticated user's UUID from the request context, or ""
// if the request did not pass through AuthMiddleware.
func UserID(r *http.Request) string {
	if v, ok := r.Context().Value(userIDKey).(string); ok {
		return v
	}
	return ""
}

type (
	// Service is the middleware service interface
	Middleware interface {
		// AuthMiddleware is the middleware for authentication
		AuthMiddleware(next http.Handler) http.Handler
	}

	middleware struct {
		jwt jwt.Jwks
	}
)

func init() {
	// Provide dependencies during app boot
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewMiddleware)
	})
}

func NewMiddleware(i *do.Injector) (Middleware, error) {
	return &middleware{
		jwt: do.MustInvoke[jwt.Jwks](i),
	}, nil
}

func (m *middleware) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")

		if !strings.HasPrefix(token, "Bearer ") {
			writeUnauthorized(w)
			return
		}

		token = strings.TrimPrefix(token, "Bearer ")

		id, err := m.jwt.ValidateToken(token)
		if err != nil {
			writeUnauthorized(w)
			return
		}

		// BE-17: carry identity in the request context instead of mutating a
		// request header, which any later handler could overwrite or spoof.
		ctx := context.WithValue(r.Context(), userIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	httpjson.WriteError(w, http.StatusUnauthorized, "unauthorized")
}
