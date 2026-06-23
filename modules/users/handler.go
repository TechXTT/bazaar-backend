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
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
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

	token, user, err := u.svc.VerifySIWE(body.Message, body.Signature)
	if err != nil {
		httpjson.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	httpjson.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

func (u *usersHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)

	var body Users
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := u.svc.UpdateUser(userID, &body); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (u *usersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)

	if err := u.svc.DeleteUser(userID); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (u *usersHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)

	user, err := u.svc.GetMe(userID)
	if err != nil {
		httpjson.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	httpjson.WriteJSON(w, http.StatusOK, user)
}

func (u *usersHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)

	token, err := u.svc.RefreshToken(userID)
	if err != nil {
		httpjson.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	httpjson.WriteJSON(w, http.StatusOK, map[string]string{"token": token})
}
