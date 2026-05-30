package disputes

import (
	"errors"

	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/samber/do"
	"gorm.io/gorm"
)

func NewDisputesService(i *do.Injector) (Service, error) {
	dbSvc := do.MustInvoke[db.DB](i)
	return &disputesService{db: dbSvc}, nil
}

func (s *disputesService) ListDisputes(userID string) ([]Disputes, error) {
	gormDB := s.db.DB()

	// userID is the authenticated user's UUID (the JWT subject set on the
	// "user_id" header), not a wallet address. Return disputes where the caller
	// is the order's buyer or owns the store the ordered product belongs to.
	var disputes []Disputes
	err := gormDB.
		Preload("Evidence").
		Joins("JOIN orders ON disputes.order_id = orders.id").
		Where("orders.buyer_id = ? OR orders.product_id IN (SELECT id FROM products WHERE store_id IN (SELECT id FROM stores WHERE owner_id = ?))",
			userID, userID).
		Order("disputes.created_at desc").
		Find(&disputes).Error
	if err != nil {
		return nil, err
	}
	return disputes, nil
}

func (s *disputesService) GetDispute(walletAddress string, orderId string) (*Disputes, error) {
	gormDB := s.db.DB()

	var dispute Disputes
	err := gormDB.
		Preload("Evidence").
		Joins("JOIN orders ON disputes.order_id = orders.id").
		Where("orders.contract_order_id = ?", orderId).
		First(&dispute).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("dispute not found")
		}
		return nil, err
	}
	return &dispute, nil
}

func (s *disputesService) GetEvidence(orderId string) ([]DisputeEvidence, error) {
	gormDB := s.db.DB()

	var evidence []DisputeEvidence
	err := gormDB.
		Joins("JOIN disputes ON dispute_evidences.dispute_id = disputes.id").
		Joins("JOIN orders ON disputes.order_id = orders.id").
		Where("orders.contract_order_id = ?", orderId).
		Order("dispute_evidences.created_at asc").
		Find(&evidence).Error
	if err != nil {
		return nil, err
	}
	return evidence, nil
}
