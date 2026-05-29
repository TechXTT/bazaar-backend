package users

import (
	"time"

	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

type Users struct {
	gorm.Model
	ID            uuid.UUID  `gorm:"primaryKey" json:"ID"`
	FirstName     string     `json:"FirstName"`
	LastName      string     `json:"LastName"`
	WalletAddress string     `gorm:"uniqueIndex;not null" json:"WalletAddress"`
	LastLoginAt   *time.Time `json:"LastLoginAt,omitempty"`
}

func (u *Users) BeforeCreate(tx *gorm.DB) (err error) {
	u.ID, err = uuid.NewV4()
	return err
}
