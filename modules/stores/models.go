package stores

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
	ID       uuid.UUID  `gorm:"primaryKey"`
	Name     string     `gorm:"unique, not null"`
	OwnerID  uuid.UUID  `gorm:"not null"`
	Owner    Users      `gorm:"foreignKey:OwnerID"`
	Products []Products `gorm:"foreignKey:StoreID"`
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
