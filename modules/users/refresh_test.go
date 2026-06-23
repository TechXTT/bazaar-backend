package users

import (
	"testing"

	"github.com/TechXTT/bazaar-backend/services/refreshtoken"
)

// TestRefreshToken_RejectsInvalid covers BE-16 at the service layer: an unknown
// or forged refresh token is rejected before any DB access (so no DB is needed).
func TestRefreshToken_RejectsInvalid(t *testing.T) {
	u := &usersService{
		refresh: refreshtoken.NewServiceWithStore(refreshtoken.NewMemoryStore()),
	}

	if _, _, err := u.RefreshToken("forged-token"); err == nil {
		t.Fatal("expected error rotating a forged refresh token")
	}
}

// TestLogout_RevokesRefreshToken covers BE-16: logout denylists the active
// refresh token so a subsequent rotate is rejected.
func TestLogout_RevokesRefreshToken(t *testing.T) {
	rt := refreshtoken.NewServiceWithStore(refreshtoken.NewMemoryStore())
	u := &usersService{refresh: rt}

	tok, err := rt.Issue("user-x")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if err := u.Logout(tok); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	if _, _, err := u.RefreshToken(tok); err == nil {
		t.Fatal("expected rotate of revoked token to fail after logout")
	}
}
