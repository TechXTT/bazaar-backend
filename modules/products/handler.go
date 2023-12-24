package products

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/samber/do"
)

// NewProductsHandler creates a new users handler
func NewProductsHandler(i *do.Injector) (Handler, error) {
	return &productsHandler{
		svc: do.MustInvoke[Service](i),
	}, nil
}

func (s *productsHandler) Gets(w http.ResponseWriter, r *http.Request) {
	products, err := s.svc.GetProducts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(products); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *productsHandler) Get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productId := vars["id"]

	product, err := s.svc.GetProduct(productId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(product)
}

func (s *productsHandler) Create(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")

	product := &Products{}
	if err := json.NewDecoder(r.Body).Decode(product); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.svc.CreateProduct(userId, product); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
}

func (s *productsHandler) Update(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")

	vars := mux.Vars(r)
	productId := vars["id"]

	product := &Products{}
	if err := json.NewDecoder(r.Body).Decode(product); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.svc.UpdateProduct(userId, productId, product); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (s *productsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")

	vars := mux.Vars(r)
	productId := vars["id"]

	if err := s.svc.DeleteProduct(userId, productId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (s *productsHandler) GetFromStore(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	storeId := vars["id"]
	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")

	if limitStr == "" {
		limitStr = "10"
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	products, err := s.svc.GetProductsFromStore(storeId, cursor, limit)
	if err != nil {
		if err.Error() == "no products found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("next-cursor", products[len(products)-1].CreatedAt.String())

	if err := json.NewEncoder(w).Encode(products); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
