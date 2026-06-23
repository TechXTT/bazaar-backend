package users

import (
	"encoding/json"
	"net/http"

	"github.com/TechXTT/bazaar-backend/pkg/httpjson"
	"github.com/TechXTT/bazaar-backend/services/middleware"
	"github.com/samber/do"
)

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
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.RefreshToken == "" {
		httpjson.WriteError(w, http.StatusBadRequest, "refreshToken is required")
		return
	}

	token, newRefresh, err := u.svc.RefreshToken(body.RefreshToken)
	if err != nil {
		httpjson.WriteError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.RefreshToken == "" {
		httpjson.WriteError(w, http.StatusBadRequest, "refreshToken is required")
		return
	}

	if err := u.svc.Logout(body.RefreshToken); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, "logout failed")
		return
	}

	w.WriteHeader(http.StatusOK)
}
