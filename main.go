package main

import (
	"log"
	"time"

	"github.com/samber/do"

	"github.com/TechXTT/bazaar-backend/pkg/app"

	// Services
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/TechXTT/bazaar-backend/services/observer"
	"github.com/TechXTT/bazaar-backend/services/web"
	_ "github.com/joho/godotenv/autoload"

	// Modules
	_ "github.com/TechXTT/bazaar-backend/modules/disputes"
	_ "github.com/TechXTT/bazaar-backend/modules/products"
	_ "github.com/TechXTT/bazaar-backend/modules/stores"
	_ "github.com/TechXTT/bazaar-backend/modules/users"
)

func main() {
	i := app.Boot()

	cfg := do.MustInvoke[config.Config](i)
	ob := do.MustInvoke[observer.Observer](i)

	if cfg.GetWs().BackfillOnStartup {
		go func() {
			if err := ob.RunBackfill("./Escrow.json", cfg.GetWs().BackfillFromBlock); err != nil {
				log.Printf("backfill failed: %v", err)
			}
		}()
	}

	// Start the observer
	go func() {
		backoff := time.Second
		for {
			startedAt := time.Now()
			if err := ob.RunSubscription("./Escrow.json"); err != nil {
				log.Printf("observer exited (%v), reconnecting in %s", err, backoff)
			}
			time.Sleep(backoff)
			if time.Since(startedAt) > 30*time.Second {
				backoff = time.Second
				continue
			}
			backoff *= 2
			if backoff > 5*time.Minute {
				backoff = 5 * time.Minute
			}
		}
	}()

	server := do.MustInvoke[web.Web](i)
	err := server.Start()
	if err != nil {
		panic(err)
	}
}
