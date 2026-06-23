// Package refreshtoken implements opaque refresh tokens with server-side
// storage, rotation, and a logout denylist (BE-16).
//
// Design:
//   - A refresh token is a 256-bit cryptographically-random opaque string. It
//     carries no claims; all state lives server-side keyed by the token's hash.
//   - Tokens are hashed (SHA-256) at rest so a store/DB leak does not yield
//     usable tokens — the raw value is only ever returned to the client at
//     issue/rotate time.
//   - Rotation: redeeming a refresh token returns a brand-new token and marks
//     the redeemed one consumed, so a refresh token is single-use. Replaying a
//     consumed (or denylisted) token fails.
//   - Logout denylists the active refresh token immediately.
//
// The Store interface is backed here by an in-memory implementation so the
// build and tests pass offline. For production (multi-replica, durable logout)
// swap in a Redis or Postgres implementation of Store — see the note on
// NewMemoryStore. The hashing, rotation, and expiry semantics live in the
// Service and are storage-agnostic.
package refreshtoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

// RefreshTTL is the lifetime of an issued refresh token. Access tokens are
// short-lived (1h, see services/jwt); refresh tokens are longer-lived but
// rotate on every use and can be revoked via logout.
const RefreshTTL = 30 * 24 * time.Hour

var (
	// ErrInvalidToken is returned when a refresh token is unknown, malformed,
	// already consumed (rotated), denylisted (logged out), or expired.
	ErrInvalidToken = errors.New("invalid or expired refresh token")
)

// Record is the server-side state for one issued refresh token, keyed by the
// token's hash.
type Record struct {
	UserID    string
	ExpiresAt time.Time
	Consumed  bool // rotated: redeemed and replaced by a successor
	Revoked   bool // denylisted via logout
}

// Active reports whether the record may still be redeemed at time now.
func (r Record) Active(now time.Time) bool {
	return !r.Consumed && !r.Revoked && now.Before(r.ExpiresAt)
}

// Store persists refresh-token records keyed by token hash. Implementations
// must be safe for concurrent use.
type Store interface {
	// Save inserts or overwrites the record for hash.
	Save(hash string, rec Record) error
	// Get returns the record for hash and whether it exists.
	Get(hash string) (Record, bool, error)
	// MarkConsumed flags the record for hash as rotated (single-use spent).
	MarkConsumed(hash string) error
	// Revoke flags the record for hash as denylisted (logout).
	Revoke(hash string) error
}

// Service issues, rotates, and revokes opaque refresh tokens.
type Service interface {
	// Issue mints a new opaque refresh token for userID and persists its hash.
	// The raw token is returned to the caller and never stored in the clear.
	Issue(userID string) (string, error)

	// Rotate validates raw, consumes it (single-use), and issues a successor for
	// the same user. The old token is invalid afterwards. Returns the new raw
	// token and the userID. Returns ErrInvalidToken if raw is unknown, already
	// consumed, revoked, or expired.
	Rotate(raw string) (newToken string, userID string, err error)

	// Revoke denylists raw so it (and any attempt to rotate it) is rejected. Used
	// by logout. Unknown/expired tokens are a no-op success.
	Revoke(raw string) error
}

type service struct {
	store Store
	now   func() time.Time
}

func init() {
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewService)
	})
}

// NewService wires the refresh-token service with the default in-memory store.
//
// PRODUCTION: replace NewMemoryStore() with a Redis- or Postgres-backed Store
// so refresh state (rotation + logout denylist) survives restarts and is shared
// across replicas. The Service logic is storage-agnostic; only the Store impl
// needs to change.
func NewService(i *do.Injector) (Service, error) {
	return NewServiceWithStore(NewMemoryStore()), nil
}

// NewServiceWithStore builds a Service over an explicit Store (used in tests and
// for swapping in a production Store).
func NewServiceWithStore(store Store) Service {
	return &service{store: store, now: time.Now}
}

func (s *service) Issue(userID string) (string, error) {
	raw, err := newOpaqueToken()
	if err != nil {
		return "", err
	}
	rec := Record{
		UserID:    userID,
		ExpiresAt: s.now().Add(RefreshTTL),
	}
	if err := s.store.Save(hashToken(raw), rec); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *service) Rotate(raw string) (string, string, error) {
	hash := hashToken(raw)
	rec, ok, err := s.store.Get(hash)
	if err != nil {
		return "", "", err
	}
	if !ok || !rec.Active(s.now()) {
		return "", "", ErrInvalidToken
	}

	// Single-use: consume the redeemed token before issuing its successor so a
	// replay of the same token cannot rotate twice.
	if err := s.store.MarkConsumed(hash); err != nil {
		return "", "", err
	}

	newToken, err := s.Issue(rec.UserID)
	if err != nil {
		return "", "", err
	}
	return newToken, rec.UserID, nil
}

func (s *service) Revoke(raw string) error {
	return s.store.Revoke(hashToken(raw))
}

// newOpaqueToken returns a 256-bit URL-safe random token.
func newOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken returns the hex SHA-256 of raw; only hashes are persisted so a store
// leak does not expose usable tokens.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// memoryStore is the default in-memory Store. It is concurrency-safe but NOT
// durable or shared across processes — see NewService's production note.
type memoryStore struct {
	mu      sync.Mutex
	records map[string]Record
}

// NewMemoryStore returns an in-memory Store suitable for single-process use,
// development, and tests.
func NewMemoryStore() Store {
	return &memoryStore{records: make(map[string]Record)}
}

func (m *memoryStore) Save(hash string, rec Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[hash] = rec
	return nil
}

func (m *memoryStore) Get(hash string) (Record, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[hash]
	return rec, ok, nil
}

func (m *memoryStore) MarkConsumed(hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec, ok := m.records[hash]; ok {
		rec.Consumed = true
		m.records[hash] = rec
	}
	return nil
}

func (m *memoryStore) Revoke(hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec, ok := m.records[hash]; ok {
		rec.Revoked = true
		m.records[hash] = rec
	}
	return nil
}
