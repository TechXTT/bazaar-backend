package db

import (
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

type RoleType string

const (
	Admin    RoleType = "admin"
	Customer RoleType = "customer"
	Seller   RoleType = "seller"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusCompleted OrderStatus = "completed"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type Users struct {
	gorm.Model
	ID            uuid.UUID `gorm:"primaryKey"`
	FirstName     string    `gorm:"not null"`
	LastName      string    `gorm:"not null"`
	Address       string
	Email         string `gorm:"not null, unique"`
	EmailVerified bool   `gorm:"default:false"`
	Password      string `gorm:"not null"`
	WalletAddress string
	Role          RoleType `gorm:"not null, type:ENUM('admin', 'customer', 'seller')"`
}

type Stores struct {
	gorm.Model
	ID      uuid.UUID `gorm:"primaryKey"`
	Name    string    `gorm:"not null" json:"name"`
	OwnerID uuid.UUID `gorm:"not null" json:"owner_id"`
	Owner   Users     `gorm:"foreignKey:OwnerID"`
}

type Products struct {
	gorm.Model
	ID       uuid.UUID `gorm:"primaryKey"`
	Name     string    `gorm:"not null"`
	ImageURL string    `gorm:"not null"`
	Price    float64   `gorm:"not null"`
	Unit     string    `gorm:"not null"`
	// TODO: Define options for products
	Description string    `gorm:"not null"`
	StoreID     uuid.UUID `gorm:"not null"`
	Store       Stores    `gorm:"foreignKey:StoreID"`
}

func (s *Stores) BeforeCreate(tx *gorm.DB) (err error) {
	s.ID, err = uuid.NewV4()
	return err
}

type Orders struct {
	gorm.Model
	ID        uuid.UUID   `gorm:"primaryKey"`
	ProductID uuid.UUID   `gorm:"not null"`
	Product   Products    `gorm:"foreignKey:ProductID"`
	BuyerID   uuid.UUID   `gorm:"not null"`
	Quantity  int         `gorm:"not null"`
	Total     float64     `gorm:"not null"`
	Status    OrderStatus `gorm:"not null, type:ENUM('pending', 'completed', 'cancelled'), default:'pending'"`
	TxHash    string
	// TODO: add tracking number and shipping address for orders, and txHash for payment
}
