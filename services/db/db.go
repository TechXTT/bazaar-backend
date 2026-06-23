package db

import (
	"fmt"
	"log"
	"time"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type (
	// DB provides a database backend
	DB interface {
		DB() *gorm.DB
	}

	db struct {
		cfg  config.Config
		conn *gorm.DB
	}
)

var HookBuildRouter = hooks.NewHook[*gorm.DB]("db.build")

func init() {
	// Provide dependencies during app boot process
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewDB)
	})
}

func NewDB(i *do.Injector) (DB, error) {
	cfg := do.MustInvoke[config.Config](i)
	dbCfg := cfg.GetDB()

	conn, err := gorm.Open(postgres.New(postgres.Config{
		DSN: fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			dbCfg.POSTGRES_HOST,
			dbCfg.POSTGRES_PORT,
			dbCfg.POSTGRES_USER,
			dbCfg.POSTGRES_PASSWORD,
			dbCfg.POSTGRES_DB,
			dbCfg.POSTGRES_SSLMODE,
		),
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := conn.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	// BE-14: serialize schema migration across replicas with a transaction-level
	// Postgres advisory lock. Previously every booting instance ran AutoMigrate
	// plus ad-hoc DDL concurrently with no version table or lock, racing on
	// multi-replica deploys. The lock ensures exactly one instance migrates at a
	// time; the rest block until it commits, then no-op.
	if err := runMigrations(conn); err != nil {
		return nil, err
	}

	return &db{cfg: cfg, conn: conn}, nil
}

// migrationAdvisoryLockKey is an arbitrary, application-specific constant used
// for pg_advisory_xact_lock so only this app's migrations contend on it.
const migrationAdvisoryLockKey int64 = 0x42415A4152 // "BAZAR"

func runMigrations(conn *gorm.DB) error {
	return conn.Transaction(func(tx *gorm.DB) error {
		// Block until we hold the migration lock; released automatically on commit.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", migrationAdvisoryLockKey).Error; err != nil {
			return fmt.Errorf("acquire migration lock: %w", err)
		}

		tables := []interface{}{
			&Users{},
			&Stores{},
			&Products{},
			&Orders{},
			&Disputes{},
			&DisputeEvidence{},
			&SellerReputation{},
		}

		for _, table := range tables {
			if !tx.Migrator().HasTable(table) {
				if err := tx.Migrator().CreateTable(table); err != nil {
					return err
				}
				log.Printf("Created table for %T", table)
			} else {
				if err := tx.AutoMigrate(table); err != nil {
					return err
				}
			}
		}

		// Drop legacy columns that were removed from the models. These are
		// idempotent and non-destructive (the columns no longer exist in any
		// model).
		if err := tx.Exec("ALTER TABLE users DROP COLUMN IF EXISTS password").Error; err != nil {
			return err
		}
		if err := tx.Exec("ALTER TABLE users DROP COLUMN IF EXISTS email").Error; err != nil {
			return err
		}

		// The Disputes model declares `order_id` as a uniqueIndex, but AutoMigrate
		// will not upgrade a pre-existing non-unique index in place. The observer's
		// `ON CONFLICT (order_id)` upsert needs the unique constraint. Create it
		// idempotently. We intentionally do NOT run an unconditional
		// `DELETE FROM disputes` (the previous boot-time destructive statement);
		// instead, only dedupe when duplicate order_ids actually exist and the
		// unique index is therefore impossible to create, and surface the action.
		if err := tx.Exec("DROP INDEX IF EXISTS idx_disputes_order_id").Error; err != nil {
			return err
		}
		if err := tx.Exec(
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_disputes_order_id ON disputes (order_id)",
		).Error; err != nil {
			// Creation failed (most likely pre-existing duplicate order_ids). Dedupe
			// the offending rows — keeping the newest per order_id — then retry.
			log.Printf("BE-14: unique index on disputes.order_id failed (%v); deduping duplicate order_ids", err)
			if derr := tx.Exec(`DELETE FROM disputes a USING disputes b
				WHERE a.order_id = b.order_id AND a.ctid < b.ctid`).Error; derr != nil {
				return fmt.Errorf("dedupe disputes: %w", derr)
			}
			if cerr := tx.Exec(
				"CREATE UNIQUE INDEX IF NOT EXISTS idx_disputes_order_id ON disputes (order_id)",
			).Error; cerr != nil {
				return fmt.Errorf("create unique index on disputes.order_id after dedupe: %w", cerr)
			}
		}

		return nil
	})
}

func (d *db) DB() *gorm.DB {
	return d.conn
}
