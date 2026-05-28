package users

import (
	"time"

	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

type Users struct {
	gorm.Model
	ID            uuid.UUID  `gorm:"primaryKey" json:"id"`
	FirstName     string     `json:"first_name"`
	LastName      string     `json:"last_name"`
	WalletAddress string     `gorm:"uniqueIndex;not null" json:"wallet_address"`
	LastLoginAt   *time.Time `json:"last_login_at"`
}

func (u *Users) BeforeCreate(tx *gorm.DB) (err error) {
	u.ID, err = uuid.NewV4()
	return err
}
