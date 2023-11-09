package middleware

import (
	"net/http"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/jwt"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

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

		if token == "" {
			http.Redirect(w, r, "/login", http.StatusUnauthorized)
			return
		}

		token = token[7:]

		id, err := m.jwt.ValidateToken(token)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusUnauthorized)
			return
		}

		r.Header.Set("user_id", id)

		next.ServeHTTP(w, r)
	})
}
