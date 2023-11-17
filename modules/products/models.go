package products

import (
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

type Stores struct {
	gorm.Model
	ID       uuid.UUID  `gorm:"primaryKey"`
	Name     string     `gorm:"unique, not null" json:"name"`
	OwnerID  uuid.UUID  `gorm:"not null" json:"owner_id"`
	Products []Products `gorm:"foreignKey:StoreID"`
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

func (s *Products) BeforeCreate(tx *gorm.DB) (err error) {
	s.ID, err = uuid.NewV4()
	return err
}
