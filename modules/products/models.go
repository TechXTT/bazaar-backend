package products

import (
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

type Stores struct {
	gorm.Model
	ID       uuid.UUID  `gorm:"primaryKey"`
	Name     string     `gorm:"unique, not null"`
	OwnerID  uuid.UUID  `gorm:"not null"`
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

func (s *Products) BeforeCreate(tx *gorm.DB) (err error) {
	s.ID, err = uuid.NewV4()
	return err
}
