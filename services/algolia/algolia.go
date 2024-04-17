package algolia

import (
	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/algolia/algoliasearch-client-go/v3/algolia/search"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

type (
	Algolia interface {
		Algolia() *search.Client
	}

	algolia struct {
		cfg config.Config
	}
)

func init() {
	// Provide dependencies during app boot process
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewAlgolia)
	})
}

func NewAlgolia(i *do.Injector) (Algolia, error) {
	algoliaCfg := &algolia{
		cfg: do.MustInvoke[config.Config](i),
	}

	return algoliaCfg, nil
}

func (a *algolia) Algolia() *search.Client {
	algoliaCfg := a.cfg.GetAlgolia()

	client := search.NewClient(algoliaCfg.AppID, algoliaCfg.APIKey)

	return client
}
