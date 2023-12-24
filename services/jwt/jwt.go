package jwt

import (
	"errors"
	"log"
	"time"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

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
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(j.cfg.GetJWT().PrivateKey))
	if err != nil {
		log.Printf("failed to parse private key: %v", err)
		return "", err
	}

	claims := jwt.RegisteredClaims{
		Issuer:    "bazaar",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24)),
		ID:        id,
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
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(j.cfg.GetJWT().PublicKey))
	if err != nil {
		log.Printf("failed to parse private key: %v", err)
		return "", err
	}

	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Name}))

	parsedToken, err := parser.ParseWithClaims(
		token,
		&jwt.RegisteredClaims{},
		func(t *jwt.Token) (interface{}, error) { return publicKey, nil },
	)
	if err != nil {
		log.Printf("failed to parse token: %v", err)
		return "", err
	} else if !parsedToken.Valid {
		log.Printf("token is invalid")
		return "", err
	} else if errors.Is(err, jwt.ErrSignatureInvalid) {
		log.Printf("token signature is invalid")
		return "", err
	} else if errors.Is(err, jwt.ErrTokenExpired) {
		log.Printf("token is expired")
		return "", err
	}

	claims, ok := parsedToken.Claims.(*jwt.RegisteredClaims)
	if !ok {
		log.Printf("failed to parse claims")
		return "", err
	}

	if claims.ExpiresAt.Time.Before(time.Now()) {
		log.Printf("token is expired")
		return "", err
	}

	return claims.ID, nil
}
