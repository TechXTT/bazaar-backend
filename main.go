package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/samber/do"

	"github.com/TechXTT/bazaar-backend/pkg/app"

	// Services
	"github.com/TechXTT/bazaar-backend/services/algolia"
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/TechXTT/bazaar-backend/services/observer"
	"github.com/TechXTT/bazaar-backend/services/web"
	_ "github.com/joho/godotenv/autoload"

	// Modules
	_ "github.com/TechXTT/bazaar-backend/modules/disputes"
	"github.com/TechXTT/bazaar-backend/modules/products"
	_ "github.com/TechXTT/bazaar-backend/modules/stores"
	_ "github.com/TechXTT/bazaar-backend/modules/users"
)

func main() {
	// BE-3: cancel the root context on SIGINT/SIGTERM so the HTTP server,
	// observer subscription, and backfill all unwind gracefully on deploy/dyno
	// cycling instead of having in-flight work killed.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	i := app.Boot()

	cfg := do.MustInvoke[config.Config](i)
	ob := do.MustInvoke[observer.Observer](i)

	if cfg.GetAlgolia().SeedOnStartup {
		go func() {
			svc := do.MustInvoke[products.Service](i)
			algoliaSvc := do.MustInvoke[algolia.AlgoliaService](i)
			all, err := svc.GetProducts()
			if err != nil {
				log.Printf("algolia seed: GetProducts: %v", err)
				return
			}
			records := make([]algolia.ProductRecord, len(all))
			for j, pr := range all {
				records[j] = algolia.ProductRecord{
					ObjectID:    pr.ID.String(),
					Name:        pr.Name,
					Description: pr.Description,
					Price:       pr.Price,
					Unit:        pr.Unit,
					ImageURL:    pr.ImageURL,
					StoreID:     pr.StoreID.String(),
					StoreName:   pr.Store.Name,
					CreatedAt:   pr.CreatedAt,
				}
			}
			if err := algoliaSvc.BulkIndex(records); err != nil {
				log.Printf("algolia seed: BulkIndex: %v", err)
				return
			}
			log.Printf("algolia: seeded %d products", len(records))
		}()
	}

	if cfg.GetWs().BackfillOnStartup {
		go func() {
			if err := ob.RunBackfill(ctx, "./Escrow.json", cfg.GetWs().BackfillFromBlock); err != nil {
				log.Printf("backfill failed: %v", err)
			}
		}()
	}

	// Start the observer
	go func() {
		backoff := time.Second
		for {
			if ctx.Err() != nil {
				return
			}
			startedAt := time.Now()
			if err := ob.RunSubscription(ctx, "./Escrow.json"); err != nil {
				log.Printf("observer exited (%v), reconnecting in %s", err, backoff)
			}
			// Stop reconnecting once the process is shutting down.
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
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
	if err := server.Start(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
	log.Println("shutdown complete")
}
