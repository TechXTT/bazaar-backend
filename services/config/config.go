package config

import (
	"log"
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
	}

	Base struct {
		HTTP     HTTPConfig
		DB       DBConfig
		App      AppConfig
		JWT      JWTConfig
		Ws       WsConfig
		S3Spaces S3SpacesConfig
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
)

func init() {
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewConfig)
	})
}

func NewConfig(i *do.Injector) (Config, error) {
	var cfg Base

	port, _ := strconv.Atoi(os.Getenv("HTTP_PORT"))
	readTimeout, _ := time.ParseDuration(os.Getenv("HTTP_READ_TIMEOUT"))
	writeTimeout, _ := time.ParseDuration(os.Getenv("HTTP_WRITE_TIMEOUT"))
	idleTimeout, _ := time.ParseDuration(os.Getenv("HTTP_IDLE_TIMEOUT"))

	allowedOrigins := os.Getenv("HTTP_ALLOWED_ORIGINS")
	allowedOriginsList := strings.Split(allowedOrigins, ",")
	log.Print(allowedOriginsList)
	allowedMethods := os.Getenv("HTTP_ALLOWED_METHODS")
	allowedMethodsList := strings.Split(allowedMethods, ",")
	allowedHeaders := os.Getenv("HTTP_ALLOWED_HEADERS")
	allowedHeadersList := strings.Split(allowedHeaders, ",")
	exposedHeaders := os.Getenv("HTTP_EXPOSED_HEADERS")
	exposedHeadersList := strings.Split(exposedHeaders, ",")
	allowCredentialsRaw := getEnv("HTTP_ALLOW_CREDENTIALS", os.Getenv("HTTP_ALLOWED_CREDENTIALS"))
	allowCredentials, _ := strconv.ParseBool(allowCredentialsRaw)

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

	port, _ = strconv.Atoi(os.Getenv("POSTGRES_PORT"))

	cfg.DB = DBConfig{
		POSTGRES_HOST:     os.Getenv("POSTGRES_HOST"),
		POSTGRES_PORT:     port,
		POSTGRES_USER:     os.Getenv("POSTGRES_USER"),
		POSTGRES_PASSWORD: os.Getenv("POSTGRES_PASSWORD"),
		POSTGRES_DB:       os.Getenv("POSTGRES_DB"),
		POSTGRES_SSLMODE:  getEnv("POSTGRES_SSLMODE", "disable"),
	}

	timeout, _ := time.ParseDuration(os.Getenv("APP_TIMEOUT"))

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
	backfillBatchSize, _ := strconv.Atoi(getEnv("BACKFILL_BATCH_SIZE", "10000"))
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

	return &cfg, nil

}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
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
