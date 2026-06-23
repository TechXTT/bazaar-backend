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
	"github.com/TechXTT/bazaar-backend/services/refreshtoken"
	"github.com/samber/do"
	"gorm.io/gorm"
)

const nonceTTL = 5 * time.Minute

// BE-6: cap the in-memory nonce store so a flood of distinct wallet addresses
// cannot exhaust process memory. When the store reaches maxNonces we sweep
// expired entries first; if it is still at capacity we reject new nonces.
const maxNonces = 50000

// NewUsersService creates a new users service
func NewUsersService(i *do.Injector) (Service, error) {
	dbSvc := do.MustInvoke[db.DB](i)
	jwks := do.MustInvoke[jwt.Jwks](i)
	cfg := do.MustInvoke[config.Config](i)
	refresh := do.MustInvoke[refreshtoken.Service](i)

	return &usersService{
		db:      dbSvc,
		jwks:    jwks,
		refresh: refresh,
		cfg:     cfg,
	}, nil
}

func (u *usersService) GetNonce(walletAddress string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(b)

	addr := strings.ToLower(walletAddress)

	// BE-6: bound the store. Overwriting an existing wallet's nonce does not grow
	// the map, so only guard when inserting a brand-new key.
	if _, exists := u.nonces.Load(addr); !exists {
		if u.nonceCount.Load() >= maxNonces {
			u.sweepNonces()
		}
		if u.nonceCount.Load() >= maxNonces {
			return "", errors.New("nonce store is full; try again shortly")
		}
		u.nonceCount.Add(1)
	}

	u.nonces.Store(addr, nonceEntry{
		nonce:     nonce,
		expiresAt: time.Now().Add(nonceTTL),
	})

	return nonce, nil
}

// deleteNonce removes a nonce entry and keeps the counter in sync (BE-6).
func (u *usersService) deleteNonce(addr string) {
	if _, loaded := u.nonces.LoadAndDelete(addr); loaded {
		u.nonceCount.Add(-1)
	}
}

// sweepNonces evicts all expired nonce entries, keeping the in-memory store
// bounded (BE-6).
func (u *usersService) sweepNonces() {
	now := time.Now()
	u.nonces.Range(func(key, value any) bool {
		entry, ok := value.(nonceEntry)
		if !ok || now.After(entry.expiresAt) {
			if _, loaded := u.nonces.LoadAndDelete(key); loaded {
				u.nonceCount.Add(-1)
			}
		}
		return true
	})
}

func (u *usersService) VerifySIWE(message string, signature string) (string, string, *Users, error) {
	parsed, err := siwe.ParseMessage(message)
	if err != nil {
		return "", "", nil, errors.New("invalid SIWE message")
	}

	addr := strings.ToLower(parsed.GetAddress().Hex())

	raw, ok := u.nonces.Load(addr)
	if !ok {
		return "", "", nil, errors.New("nonce not found; request a new nonce first")
	}
	entry := raw.(nonceEntry)
	if time.Now().After(entry.expiresAt) {
		u.deleteNonce(addr)
		return "", "", nil, errors.New("nonce expired")
	}
	if parsed.GetNonce() != entry.nonce {
		return "", "", nil, errors.New("nonce mismatch")
	}

	_, err = parsed.Verify(signature, nil, &entry.nonce, nil)
	if err != nil {
		return "", "", nil, errors.New("signature verification failed")
	}

	// Nonce is single-use
	u.deleteNonce(addr)

	user, err := u.upsertWallet(addr)
	if err != nil {
		return "", "", nil, err
	}

	// Store user UUID (not wallet address) so all service lookups work by UUID.
	token, err := u.jwks.GenerateToken(user.ID.String())
	if err != nil {
		return "", "", nil, err
	}

	// BE-16: issue an opaque, rotating refresh token alongside the short-lived
	// access JWT so the client can renew without re-running SIWE.
	refresh, err := u.refresh.Issue(user.ID.String())
	if err != nil {
		return "", "", nil, err
	}

	return token, refresh, user, nil
}

func (u *usersService) GetMe(userID string) (*Users, error) {
	gormDB := u.db.DB()
	var user Users
	if err := gormDB.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (u *usersService) UpdateUser(userID string, updated *Users) error {
	gormDB := u.db.DB()
	return gormDB.Model(&Users{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"first_name": updated.FirstName,
			"last_name":  updated.LastName,
		}).Error
}

func (u *usersService) DeleteUser(userID string) error {
	gormDB := u.db.DB()
	return gormDB.Where("id = ?", userID).Delete(&Users{}).Error
}

// RefreshToken rotates an opaque refresh token (BE-16). The supplied token is
// validated and consumed (single-use); a fresh access JWT and a successor
// refresh token are returned. A consumed, revoked, expired, or forged token is
// rejected.
func (u *usersService) RefreshToken(refreshToken string) (string, string, error) {
	newRefresh, userID, err := u.refresh.Rotate(refreshToken)
	if err != nil {
		return "", "", err
	}

	// Defence in depth: don't mint an access token for a user that no longer
	// exists (e.g. deleted account whose refresh token wasn't revoked).
	if _, err := u.GetMe(userID); err != nil {
		return "", "", err
	}

	token, err := u.jwks.GenerateToken(userID)
	if err != nil {
		return "", "", err
	}
	return token, newRefresh, nil
}

// Logout denylists the supplied refresh token (BE-16). After logout the token
// can no longer be rotated. Unknown/expired tokens are a no-op success so
// logout is idempotent and does not leak token validity.
func (u *usersService) Logout(refreshToken string) error {
	return u.refresh.Revoke(refreshToken)
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
