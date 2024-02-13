package disputes

import (
	"log"

	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type (
	Client struct {

		// socket is the websocket for this client.
		Socket *websocket.Conn

		// message is the channel for this client to recieve messages.
		Message chan *Message

		// id is the unique identifier for this client.
		ID string

		// roomID is the unique identifier for the room related to the order.
		RoomID string

		// username is the client that sent the message.
		Username string
	}

	Message struct {

		// content is the message content.
		Content string

		// RoomID is the unique identifier for the room related to the order.
		RoomID string

		// Username is the client that sent the message.
		Username string
	}
)

func (c *Client) Read(hub *Hub) {
	defer func() {
		hub.Unregister <- c
		c.Socket.Close()
	}()

	for {
		_, m, err := c.Socket.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		msg := &Message{Content: string(m), RoomID: c.RoomID, Username: c.Username}

		hub.Broadcast <- msg
	}
}

func (c *Client) Write(db *gorm.DB) {
	defer c.Socket.Close()
	for {
		message, ok := <-c.Message
		if !ok {
			return
		}

		if message.Username == c.Username {
			continue
		}

		msg := &Messages{
			Message:   message.Content,
			DisputeID: uuid.FromStringOrNil(message.RoomID),
			SenderID:  uuid.FromStringOrNil(c.ID),
		}

		log.Println("Creating message", msg)

		if err := db.Create(msg).Error; err != nil {
			log.Printf("Error creating message: %s", err)
			return
		}

		c.Socket.WriteJSON(message)
	}
}
