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
	"github.com/gorilla/websocket"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

type (
	Service interface {
		CreateRoom(req CreateRoomRequest, userId string) error

		JoinRoom(roomID string, clientID string, username string, conn *websocket.Conn) (*Client, error)

		CreateDispute(userId string, d *Disputes) (string, error)

		GetDispute(userId string, id string) (*Disputes, error)

		CloseDispute(userId string, id string) error

		CreateDisputeImage(userId string, d *DisputeImages) error

		SaveFile(file *multipart.FileHeader, bucket string) (string, error)
	}

	Handler interface {
		// CreateRoom handles a request for a websocket connection
		CreateRoom(w http.ResponseWriter, r *http.Request)
		// JoinRoom handles a request for a websocket connection
		JoinRoom(w http.ResponseWriter, r *http.Request)
		// GetRooms responds with a list of rooms
		GetRooms(w http.ResponseWriter, r *http.Request)
		// CreateDispute handles a request to create a dispute
		CreateDispute(w http.ResponseWriter, r *http.Request)
		// GetDispute handles a request to get a dispute
		GetDispute(w http.ResponseWriter, r *http.Request)
		// CloseDispute handles a request to close a dispute
		CloseDispute(w http.ResponseWriter, r *http.Request)
	}

	wsService struct {
		db       db.DB
		hub      *Hub
		s3spaces s3spaces.S3Spaces
	}

	wsHandler struct {
		svc Service
	}
)

func init() {

	// Provide dependencies during boot
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewWSService)
		do.Provide(e.Msg, NewWSHandler)
	})

	// Register routes during router build
	web.HookBuildRouter.Listen(func(e hooks.Event[*mux.Router]) {
		h := do.MustInvoke[Handler](do.DefaultInjector)

		middleware := do.MustInvoke[middleware.Middleware](do.DefaultInjector)
		authenticatedHandler := e.Msg.NewRoute().Subrouter()
		authenticatedHandler.Use(middleware.AuthMiddleware)

		// middleware := do.MustInvoke[middleware.Middleware](do.DefaultInjector)
		// authenticatedHandler := e.Msg.NewRoute().Subrouter()
		// authenticatedHandler.Use(middleware.AuthMiddleware)

		authenticatedHandler.HandleFunc("/ws/create", h.CreateRoom).Methods("POST")
		e.Msg.HandleFunc("/ws/join/{id}", h.JoinRoom).Methods("GET")
		e.Msg.HandleFunc("/ws/rooms", h.GetRooms).Methods("GET")

		authenticatedHandler.HandleFunc("/disputes", h.CreateDispute).Methods("POST")
		authenticatedHandler.HandleFunc("/disputes/order/{id}", h.GetDispute).Methods("GET")
		authenticatedHandler.HandleFunc("/disputes/{id}", h.CloseDispute).Methods("PUT")
	})
}
