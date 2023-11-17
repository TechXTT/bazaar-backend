package products

import (
	"errors"

	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/jwt"
	"github.com/gofrs/uuid/v5"
	"github.com/samber/do"
)

// NewProductsService creates a new users service
func NewProductsService(i *do.Injector) (Service, error) {
	db := do.MustInvoke[db.DB](i)
	jwks := do.MustInvoke[jwt.Jwks](i)

	return &productsService{
		db:   db,
		jwks: jwks,
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

func (p *productsService) CreateProduct(userId string, product *Products) error {

	if err := p.save(uuid.FromStringOrNil(userId), product); err != nil {
		return err
	}

	return nil
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

func (p *productsService) load() []Products {
	var products []Products
	p.db.DB().Joins("Store").Find(&products)
	return products
}

func (p *productsService) save(userId uuid.UUID, product *Products) error {
	db := p.db.DB()

	existingProduct := Products{}
	result := db.Where("name = ?", product.Name).First(&existingProduct)
	if result.RowsAffected == 1 {
		return errors.New("product already exists")
	}

	existingStore := Stores{}
	result = db.Where("id = ?", product.StoreID).First(&existingStore)
	if result.RowsAffected == 0 {
		return errors.New("store not found")
	}

	if existingStore.OwnerID != userId {
		return errors.New("unauthorized")
	}

	result = db.Create(&product)
	if result.Error != nil {
		return result.Error
	}

	return nil
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
