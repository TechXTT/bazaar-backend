package db

import (
	"time"

	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusCompleted OrderStatus = "completed"
	OrderStatusCancelled OrderStatus = "cancelled"
	OrderStatusReleased  OrderStatus = "released"
	OrderStatusDisputed  OrderStatus = "disputed"
	OrderStatusShipped   OrderStatus = "shipped"
)

type DisputeStatus string

const (
	DisputeStatusFeePending  DisputeStatus = "fee_pending"
	DisputeStatusArbitrating DisputeStatus = "arbitrating"
	DisputeStatusResolved    DisputeStatus = "resolved"
	DisputeStatusTimedOut    DisputeStatus = "timed_out"
)

// Users is the single source-of-truth model for the users table. The JSON tags
// preserve the wire contract previously defined by the per-module copy
// (BE-7).
type Users struct {
	gorm.Model
	ID            uuid.UUID  `gorm:"primaryKey" json:"ID"`
	FirstName     string     `json:"FirstName"`
	LastName      string     `json:"LastName"`
	Address       string     `json:"Address,omitempty"`
	WalletAddress string     `gorm:"uniqueIndex;not null" json:"WalletAddress"`
	LastLoginAt   *time.Time `json:"LastLoginAt,omitempty"`
}

func (u *Users) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID, err = uuid.NewV4()
	}
	return err
}

type Stores struct {
	gorm.Model
	ID         uuid.UUID         `gorm:"primaryKey"`
	Name       string            `gorm:"not null" json:"name"`
	OwnerID    uuid.UUID         `gorm:"not null;index" json:"owner_id"`
	Owner      Users             `gorm:"foreignKey:OwnerID"`
	Products   []Products        `gorm:"foreignKey:StoreID"`
	Reputation *SellerReputation `gorm:"-"`
}

func (s *Stores) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID, err = uuid.NewV4()
	}
	return err
}

type Products struct {
	gorm.Model
	ID          uuid.UUID `gorm:"primaryKey"`
	Name        string    `gorm:"not null"`
	ImageURL    string    `gorm:"not null"`
	Price       float64   `gorm:"not null"`
	Unit        string    `gorm:"not null"`
	Description string    `gorm:"not null"`
	StoreID     uuid.UUID `gorm:"not null;index"`
	Store       Stores    `gorm:"foreignKey:StoreID"`
}

func (p *Products) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID, err = uuid.NewV4()
	}
	return err
}

type Orders struct {
	gorm.Model
	ID              uuid.UUID   `gorm:"primaryKey"`
	ContractOrderID string      `gorm:"index"`
	ProductID       uuid.UUID   `gorm:"not null;index"`
	Product         Products    `gorm:"foreignKey:ProductID"`
	BuyerID         uuid.UUID   `gorm:"not null;index"`
	Buyer           Users       `gorm:"foreignKey:BuyerID"`
	Quantity        int         `gorm:"not null"`
	Total           float64     `gorm:"not null"`
	Status          OrderStatus `gorm:"not null;type:varchar(20);default:'pending'"`
	TxHash          string
	// On-chain fields populated by observer
	Token            string `gorm:"type:varchar(10)"` // "eth" or "usdc"
	OnChainProductID string `gorm:"index"`            // hex of bytes32 product id
	Fee              string `gorm:"type:numeric"`     // platform fee amount as string
	MetaEvidenceURI  string
}

func (o *Orders) BeforeCreate(tx *gorm.DB) (err error) {
	if o.ID == uuid.Nil {
		o.ID, err = uuid.NewV4()
		if err != nil {
			return err
		}
	}
	if o.Status == "" {
		o.Status = OrderStatusPending
	}
	return nil
}

type Disputes struct {
	gorm.Model
	ID                  uuid.UUID `gorm:"primaryKey"`
	OrderID             uuid.UUID `gorm:"not null;uniqueIndex"`
	Order               Orders    `gorm:"foreignKey:OrderID"`
	RaisedBy            string    `gorm:"not null"`
	ArbitratorDisputeID *int64
	Ruling              *uint8
	Status              DisputeStatus `gorm:"not null;type:varchar(20);default:'fee_pending'"`
	TxHash              string
	LogIndex            uint
	Evidence            []DisputeEvidence `gorm:"foreignKey:DisputeID"`
}

type DisputeEvidence struct {
	gorm.Model
	ID        uuid.UUID `gorm:"primaryKey"`
	DisputeID uuid.UUID `gorm:"not null;index"`
	Dispute   Disputes  `gorm:"foreignKey:DisputeID"`
	OrderID   uuid.UUID `gorm:"not null;index"`
	Party     string    `gorm:"not null"`
	URI       string    `gorm:"not null"`
	TxHash    string    `gorm:"uniqueIndex:idx_dispute_evidence_tx_log"`
	LogIndex  uint      `gorm:"uniqueIndex:idx_dispute_evidence_tx_log"`
}

type SellerReputation struct {
	gorm.Model
	ID               uuid.UUID `gorm:"primaryKey"`
	SellerWallet     string    `gorm:"uniqueIndex;not null"`
	TotalOrders      int       `gorm:"not null;default:0"`
	CompletedOrders  int       `gorm:"not null;default:0"`
	CancelledOrders  int       `gorm:"not null;default:0"`
	DisputeCount     int       `gorm:"not null;default:0"`
	DisputeSellerWon int       `gorm:"not null;default:0"`
	DisputeBuyerWon  int       `gorm:"not null;default:0"`
	Score            *int      `gorm:"type:smallint"`
}
