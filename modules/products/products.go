package products

import (
	"mime/multipart"
	"net/http"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/jwt"
	"github.com/TechXTT/bazaar-backend/services/middleware"
	"github.com/TechXTT/bazaar-backend/services/s3spaces"
	"github.com/TechXTT/bazaar-backend/services/web"
	"github.com/gorilla/mux"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

type (
	// Service is the products service interface
	Service interface {
		// GetProducts returns all products
		GetProducts() ([]Products, error)

		// GetProduct returns a product by id
		GetProduct(id string) (*Products, error)

		// CreateProduct creates a new product
		CreateProduct(userId string, p *Products) (string, error)

		// UpdateProduct updates a product
		UpdateProduct(userId string, id string, p *Products) error

		// DeleteProduct deletes a product
		DeleteProduct(userId string, id string) error

		// GetProductsFromStore returns paginated products from a store
		GetProductsFromStore(storeId string, cursor string, limit int) ([]Products, error)

		// CreateOrders returns order_ids for given products
		CreateOrders(userId string, orders []DataRequest) ([]OrderResponse, error)

		// GetOrders returns all orders
		GetOrders(userId string) ([]Orders, error)

		// SaveFile saves a file to the object storage
		SaveFile(file multipart.File, filepath string) (string, error)

		// TODO: Add methods for categories and orders
	}

	// Handler provides the products handlers
	Handler interface {

		// Gets handles a request to get all products
		Gets(w http.ResponseWriter, r *http.Request)

		// Get handles a request to get a product
		Get(w http.ResponseWriter, r *http.Request)

		// Create handles a request to create a new product
		Create(w http.ResponseWriter, r *http.Request)

		// Update handles a request to update a product
		Update(w http.ResponseWriter, r *http.Request)

		// Delete handles a request to delete a product
		Delete(w http.ResponseWriter, r *http.Request)

		// GetFromStore handles a request to get products from a store using pagination and store id
		GetFromStore(w http.ResponseWriter, r *http.Request)

		// CreateOrder handles a request to create a new order
		CreateOrder(w http.ResponseWriter, r *http.Request)

		// GetOrders handles a request to get all orders
		GetOrders(w http.ResponseWriter, r *http.Request)
	}

	productsService struct {
		db       db.DB
		jwks     jwt.Jwks
		s3spaces s3spaces.S3Spaces
	}

	productsHandler struct {
		svc Service
	}
)

func init() {

	// Provide dependencies during app boot
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewProductsService)
		do.Provide(e.Msg, NewProductsHandler)
	})

	web.HookBuildRouter.Listen(func(e hooks.Event[*mux.Router]) {
		h := do.MustInvoke[Handler](do.DefaultInjector)

		middleware := do.MustInvoke[middleware.Middleware](do.DefaultInjector)
		authenticatedHandler := e.Msg.NewRoute().Subrouter()
		authenticatedHandler.Use(middleware.AuthMiddleware)

		authenticatedHandler.HandleFunc("/products/orders", h.GetOrders).Methods("GET")

		e.Msg.HandleFunc("/products", h.Gets).Methods("GET")
		e.Msg.HandleFunc("/products/store/{id}", h.GetFromStore).Methods("GET")
		e.Msg.HandleFunc("/products/{id}", h.Get).Methods("GET")

		authenticatedHandler.HandleFunc("/products", h.Create).Methods("POST")
		authenticatedHandler.HandleFunc("/products/{id}", h.Update).Methods("PUT")
		authenticatedHandler.HandleFunc("/products/{id}", h.Delete).Methods("DELETE")

		authenticatedHandler.HandleFunc("/products/orders", h.CreateOrder).Methods("POST")

	})
}
