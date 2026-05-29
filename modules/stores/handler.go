package stores

import (
	"encoding/json"
	"net/http"

	"github.com/TechXTT/bazaar-backend/pkg/httpjson"
	"github.com/gorilla/mux"
	"github.com/samber/do"
)

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

	store, err := s.svc.GetStore(storeId)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(store)
}

func (s *storesHandler) GetReputation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	storeId := vars["id"]

	rep, err := s.svc.GetStoreReputation(storeId)
	if err != nil {
		httpjson.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rep)
}

func (s *storesHandler) Create(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")

	store := &Stores{}
	if err := json.NewDecoder(r.Body).Decode(store); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.svc.CreateStore(userId, store); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(store)
}

func (s *storesHandler) Update(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")

	vars := mux.Vars(r)
	storeId := vars["id"]

	store := &Stores{}
	if err := json.NewDecoder(r.Body).Decode(store); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.svc.UpdateStore(userId, storeId, store); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (s *storesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")

	vars := mux.Vars(r)
	storeId := vars["id"]

	if err := s.svc.DeleteStore(userId, storeId); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (s *storesHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")

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
