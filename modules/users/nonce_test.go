package users

import (
	"testing"
	"time"
)

func TestGetNonceTracksCount(t *testing.T) {
	u := &usersService{}

	if _, err := u.GetNonce("0xABC"); err != nil {
		t.Fatalf("GetNonce: %v", err)
	}
	if got := u.nonceCount.Load(); got != 1 {
		t.Fatalf("count after first nonce: got %d want 1", got)
	}

	// Re-requesting for the same wallet (case-insensitive) must not grow the store.
	if _, err := u.GetNonce("0xabc"); err != nil {
		t.Fatalf("GetNonce repeat: %v", err)
	}
	if got := u.nonceCount.Load(); got != 1 {
		t.Fatalf("count after repeat nonce: got %d want 1", got)
	}

	// A distinct wallet grows the store.
	if _, err := u.GetNonce("0xDEF"); err != nil {
		t.Fatalf("GetNonce second wallet: %v", err)
	}
	if got := u.nonceCount.Load(); got != 2 {
		t.Fatalf("count after second wallet: got %d want 2", got)
	}
}

func TestSweepNoncesEvictsExpired(t *testing.T) {
	u := &usersService{}

	u.nonces.Store("0xexpired", nonceEntry{nonce: "a", expiresAt: time.Now().Add(-time.Minute)})
	u.nonceCount.Add(1)
	u.nonces.Store("0xfresh", nonceEntry{nonce: "b", expiresAt: time.Now().Add(time.Minute)})
	u.nonceCount.Add(1)

	u.sweepNonces()

	if _, ok := u.nonces.Load("0xexpired"); ok {
		t.Fatal("expired nonce should have been evicted")
	}
	if _, ok := u.nonces.Load("0xfresh"); !ok {
		t.Fatal("fresh nonce should remain")
	}
	if got := u.nonceCount.Load(); got != 1 {
		t.Fatalf("count after sweep: got %d want 1", got)
	}
}
