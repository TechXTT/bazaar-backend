package disputes

import (
	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/gofrs/uuid/v5"
)

// Disputes mirrors db.Disputes but is the local view type for the module.
type Disputes = db.Disputes

// DisputeEvidence mirrors db.DisputeEvidence for use within this module.
type DisputeEvidence = db.DisputeEvidence

// Orders is a local alias for the db Orders model used by dispute queries.
type Orders = db.Orders

// UUID alias for convenience.
type UUID = uuid.UUID
