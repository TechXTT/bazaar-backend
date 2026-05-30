package products

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/TechXTT/bazaar-backend/pkg/httpjson"
	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/mux"
	"github.com/samber/do"
)

type (
	DataRequest struct {
		CreatedAt    time.Time
		ProductID    uuid.UUID
		Quantity     int
		BuyerAddress string
	}

	OrderRequest struct {
		Data []DataRequest `json:"data"`
	}
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
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(products); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func (s *productsHandler) Get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productId := vars["id"]

	product, err := s.svc.GetProduct(productId)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(product)
}

func (s *productsHandler) Create(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")

	product := &Products{}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		httpjson.WriteError(w, http.StatusRequestEntityTooLarge, "file too large")
		return
	}

	// read from form data
	file, _, err := r.FormFile("image")
	if err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer file.Close()

	// read from form data
	product.Name = r.FormValue("name")
	product.Description = r.FormValue("description")
	product.Price, err = strconv.ParseFloat(r.FormValue("price"), 64)
	if err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	product.Unit = "ETH" // TODO: future implementation to allow other coins
	product.StoreID = uuid.FromStringOrNil(r.FormValue("storeId"))

	id, err := s.svc.CreateProduct(userId, product)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// save file to object storage
	filepath := "products/" + r.FormValue("storeId") + "/" + id
	imageURL, err := s.svc.SaveFile(file, filepath)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	product.ImageURL = imageURL

	if err := s.svc.UpdateProduct(userId, id, product); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
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
		httpjson.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.svc.UpdateProduct(userId, productId, product); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
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
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
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
		httpjson.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	products, err := s.svc.GetProductsFromStore(storeId, cursor, limit)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if len(products) > 0 {
		w.Header().Set("next-cursor", products[len(products)-1].CreatedAt.String())
	}

	if err := json.NewEncoder(w).Encode(products); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func (s *productsHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")

	// body is data:  []Orders
	orders := &OrderRequest{}
	if err := json.NewDecoder(r.Body).Decode(orders); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Assign the returned values from s.svc.CreateOrders to separate variables
	orderIds, err := s.svc.CreateOrders(userId, orders.Data)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(orderIds)
}

func (s *productsHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")
	filter := r.URL.Query().Get("filter")

	orders, err := s.svc.GetOrders(userId, filter)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

func (s *productsHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")
	vars := mux.Vars(r)
	orderId := vars["id"]

	order, err := s.svc.GetOrder(userId, orderId)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func (s *productsHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "file too large")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer file.Close()

	url, err := s.svc.SaveFile(file, "evidence/"+header.Filename)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"url": url})
}
