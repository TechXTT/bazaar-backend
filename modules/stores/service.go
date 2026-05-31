package stores

import (
	"errors"
	"fmt"
	"strings"

	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/gofrs/uuid/v5"
	"github.com/samber/do"
	"gorm.io/gorm"
)

// Sentinel errors so handlers can map service failures to HTTP statuses.
var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrConflict     = errors.New("already exists")
	ErrInvalidInput = errors.New("invalid input")
)

// NewStoresService creates a new users service
func NewStoresService(i *do.Injector) (Service, error) {
	db := do.MustInvoke[db.DB](i)

	return &storesService{
		db: db,
	}, nil
}

func (s *storesService) GetStores() ([]Stores, error) {
	return s.loads()
}

func (s *storesService) GetStore(id string) (*Stores, error) {
	store, err := s.load(uuid.FromStringOrNil(id))
	if err != nil {
		return nil, err
	}
	return &store, nil
}

func (s *storesService) GetStoreReputation(id string) (*db.SellerReputation, error) {
	store, err := s.load(uuid.FromStringOrNil(id))
	if err != nil {
		return nil, err
	}
	if store.Owner.WalletAddress == "" {
		return nil, ErrNotFound
	}

	var rep db.SellerReputation
	if err := s.db.DB().Where("seller_wallet = ?", strings.ToLower(store.Owner.WalletAddress)).First(&rep).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &rep, nil
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
	if err := s.db.DB().Where("owner_id = ?", userId).Find(&stores).Error; err != nil {
		return nil, err
	}
	return stores, nil
}

func (s *storesService) loads() ([]Stores, error) {
	var stores []Stores
	if err := s.db.DB().Preload("Owner").Find(&stores).Error; err != nil {
		return nil, err
	}

	var wallets []string
	for _, st := range stores {
		if st.Owner.WalletAddress != "" {
			wallets = append(wallets, strings.ToLower(st.Owner.WalletAddress))
		}
	}
	if len(wallets) > 0 {
		var reps []db.SellerReputation
		s.db.DB().Where("seller_wallet IN ?", wallets).Find(&reps)
		repMap := make(map[string]*db.SellerReputation, len(reps))
		for i := range reps {
			repMap[reps[i].SellerWallet] = &reps[i]
		}
		for i := range stores {
			if w := strings.ToLower(stores[i].Owner.WalletAddress); w != "" {
				stores[i].Reputation = repMap[w]
			}
		}
	}
	return stores, nil
}

func (s *storesService) load(storeId uuid.UUID) (Stores, error) {
	var store Stores
	if err := s.db.DB().Preload("Owner").Where("id = ?", storeId).First(&store).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return store, ErrNotFound
		}
		return store, err
	}

	if store.Owner.WalletAddress != "" {
		var rep db.SellerReputation
		if s.db.DB().Where("seller_wallet = ?", strings.ToLower(store.Owner.WalletAddress)).First(&rep).Error == nil {
			store.Reputation = &rep
		}
	}
	return store, nil
}

func (s *storesService) save(userId uuid.UUID, store *Stores) error {
	db := s.db.DB()

	user := Users{}
	db.Where("id = ?", userId).First(&user)
	if user.WalletAddress == "" {
		return fmt.Errorf("%w: user has not set wallet address", ErrInvalidInput)
	}

	existingStore := Stores{}
	result := db.Where("name = ?", store.Name).First(&existingStore)
	if result.RowsAffected == 1 {
		return ErrConflict
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
	if err := db.Where("id = ?", id).First(&existingStore).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}
	if existingStore.OwnerID != userId {
		return ErrUnauthorized
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
	if err := db.Where("id = ?", id).First(&store).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}
	if store.OwnerID != userId {
		return ErrUnauthorized
	}

	result := db.Delete(&store)
	if result.Error != nil {
		return result.Error
	}

	return nil
}
