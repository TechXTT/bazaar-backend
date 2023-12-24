package config

import (
	"os"
	"strconv"
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
	}

	Base struct {
		HTTP HTTPConfig
		DB   DBConfig
		App  AppConfig
		JWT  JWTConfig
	}

	HTTPConfig struct {
		Hostname     string
		Port         int
		ReadTimeout  time.Duration
		WriteTimeout time.Duration
		IdleTimeout  time.Duration
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

	cfg.HTTP = HTTPConfig{
		Hostname:     os.Getenv("HTTP_HOSTNAME"),
		Port:         port,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
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
