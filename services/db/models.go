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

type Users struct {
	gorm.Model
	ID            uuid.UUID `gorm:"primaryKey"`
	FirstName     string    `gorm:"not null" json:"first_name"`
	LastName      string    `gorm:"not null" json:"last_name"`
	Address       string
	Email         string   `gorm:"not null, unique"`
	Password      string   `gorm:"not null"`
	WalletAddress string   `gorm:"not null, unique" json:"wallet_address"`
	Role          RoleType `gorm:"not null, type:ENUM('admin', 'customer', 'seller');default:'customer'"`
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
	Name     string    `gorm:"not null" json:"name"`
	ImageURL string    `gorm:"not null" json:"image_url"`
	Price    float64   `gorm:"not null" json:"price"`
	// TODO: Define options for products
	Description string    `gorm:"not null" json:"description"`
	StoreID     uuid.UUID `gorm:"not null" json:"store_id"`
	Store       Stores    `gorm:"foreignKey:StoreID"`
}

func (s *Stores) BeforeCreate(tx *gorm.DB) (err error) {
	s.ID, err = uuid.NewV4()
	return err
}

func (u *Users) BeforeCreate(tx *gorm.DB) (err error) {
	u.ID, err = uuid.NewV4()
	return err
}