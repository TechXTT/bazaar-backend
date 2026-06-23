package users

import (
	"strings"
	"testing"
	"time"
)

// TestVerifySIWE_Negatives covers BE-11: VerifySIWE must reject invalid
// messages, missing/expired/mismatched nonces, and bad signatures before any
// token is ever issued. These paths return prior to any DB access.
func TestVerifySIWE_Negatives(t *testing.T) {
	t.Run("malformed message", func(t *testing.T) {
		u := &usersService{}
		if _, _, err := u.VerifySIWE("not a siwe message", "0xsig"); err == nil {
			t.Fatal("expected error for malformed SIWE message")
		}
	})

	t.Run("nonce not found", func(t *testing.T) {
		u := &usersService{}
		msg := validSIWEMessage(t, "0x0000000000000000000000000000000000000001", "abcdef0123456789")
		_, _, err := u.VerifySIWE(msg, "0xsig")
		if err == nil || !strings.Contains(err.Error(), "nonce not found") {
			t.Fatalf("expected nonce-not-found error, got %v", err)
		}
	})

	t.Run("nonce expired", func(t *testing.T) {
		u := &usersService{}
		addr := "0x0000000000000000000000000000000000000002"
		u.nonces.Store(strings.ToLower(addr), nonceEntry{
			nonce:     "abcdef0123456789",
			expiresAt: time.Now().Add(-time.Minute),
		})
		u.nonceCount.Add(1)
		msg := validSIWEMessage(t, addr, "abcdef0123456789")
		_, _, err := u.VerifySIWE(msg, "0xsig")
		if err == nil || !strings.Contains(err.Error(), "expired") {
			t.Fatalf("expected nonce-expired error, got %v", err)
		}
	})

	t.Run("nonce mismatch", func(t *testing.T) {
		u := &usersService{}
		addr := "0x0000000000000000000000000000000000000003"
		u.nonces.Store(strings.ToLower(addr), nonceEntry{
			nonce:     "0000000000000000",
			expiresAt: time.Now().Add(time.Minute),
		})
		u.nonceCount.Add(1)
		msg := validSIWEMessage(t, addr, "abcdef0123456789")
		_, _, err := u.VerifySIWE(msg, "0xsig")
		if err == nil || !strings.Contains(err.Error(), "mismatch") {
			t.Fatalf("expected nonce-mismatch error, got %v", err)
		}
	})
}

// validSIWEMessage builds a minimally-valid EIP-4361 message string for the
// given address and nonce so siwe.ParseMessage succeeds and validation proceeds
// to the nonce checks.
func validSIWEMessage(t *testing.T, address, nonce string) string {
	t.Helper()
	return strings.Join([]string{
		"example.com wants you to sign in with your Ethereum account:",
		address,
		"",
		"Sign in to Bazaar",
		"",
		"URI: https://example.com",
		"Version: 1",
		"Chain ID: 1",
		"Nonce: " + nonce,
		"Issued At: 2026-01-01T00:00:00Z",
	}, "\n")
}
