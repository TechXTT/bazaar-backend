package refreshtoken

import (
	"errors"
	"testing"
	"time"
)

// newTestService returns a service over a fresh in-memory store with a
// controllable clock.
func newTestService(now func() time.Time) *service {
	return &service{store: NewMemoryStore(), now: now}
}

// TestRotation covers BE-16: rotating a refresh token invalidates the prior
// token (single-use) and the successor is itself rotatable for the same user.
func TestRotation(t *testing.T) {
	svc := newTestService(time.Now)

	first, err := svc.Issue("user-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	second, uid, err := svc.Rotate(first)
	if err != nil {
		t.Fatalf("Rotate(first): %v", err)
	}
	if uid != "user-1" {
		t.Fatalf("rotated userID: got %q want user-1", uid)
	}
	if second == first {
		t.Fatal("rotation must produce a different token")
	}

	// The prior token is now consumed and must be rejected (no double-spend).
	if _, _, err := svc.Rotate(first); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("re-rotating consumed token: got %v want ErrInvalidToken", err)
	}

	// The successor remains valid and rotatable.
	third, _, err := svc.Rotate(second)
	if err != nil {
		t.Fatalf("Rotate(second): %v", err)
	}
	if third == second {
		t.Fatal("second rotation must produce a different token")
	}
}

// TestLogoutDenylists covers BE-16: logout revokes the active refresh token so
// it can no longer be rotated.
func TestLogoutDenylists(t *testing.T) {
	svc := newTestService(time.Now)

	tok, err := svc.Issue("user-2")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if err := svc.Revoke(tok); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	if _, _, err := svc.Rotate(tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("rotating revoked token: got %v want ErrInvalidToken", err)
	}

	// Logout is idempotent: revoking an unknown token is a no-op success.
	if err := svc.Revoke("never-issued"); err != nil {
		t.Fatalf("Revoke unknown: %v", err)
	}
}

// TestRotateRejectsBadTokens is table-driven over the rejection cases: forged
// (never issued), expired, consumed, and revoked tokens must all be rejected.
func TestRotateRejectsBadTokens(t *testing.T) {
	cases := []struct {
		name  string
		setup func(svc *service) (token string)
	}{
		{
			name: "forged/unknown token",
			setup: func(svc *service) string {
				return "this-token-was-never-issued"
			},
		},
		{
			name: "expired token",
			setup: func(svc *service) string {
				// Issue at t0, then advance the clock past the TTL.
				tok, err := svc.Issue("user-exp")
				if err != nil {
					t.Fatalf("Issue: %v", err)
				}
				svc.now = func() time.Time { return time.Now().Add(RefreshTTL + time.Hour) }
				return tok
			},
		},
		{
			name: "consumed token",
			setup: func(svc *service) string {
				tok, err := svc.Issue("user-consumed")
				if err != nil {
					t.Fatalf("Issue: %v", err)
				}
				if _, _, err := svc.Rotate(tok); err != nil {
					t.Fatalf("Rotate (to consume): %v", err)
				}
				return tok
			},
		},
		{
			name: "revoked token",
			setup: func(svc *service) string {
				tok, err := svc.Issue("user-revoked")
				if err != nil {
					t.Fatalf("Issue: %v", err)
				}
				if err := svc.Revoke(tok); err != nil {
					t.Fatalf("Revoke: %v", err)
				}
				return tok
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestService(time.Now)
			tok := tc.setup(svc)
			if _, _, err := svc.Rotate(tok); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("Rotate(%s): got %v want ErrInvalidToken", tc.name, err)
			}
		})
	}
}

// TestTokensHashedAtRest verifies the raw token is never stored in the clear:
// the store key is the hash, and the raw token does not appear as a key.
func TestTokensHashedAtRest(t *testing.T) {
	store := NewMemoryStore().(*memoryStore)
	svc := &service{store: store, now: time.Now}

	tok, err := svc.Issue("user-3")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if _, ok := store.records[tok]; ok {
		t.Fatal("raw token must not be a store key (must be hashed at rest)")
	}
	if _, ok := store.records[hashToken(tok)]; !ok {
		t.Fatal("hashed token should be the store key")
	}
}
