package stores

import (
	"github.com/TechXTT/bazaar-backend/services/db"
)

// BE-7: services/db is the single source of truth for GORM models. The stores
// module operates on the shared models via these aliases rather than divergent
// copies.
type (
	Users    = db.Users
	Stores   = db.Stores
	Products = db.Products
)
