package users

import (
	"encoding/json"
	"net/http"

	"github.com/samber/do"
)

// NewUsersHandler creates a new users handler
func NewUsersHandler(i *do.Injector) (Handler, error) {
	return &usersHandler{
		svc: do.MustInvoke[Service](i),
	}, nil
}

func (u *usersHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := &Users{}
	if err := json.NewDecoder(r.Body).Decode(user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := u.svc.CreateUser(user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
}

func (u *usersHandler) Update(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "missing Authorization header", http.StatusBadRequest)
		return
	}

	token = token[7:]

	user := &Users{}
	if err := json.NewDecoder(r.Body).Decode(user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := u.svc.UpdateUser(token, user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (u *usersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "missing Authorization header", http.StatusBadRequest)
		return
	}

	token = token[7:]

	if err := u.svc.DeleteUser(token); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (u *usersHandler) Me(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "missing Authorization header", http.StatusBadRequest)
		return
	}

	token = token[7:]

	user, err := u.svc.GetMe(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (u *usersHandler) Login(w http.ResponseWriter, r *http.Request) {
	creds := &Credentials{}
	if err := json.NewDecoder(r.Body).Decode(creds); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	token, err := u.svc.LoginUser(creds.Email, creds.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}
