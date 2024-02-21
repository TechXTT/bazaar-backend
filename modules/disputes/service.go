package disputes

import (
	"errors"
	"mime/multipart"

	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/s3spaces"
	"github.com/samber/do"
	"gorm.io/gorm"
)

type DisputeRequest struct {
	OrderID string `json:"orderID"`
	Dispute string `json:"dispute"`
}

func NewDisputesService(i *do.Injector) (Service, error) {
	db := do.MustInvoke[db.DB](i)
	s3spaces := do.MustInvoke[s3spaces.S3Spaces](i)

	w := &disputesService{
		db:       db,
		s3spaces: s3spaces,
	}
	return w, nil
}

func (w *disputesService) CreateDispute(userId string, d *Disputes) (string, error) {
	db := w.db.DB()

	var result string

	err := db.Raw(`
	SELECT 
		   CASE 
			  WHEN orders.buyer_id = ? THEN 'buyer' 
			  WHEN stores.owner_id = ? THEN 'seller' 
			  ELSE 'unrelated'
		  END AS role
		FROM orders
		JOIN products ON orders.product_id = products.id
		JOIN stores ON products.store_id = stores.id
		WHERE orders.id = ?;
	`, userId, userId, d.ID).Scan(&result)

	if err.Error != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			return "", errors.New("order not found")
		} else {
			return "", err.Error
		}
	}

	if result == "unrelated" {
		return "", errors.New("user is not related to this order")
	}

	dispute := Disputes{
		OrderID: d.OrderID,
		Dispute: d.Dispute,
	}

	res := db.Create(&dispute)
	if res.Error != nil {
		return "", res.Error
	}

	return dispute.ID.String(), nil
}

func (w *disputesService) GetDispute(userId string, id string) (*Disputes, error) {
	db := w.db.DB()

	var dispute Disputes
	// get if dispute exists with order id
	err := db.Preload("Images").Where("order_id = ?", id).First(&dispute)
	if err.Error != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("dispute not found")
		} else {
			return nil, err.Error
		}
	}

	// check if user is related to the dispute
	var result string
	err = db.Raw(`
	SELECT
           CASE
              WHEN disputes.resolved = true THEN 'resolved'
              WHEN orders.buyer_id = ? THEN 'buyer'
              WHEN stores.owner_id = ? THEN 'seller'
              ELSE 'unrelated'
          END AS role
        FROM disputes
        JOIN orders ON disputes.order_id = orders.id
        JOIN products ON orders.product_id = products.id
        JOIN stores ON products.store_id = stores.id
        WHERE disputes.order_id = ?;
	`, userId, userId, id).Scan(&result)

	if err.Error != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("dispute not found")
		} else {
			return nil, err.Error
		}
	}

	if result == "unrelated" {
		return nil, errors.New("user is not related to this dispute")
	} else if result == "resolved" {
		return nil, errors.New("dispute is resolved")
	}

	return &dispute, nil
}

func (w *disputesService) CreateDisputeImage(userId string, d *DisputeImages) error {
	db := w.db.DB()

	result := db.Create(&d)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (w *disputesService) SaveFile(fileHeader *multipart.FileHeader, filepath string) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	return w.s3spaces.SaveFile(file, filepath)
}

func (w *disputesService) CloseDispute(userId string, id string) error {
	db := w.db.DB()

	var dispute Disputes
	// get if dispute exists with order id
	err := db.Where("id = ?", id).First(&dispute)
	if err.Error != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			return errors.New("dispute not found")
		} else {
			return err.Error
		}
	}

	// check if user is related to the dispute
	var result string
	err = db.Raw(`
	SELECT 
		   CASE 
			  WHEN orders.buyer_id = ? THEN 'buyer' 
			  WHEN stores.owner_id = ? THEN 'seller' 
			  ELSE 'unrelated'
		  END AS role
		FROM disputes
		JOIN orders ON disputes.order_id = orders.id
		JOIN products ON orders.product_id = products.id
		JOIN stores ON products.store_id = stores.id
		WHERE disputes.id = ? AND disputes.resolved = false;
	`, userId, userId, dispute.ID).Scan(&result)

	if err.Error != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			return errors.New("dispute not found")
		} else {
			return err.Error
		}
	}

	if result == "unrelated" {
		return errors.New("user is not related to this dispute")
	}

	dispute.Resolved = true
	res := db.Save(&dispute)
	if res.Error != nil {
		return res.Error
	}

	return nil
}
