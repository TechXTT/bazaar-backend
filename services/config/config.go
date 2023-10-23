package config

import (
	"time"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/joeshaw/envdecode"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

type (
	Config interface {
		GetHTTP() HTTPConfig

		GetApp() AppConfig
	}

	Base struct {
		HTTP HTTPConfig
		App  AppConfig
	}

	HTTPConfig struct {
		Hostname     string        `env:"HTTP_HOSTNAME,default=localhost"`
		Port         int           `env:"HTTP_PORT,default=8000"`
		ReadTimeout  time.Duration `env:"HTTP_READ_TIMEOUT,default=5s"`
		WriteTimeout time.Duration `env:"HTTP_WRITE_TIMEOUT,default=10s"`
		IdleTimeout  time.Duration `env:"HTTP_IDLE_TIMEOUT,default=2m"`
	}

	AppConfig struct {
		Name    string        `env:"APP_NAME,default=bazaar-backend"`
		Timeout time.Duration `env:"APP_TIMEOUT,default=5s"`
	}
)

func init() {
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewConfig)
	})
}

func NewConfig(i *do.Injector) (Config, error) {
	var cfg Base
	err := envdecode.StrictDecode(&cfg)
	return &cfg, err
}

func (c *Base) GetHTTP() HTTPConfig {
	return c.HTTP
}

func (c *Base) GetApp() AppConfig {
	return c.App
}
