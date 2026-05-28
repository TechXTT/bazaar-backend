package users

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	siwe "github.com/spruceid/siwe-go"

	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/jwt"
	"github.com/samber/do"
)

const nonceTTL = 5 * time.Minute

// NewUsersService creates a new users service
func NewUsersService(i *do.Injector) (Service, error) {
	dbSvc := do.MustInvoke[db.DB](i)
	jwks := do.MustInvoke[jwt.Jwks](i)
	cfg := do.MustInvoke[config.Config](i)

	return &usersService{
		db:   dbSvc,
		jwks: jwks,
		cfg:  cfg,
	}, nil
}

func (u *usersService) GetNonce(walletAddress string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(b)

	u.nonces.Store(strings.ToLower(walletAddress), nonceEntry{
		nonce:     nonce,
		expiresAt: time.Now().Add(nonceTTL),
	})

	return nonce, nil
}

func (u *usersService) VerifySIWE(message string, signature string) (string, *Users, error) {
	parsed, err := siwe.ParseMessage(message)
	if err != nil {
		return "", nil, errors.New("invalid SIWE message")
	}

	addr := strings.ToLower(parsed.GetAddress().Hex())

	raw, ok := u.nonces.Load(addr)
	if !ok {
		return "", nil, errors.New("nonce not found; request a new nonce first")
	}
	entry := raw.(nonceEntry)
	if time.Now().After(entry.expiresAt) {
		u.nonces.Delete(addr)
		return "", nil, errors.New("nonce expired")
	}
	if parsed.GetNonce() != entry.nonce {
		return "", nil, errors.New("nonce mismatch")
	}

	_, err = parsed.Verify(signature, nil, &entry.nonce, nil)
	if err != nil {
		return "", nil, errors.New("signature verification failed")
	}

	// Nonce is single-use
	u.nonces.Delete(addr)

	user, err := u.upsertWallet(addr)
	if err != nil {
		return "", nil, err
	}

	token, err := u.jwks.GenerateToken(addr)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
}

func (u *usersService) GetMe(walletAddress string) (*Users, error) {
	gormDB := u.db.DB()
	var user Users
	if err := gormDB.Where("wallet_address = ?", walletAddress).First(&user).Error; err != nil {
		return nil, errors.New("user not found")
	}
	return &user, nil
}

func (u *usersService) UpdateUser(walletAddress string, updated *Users) error {
	gormDB := u.db.DB()
	return gormDB.Model(&Users{}).
		Where("wallet_address = ?", walletAddress).
		Updates(map[string]interface{}{
			"first_name": updated.FirstName,
			"last_name":  updated.LastName,
		}).Error
}

func (u *usersService) DeleteUser(walletAddress string) error {
	gormDB := u.db.DB()
	return gormDB.Where("wallet_address = ?", walletAddress).Delete(&Users{}).Error
}

func (u *usersService) RefreshToken(walletAddress string) (string, error) {
	if _, err := u.GetMe(walletAddress); err != nil {
		return "", err
	}
	return u.jwks.GenerateToken(walletAddress)
}

func (u *usersService) upsertWallet(walletAddress string) (*Users, error) {
	gormDB := u.db.DB()

	var user Users
	result := gormDB.Where("wallet_address = ?", walletAddress).First(&user)
	if result.Error != nil {
		// First-time sign-in: create the record
		user = Users{WalletAddress: walletAddress}
		if err := gormDB.Create(&user).Error; err != nil {
			return nil, err
		}
	}

	now := time.Now()
	user.LastLoginAt = &now
	gormDB.Save(&user)

	return &user, nil
}
