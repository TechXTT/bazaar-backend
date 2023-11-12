package stores

import (
	"errors"

	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/jwt"
	"github.com/gofrs/uuid/v5"
	"github.com/samber/do"
)

// NewStoresService creates a new users service
func NewStoresService(i *do.Injector) (Service, error) {
	db := do.MustInvoke[db.DB](i)
	jwks := do.MustInvoke[jwt.Jwks](i)

	return &storesService{
		db:   db,
		jwks: jwks,
	}, nil
}

func (s *storesService) GetStores() ([]Stores, error) {
	stores := s.load()

	return stores, nil
}

func (s *storesService) GetStore(id string) (*Stores, error) {
	stores := s.load()

	for _, store := range stores {
		if store.ID == uuid.FromStringOrNil(id) {
			return &store, nil
		}
	}

	return nil, errors.New("store not found")
}

func (s *storesService) CreateStore(userId string, store *Stores) error {

	if err := s.save(uuid.FromStringOrNil(userId), store); err != nil {
		return err
	}

	return nil
}

func (s *storesService) UpdateStore(userId string, id string, store *Stores) error {

	if err := s.update(uuid.FromStringOrNil(userId), id, store); err != nil {
		return err
	}

	return nil
}

func (s *storesService) DeleteStore(userId string, id string) error {

	if err := s.delete(uuid.FromStringOrNil(userId), id); err != nil {
		return err
	}

	return nil
}

func (s *storesService) load() []Stores {
	var stores []Stores
	s.db.DB().Joins("Owner").Find(&stores)
	return stores
}

func (s *storesService) save(userId uuid.UUID, store *Stores) error {
	db := s.db.DB()

	existingStore := Stores{}
	result := db.Where("name = ?", store.Name).First(&existingStore)
	if result.RowsAffected == 1 {
		return errors.New("store already exists")
	}

	store.OwnerID = userId

	result = db.Create(&store)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (s *storesService) update(userId uuid.UUID, id string, store *Stores) error {
	db := s.db.DB()

	existingStore := Stores{}
	db.Where("id = ?", id).First(&existingStore)
	if existingStore.OwnerID != userId {
		return errors.New("unauthorized")
	}

	result := db.Model(&store).Omit("owner_id").Where("id = ?", id).Updates(store)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (s *storesService) delete(userId uuid.UUID, id string) error {
	db := s.db.DB()

	store := Stores{}
	db.Where("id = ?", id).First(&store)
	if store.OwnerID != userId {
		return errors.New("unauthorized")
	}

	result := db.Delete(&store)
	if result.Error != nil {
		return result.Error
	}

	return nil
}
