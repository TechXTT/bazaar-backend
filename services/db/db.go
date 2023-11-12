package db

import (
	"fmt"
	"log"

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
		cfg config.Config
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
	dbCfg := &db{
		cfg: do.MustInvoke[config.Config](i),
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=require",
			dbCfg.cfg.GetDB().POSTGRES_HOST,
			dbCfg.cfg.GetDB().POSTGRES_PORT,
			dbCfg.cfg.GetDB().POSTGRES_USER,
			dbCfg.cfg.GetDB().POSTGRES_PASSWORD,
			dbCfg.cfg.GetDB().POSTGRES_DB,
		), PreferSimpleProtocol: true}), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	if !db.Migrator().HasTable(&Users{}) {
		db.Migrator().CreateTable(&Users{})
		log.Println("Created users table")
	}
	if !db.Migrator().HasTable(&Stores{}) {
		db.Migrator().CreateTable(&Stores{})
		log.Println("Created stores table")
	}

	return dbCfg, nil
}

func (d *db) DB() *gorm.DB {
	dbCfg := d.cfg.GetDB()

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=require",
			dbCfg.POSTGRES_HOST,
			dbCfg.POSTGRES_PORT,
			dbCfg.POSTGRES_USER,
			dbCfg.POSTGRES_PASSWORD,
			dbCfg.POSTGRES_DB,
		), PreferSimpleProtocol: true}), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	return db
}
