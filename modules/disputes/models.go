package disputes

import (
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

type Disputes struct {
	gorm.Model
	ID       uuid.UUID       `gorm:"not null"`
	OrderID  uuid.UUID       `gorm:"not null"`
	Order    Orders          `gorm:"foreignKey:OrderID"`
	Dispute  string          `gorm:"not null"`
	Resolved bool            `gorm:"default:false"`
	Images   []DisputeImages `gorm:"foreignKey:DisputeID"`
}

func (d *Disputes) BeforeCreate(tx *gorm.DB) (err error) {
	d.ID, err = uuid.NewV4()
	return err
}

type DisputeImages struct {
	gorm.Model
	ID        uuid.UUID `gorm:"primaryKey"`
	Dispute   Disputes  `gorm:"foreignKey:DisputeID"`
	DisputeID uuid.UUID `gorm:"not null"`
	Image     string    `gorm:"not null"`
}

func (d *DisputeImages) BeforeCreate(tx *gorm.DB) (err error) {
	d.ID, err = uuid.NewV4()
	return err
}

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

type Orders struct {
	gorm.Model
	ID        uuid.UUID `gorm:"primaryKey"`
	ProductID uuid.UUID `gorm:"not null"`
	BuyerID   uuid.UUID `gorm:"not null"`
	Buyer     Users     `gorm:"foreignKey:BuyerID"`
	Quantity  int       `gorm:"not null"`
	Total     float64   `gorm:"not null"`
	TxHash    string
	// TODO: add tracking number and shipping address for orders, and txHash for payment
}
