package wsclient

import (
	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

type (
	// Service is the wsclient service interface
	WsClient interface {
		// InitEthClient initializes the ethereum client
		InitEthClient() *ethclient.Client
	}

	wsclient struct {
		cfg config.Config
	}
)

func init() {
	// Provide dependencies during app boot process
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewWsClient)
	})
}

func NewWsClient(i *do.Injector) (WsClient, error) {
	return &wsclient{
		cfg: do.MustInvoke[config.Config](i),
	}, nil
}

func (w *wsclient) InitEthClient() *ethclient.Client {
	client, err := ethclient.Dial(w.cfg.GetWs().ETH_URL)
	if err != nil {
		panic(err)
	}

	return client
}
