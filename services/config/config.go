package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

type (
	Config interface {
		GetHTTP() HTTPConfig

		GetDB() DBConfig

		GetApp() AppConfig

		GetJWT() JWTConfig

		GetWs() WsConfig

		GetS3Spaces() S3SpacesConfig

		GetAlgolia() AlgoliaConfig
	}

	Base struct {
		HTTP     HTTPConfig
		DB       DBConfig
		App      AppConfig
		JWT      JWTConfig
		Ws       WsConfig
		S3Spaces S3SpacesConfig
		Algolia  AlgoliaConfig
	}

	HTTPConfig struct {
		Hostname     string
		Port         int
		ReadTimeout  time.Duration
		WriteTimeout time.Duration
		IdleTimeout  time.Duration

		AllowedOrigins   []string
		AllowedMethods   []string
		AllowedHeaders   []string
		ExposedHeaders   []string
		AllowCredentials bool
	}

	DBConfig struct {
		POSTGRES_HOST     string
		POSTGRES_PORT     int
		POSTGRES_USER     string
		POSTGRES_PASSWORD string
		POSTGRES_DB       string
		POSTGRES_SSLMODE  string
	}

	AppConfig struct {
		Name        string
		Timeout     time.Duration
		BackendURL  string
		FrontendURL string
	}

	JWTConfig struct {
		JwksUri    string
		PrivateKey string
		PublicKey  string
		DevKeyFile string
	}

	WsConfig struct {
		ETH_URL          string
		ContractAddress  string
		KlerosCourtURL   string
		BackfillOnStartup bool
		BackfillFromBlock uint64
		BackfillBatchSize int
	}

	S3SpacesConfig struct {
		SpacesKey      string
		SpacesSecret   string
		SpacesName     string
		SpacesEndpoint string
		SpacesCDNBase  string
		SpacesRegion   string
		StorageDriver  string
		LocalUploadDir string
		PinataJWT      string
		IPFSGateway    string
	}

	AlgoliaConfig struct {
		AppID         string
		WriteKey      string
		SearchKey     string
		ProductsIndex string
		SeedOnStartup bool
	}
)

func init() {
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewConfig)
	})
}

func NewConfig(i *do.Injector) (Config, error) {
	var cfg Base

	// BE-20: previously every parse error was discarded, so a typo'd port or
	// timeout silently became 0 (an unusable server). Parse with explicit error
	// handling and sensible defaults, and validate required values, failing fast
	// at boot rather than starting a misconfigured process.
	port, err := parseIntDefault("HTTP_PORT", 8000)
	if err != nil {
		return nil, err
	}
	readTimeout, err := parseDurationDefault("HTTP_READ_TIMEOUT", 15*time.Second)
	if err != nil {
		return nil, err
	}
	writeTimeout, err := parseDurationDefault("HTTP_WRITE_TIMEOUT", 15*time.Second)
	if err != nil {
		return nil, err
	}
	idleTimeout, err := parseDurationDefault("HTTP_IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return nil, err
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid HTTP_PORT %d: must be 1-65535", port)
	}

	allowedOriginsList := splitAndTrim(os.Getenv("HTTP_ALLOWED_ORIGINS"))
	allowedMethodsList := splitAndTrim(os.Getenv("HTTP_ALLOWED_METHODS"))
	allowedHeadersList := splitAndTrim(os.Getenv("HTTP_ALLOWED_HEADERS"))
	exposedHeadersList := splitAndTrim(os.Getenv("HTTP_EXPOSED_HEADERS"))
	allowCredentialsRaw := getEnv("HTTP_ALLOW_CREDENTIALS", os.Getenv("HTTP_ALLOWED_CREDENTIALS"))
	allowCredentials, _ := strconv.ParseBool(allowCredentialsRaw)

	// BE-1: allowing "*" origins (or reflecting any origin) together with
	// credentials is the canonical dangerous CORS misconfiguration. Fail fast
	// rather than ship a backend any website can drive authenticated requests against.
	if allowCredentials {
		if containsWildcard(allowedOriginsList) {
			return nil, fmt.Errorf(`invalid CORS config: HTTP_ALLOWED_CREDENTIALS=true cannot be combined with a "*" entry in HTTP_ALLOWED_ORIGINS; pin explicit origins`)
		}
		if containsWildcard(allowedHeadersList) {
			return nil, fmt.Errorf(`invalid CORS config: HTTP_ALLOWED_CREDENTIALS=true cannot be combined with "*" in HTTP_ALLOWED_HEADERS; pin an explicit allowlist (e.g. Content-Type, Authorization)`)
		}
	}

	cfg.HTTP = HTTPConfig{
		Hostname:         os.Getenv("HTTP_HOSTNAME"),
		Port:             port,
		ReadTimeout:      readTimeout,
		WriteTimeout:     writeTimeout,
		IdleTimeout:      idleTimeout,
		AllowedOrigins:   allowedOriginsList,
		AllowedMethods:   allowedMethodsList,
		AllowedHeaders:   allowedHeadersList,
		ExposedHeaders:   exposedHeadersList,
		AllowCredentials: allowCredentials,
	}

	pgPort, err := parseIntDefault("POSTGRES_PORT", 5432)
	if err != nil {
		return nil, err
	}
	if pgPort < 1 || pgPort > 65535 {
		return nil, fmt.Errorf("invalid POSTGRES_PORT %d: must be 1-65535", pgPort)
	}

	cfg.DB = DBConfig{
		POSTGRES_HOST:     os.Getenv("POSTGRES_HOST"),
		POSTGRES_PORT:     pgPort,
		POSTGRES_USER:     os.Getenv("POSTGRES_USER"),
		POSTGRES_PASSWORD: os.Getenv("POSTGRES_PASSWORD"),
		POSTGRES_DB:       os.Getenv("POSTGRES_DB"),
		POSTGRES_SSLMODE:  getEnv("POSTGRES_SSLMODE", "disable"),
	}

	// Required DB connection settings — a process with no database is useless.
	for _, req := range []struct{ name, val string }{
		{"POSTGRES_HOST", cfg.DB.POSTGRES_HOST},
		{"POSTGRES_USER", cfg.DB.POSTGRES_USER},
		{"POSTGRES_DB", cfg.DB.POSTGRES_DB},
	} {
		if strings.TrimSpace(req.val) == "" {
			return nil, fmt.Errorf("missing required config %s", req.name)
		}
	}

	timeout, err := parseDurationDefault("APP_TIMEOUT", 30*time.Second)
	if err != nil {
		return nil, err
	}

	cfg.App = AppConfig{
		Name:        os.Getenv("APP_NAME"),
		Timeout:     timeout,
		BackendURL:  getEnv("APP_BACKEND_URL", "http://localhost:8000"),
		FrontendURL: getEnv("APP_FRONTEND_URL", "http://localhost:3000"),
	}

	cfg.JWT = JWTConfig{
		JwksUri:    os.Getenv("JWKS_URI"),
		PrivateKey: os.Getenv("PRIVATE_KEY"),
		PublicKey:  os.Getenv("PUBLIC_KEY"),
		DevKeyFile: getEnv("JWT_DEV_KEY_FILE", ".dev/jwt-dev.pem"),
	}

	backfillFromBlock, _ := strconv.ParseUint(os.Getenv("CONTRACT_DEPLOY_BLOCK"), 10, 64)
	backfillBatchSize, err := parseIntDefault("BACKFILL_BATCH_SIZE", 10000)
	if err != nil {
		return nil, err
	}
	if backfillBatchSize < 1 {
		return nil, fmt.Errorf("invalid BACKFILL_BATCH_SIZE %d: must be >= 1", backfillBatchSize)
	}
	backfillOnStartup, _ := strconv.ParseBool(os.Getenv("BACKFILL_ON_STARTUP"))

	cfg.Ws = WsConfig{
		ETH_URL:           os.Getenv("ETH_URL"),
		ContractAddress:   os.Getenv("CONTRACT_ADDRESS"),
		KlerosCourtURL:    getEnv("KLEROS_COURT_URL", "https://resolve.kleros.io"),
		BackfillOnStartup: backfillOnStartup,
		BackfillFromBlock: backfillFromBlock,
		BackfillBatchSize: backfillBatchSize,
	}

	cfg.S3Spaces = S3SpacesConfig{
		SpacesKey:      os.Getenv("SPACES_KEY"),
		SpacesSecret:   os.Getenv("SPACES_SECRET"),
		SpacesName:     os.Getenv("SPACES_NAME"),
		SpacesEndpoint: getEnv("SPACES_ENDPOINT", "https://fra1.digitaloceanspaces.com"),
		SpacesCDNBase:  os.Getenv("SPACES_CDN_BASE"),
		SpacesRegion:   getEnv("SPACES_REGION", "us-east-1"),
		StorageDriver:  getEnv("STORAGE_DRIVER", "s3"),
		LocalUploadDir: getEnv("LOCAL_UPLOAD_DIR", "uploads"),
		PinataJWT:      os.Getenv("PINATA_JWT"),
		IPFSGateway:    getEnv("IPFS_GATEWAY", "https://ipfs.io"),
	}

	algoliaSeed, _ := strconv.ParseBool(os.Getenv("ALGOLIA_SEED_ON_STARTUP"))
	cfg.Algolia = AlgoliaConfig{
		AppID:         os.Getenv("ALGOLIA_APP_ID"),
		WriteKey:      os.Getenv("ALGOLIA_WRITE_KEY"),
		SearchKey:     os.Getenv("ALGOLIA_SEARCH_KEY"),
		ProductsIndex: getEnv("ALGOLIA_PRODUCTS_INDEX", "products"),
		SeedOnStartup: algoliaSeed,
	}

	return &cfg, nil

}

// parseIntDefault parses an int env var, returning the default when unset/empty
// and an error when set but unparseable (BE-20).
func parseIntDefault(key string, def int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, raw, err)
	}
	return v, nil
}

// parseDurationDefault parses a duration env var, returning the default when
// unset/empty and an error when set but unparseable (BE-20).
func parseDurationDefault(key string, def time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, raw, err)
	}
	return v, nil
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

// splitAndTrim splits a comma-separated env value and trims whitespace from
// each entry, dropping empties. Centralising the trimming keeps the CORS
// wildcard check (BE-1) from being fooled by " *" with stray spaces.
func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// containsWildcard reports whether the list contains a "*" entry.
func containsWildcard(values []string) bool {
	for _, v := range values {
		if v == "*" {
			return true
		}
	}
	return false
}

func (c *Base) GetHTTP() HTTPConfig {
	return c.HTTP
}

func (c *Base) GetDB() DBConfig {
	return c.DB
}

func (c *Base) GetApp() AppConfig {
	return c.App
}

func (c *Base) GetJWT() JWTConfig {
	return c.JWT
}

func (c *Base) GetWs() WsConfig {
	return c.Ws
}

func (c *Base) GetS3Spaces() S3SpacesConfig {
	return c.S3Spaces
}

func (c *Base) GetAlgolia() AlgoliaConfig {
	return c.Algolia
}
