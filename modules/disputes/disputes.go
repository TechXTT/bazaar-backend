package disputes

import (
	"mime/multipart"
	"net/http"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/middleware"
	"github.com/TechXTT/bazaar-backend/services/s3spaces"
	"github.com/TechXTT/bazaar-backend/services/web"
	"github.com/gorilla/mux"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

type (
	Service interface {
		CreateDispute(userId string, d *Disputes) (string, error)

		GetDispute(userId string, id string) (*Disputes, error)

		CloseDispute(userId string, id string) error

		CreateDisputeImage(userId string, d *DisputeImages) error

		SaveFile(file *multipart.FileHeader, bucket string) (string, error)
	}

	Handler interface {
		// CreateDispute handles a request to create a dispute
		CreateDispute(w http.ResponseWriter, r *http.Request)
		// GetDispute handles a request to get a dispute
		GetDispute(w http.ResponseWriter, r *http.Request)
		// CloseDispute handles a request to close a dispute
		CloseDispute(w http.ResponseWriter, r *http.Request)
	}

	disputesService struct {
		db       db.DB
		s3spaces s3spaces.S3Spaces
	}

	disputesHandler struct {
		svc Service
	}
)

func init() {

	// Provide dependencies during boot
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewDisputeService)
		do.Provide(e.Msg, NewDisputeHandler)
	})

	// Register routes during router build
	web.HookBuildRouter.Listen(func(e hooks.Event[*mux.Router]) {
		h := do.MustInvoke[Handler](do.DefaultInjector)

		middleware := do.MustInvoke[middleware.Middleware](do.DefaultInjector)
		authenticatedHandler := e.Msg.NewRoute().Subrouter()
		authenticatedHandler.Use(middleware.AuthMiddleware)

		authenticatedHandler.HandleFunc("/disputes", h.CreateDispute).Methods("POST")
		authenticatedHandler.HandleFunc("/disputes/order/{id}", h.GetDispute).Methods("GET")
		authenticatedHandler.HandleFunc("/disputes/{id}", h.CloseDispute).Methods("PUT")
	})
}
