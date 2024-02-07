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
	}

	AppConfig struct {
		Name    string
		Timeout time.Duration
	}

	JWTConfig struct {
		JwksUri    string
		PrivateKey string
		PublicKey  string
	}

	WsConfig struct {
		ETH_URL         string
		ContractAddress string
	}
  
  S3SpacesConfig struct {
		SpacesKey    string
		SpacesSecret string
		SpacesName   string
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
	allowCredentials, _ := strconv.ParseBool(os.Getenv("HTTP_ALLOW_CREDENTIALS"))

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
	}

	timeout, _ := time.ParseDuration(os.Getenv("APP_TIMEOUT"))

	cfg.App = AppConfig{
		Name:    os.Getenv("APP_NAME"),
		Timeout: timeout,
	}

	cfg.JWT = JWTConfig{
		JwksUri:    os.Getenv("JWKS_URI"),
		PrivateKey: os.Getenv("PRIVATE_KEY"),
		PublicKey:  os.Getenv("PUBLIC_KEY"),
	}
  
  cfg.Ws = WsConfig{
		ETH_URL:         os.Getenv("ETH_URL"),
		ContractAddress: os.Getenv("CONTRACT_ADDRESS"),
  }
  
	cfg.S3Spaces = S3SpacesConfig{
		SpacesKey:    os.Getenv("SPACES_KEY"),
		SpacesSecret: os.Getenv("SPACES_SECRET"),
		SpacesName:   os.Getenv("SPACES_NAME"),
	}

	return &cfg, nil

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

