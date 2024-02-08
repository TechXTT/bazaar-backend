package products

import (
	"errors"
	"mime/multipart"
	"strings"

	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/jwt"
	"github.com/TechXTT/bazaar-backend/services/s3spaces"
	"github.com/gofrs/uuid/v5"
	"github.com/samber/do"
	"gorm.io/gorm/clause"
)

type OrderResponse struct {
	ID           string `json:"id"`
	OwnerAddress string `json:"owner_address"`
}

// NewProductsService creates a new users service
func NewProductsService(i *do.Injector) (Service, error) {
	db := do.MustInvoke[db.DB](i)
	jwks := do.MustInvoke[jwt.Jwks](i)
	s3spaces := do.MustInvoke[s3spaces.S3Spaces](i)

	return &productsService{
		db:       db,
		jwks:     jwks,
		s3spaces: s3spaces,
	}, nil
}

func (p *productsService) GetProducts() ([]Products, error) {
	products := p.load()

	return products, nil
}

func (p *productsService) GetProduct(id string) (*Products, error) {
	products := p.load()

	for _, product := range products {
		if product.ID == uuid.FromStringOrNil(id) {
			return &product, nil
		}
	}

	return nil, errors.New("product not found")
}

func (p *productsService) CreateProduct(userId string, product *Products) (string, error) {

	id, err := p.save(uuid.FromStringOrNil(userId), product)
	if err != nil {
		return "", err
	}

	return id, nil
}

func (p *productsService) UpdateProduct(userId string, id string, product *Products) error {

	if err := p.update(uuid.FromStringOrNil(userId), id, product); err != nil {
		return err
	}

	return nil
}

func (p *productsService) DeleteProduct(userId string, id string) error {

	if err := p.delete(uuid.FromStringOrNil(userId), id); err != nil {
		return err
	}

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

	db.Preload(clause.Associations).Find(&products)

	if len(products) == 0 {
		return nil, errors.New("no products found")
	}

	return products, nil
}

func (p *productsService) CreateOrders(userId string, ordersData []DataRequest) ([]OrderResponse, error) {
	db := p.db.DB()

	orders := []Orders{}
	for _, orderData := range ordersData {
		order := Orders{
			ProductID: orderData.ProductID,
			Quantity:  orderData.Quantity,
		}
		order.CreatedAt = orderData.CreatedAt
		orders = append(orders, order)
	}
	var orderResponses []OrderResponse

	for i, order := range orders {
		order.BuyerID = uuid.FromStringOrNil(userId)
		product, err := p.GetProduct(order.ProductID.String())
		if err != nil {
			return nil, err
		}

		var owner Users
		db.Where("id = ?", product.Store.OwnerID).First(&owner)

		if owner.WalletAddress == "" {
			return nil, errors.New("owner wallet address not found")
		}

		if strings.EqualFold(owner.WalletAddress, ordersData[i].BuyerAddress) {
			return nil, errors.New("owner and buyer cannot be the same")
		}

		if owner.ID == order.BuyerID {
			return nil, errors.New("owner and buyer cannot be the same")
		}

		order.Total = float64(order.Quantity) * product.Price
		if err := db.Create(&order).Error; err != nil {
			return nil, err
		}

		orderResponses = append(orderResponses, OrderResponse{ID: order.ID.String(), OwnerAddress: owner.WalletAddress})
	}

	return orderResponses, nil
}

func (p *productsService) GetOrders(userId string, filter string) ([]Orders, error) {
	db := p.db.DB()

	var orders []Orders
	if filter == "receiving" {
		db.Preload("Product").Where("buyer_id = ?", userId).Find(&orders)
	} else if filter == "sending" {
		db.Preload("Product").Where("product_id IN (SELECT id FROM products WHERE store_id IN (SELECT id FROM stores WHERE owner_id = ?))", userId).Find(&orders)
	} else {
		db.Preload("Product").Where("buyer_id = ? OR product_id IN (SELECT id FROM products WHERE store_id IN (SELECT id FROM stores WHERE owner_id = ?))", userId, userId).Find(&orders)
	}

	return orders, nil
}

func (p *productsService) SaveFile(file multipart.File, filepath string) (string, error) {
	return p.s3spaces.SaveFile(file, filepath)
}

func (p *productsService) load() []Products {
	var products []Products
	p.db.DB().Joins("Store").Find(&products)
	return products
}

func (p *productsService) save(userId uuid.UUID, product *Products) (string, error) {
	db := p.db.DB()

	existingProduct := Products{}
	result := db.Where("name = ?", product.Name).First(&existingProduct)
	if result.RowsAffected == 1 {
		return "", errors.New("product already exists")
	}

	existingStore := Stores{}
	result = db.Where("id = ?", product.StoreID).First(&existingStore)
	if result.RowsAffected == 0 {
		return "", errors.New("store not found")
	}

	if existingStore.OwnerID != userId {
		return "", errors.New("unauthorized")
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
	db.Preload("Store").Where("id = ?", id).First(&existingProduct)
	if existingProduct.Store.OwnerID != userId {
		return errors.New("unauthorized")
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
	db.Preload("Store").Where("id = ?", id).First(&product)
	if product.Store.OwnerID != userId {
		return errors.New("unauthorized")
	}

	result := db.Delete(&product)
	if result.Error != nil {
		return result.Error
	}

	return nil
}
