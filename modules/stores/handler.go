package stores

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/TechXTT/bazaar-backend/pkg/httpjson"
	"github.com/TechXTT/bazaar-backend/services/middleware"
	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/mux"
	"github.com/samber/do"
)

// statusForError maps service sentinel errors to HTTP status codes.
func statusForError(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrUnauthorized):
		return http.StatusForbidden
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidInput):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// NewStoresHandler creates a new users handler
func NewStoresHandler(i *do.Injector) (Handler, error) {
	return &storesHandler{
		svc: do.MustInvoke[Service](i),
	}, nil
}

func (s *storesHandler) Gets(w http.ResponseWriter, r *http.Request) {
	stores, err := s.svc.GetStores()
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(stores); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func (s *storesHandler) Get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	storeId := vars["id"]
	if _, err := uuid.FromString(storeId); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "invalid store id")
		return
	}

	store, err := s.svc.GetStore(storeId)
	if err != nil {
		httpjson.WriteError(w, statusForError(err), err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(store)
}

func (s *storesHandler) GetReputation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	storeId := vars["id"]
	if _, err := uuid.FromString(storeId); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "invalid store id")
		return
	}

	rep, err := s.svc.GetStoreReputation(storeId)
	if err != nil {
		httpjson.WriteError(w, statusForError(err), err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rep)
}

func (s *storesHandler) Create(w http.ResponseWriter, r *http.Request) {
	userId := middleware.UserID(r)

	store := &Stores{}
	if err := json.NewDecoder(r.Body).Decode(store); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if store.Name == "" {
		httpjson.WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := s.svc.CreateStore(userId, store); err != nil {
		httpjson.WriteError(w, statusForError(err), err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(store)
}

func (s *storesHandler) Update(w http.ResponseWriter, r *http.Request) {
	userId := middleware.UserID(r)

	vars := mux.Vars(r)
	storeId := vars["id"]
	if _, err := uuid.FromString(storeId); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "invalid store id")
		return
	}

	store := &Stores{}
	if err := json.NewDecoder(r.Body).Decode(store); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.svc.UpdateStore(userId, storeId, store); err != nil {
		httpjson.WriteError(w, statusForError(err), err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (s *storesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userId := middleware.UserID(r)

	vars := mux.Vars(r)
	storeId := vars["id"]
	if _, err := uuid.FromString(storeId); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "invalid store id")
		return
	}

	if err := s.svc.DeleteStore(userId, storeId); err != nil {
		httpjson.WriteError(w, statusForError(err), err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (s *storesHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userId := middleware.UserID(r)

	stores, err := s.svc.GetUserStores(userId)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(stores); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
}
