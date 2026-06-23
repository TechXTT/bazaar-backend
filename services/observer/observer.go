package observer

import (
	"context"
	"encoding/hex"
	"errors"
	"log"
	"math"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/metrics"
	"github.com/TechXTT/bazaar-backend/services/wsclient"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gofrs/uuid/v5"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
	"gorm.io/gorm/clause"
)

type (
	Observer interface {
		SubscribeToEvents(ctx context.Context, contractAddress common.Address, logs chan<- types.Log, contractABI abi.ABI) (ethereum.Subscription, error)
		RunSubscription(ctx context.Context, contractABIPath string) error
		RunBackfill(ctx context.Context, contractABIPath string, fromBlock uint64) error
	}

	observer struct {
		cfg            config.Config
		wsClient       *ethclient.Client
		wsSvc          wsclient.WsClient
		db             db.DB
		pendingRulings sync.Map // txHash string → arbitratorDisputeID int64
	}
)

func init() {
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewObserver)
	})
}

func NewObserver(i *do.Injector) (Observer, error) {
	wsClient := do.MustInvoke[wsclient.WsClient](i)
	dbSvc := do.MustInvoke[db.DB](i)
	cfg := do.MustInvoke[config.Config](i)

	return &observer{
		wsSvc: wsClient,
		db:    dbSvc,
		cfg:   cfg,
	}, nil
}

func (o *observer) SubscribeToEvents(ctx context.Context, contractAddress common.Address, logs chan<- types.Log, contractABI abi.ABI) (ethereum.Subscription, error) {
	eventNames := []string{
		"OrderCreated", "OrderCompleted", "OrderReleased", "OrderRefunded",
		"OrderShipped",
		"DisputeRaised", "DisputeResolved", "Ruling", "Evidence", "MetaEvidence",
	}

	var topics []common.Hash
	for _, name := range eventNames {
		if ev, ok := contractABI.Events[name]; ok {
			topics = append(topics, ev.ID)
		}
	}

	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddress},
		Topics:    [][]common.Hash{topics},
	}

	return o.wsClient.SubscribeFilterLogs(ctx, query, logs)
}

func (o *observer) RunSubscription(ctx context.Context, contractABIPath string) error {
	logs := make(chan types.Log)
	contractAddress := common.HexToAddress(o.cfg.GetWs().ContractAddress)

	ethClient, err := o.wsSvc.InitEthClient()
	if err != nil {
		log.Println("Error connecting to Ethereum websocket")
		return err
	}
	o.wsClient = ethClient
	defer ethClient.Close()

	fileBytes, err := os.ReadFile(contractABIPath)
	if err != nil {
		log.Println("Error reading contract ABI file")
		return err
	}

	contractABI, err := abi.JSON(strings.NewReader(string(fileBytes)))
	if err != nil {
		log.Println("Error parsing contract ABI")
		return err
	}

	subscription, err := o.SubscribeToEvents(ctx, contractAddress, logs, contractABI)
	if err != nil {
		log.Println("Error subscribing to events")
		return err
	}
	defer subscription.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			// BE-3: graceful shutdown — stop consuming and unwind cleanly.
			log.Println("observer: context cancelled, shutting down subscription")
			return ctx.Err()
		case err := <-subscription.Err():
			log.Println("observer subscription dropped:", err)
			return err
		case vLog := <-logs:
			o.handleLog(vLog, contractABI)
		}
	}
}

func (o *observer) handleLog(vLog types.Log, contractABI abi.ABI) {
	// BE-2: a panic in any handler (e.g. an out-of-range Topics index from a
	// malformed/anonymous log) must not take down the observer goroutine and,
	// with it, the whole backend. Recover, log, and keep consuming events.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("observer: recovered from panic handling log (tx=%s logIndex=%d): %v",
				vLog.TxHash.Hex(), vLog.Index, r)
		}
	}()

	// An anonymous or malformed log can have no topics; Topics[0] is the event
	// signature, so without it we cannot dispatch.
	if len(vLog.Topics) == 0 {
		log.Printf("observer: skipping log with no topics (tx=%s logIndex=%d)",
			vLog.TxHash.Hex(), vLog.Index)
		return
	}

	topic := vLog.Topics[0].Hex()

	switch {
	case topic == contractABI.Events["OrderCreated"].ID.Hex():
		o.handleOrderCreated(vLog, contractABI)
		metrics.EventProcessed("OrderCreated")
	case topic == contractABI.Events["OrderCompleted"].ID.Hex():
		o.handleOrderCompleted(vLog, contractABI)
		metrics.EventProcessed("OrderCompleted")
	case topic == contractABI.Events["OrderReleased"].ID.Hex():
		o.handleOrderReleased(vLog, contractABI)
		metrics.EventProcessed("OrderReleased")
	case topic == contractABI.Events["OrderRefunded"].ID.Hex():
		o.handleOrderRefunded(vLog, contractABI)
		metrics.EventProcessed("OrderRefunded")
	case topic == contractABI.Events["OrderShipped"].ID.Hex():
		o.handleOrderShipped(vLog, contractABI)
		metrics.EventProcessed("OrderShipped")
	case topic == contractABI.Events["DisputeRaised"].ID.Hex():
		o.handleDisputeRaised(vLog, contractABI)
		metrics.EventProcessed("DisputeRaised")
	case topic == contractABI.Events["DisputeResolved"].ID.Hex():
		o.handleDisputeResolved(vLog, contractABI)
		metrics.EventProcessed("DisputeResolved")
	case topic == contractABI.Events["Ruling"].ID.Hex():
		o.handleRuling(vLog, contractABI)
		metrics.EventProcessed("Ruling")
	case topic == contractABI.Events["Evidence"].ID.Hex():
		o.handleEvidence(vLog, contractABI)
		metrics.EventProcessed("Evidence")
	case topic == contractABI.Events["MetaEvidence"].ID.Hex():
		o.handleMetaEvidence(vLog, contractABI)
		metrics.EventProcessed("MetaEvidence")
	default:
		log.Println("observer: unknown event topic", topic)
	}
}

// logDBErr records an observer DB failure with the operation, topic, and order
// context (BE-8). The observer's whole purpose is mirroring chain events to the
// DB, so a swallowed write silently leaves order/dispute state wrong with no
// retry — surfacing the error is the minimum so it can be alerted on and
// replayed by RunBackfill.
func logDBErr(op, topic, orderID string, err error) {
	if err == nil {
		return
	}
	// BE-15: surface the failure as a Prometheus counter so swallowed mirror
	// writes can be alerted on, not just logged.
	metrics.DBError(op, topic)
	log.Printf("observer: DB error op=%s topic=%s orderId=%s: %v", op, topic, orderID, err)
}

// bytes32ToUUID converts a [32]uint8 on-chain orderId (UUID bytes packed into bytes32)
// back to a UUID string of the form xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.
func bytes32ToUUID(b [32]uint8) (string, error) {
	trimmed := strings.TrimRight(string(b[:]), "\x00")
	if len(trimmed) == 32 {
		// UUID encoded as 32 lowercase hex chars without dashes
		return strings.Join([]string{trimmed[:8], trimmed[8:12], trimmed[12:16], trimmed[16:20], trimmed[20:]}, "-"), nil
	}
	if len(trimmed) == 36 {
		return trimmed, nil
	}
	return "", errors.New("unexpected bytes32 UUID length")
}

func bytes32ToHex(b [32]uint8) string {
	return "0x" + hex.EncodeToString(b[:])
}

func tokenLabel(addr common.Address) string {
	if addr == (common.Address{}) {
		return "eth"
	}
	return "usdc"
}

func (o *observer) handleOrderCreated(vLog types.Log, contractABI abi.ABI) {
	type OrderCreated struct {
		ProductId   [32]uint8
		Receiver    common.Address
		Token       common.Address
		Amount      *big.Int
		ReleaseTime *big.Int
	}

	// indexed: orderId (topic[1]), buyer (topic[2]... may vary by ABI)
	// non-indexed fields in Data
	var ev OrderCreated
	if err := contractABI.UnpackIntoInterface(&ev, "OrderCreated", vLog.Data); err != nil {
		log.Println("observer: unpack OrderCreated:", err)
		return
	}

	// orderId is indexed — recover from topic[1]
	var orderIdBytes [32]uint8
	if len(vLog.Topics) < 2 {
		log.Printf("observer: log missing indexed orderId topic (tx=%s logIndex=%d)", vLog.TxHash.Hex(), vLog.Index)
		return
	}
	copy(orderIdBytes[:], vLog.Topics[1].Bytes())
	orderId, err := bytes32ToUUID(orderIdBytes)
	if err != nil {
		log.Println("observer: invalid orderId in OrderCreated:", err)
		return
	}

	// buyer is indexed in topic[2] if present
	var buyerAddr common.Address
	if len(vLog.Topics) > 2 {
		buyerAddr = common.BytesToAddress(vLog.Topics[2].Bytes())
	}

	gormDB := o.db.DB()
	var order db.Orders
	gormDB.Where("contract_order_id = ?", orderId).First(&order)

	// Find buyer by wallet address
	var buyer db.Users
	gormDB.Where("LOWER(wallet_address) = LOWER(?)", buyerAddr.Hex()).First(&buyer)

	now := time.Now()
	updates := map[string]interface{}{
		"status":              db.OrderStatusPending,
		"tx_hash":             vLog.TxHash.Hex(),
		"token":               tokenLabel(ev.Token),
		"on_chain_product_id": bytes32ToHex(ev.ProductId),
		"updated_at":          now,
	}

	if order.ID == uuid.Nil {
		// No matching row yet (order created on-chain before backend API call) — skip
		log.Println("observer: no backend order row for OrderCreated", orderId)
	} else {
		res := gormDB.Model(&db.Orders{}).Where("contract_order_id = ?", orderId).Updates(updates)
		logDBErr("OrderCreated.Updates", "OrderCreated", orderId, res.Error)
	}
}

func (o *observer) handleOrderCompleted(vLog types.Log, contractABI abi.ABI) {
	type OrderCompleted struct {
		ProductId [32]uint8
		Buyer     common.Address
		Amount    *big.Int
		Fee       *big.Int
	}

	var ev OrderCompleted
	if err := contractABI.UnpackIntoInterface(&ev, "OrderCompleted", vLog.Data); err != nil {
		log.Println("observer: unpack OrderCompleted:", err)
		return
	}

	var orderIdBytes [32]uint8
	if len(vLog.Topics) < 2 {
		log.Printf("observer: log missing indexed orderId topic (tx=%s logIndex=%d)", vLog.TxHash.Hex(), vLog.Index)
		return
	}
	copy(orderIdBytes[:], vLog.Topics[1].Bytes())
	orderId, err := bytes32ToUUID(orderIdBytes)
	if err != nil {
		log.Println("observer: invalid orderId in OrderCompleted:", err)
		return
	}

	gormDB := o.db.DB()

	// BE-18 groundwork: reconcile the DB order against the on-chain settlement
	// amount/fee before overwriting fee. We can only log/alert today because
	// Total is stored as a float64 in token units while Amount is an integer in
	// minor units (the float→integer migration is BE-18); an exact equality
	// check waits on that. Reading the row first is best-effort — a miss here
	// must not block the status write.
	var existing db.Orders
	if lookup := gormDB.Where("contract_order_id = ?", orderId).First(&existing); lookup.Error == nil {
		o.reconcileOrder("OrderCompleted", orderId, existing, ev.Amount, ev.Fee)
	}

	res := gormDB.Model(&db.Orders{}).
		Where("contract_order_id = ?", orderId).
		Updates(map[string]interface{}{
			"status":  db.OrderStatusCompleted,
			"tx_hash": vLog.TxHash.Hex(),
			"fee":     ev.Fee.String(),
		})
	logDBErr("OrderCompleted.Updates", "OrderCompleted", orderId, res.Error)

	o.upsertReputation(o.sellerWalletForContractOrder(orderId))
}

// reconcileOrder compares the DB order against the on-chain settlement
// amount/fee and logs a warning + increments a metric on a clear mismatch
// (BE-18 groundwork). It is intentionally conservative: because Total is a
// float64 in token units and amount/fee are integers in minor units, it only
// flags unambiguous divergences (e.g. a non-zero on-chain settlement against a
// zero/unset DB Total, or an on-chain fee that disagrees with the fee already
// recorded on the order) rather than attempting a unit-aware equality that
// belongs in BE-18.
func (o *observer) reconcileOrder(topic, orderID string, order db.Orders, amount, fee *big.Int) {
	if amount != nil && amount.Sign() > 0 && order.Total <= 0 {
		metrics.ReconcileMismatch("total", topic)
		log.Printf("observer: reconcile mismatch op=%s orderId=%s: on-chain amount=%s but DB Total=%v",
			topic, orderID, amount.String(), order.Total)
	}

	// If the order already carried a recorded fee, a different on-chain fee is a
	// divergence worth surfacing (e.g. feeBps changed between create and settle).
	if fee != nil && order.Fee != "" && order.Fee != fee.String() {
		metrics.ReconcileMismatch("fee", topic)
		log.Printf("observer: reconcile mismatch op=%s orderId=%s: on-chain fee=%s but DB fee=%s",
			topic, orderID, fee.String(), order.Fee)
	}
}

func (o *observer) handleOrderReleased(vLog types.Log, contractABI abi.ABI) {
	var orderIdBytes [32]uint8
	if len(vLog.Topics) < 2 {
		log.Printf("observer: log missing indexed orderId topic (tx=%s logIndex=%d)", vLog.TxHash.Hex(), vLog.Index)
		return
	}
	copy(orderIdBytes[:], vLog.Topics[1].Bytes())
	orderId, err := bytes32ToUUID(orderIdBytes)
	if err != nil {
		log.Println("observer: invalid orderId in OrderReleased:", err)
		return
	}

	gormDB := o.db.DB()
	res := gormDB.Model(&db.Orders{}).
		Where("contract_order_id = ?", orderId).
		Updates(map[string]interface{}{
			"status":  db.OrderStatusReleased,
			"tx_hash": vLog.TxHash.Hex(),
		})
	logDBErr("OrderReleased.Updates", "OrderReleased", orderId, res.Error)
}

func (o *observer) handleOrderRefunded(vLog types.Log, contractABI abi.ABI) {
	type OrderRefunded struct {
		ProductId [32]uint8
		Receiver  common.Address
		Amount    *big.Int
	}

	var ev OrderRefunded
	if err := contractABI.UnpackIntoInterface(&ev, "OrderRefunded", vLog.Data); err != nil {
		log.Println("observer: unpack OrderRefunded:", err)
		return
	}

	var orderIdBytes [32]uint8
	if len(vLog.Topics) < 2 {
		log.Printf("observer: log missing indexed orderId topic (tx=%s logIndex=%d)", vLog.TxHash.Hex(), vLog.Index)
		return
	}
	copy(orderIdBytes[:], vLog.Topics[1].Bytes())
	orderId, err := bytes32ToUUID(orderIdBytes)
	if err != nil {
		log.Println("observer: invalid orderId in OrderRefunded:", err)
		return
	}

	gormDB := o.db.DB()
	res := gormDB.Model(&db.Orders{}).
		Where("contract_order_id = ?", orderId).
		Updates(map[string]interface{}{
			"status":  db.OrderStatusCancelled,
			"tx_hash": vLog.TxHash.Hex(),
		})
	logDBErr("OrderRefunded.Updates", "OrderRefunded", orderId, res.Error)

	o.upsertReputation(o.sellerWalletForContractOrder(orderId))
}

func (o *observer) handleOrderShipped(vLog types.Log, contractABI abi.ABI) {
	type OrderShipped struct {
		TrackingHash     [32]uint8
		DeliveryDeadline *big.Int
	}

	// indexed: orderId (topic[1]), receiver (topic[2])
	// non-indexed fields in Data: trackingHash, deliveryDeadline
	var ev OrderShipped
	if err := contractABI.UnpackIntoInterface(&ev, "OrderShipped", vLog.Data); err != nil {
		log.Println("observer: unpack OrderShipped:", err)
		return
	}

	var orderIdBytes [32]uint8
	if len(vLog.Topics) < 2 {
		log.Printf("observer: log missing indexed orderId topic (tx=%s logIndex=%d)", vLog.TxHash.Hex(), vLog.Index)
		return
	}
	copy(orderIdBytes[:], vLog.Topics[1].Bytes())
	orderId, err := bytes32ToUUID(orderIdBytes)
	if err != nil {
		log.Println("observer: invalid orderId in OrderShipped:", err)
		return
	}

	gormDB := o.db.DB()
	if err := gormDB.Model(&db.Orders{}).
		Where("contract_order_id = ?", orderId).
		Updates(map[string]interface{}{
			"status":  db.OrderStatusShipped,
			"tx_hash": vLog.TxHash.Hex(),
		}).Error; err != nil {
		log.Println("observer: failed to update order for OrderShipped", orderId, ":", err)
	}
}

func (o *observer) handleDisputeRaised(vLog types.Log, contractABI abi.ABI) {
	type DisputeRaised struct {
		By common.Address
	}

	var ev DisputeRaised
	if err := contractABI.UnpackIntoInterface(&ev, "DisputeRaised", vLog.Data); err != nil {
		log.Println("observer: unpack DisputeRaised:", err)
		return
	}

	var orderIdBytes [32]uint8
	if len(vLog.Topics) < 2 {
		log.Printf("observer: log missing indexed orderId topic (tx=%s logIndex=%d)", vLog.TxHash.Hex(), vLog.Index)
		return
	}
	copy(orderIdBytes[:], vLog.Topics[1].Bytes())
	orderId, err := bytes32ToUUID(orderIdBytes)
	if err != nil {
		log.Println("observer: invalid orderId in DisputeRaised:", err)
		return
	}

	gormDB := o.db.DB()

	// Update order status to disputed
	res := gormDB.Model(&db.Orders{}).
		Where("contract_order_id = ?", orderId).
		Update("status", db.OrderStatusDisputed)
	logDBErr("DisputeRaised.OrderStatus", "DisputeRaised", orderId, res.Error)

	// Find order row
	var order db.Orders
	if gormDB.Where("contract_order_id = ?", orderId).First(&order).Error != nil {
		log.Println("observer: order not found for DisputeRaised", orderId)
		return
	}

	disputeID, _ := uuid.NewV4()
	dispute := db.Disputes{
		ID:       disputeID,
		OrderID:  order.ID,
		RaisedBy: strings.ToLower(ev.By.Hex()),
		Status:   db.DisputeStatusFeePending,
		TxHash:   vLog.TxHash.Hex(),
		LogIndex: uint(vLog.Index),
	}

	if err := gormDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "order_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"raised_by", "status", "tx_hash", "log_index"}),
	}).Create(&dispute).Error; err != nil {
		log.Println("observer: failed to upsert dispute for order", order.ID, ":", err)
	}
}

func (o *observer) handleDisputeResolved(vLog types.Log, contractABI abi.ABI) {
	type DisputeResolved struct {
		Ruling *big.Int
	}

	var ev DisputeResolved
	if err := contractABI.UnpackIntoInterface(&ev, "DisputeResolved", vLog.Data); err != nil {
		log.Println("observer: unpack DisputeResolved:", err)
		return
	}

	var orderIdBytes [32]uint8
	if len(vLog.Topics) < 2 {
		log.Printf("observer: log missing indexed orderId topic (tx=%s logIndex=%d)", vLog.TxHash.Hex(), vLog.Index)
		return
	}
	copy(orderIdBytes[:], vLog.Topics[1].Bytes())
	orderId, err := bytes32ToUUID(orderIdBytes)
	if err != nil {
		log.Println("observer: invalid orderId in DisputeResolved:", err)
		return
	}

	ruling := uint8(ev.Ruling.Uint64())
	gormDB := o.db.DB()

	var order db.Orders
	if gormDB.Where("contract_order_id = ?", orderId).First(&order).Error != nil {
		return
	}

	updates := map[string]interface{}{
		"ruling": ruling,
		"status": db.DisputeStatusResolved,
	}
	if val, ok := o.pendingRulings.LoadAndDelete(vLog.TxHash.Hex()); ok {
		updates["arbitrator_dispute_id"] = val
	}
	res := gormDB.Model(&db.Disputes{}).
		Where("order_id = ?", order.ID).
		Updates(updates)
	logDBErr("DisputeResolved.Updates", "DisputeResolved", orderId, res.Error)

	// BE-9: move the order out of `disputed` to a terminal state derived from the
	// ruling. The escrow's `rule` callback settles RULING_BUYER (1) by refunding
	// the buyer (order cancelled); RULING_RECEIVER (2) and any refused/invalid
	// ruling settle in favour of the receiver/seller (funds released). Without
	// this the order can read `disputed` forever and reputation undercounts
	// resolved outcomes.
	orderStatus := orderStatusForRuling(ruling)
	orderRes := gormDB.Model(&db.Orders{}).
		Where("contract_order_id = ?", orderId).
		Update("status", orderStatus)
	logDBErr("DisputeResolved.OrderStatus", "DisputeResolved", orderId, orderRes.Error)

	o.upsertReputation(o.sellerWalletForContractOrder(orderId))
}

// orderStatusForRuling maps an on-chain dispute ruling to the order's terminal
// status, matching the escrow contract: 1 (buyer wins) refunds the buyer, any
// other value (receiver wins or refused→receiver) releases funds to the seller.
func orderStatusForRuling(ruling uint8) db.OrderStatus {
	const rulingBuyer = 1
	if ruling == rulingBuyer {
		return db.OrderStatusCancelled
	}
	return db.OrderStatusReleased
}

func (o *observer) handleRuling(vLog types.Log, contractABI abi.ABI) {
	// Ruling(IArbitrator indexed _arbitrator, uint256 indexed _disputeID, uint256 _ruling)
	// topic[1]=arbitrator  topic[2]=disputeID  data=ruling
	if len(vLog.Topics) < 3 {
		return
	}
	disputeID := new(big.Int).SetBytes(vLog.Topics[2].Bytes())
	// Store by txHash so handleDisputeResolved (same tx) can pick it up
	o.pendingRulings.Store(vLog.TxHash.Hex(), disputeID.Int64())
}

func (o *observer) handleEvidence(vLog types.Log, contractABI abi.ABI) {
	type Evidence struct {
		Arbitrator      common.Address
		EvidenceGroupID *big.Int
		Party           common.Address
		Evidence        string
	}

	var ev Evidence
	if err := contractABI.UnpackIntoInterface(&ev, "Evidence", vLog.Data); err != nil {
		log.Println("observer: unpack Evidence:", err)
		return
	}

	// evidenceGroupID == uint256(orderId)
	var orderIdBytes [32]uint8
	if ev.EvidenceGroupID != nil {
		b := ev.EvidenceGroupID.Bytes()
		copy(orderIdBytes[32-len(b):], b)
	}
	orderId, err := bytes32ToUUID(orderIdBytes)
	if err != nil {
		log.Println("observer: invalid evidenceGroupID in Evidence:", err)
		return
	}

	gormDB := o.db.DB()

	var order db.Orders
	if gormDB.Where("contract_order_id = ?", orderId).First(&order).Error != nil {
		log.Println("observer: order not found for Evidence", orderId)
		return
	}

	var dispute db.Disputes
	if gormDB.Where("order_id = ?", order.ID).First(&dispute).Error != nil {
		log.Println("observer: dispute not found for Evidence", orderId)
		return
	}

	evidenceID, _ := uuid.NewV4()
	evidence := db.DisputeEvidence{
		ID:        evidenceID,
		DisputeID: dispute.ID,
		OrderID:   order.ID,
		Party:     strings.ToLower(ev.Party.Hex()),
		URI:       ev.Evidence,
		TxHash:    vLog.TxHash.Hex(),
		LogIndex:  uint(vLog.Index),
	}

	res := gormDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tx_hash"}, {Name: "log_index"}},
		DoNothing: true,
	}).Create(&evidence)
	logDBErr("Evidence.Create", "Evidence", orderId, res.Error)
}

func (o *observer) RunBackfill(ctx context.Context, contractABIPath string, fromBlock uint64) error {
	ethClient, err := o.wsSvc.InitEthClient()
	if err != nil {
		log.Println("observer backfill: failed to connect to Ethereum")
		return err
	}
	defer ethClient.Close()

	fileBytes, err := os.ReadFile(contractABIPath)
	if err != nil {
		return err
	}
	contractABI, err := abi.JSON(strings.NewReader(string(fileBytes)))
	if err != nil {
		return err
	}

	contractAddress := common.HexToAddress(o.cfg.GetWs().ContractAddress)

	latest, err := ethClient.BlockNumber(ctx)
	if err != nil {
		return err
	}

	batchSize := uint64(o.cfg.GetWs().BackfillBatchSize)
	if batchSize == 0 {
		batchSize = 10000
	}

	log.Printf("observer backfill: scanning blocks %d → %d (batch=%d)", fromBlock, latest, batchSize)

	prevClient := o.wsClient
	o.wsClient = ethClient
	defer func() { o.wsClient = prevClient }()

	for start := fromBlock; start <= latest; start += batchSize {
		// BE-3: stop the (potentially long) backfill promptly on shutdown.
		if err := ctx.Err(); err != nil {
			log.Println("observer backfill: context cancelled, stopping")
			return err
		}

		end := start + batchSize - 1
		if end > latest {
			end = latest
		}

		logs, err := ethClient.FilterLogs(ctx, ethereum.FilterQuery{
			FromBlock: new(big.Int).SetUint64(start),
			ToBlock:   new(big.Int).SetUint64(end),
			Addresses: []common.Address{contractAddress},
		})
		if err != nil {
			log.Printf("observer backfill: FilterLogs %d→%d: %v", start, end, err)
			continue
		}

		for _, vLog := range logs {
			o.handleLog(vLog, contractABI)
		}
		if len(logs) > 0 {
			log.Printf("observer backfill: processed %d logs in blocks %d→%d", len(logs), start, end)
		}
	}

	log.Println("observer backfill: complete")
	return nil
}

func (o *observer) sellerWalletForContractOrder(contractOrderID string) string {
	var result struct{ WalletAddress string }
	res := o.db.DB().Raw(`
		SELECT u.wallet_address
		FROM orders o
		JOIN products pr ON o.product_id = pr.id
		JOIN stores s ON pr.store_id = s.id
		JOIN users u ON s.owner_id = u.id
		WHERE o.contract_order_id = ?
		LIMIT 1
	`, contractOrderID).Scan(&result)
	logDBErr("sellerWalletForContractOrder.Scan", "", contractOrderID, res.Error)
	return strings.ToLower(result.WalletAddress)
}

func (o *observer) upsertReputation(sellerWallet string) {
	if sellerWallet == "" {
		return
	}
	gormDB := o.db.DB()

	var stats struct {
		TotalOrders     int
		CompletedOrders int
		CancelledOrders int
		DisputeCount    int
		SellerWon       int
		BuyerWon        int
	}

	statsRes := gormDB.Raw(`
		SELECT
			COUNT(DISTINCT o.id) AS total_orders,
			COUNT(DISTINCT o.id) FILTER (WHERE o.status = 'completed') AS completed_orders,
			COUNT(DISTINCT o.id) FILTER (WHERE o.status = 'cancelled') AS cancelled_orders,
			COUNT(DISTINCT d.id) AS dispute_count,
			COUNT(DISTINCT d.id) FILTER (WHERE d.ruling = 2) AS seller_won,
			COUNT(DISTINCT d.id) FILTER (WHERE d.ruling = 1) AS buyer_won
		FROM orders o
		JOIN products pr ON o.product_id = pr.id
		JOIN stores s ON pr.store_id = s.id
		JOIN users u ON s.owner_id = u.id
		LEFT JOIN disputes d ON d.order_id = o.id AND d.status = 'resolved'
		WHERE LOWER(u.wallet_address) = LOWER(?)
	`, sellerWallet).Scan(&stats)
	if statsRes.Error != nil {
		logDBErr("upsertReputation.StatsScan", "", "", statsRes.Error)
		return
	}

	score := computeScore(stats.CompletedOrders, stats.TotalOrders, stats.DisputeCount)
	repID, _ := uuid.NewV4()

	rep := db.SellerReputation{
		ID:               repID,
		SellerWallet:     strings.ToLower(sellerWallet),
		TotalOrders:      stats.TotalOrders,
		CompletedOrders:  stats.CompletedOrders,
		CancelledOrders:  stats.CancelledOrders,
		DisputeCount:     stats.DisputeCount,
		DisputeSellerWon: stats.SellerWon,
		DisputeBuyerWon:  stats.BuyerWon,
		Score:            score,
	}

	repRes := gormDB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "seller_wallet"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"total_orders", "completed_orders", "cancelled_orders",
			"dispute_count", "dispute_seller_won", "dispute_buyer_won", "score", "updated_at",
		}),
	}).Create(&rep)
	logDBErr("upsertReputation.Create", "", sellerWallet, repRes.Error)
}

func computeScore(completed, total, disputes int) *int {
	if total == 0 {
		return nil
	}
	c := float64(completed) / float64(total)
	d := float64(disputes) / float64(total)
	raw := int(math.Round(c*80 - d*40))
	if raw < 0 {
		raw = 0
	}
	if raw > 100 {
		raw = 100
	}
	return &raw
}

func (o *observer) handleMetaEvidence(vLog types.Log, contractABI abi.ABI) {
	type MetaEvidence struct {
		MetaEvidenceID *big.Int
		Evidence       string
	}

	var ev MetaEvidence
	if err := contractABI.UnpackIntoInterface(&ev, "MetaEvidence", vLog.Data); err != nil {
		log.Println("observer: unpack MetaEvidence:", err)
		return
	}

	if ev.MetaEvidenceID == nil {
		return
	}

	var orderIdBytes [32]uint8
	b := ev.MetaEvidenceID.Bytes()
	copy(orderIdBytes[32-len(b):], b)
	orderId, err := bytes32ToUUID(orderIdBytes)
	if err != nil {
		log.Println("observer: invalid metaEvidenceID in MetaEvidence:", err)
		return
	}

	gormDB := o.db.DB()
	res := gormDB.Model(&db.Orders{}).
		Where("contract_order_id = ?", orderId).
		Update("meta_evidence_uri", ev.Evidence)
	logDBErr("MetaEvidence.Update", "MetaEvidence", orderId, res.Error)
}
