package disputes

type Room struct {
	// ID is the unique identifier for this room related to the order.
	ID string `json:"id"`
	// Clients holds all clients in this room.
	Clients map[string]*Client `json:"clients"`
}
