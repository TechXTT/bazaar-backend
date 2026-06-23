package users

import (
	"github.com/TechXTT/bazaar-backend/services/db"
)

// BE-7: services/db is the single source of truth for GORM models. The users
// module operates on the same table via this alias rather than a divergent copy.
type Users = db.Users
