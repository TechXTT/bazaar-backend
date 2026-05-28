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

func (s *disputesService) ListDisputes(walletAddress string) ([]Disputes, error) {
	gormDB := s.db.DB()

	var disputes []Disputes
	err := gormDB.
		Preload("Evidence").
		Joins("JOIN orders ON disputes.order_id = orders.id").
		Where("orders.buyer_id IN (SELECT id FROM users WHERE wallet_address = ?) OR orders.buyer_id IN (SELECT id FROM users WHERE wallet_address = ?)",
			walletAddress, walletAddress).
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
