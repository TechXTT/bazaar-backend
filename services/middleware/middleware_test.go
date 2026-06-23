package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/TechXTT/bazaar-backend/services/jwt"
	"github.com/samber/do"
)

type testConfig struct{ jwt config.JWTConfig }

func (c testConfig) GetHTTP() config.HTTPConfig         { return config.HTTPConfig{} }
func (c testConfig) GetDB() config.DBConfig             { return config.DBConfig{} }
func (c testConfig) GetApp() config.AppConfig           { return config.AppConfig{} }
func (c testConfig) GetJWT() config.JWTConfig           { return c.jwt }
func (c testConfig) GetWs() config.WsConfig             { return config.WsConfig{} }
func (c testConfig) GetS3Spaces() config.S3SpacesConfig { return config.S3SpacesConfig{} }
func (c testConfig) GetAlgolia() config.AlgoliaConfig   { return config.AlgoliaConfig{} }

func newJwks(t *testing.T) jwt.Jwks {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal pub: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})

	inj := do.New()
	do.ProvideValue[config.Config](inj, testConfig{jwt: config.JWTConfig{
		PrivateKey: string(privPEM),
		PublicKey:  string(pubPEM),
	}})
	jwks, err := jwt.NewJwk(inj)
	if err != nil {
		t.Fatalf("new jwk: %v", err)
	}
	return jwks
}

// TestAuthMiddleware_Flow covers BE-11: an httptest auth-flow exercising the
// reject/accept paths of AuthMiddleware end to end.
func TestAuthMiddleware_Flow(t *testing.T) {
	jwks := newJwks(t)
	mw := &middleware{jwt: jwks}

	var seenUserID string
	protected := mw.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUserID = UserID(r)
		w.WriteHeader(http.StatusOK)
	}))

	validToken, err := jwks.GenerateToken("user-42")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	cases := []struct {
		name       string
		authHeader string
		wantStatus int
		wantUserID string
	}{
		{"no header", "", http.StatusUnauthorized, ""},
		{"missing bearer prefix", validToken, http.StatusUnauthorized, ""},
		{"garbage token", "Bearer not.a.jwt", http.StatusUnauthorized, ""},
		{"valid token", "Bearer " + validToken, http.StatusOK, "user-42"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seenUserID = ""
			req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			protected.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status: got %d want %d", rec.Code, tc.wantStatus)
			}
			if seenUserID != tc.wantUserID {
				t.Fatalf("propagated user_id: got %q want %q", seenUserID, tc.wantUserID)
			}
		})
	}
}
