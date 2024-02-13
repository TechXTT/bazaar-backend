package disputes

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/samber/do"
)

type (
	CreateRoomRequest struct {
		ID string `json:"id"`
	}

	RoomRes struct {
		ID string `json:"id"`
	}
)

const (
	socketBufferSize  = 1024
	messageBufferSize = 1024
)

var upgrader = websocket.Upgrader{ReadBufferSize: socketBufferSize, WriteBufferSize: socketBufferSize, CheckOrigin: func(r *http.Request) bool { return true }}

func NewWSHandler(i *do.Injector) (Handler, error) {
	return &wsHandler{
		svc: do.MustInvoke[Service](i),
	}, nil
}

func (ws *wsHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	var req CreateRoomRequest
	userId := r.Header.Get("user_id")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := ws.svc.CreateRoom(req, userId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)

}

func (ws *wsHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID := vars["id"]
	userId := r.URL.Query().Get("userId")
	username := r.URL.Query().Get("username")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	client, err := ws.svc.JoinRoom(roomID, userId, username, conn)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	db := ws.svc.(*wsService).db.DB()
	go client.Write(db)
	client.Read(ws.svc.(*wsService).hub)
}

func (ws *wsHandler) GetRooms(w http.ResponseWriter, r *http.Request) {
	rooms := make([]RoomRes, 0)

	for _, room := range ws.svc.(*wsService).hub.Rooms {
		rooms = append(rooms, RoomRes{ID: room.ID})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rooms)

}

func (ws *wsHandler) CreateDispute(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")
	dispute := &Disputes{}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files := r.MultipartForm.File["images"]

	log.Println(files)

	dispute.Dispute = r.FormValue("dispute")
	dispute.OrderID = uuid.FromStringOrNil(r.FormValue("orderId"))

	id, err := ws.svc.CreateDispute(userId, dispute)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// createRoomReq := CreateRoomRequest{ID: id}

	// if err := ws.svc.CreateRoom(createRoomReq, userId); err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }

	for _, file := range files {
		filepath := "disputes/" + id + "/" + file.Filename
		imageURL, err := ws.svc.SaveFile(file, filepath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		disputeImage := DisputeImages{
			DisputeID: uuid.FromStringOrNil(id),
			Image:     imageURL,
		}

		if err := ws.svc.CreateDisputeImage(userId, &disputeImage); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (ws *wsHandler) GetDispute(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")
	vars := mux.Vars(r)
	id := vars["id"]

	dispute, err := ws.svc.GetDispute(userId, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	createRoomReq := CreateRoomRequest{ID: dispute.ID.String()}

	if err := ws.svc.CreateRoom(createRoomReq, userId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dispute)
}

func (ws *wsHandler) CloseDispute(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")
	vars := mux.Vars(r)
	id := vars["id"]

	if err := ws.svc.CloseDispute(userId, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
