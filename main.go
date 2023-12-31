package main

import (
	"github.com/samber/do"

	"github.com/TechXTT/bazaar-backend/pkg/app"

	// Services
	_ "github.com/TechXTT/bazaar-backend/services/cache"
	_ "github.com/TechXTT/bazaar-backend/services/config"
	"github.com/TechXTT/bazaar-backend/services/web"
)

func main() {
	i := app.Boot()

	server := do.MustInvoke[web.Web](i)
	err := server.Start()
	if err != nil {
		panic(err)
	}
}
