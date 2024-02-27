package users

import (
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
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
}

func (u *Users) BeforeCreate(tx *gorm.DB) (err error) {
	u.ID, err = uuid.NewV4()
	return err
}
