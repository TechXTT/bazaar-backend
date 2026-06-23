package products

import (
	"errors"
	"fmt"
	"mime/multipart"
	"strings"

	"github.com/TechXTT/bazaar-backend/services/algolia"
	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/s3spaces"
	"github.com/gofrs/uuid/v5"
	"github.com/samber/do"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Sentinel errors so handlers can map service failures to HTTP statuses.
var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrConflict     = errors.New("already exists")
	ErrInvalidInput = errors.New("invalid input")
)

type OrderResponse struct {
	ID           string `json:"id"`
	OwnerAddress string `json:"owner_address"`
}

// NewProductsService creates a new users service
func NewProductsService(i *do.Injector) (Service, error) {
	db := do.MustInvoke[db.DB](i)
	s3spaces := do.MustInvoke[s3spaces.S3Spaces](i)
	algoliaSvc := do.MustInvoke[algolia.AlgoliaService](i)

	return &productsService{
		db:       db,
		s3spaces: s3spaces,
		algolia:  algoliaSvc,
	}, nil
}

func (p *productsService) GetProducts() ([]Products, error) {
	var products []Products
	if err := p.db.DB().Joins("Store").Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}

func (p *productsService) GetProduct(id string) (*Products, error) {
	var product Products
	err := p.db.DB().Joins("Store").Where("products.id = ?", uuid.FromStringOrNil(id)).First(&product).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &product, nil
}

func (p *productsService) CreateProduct(userId string, product *Products) (string, error) {

	id, err := p.save(uuid.FromStringOrNil(userId), product)
	if err != nil {
		return "", err
	}

	saved := p.load(uuid.FromStringOrNil(id))
	go p.algolia.IndexProduct(toAlgoliaRecord(saved))

	return id, nil
}

func (p *productsService) UpdateProduct(userId string, id string, product *Products) error {

	if err := p.update(uuid.FromStringOrNil(userId), id, product); err != nil {
		return err
	}

	updated := p.load(uuid.FromStringOrNil(id))
	go p.algolia.IndexProduct(toAlgoliaRecord(updated))

	return nil
}

func (p *productsService) DeleteProduct(userId string, id string) error {

	if err := p.delete(uuid.FromStringOrNil(userId), id); err != nil {
		return err
	}

	go p.algolia.DeleteProduct(id)

	return nil
}

func (p *productsService) GetProductsFromStore(storeId string, cursor string, limit int) ([]Products, error) {
	var products []Products
	db := p.db.DB()

	if cursor == "" {
		db = db.Where("store_id = ?", storeId).Limit(limit).Order("created_at desc")
	} else {
		db = db.Where("store_id = ?", storeId).Where("created_at < ?", cursor).Limit(limit).Order("created_at desc")
	}

	if err := db.Preload(clause.Associations).Find(&products).Error; err != nil {
		return nil, err
	}

	return products, nil
}

func (p *productsService) CreateOrders(userId string, ordersData []DataRequest) ([]OrderResponse, error) {
	db := p.db.DB()

	productIDs := make([]uuid.UUID, 0, len(ordersData))
	for _, orderData := range ordersData {
		productIDs = append(productIDs, orderData.ProductID)
	}

	var products []Products
	if err := db.Preload("Store.Owner").Where("id IN ?", productIDs).Find(&products).Error; err != nil {
		return nil, err
	}

	productByID := make(map[uuid.UUID]Products, len(products))
	for _, product := range products {
		productByID[product.ID] = product
	}

	var orderResponses []OrderResponse

	// BE-4: create every order for a (possibly multi-item) cart atomically.
	// Previously each order was inserted and then a *second*, untransacted write
	// set contract_order_id — so a failure left an orphaned order with an empty
	// contract_order_id the observer could never link to chain, and a mid-loop
	// failure committed earlier items while later ones failed, diverging the DB
	// from on-chain escrow. Wrapping the batch in a transaction makes both the
	// per-item insert+update and the whole batch all-or-nothing.
	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, orderData := range ordersData {
			product, ok := productByID[orderData.ProductID]
			if !ok {
				return ErrNotFound
			}

			owner := product.Store.Owner

			if owner.WalletAddress == "" {
				return fmt.Errorf("%w: owner wallet address not found", ErrInvalidInput)
			}

			if strings.EqualFold(owner.WalletAddress, orderData.BuyerAddress) {
				return fmt.Errorf("%w: owner and buyer cannot be the same", ErrInvalidInput)
			}

			order := Orders{
				ProductID: orderData.ProductID,
				BuyerID:   uuid.FromStringOrNil(userId),
				Quantity:  orderData.Quantity,
			}
			order.CreatedAt = orderData.CreatedAt

			if owner.ID == order.BuyerID {
				return fmt.Errorf("%w: owner and buyer cannot be the same", ErrInvalidInput)
			}

			order.Total = float64(order.Quantity) * product.Price
			if err := tx.Create(&order).Error; err != nil {
				return err
			}

			// The on-chain escrow keys orders by this backend UUID, and the observer
			// matches incoming events on contract_order_id. Seed it with the order's
			// UUID so the two stay linked (the local Orders struct has no such field,
			// so update the column directly).
			if err := tx.Model(&Orders{}).Where("id = ?", order.ID).
				Update("contract_order_id", order.ID.String()).Error; err != nil {
				return err
			}

			orderResponses = append(orderResponses, OrderResponse{ID: order.ID.String(), OwnerAddress: owner.WalletAddress})
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return orderResponses, nil
}

func (p *productsService) GetOrders(userId string, filter string, cursor string, limit int) ([]Orders, error) {
	// BE-10: page the result with the existing created_at cursor pattern instead
	// of returning every row unbounded.
	if limit <= 0 {
		limit = 20
	}

	db := p.db.DB().Preload("Product")

	switch filter {
	case "receiving":
		db = db.Where("buyer_id = ?", userId)
	case "sending":
		db = db.Where("product_id IN (SELECT id FROM products WHERE store_id IN (SELECT id FROM stores WHERE owner_id = ?))", userId)
	default:
		db = db.Where("buyer_id = ? OR product_id IN (SELECT id FROM products WHERE store_id IN (SELECT id FROM stores WHERE owner_id = ?))", userId, userId)
	}

	if cursor != "" {
		db = db.Where("orders.created_at < ?", cursor)
	}

	var orders []Orders
	if err := db.Order("orders.created_at desc").Limit(limit).Find(&orders).Error; err != nil {
		return nil, err
	}

	return orders, nil
}

func (p *productsService) SaveImage(file multipart.File, keyPrefix string) (string, error) {
	return p.s3spaces.SaveImage(file, keyPrefix)
}

func (p *productsService) GetOrder(userId string, id string) (*Orders, error) {
	db := p.db.DB()

	order := Orders{}
	if err := db.Preload("Product.Store").Where("orders.id = ?", id).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	requesterID := uuid.FromStringOrNil(userId)
	if !canAccessOrder(&order, requesterID) {
		return nil, ErrUnauthorized
	}

	return &order, nil
}

// canAccessOrder is the IDOR guard for GetOrder: only the buyer or the seller
// (the owner of the product's store) may read an order.
func canAccessOrder(order *Orders, requesterID uuid.UUID) bool {
	return order.BuyerID == requesterID || order.Product.Store.OwnerID == requesterID
}

func toAlgoliaRecord(p Products) algolia.ProductRecord {
	return algolia.ProductRecord{
		ObjectID:    p.ID.String(),
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Unit:        p.Unit,
		ImageURL:    p.ImageURL,
		StoreID:     p.StoreID.String(),
		StoreName:   p.Store.Name,
		CreatedAt:   p.CreatedAt,
	}
}

func (p *productsService) load(productId uuid.UUID) Products {
	var product Products
	p.db.DB().Joins("Store").Where("products.id = ?", productId).First(&product)
	return product
}

func (p *productsService) save(userId uuid.UUID, product *Products) (string, error) {
	db := p.db.DB()

	existingProduct := Products{}
	result := db.Where("name = ?", product.Name).First(&existingProduct)
	if result.RowsAffected == 1 {
		return "", ErrConflict
	}

	existingStore := Stores{}
	result = db.Where("id = ?", product.StoreID).First(&existingStore)
	if result.RowsAffected == 0 {
		return "", ErrNotFound
	}

	if existingStore.OwnerID != userId {
		return "", ErrUnauthorized
	}

	result = db.Create(&product)
	if result.Error != nil {
		return "", result.Error
	}

	return product.ID.String(), nil
}

func (p *productsService) update(userId uuid.UUID, id string, product *Products) error {
	db := p.db.DB()

	existingProduct := Products{}
	if err := db.Preload("Store").Where("id = ?", id).First(&existingProduct).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}
	if existingProduct.Store.OwnerID != userId {
		return ErrUnauthorized
	}

	result := db.Model(&product).Omit("store_id").Where("id = ?", id).Updates(product)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (p *productsService) delete(userId uuid.UUID, id string) error {
	db := p.db.DB()

	product := Products{}
	if err := db.Preload("Store").Where("id = ?", id).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}
	if product.Store.OwnerID != userId {
		return ErrUnauthorized
	}

	result := db.Delete(&product)
	if result.Error != nil {
		return result.Error
	}

	return nil
}
