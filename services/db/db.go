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
		if !conn.Migrator().HasTable(table) {
			if err := conn.Migrator().CreateTable(table); err != nil {
				return nil, err
			}
			log.Printf("Created table for %T", table)
		} else {
			if err := conn.AutoMigrate(table); err != nil {
				return nil, err
			}
		}
	}

	// Drop legacy columns that were removed from the models
	conn.Exec("ALTER TABLE users DROP COLUMN IF EXISTS password")
	conn.Exec("ALTER TABLE users DROP COLUMN IF EXISTS email")

	// The Disputes model declares `order_id` as a uniqueIndex, but AutoMigrate will
	// not upgrade a pre-existing non-unique index in place. Without the unique
	// constraint the observer's `ON CONFLICT (order_id)` upsert fails (SQLSTATE
	// 42P10), leaving orders marked "disputed" with no dispute row. Ensure the
	// unique index exists: drop the legacy non-unique one, dedupe, then create it.
	conn.Exec("DROP INDEX IF EXISTS idx_disputes_order_id")
	conn.Exec(`DELETE FROM disputes a USING disputes b
		WHERE a.order_id = b.order_id AND a.ctid < b.ctid`)
	if err := conn.Exec(
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_disputes_order_id ON disputes (order_id)",
	).Error; err != nil {
		log.Printf("warning: could not ensure unique index on disputes.order_id: %v", err)
	}

	return &db{cfg: cfg, conn: conn}, nil
}

func (d *db) DB() *gorm.DB {
	return d.conn
}
