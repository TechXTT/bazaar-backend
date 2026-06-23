package products

import (
	"github.com/TechXTT/bazaar-backend/services/db"
)

// BE-7: services/db is the single source of truth for GORM models. The products
// module previously carried a divergent, *older* Orders shape (4 statuses, none
// of the on-chain fields the observer writes), which silently dropped data.
// Alias to the shared models so reads and writes go through one definition.
type (
	Users    = db.Users
	Stores   = db.Stores
	Products = db.Products
	Orders   = db.Orders
)

// OrderStatus and its values mirror the shared db definitions.
type OrderStatus = db.OrderStatus

const (
	OrderStatusPending   = db.OrderStatusPending
	OrderStatusCompleted = db.OrderStatusCompleted
	OrderStatusCancelled = db.OrderStatusCancelled
	OrderStatusReleased  = db.OrderStatusReleased
)
