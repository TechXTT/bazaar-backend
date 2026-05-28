package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/TechXTT/bazaar-backend/services/config"
)

type testConfig struct {
	jwt config.JWTConfig
}

func (c testConfig) GetHTTP() config.HTTPConfig         { return config.HTTPConfig{} }
func (c testConfig) GetDB() config.DBConfig             { return config.DBConfig{} }
func (c testConfig) GetApp() config.AppConfig           { return config.AppConfig{} }
func (c testConfig) GetJWT() config.JWTConfig           { return c.jwt }
func (c testConfig) GetWs() config.WsConfig             { return config.WsConfig{} }
func (c testConfig) GetS3Spaces() config.S3SpacesConfig { return config.S3SpacesConfig{} }

func TestGenerateAndValidateToken(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	service := &jwks{
		cfg: testConfig{
			jwt: config.JWTConfig{
				PrivateKey: string(privateKeyPEM),
				PublicKey:  string(publicKeyPEM),
				DevKeyFile: "",
			},
		},
	}

	token, err := service.GenerateToken("user-id")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	id, err := service.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if id != "user-id" {
		t.Fatalf("got id %q, want %q", id, "user-id")
	}
}
