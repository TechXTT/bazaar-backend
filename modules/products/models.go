package products

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
)

// Users mirrors the columns of the live users table (see services/db.Users).
// The legacy email/password columns were dropped, so they must not appear here
// or a Preload("Store.Owner") would try to scan non-existent columns.
type Users struct {
	gorm.Model
	ID            uuid.UUID `gorm:"primaryKey"`
	FirstName     string    `gorm:"not null"`
	LastName      string    `gorm:"not null"`
	Address       string
	WalletAddress string `gorm:"uniqueIndex;not null"`
	LastLoginAt   *time.Time
}

type Stores struct {
	gorm.Model
	ID       uuid.UUID  `gorm:"primaryKey"`
	Name     string     `gorm:"unique, not null"`
	OwnerID  uuid.UUID  `gorm:"not null;index"`
	Owner    Users      `gorm:"foreignKey:OwnerID"`
	Products []Products `gorm:"foreignKey:StoreID"`
}

type Products struct {
	gorm.Model
	ID    uuid.UUID `gorm:"primaryKey"`
	Name  string    `gorm:"not null"`
	Price float64   `gorm:"not null"`
	Unit  string    `gorm:"not null"`

	ImageURL string
	// TODO: Define options for products
	Description string    `gorm:"not null"`
	StoreID     uuid.UUID `gorm:"not null;index"`
	Store       Stores    `gorm:"foreignKey:StoreID"`
}

type Orders struct {
	gorm.Model
	ID        uuid.UUID   `gorm:"primaryKey"`
	ProductID uuid.UUID   `gorm:"not null;index"`
	Product   Products    `gorm:"foreignKey:ProductID"`
	BuyerID   uuid.UUID   `gorm:"not null;index"`
	Quantity  int         `gorm:"not null"`
	Total     float64     `gorm:"not null"`
	Status    OrderStatus `gorm:"not null;type:varchar(20);default:'pending'"`
	TxHash    string
	// TODO: add tracking number and shipping address for orders, and txHash for payment
}

func (o *Orders) BeforeCreate(tx *gorm.DB) (err error) {
	o.ID, err = uuid.NewV4()
	o.Status = OrderStatusPending
	return err
}

func (s *Products) BeforeCreate(tx *gorm.DB) (err error) {
	s.ID, err = uuid.NewV4()
	return err
}
