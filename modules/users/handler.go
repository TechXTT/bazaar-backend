package users

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/TechXTT/bazaar-backend/pkg/httpjson"
	"github.com/TechXTT/bazaar-backend/services/middleware"
	"github.com/samber/do"
)

// refreshCookieName carries the opaque, rotating refresh token (BE-16). It is
// httpOnly so client JS can never read it (FE-4: the access JWT is never
// persisted either), and scoped to the auth path so it's only sent to the
// refresh/logout endpoints.
const refreshCookieName = "refresh_token"

// cookieSecure marks the refresh cookie Secure in production. Disabled for local
// http dev so the cookie is still delivered over http://localhost. Enable with
// COOKIE_SECURE=true or APP_ENV=production.
func cookieSecure() bool {
	if v, err := strconv.ParseBool(os.Getenv("COOKIE_SECURE")); err == nil && v {
		return true
	}
	return strings.EqualFold(os.Getenv("APP_ENV"), "production")
}

func setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   cookieSecure(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60, // 7d; the server-side store enforces real expiry
	})
}

func clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   cookieSecure(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// refreshTokenFromRequest prefers the httpOnly cookie (the secure default the
// frontend uses) and falls back to a JSON body field for non-browser clients
// and the existing tests.
func refreshTokenFromRequest(r *http.Request, bodyToken string) string {
	if c, err := r.Cookie(refreshCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	return bodyToken
}

// NewUsersHandler creates a new users handler
func NewUsersHandler(i *do.Injector) (Handler, error) {
	return &usersHandler{
		svc: do.MustInvoke[Service](i),
	}, nil
}

func (u *usersHandler) Nonce(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WalletAddress string `json:"walletAddress"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.WalletAddress == "" {
		httpjson.WriteError(w, http.StatusBadRequest, "walletAddress is required")
		return
	}

	nonce, err := u.svc.GetNonce(body.WalletAddress)
	if err != nil {
		httpjson.WriteAppError(w, http.StatusInternalServerError, err)
		return
	}

	httpjson.WriteJSON(w, http.StatusOK, map[string]string{"nonce": nonce})
}

func (u *usersHandler) Verify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Message   string `json:"message"`
		Signature string `json:"signature"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Message == "" || body.Signature == "" {
		httpjson.WriteError(w, http.StatusBadRequest, "message and signature are required")
		return
	}

	token, refresh, user, err := u.svc.VerifySIWE(body.Message, body.Signature)
	if err != nil {
		httpjson.WriteAppError(w, http.StatusUnauthorized, err)
		return
	}

	// Deliver the refresh token as an httpOnly cookie (the secure default the
	// frontend relies on); also return it in the body for non-browser clients.
	setRefreshCookie(w, refresh)
	httpjson.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"token":        token,
		"refreshToken": refresh,
		"user":         user,
	})
}

func (u *usersHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)

	var body Users
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpjson.WriteAppError(w, http.StatusBadRequest, err)
		return
	}

	if err := u.svc.UpdateUser(userID, &body); err != nil {
		httpjson.WriteAppError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (u *usersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)

	if err := u.svc.DeleteUser(userID); err != nil {
		httpjson.WriteAppError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (u *usersHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)

	user, err := u.svc.GetMe(userID)
	if err != nil {
		httpjson.WriteAppError(w, http.StatusUnauthorized, err)
		return
	}

	httpjson.WriteJSON(w, http.StatusOK, user)
}

// Refresh rotates an opaque refresh token (BE-16). The client posts its current
// refresh token; on success it receives a new access JWT and a new refresh
// token, and the old refresh token is invalidated.
func (u *usersHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	// The refresh token comes from the httpOnly cookie for browsers; a JSON body
	// is accepted as a fallback for non-browser clients/tests. The body is
	// optional, so a decode error (e.g. empty body) is not fatal.
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	refreshToken := refreshTokenFromRequest(r, body.RefreshToken)
	if refreshToken == "" {
		httpjson.WriteError(w, http.StatusBadRequest, "refreshToken is required")
		return
	}

	token, newRefresh, err := u.svc.RefreshToken(refreshToken)
	if err != nil {
		clearRefreshCookie(w)
		httpjson.WriteError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	setRefreshCookie(w, newRefresh)
	httpjson.WriteJSON(w, http.StatusOK, map[string]string{
		"token":        token,
		"refreshToken": newRefresh,
	})
}

// Logout denylists the supplied refresh token (BE-16). Idempotent: a missing or
// already-invalid token still returns 200 so logout never leaks token validity.
func (u *usersHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	refreshToken := refreshTokenFromRequest(r, body.RefreshToken)

	// Always clear the cookie and return 200 — logout is idempotent and must not
	// leak token validity. Denylist the token if we have one.
	clearRefreshCookie(w)
	if refreshToken != "" {
		if err := u.svc.Logout(refreshToken); err != nil {
			httpjson.WriteError(w, http.StatusInternalServerError, "logout failed")
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}
