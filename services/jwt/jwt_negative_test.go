package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"github.com/TechXTT/bazaar-backend/services/config"
	gojwt "github.com/golang-jwt/jwt/v5"
)

// newTestJwks builds a jwks service backed by a fresh RSA keypair and returns
// the service plus the private key so tests can mint adversarial tokens.
func newTestJwks(t *testing.T) (*jwks, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})

	svc := &jwks{cfg: testConfig{jwt: config.JWTConfig{
		PrivateKey: string(privPEM),
		PublicKey:  string(pubPEM),
	}}}
	return svc, priv
}

// TestValidateToken_Negatives covers BE-11: ValidateToken must reject malformed,
// expired, wrong-key, and algorithm-confusion tokens — never returning a subject.
func TestValidateToken_Negatives(t *testing.T) {
	svc, priv := newTestJwks(t)

	// A different keypair, to simulate a token signed by the wrong issuer.
	otherPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate other key: %v", err)
	}

	signRS256 := func(claims gojwt.RegisteredClaims, key *rsa.PrivateKey) string {
		tok := gojwt.NewWithClaims(gojwt.SigningMethodRS256, claims)
		s, err := tok.SignedString(key)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		return s
	}

	cases := []struct {
		name  string
		token func() string
	}{
		{"empty", func() string { return "" }},
		{"garbage", func() string { return "not.a.jwt" }},
		{
			"expired",
			func() string {
				return signRS256(gojwt.RegisteredClaims{
					ID:        "user-1",
					ExpiresAt: gojwt.NewNumericDate(time.Now().Add(-time.Hour)),
				}, priv)
			},
		},
		{
			"signed with wrong key",
			func() string {
				return signRS256(gojwt.RegisteredClaims{
					ID:        "user-1",
					ExpiresAt: gojwt.NewNumericDate(time.Now().Add(time.Hour)),
				}, otherPriv)
			},
		},
		{
			"alg none confusion",
			func() string {
				tok := gojwt.NewWithClaims(gojwt.SigningMethodNone, gojwt.RegisteredClaims{
					ID:        "user-1",
					ExpiresAt: gojwt.NewNumericDate(time.Now().Add(time.Hour)),
				})
				s, err := tok.SignedString(gojwt.UnsafeAllowNoneSignatureType)
				if err != nil {
					t.Fatalf("sign none: %v", err)
				}
				return s
			},
		},
		{
			"HS256 forgery using public key as secret",
			func() string {
				// Algorithm-confusion attack: forge an HS256 token whose HMAC secret
				// is the server's PEM public key. The alg-pinned parser must refuse it.
				pubBytes, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
				pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
				tok := gojwt.NewWithClaims(gojwt.SigningMethodHS256, gojwt.RegisteredClaims{
					ID:        "attacker",
					ExpiresAt: gojwt.NewNumericDate(time.Now().Add(time.Hour)),
				})
				s, err := tok.SignedString(pubPEM)
				if err != nil {
					t.Fatalf("sign hs256: %v", err)
				}
				return s
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := svc.ValidateToken(tc.token())
			if err == nil {
				t.Fatalf("expected error, got id=%q", id)
			}
			if id != "" {
				t.Fatalf("expected empty subject on failure, got %q", id)
			}
		})
	}
}

// TestValidateToken_TamperedSignature ensures flipping the signature invalidates
// an otherwise-valid token.
func TestValidateToken_TamperedSignature(t *testing.T) {
	svc, _ := newTestJwks(t)
	good, err := svc.GenerateToken("user-1")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// Corrupt a byte in the middle of the signature segment so the change is not
	// absorbed by base64 padding.
	dot := strings.LastIndexByte(good, '.')
	if dot < 0 || dot+1 >= len(good) {
		t.Fatalf("unexpected token shape: %q", good)
	}
	mid := dot + 1 + (len(good)-dot-1)/2
	b := []byte(good)
	if b[mid] == 'A' {
		b[mid] = 'B'
	} else {
		b[mid] = 'A'
	}
	if _, err := svc.ValidateToken(string(b)); err == nil {
		t.Fatal("expected tampered token to be rejected")
	}
}

// TestGeneratedTokenClaims covers BE-16: minted tokens carry a unique jti, the
// API audience, the bazaar issuer, an iat, and a ~1h expiry.
func TestGeneratedTokenClaims(t *testing.T) {
	svc, _ := newTestJwks(t)

	t1, err := svc.GenerateToken("user-7")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	t2, err := svc.GenerateToken("user-7")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	parse := func(tok string) *gojwt.RegisteredClaims {
		claims := &gojwt.RegisteredClaims{}
		_, _, err := gojwt.NewParser().ParseUnverified(tok, claims)
		if err != nil {
			t.Fatalf("parse unverified: %v", err)
		}
		return claims
	}

	c1 := parse(t1)
	c2 := parse(t2)

	if c1.Subject != "user-7" {
		t.Fatalf("subject: got %q want user-7", c1.Subject)
	}
	if len(c1.Audience) == 0 || c1.Audience[0] != tokenAudience {
		t.Fatalf("audience: got %v want %q", c1.Audience, tokenAudience)
	}
	if c1.Issuer != tokenIssuer {
		t.Fatalf("issuer: got %q want %q", c1.Issuer, tokenIssuer)
	}
	if c1.ID == "" {
		t.Fatal("expected non-empty jti")
	}
	if c1.ID == c2.ID {
		t.Fatal("jti must be unique per token")
	}
	if c1.IssuedAt == nil {
		t.Fatal("expected iat")
	}
	if c1.ExpiresAt == nil {
		t.Fatal("expected exp")
	}
	ttl := c1.ExpiresAt.Sub(c1.IssuedAt.Time)
	if ttl != tokenTTL {
		t.Fatalf("ttl: got %s want %s", ttl, tokenTTL)
	}
}
