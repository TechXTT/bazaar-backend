package observer

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/wsclient"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

type (
	// Observer is the observer service interface
	Observer interface {
		// SubscribeToEvents subscribes to events
		SubscribeToEvents(contractAddress common.Address, logs chan<- types.Log, contractABI abi.ABI) (ethereum.Subscription, error)
		// UpdateOrderStatus updates the order status
		UpdateOrderStatus(orderID string, status string) error
		// RunSubscription runs the subscription
		RunSubscription(contractABIPath string) error
	}

	observer struct {
		cfg      config.Config
		wsClient ethclient.Client
		db       db.DB
	}
)

func init() {
	// Provide dependencies during app boot process
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewObserver)
	})
}

func NewObserver(i *do.Injector) (Observer, error) {
	wsClient := do.MustInvoke[wsclient.WsClient](i)
	db := do.MustInvoke[db.DB](i)
	cfg := do.MustInvoke[config.Config](i)
	return &observer{
		wsClient: *wsClient.InitEthClient(),
		db:       db,
		cfg:      cfg,
	}, nil
}

func (o *observer) SubscribeToEvents(contractAddress common.Address, logs chan<- types.Log, contractABI abi.ABI) (ethereum.Subscription, error) {

	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddress},
		Topics:    [][]common.Hash{{contractABI.Events["OrderCompleted"].ID, contractABI.Events["OrderRefunded"].ID, contractABI.Events["OrderReleased"].ID}},
	}

	subscription, err := o.wsClient.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		return nil, err
	}

	return subscription, nil
}

func (o *observer) UpdateOrderStatus(orderID string, status string) error {
	db := o.db.DB()

	db.Exec("UPDATE orders SET status = $1 WHERE id = $2", status, orderID)

	return nil
}

func (o *observer) RunSubscription(contractABIPath string) error {

	logs := make(chan types.Log)
	contractAddress := common.HexToAddress(o.cfg.GetWs().ContractAddress)

	fileBytes, err := os.ReadFile(contractABIPath)
	if err != nil {
		log.Println("Error reading contract ABI file")
		return err
	}

	fileContents := string(fileBytes)

	contractABI, err := abi.JSON(strings.NewReader(fileContents))
	if err != nil {
		log.Println("Error parsing contract ABI")
		return err
	}

	subscription, err := o.SubscribeToEvents(contractAddress, logs, contractABI)
	if err != nil {
		log.Println("Error subscribing to events")
		return err
	}
	defer subscription.Unsubscribe()

	for {
		select {
		case err := <-subscription.Err():
			log.Println("Subscription error", err)
			return err
		case vLog := <-logs:

			eventMap := make(map[string]interface{})
			err := contractABI.UnpackIntoMap(eventMap, "OrderCompleted", vLog.Data)
			if err != nil {
				log.Println("Error unpacking event", err)
				return err
			}

			data := eventMap["orderId"].([32]uint8)

			orderId := string(data[:])
			formattedOrderId := strings.Join([]string{orderId[:8], orderId[8:12], orderId[12:16], orderId[16:20], orderId[20:]}, "-")

			switch vLog.Topics[0].Hex() {
			case contractABI.Events["OrderCompleted"].ID.Hex():
				err := o.UpdateOrderStatus(formattedOrderId, "completed")
				if err != nil {
					log.Println("Error updating order status")
					return err
				}
			case contractABI.Events["OrderRefunded"].ID.Hex():
				err := o.UpdateOrderStatus(formattedOrderId, "cancelled")
				if err != nil {
					log.Println("Error updating order status")
					return err
				}
			case contractABI.Events["OrderReleased"].ID.Hex():
				err := o.UpdateOrderStatus(formattedOrderId, "released")
				if err != nil {
					log.Println("Error updating order status")
					return err
				}
			}
		}
	}
}
