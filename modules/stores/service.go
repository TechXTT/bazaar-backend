package stores

import (
	"errors"

	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/gofrs/uuid/v5"
	"github.com/samber/do"
)

// NewStoresService creates a new users service
func NewStoresService(i *do.Injector) (Service, error) {
	db := do.MustInvoke[db.DB](i)

	return &storesService{
		db: db,
	}, nil
}

func (s *storesService) GetStores() ([]Stores, error) {
	stores := s.loads()

	return stores, nil
}

func (s *storesService) GetStore(id string) (*Stores, error) {
	store := s.load(uuid.FromStringOrNil(id))

	if store.ID != uuid.Nil {
		return &store, nil
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

func (s *storesService) GetUserStores(userId string) ([]Stores, error) {
	var stores []Stores
	s.db.DB().Where("owner_id = ?", userId).Find(&stores)
	return stores, nil
}

func (s *storesService) loads() []Stores {
	var stores []Stores
	s.db.DB().Find(&stores)
	return stores
}

func (s *storesService) load(storeId uuid.UUID) Stores {
	var store Stores
	s.db.DB().Preload("Owner").Where("id = ?", storeId).First(&store)
	return store
}

func (s *storesService) save(userId uuid.UUID, store *Stores) error {
	db := s.db.DB()

	user := Users{}
	db.Where("id = ?", userId).First(&user)
	if user.WalletAddress == "" {
		return errors.New("user has not set wallet address")
	}

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
