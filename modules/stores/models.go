package stores

import (
	"time"

	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

type Users struct {
	gorm.Model
	ID            uuid.UUID  `gorm:"primaryKey"`
	FirstName     string     `gorm:"not null"`
	LastName      string     `gorm:"not null"`
	Address       string
	WalletAddress string     `gorm:"uniqueIndex;not null"`
	LastLoginAt   *time.Time
}

type Stores struct {
	gorm.Model
	ID         uuid.UUID            `gorm:"primaryKey"`
	Name       string               `gorm:"unique, not null"`
	OwnerID    uuid.UUID            `gorm:"not null"`
	Owner      Users                `gorm:"foreignKey:OwnerID"`
	Products   []Products           `gorm:"foreignKey:StoreID"`
	Reputation *db.SellerReputation `gorm:"-"`
}

type Products struct {
	gorm.Model
	ID          uuid.UUID `gorm:"primaryKey"`
	Name        string    `gorm:"not null"`
	ImageURL    string    `gorm:"not null"`
	Price       float64   `gorm:"not null"`
	Unit        string    `gorm:"not null"`
	Description string    `gorm:"not null"`
	StoreID     uuid.UUID `gorm:"not null"`
	Store       Stores    `gorm:"foreignKey:StoreID"`
}

func (s *Stores) BeforeCreate(tx *gorm.DB) (err error) {
	s.ID, err = uuid.NewV4()
	return err
}
