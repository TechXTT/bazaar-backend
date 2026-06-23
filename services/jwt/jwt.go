package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

// BE-16: token policy constants.
const (
	tokenTTL      = time.Hour
	tokenIssuer   = "bazaar"
	tokenAudience = "bazaar-api"
)

// newJTI returns a random, URL-safe unique token id (jti).
func newJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

var devKey struct {
	sync.Once
	private *rsa.PrivateKey
	err     error
}

var devKeyLog sync.Once

type (
	Jwks interface {
		// GenerateToken generates a new JWT token
		GenerateToken(id string) (string, error)

		// ValidateToken validates a JWT token
		ValidateToken(token string) (string, error)
	}

	jwks struct {
		cfg config.Config
	}
)

func init() {
	// Provide dependencies during app boot process
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewJwk)
	})
}

func NewJwk(i *do.Injector) (Jwks, error) {
	return &jwks{
		cfg: do.MustInvoke[config.Config](i),
	}, nil
}

func (j *jwks) GenerateToken(id string) (string, error) {
	privateKey, err := j.privateKey()
	if err != nil {
		return "", err
	}

	// BE-16: shorten the lifetime from 24h to 1h and add jti (unique token id),
	// iat (issued-at), and aud (audience) so tokens are individually
	// identifiable (enabling future revocation/denylist) and bound to this API.
	jti, err := newJTI()
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    tokenIssuer,
		Audience:  jwt.ClaimStrings{tokenAudience},
		Subject:   id,  // the user UUID
		ID:        jti, // unique token id (jti) for future revocation
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		log.Printf("failed to sign token: %v", err)
		return "", err
	}

	return signedToken, nil
}

func (j *jwks) ValidateToken(token string) (string, error) {
	publicKey, err := j.publicKey()
	if err != nil {
		return "", err
	}

	// BE-16: pin the algorithm, require an expiry, and bind to this API's
	// audience and issuer so tokens minted for another service cannot be
	// replayed here.
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Name}),
		jwt.WithAudience(tokenAudience),
		jwt.WithIssuer(tokenIssuer),
	)

	parsedToken, err := parser.ParseWithClaims(
		token,
		&jwt.RegisteredClaims{},
		func(t *jwt.Token) (interface{}, error) { return publicKey, nil },
	)
	if err != nil {
		log.Printf("failed to parse token: %v", err)
		return "", fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := parsedToken.Claims.(*jwt.RegisteredClaims)
	if !ok || !parsedToken.Valid {
		return "", errors.New("invalid claims")
	}

	if claims.Subject == "" {
		return "", errors.New("token missing subject")
	}
	// Require an explicit expiry (v5.0.0 has no WithExpirationRequired option).
	if claims.ExpiresAt == nil {
		return "", errors.New("token missing expiry")
	}

	return claims.Subject, nil
}

func (j *jwks) privateKey() (*rsa.PrivateKey, error) {
	if isConfiguredPEM(j.cfg.GetJWT().PrivateKey, "RSA PRIVATE KEY") {
		privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(j.cfg.GetJWT().PrivateKey))
		if err == nil {
			return privateKey, nil
		}
		return nil, err
	}

	return developmentKey(j.cfg.GetJWT().DevKeyFile)
}

func (j *jwks) publicKey() (*rsa.PublicKey, error) {
	if isConfiguredPEM(j.cfg.GetJWT().PublicKey, "PUBLIC KEY") {
		publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(j.cfg.GetJWT().PublicKey))
		if err == nil {
			return publicKey, nil
		}
		return nil, err
	}

	privateKey, err := developmentKey(j.cfg.GetJWT().DevKeyFile)
	if err != nil {
		return nil, err
	}
	return &privateKey.PublicKey, nil
}

func developmentKey(path string) (*rsa.PrivateKey, error) {
	devKey.Do(func() {
		devKey.private, devKey.err = loadOrCreateDevelopmentKey(path)
		if devKey.err == nil {
			devKeyLog.Do(func() {
				log.Printf("using development JWT keypair from %s", path)
			})
		}
	})
	return devKey.private, devKey.err
}

func loadOrCreateDevelopmentKey(path string) (*rsa.PrivateKey, error) {
	if path != "" {
		if existing, err := os.ReadFile(path); err == nil {
			privateKey, parseErr := jwt.ParseRSAPrivateKeyFromPEM(existing)
			if parseErr == nil {
				return privateKey, nil
			}
			return nil, parseErr
		}
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	if path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}

		privatePEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		})
		if err := os.WriteFile(path, privatePEM, 0o600); err != nil {
			return nil, err
		}
	}

	return privateKey, nil
}

func isConfiguredPEM(value string, marker string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if trimmed == fmt.Sprintf("-----BEGIN %s-----\n-----END %s-----", marker, marker) {
		return false
	}
	if trimmed == fmt.Sprintf("-----BEGIN %s-----\r\n-----END %s-----", marker, marker) {
		return false
	}
	return strings.Contains(trimmed, fmt.Sprintf("BEGIN %s", marker)) &&
		strings.Contains(trimmed, fmt.Sprintf("END %s", marker))
}
