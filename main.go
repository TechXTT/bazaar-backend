package main

import (
	"github.com/samber/do"

	"github.com/TechXTT/bazaar-backend/pkg/app"

	// Services
	_ "github.com/TechXTT/bazaar-backend/services/config"
	"github.com/TechXTT/bazaar-backend/services/web"
	_ "github.com/joho/godotenv/autoload"

	// Modules
	_ "github.com/TechXTT/bazaar-backend/modules/stores"
	_ "github.com/TechXTT/bazaar-backend/modules/users"
)

func main() {
	i := app.Boot()

	server := do.MustInvoke[web.Web](i)
	err := server.Start()
	if err != nil {
		panic(err)
	}
}
