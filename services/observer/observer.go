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
		SubscribeToEvents(contractAddress common.Address, logs chan<- types.Log, contractABI abi.ABI) (ethereum.Subscription, error)
		RunSubscription(contractABIPath string) error
		RunBackfill(contractABIPath string, fromBlock uint64) error
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

func (o *observer) SubscribeToEvents(contractAddress common.Address, logs chan<- types.Log, contractABI abi.ABI) (ethereum.Subscription, error) {
	eventNames := []string{
		"OrderCreated", "OrderCompleted", "OrderReleased", "OrderRefunded",
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

	return o.wsClient.SubscribeFilterLogs(context.Background(), query, logs)
}

func (o *observer) RunSubscription(contractABIPath string) error {
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

	subscription, err := o.SubscribeToEvents(contractAddress, logs, contractABI)
	if err != nil {
		log.Println("Error subscribing to events")
		return err
	}
	defer subscription.Unsubscribe()

	for {
		select {
		case err := <-subscription.Err():
			log.Println("observer subscription dropped:", err)
			return err
		case vLog := <-logs:
			o.handleLog(vLog, contractABI)
		}
	}
}

func (o *observer) handleLog(vLog types.Log, contractABI abi.ABI) {
	topic := vLog.Topics[0].Hex()

	switch {
	case topic == contractABI.Events["OrderCreated"].ID.Hex():
		o.handleOrderCreated(vLog, contractABI)
	case topic == contractABI.Events["OrderCompleted"].ID.Hex():
		o.handleOrderCompleted(vLog, contractABI)
	case topic == contractABI.Events["OrderReleased"].ID.Hex():
		o.handleOrderReleased(vLog, contractABI)
	case topic == contractABI.Events["OrderRefunded"].ID.Hex():
		o.handleOrderRefunded(vLog, contractABI)
	case topic == contractABI.Events["DisputeRaised"].ID.Hex():
		o.handleDisputeRaised(vLog, contractABI)
	case topic == contractABI.Events["DisputeResolved"].ID.Hex():
		o.handleDisputeResolved(vLog, contractABI)
	case topic == contractABI.Events["Ruling"].ID.Hex():
		o.handleRuling(vLog, contractABI)
	case topic == contractABI.Events["Evidence"].ID.Hex():
		o.handleEvidence(vLog, contractABI)
	case topic == contractABI.Events["MetaEvidence"].ID.Hex():
		o.handleMetaEvidence(vLog, contractABI)
	default:
		log.Println("observer: unknown event topic", topic)
	}
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
		gormDB.Model(&db.Orders{}).Where("contract_order_id = ?", orderId).Updates(updates)
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
	copy(orderIdBytes[:], vLog.Topics[1].Bytes())
	orderId, err := bytes32ToUUID(orderIdBytes)
	if err != nil {
		log.Println("observer: invalid orderId in OrderCompleted:", err)
		return
	}

	gormDB := o.db.DB()
	gormDB.Model(&db.Orders{}).
		Where("contract_order_id = ?", orderId).
		Updates(map[string]interface{}{
			"status":  db.OrderStatusCompleted,
			"tx_hash": vLog.TxHash.Hex(),
			"fee":     ev.Fee.String(),
		})

	o.upsertReputation(o.sellerWalletForContractOrder(orderId))
}

func (o *observer) handleOrderReleased(vLog types.Log, contractABI abi.ABI) {
	var orderIdBytes [32]uint8
	copy(orderIdBytes[:], vLog.Topics[1].Bytes())
	orderId, err := bytes32ToUUID(orderIdBytes)
	if err != nil {
		log.Println("observer: invalid orderId in OrderReleased:", err)
		return
	}

	gormDB := o.db.DB()
	gormDB.Model(&db.Orders{}).
		Where("contract_order_id = ?", orderId).
		Updates(map[string]interface{}{
			"status":  db.OrderStatusReleased,
			"tx_hash": vLog.TxHash.Hex(),
		})
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
	copy(orderIdBytes[:], vLog.Topics[1].Bytes())
	orderId, err := bytes32ToUUID(orderIdBytes)
	if err != nil {
		log.Println("observer: invalid orderId in OrderRefunded:", err)
		return
	}

	gormDB := o.db.DB()
	gormDB.Model(&db.Orders{}).
		Where("contract_order_id = ?", orderId).
		Updates(map[string]interface{}{
			"status":  db.OrderStatusCancelled,
			"tx_hash": vLog.TxHash.Hex(),
		})

	o.upsertReputation(o.sellerWalletForContractOrder(orderId))
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
	copy(orderIdBytes[:], vLog.Topics[1].Bytes())
	orderId, err := bytes32ToUUID(orderIdBytes)
	if err != nil {
		log.Println("observer: invalid orderId in DisputeRaised:", err)
		return
	}

	gormDB := o.db.DB()

	// Update order status to disputed
	gormDB.Model(&db.Orders{}).
		Where("contract_order_id = ?", orderId).
		Update("status", db.OrderStatusDisputed)

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
	gormDB.Model(&db.Disputes{}).
		Where("order_id = ?", order.ID).
		Updates(updates)

	o.upsertReputation(o.sellerWalletForContractOrder(orderId))
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

	gormDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tx_hash"}, {Name: "log_index"}},
		DoNothing: true,
	}).Create(&evidence)
}

func (o *observer) RunBackfill(contractABIPath string, fromBlock uint64) error {
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

	latest, err := ethClient.BlockNumber(context.Background())
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
		end := start + batchSize - 1
		if end > latest {
			end = latest
		}

		logs, err := ethClient.FilterLogs(context.Background(), ethereum.FilterQuery{
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
	o.db.DB().Raw(`
		SELECT u.wallet_address
		FROM orders o
		JOIN products pr ON o.product_id = pr.id
		JOIN stores s ON pr.store_id = s.id
		JOIN users u ON s.owner_id = u.id
		WHERE o.contract_order_id = ?
		LIMIT 1
	`, contractOrderID).Scan(&result)
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

	gormDB.Raw(`
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

	gormDB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "seller_wallet"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"total_orders", "completed_orders", "cancelled_orders",
			"dispute_count", "dispute_seller_won", "dispute_buyer_won", "score", "updated_at",
		}),
	}).Create(&rep)
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
	gormDB.Model(&db.Orders{}).
		Where("contract_order_id = ?", orderId).
		Update("meta_evidence_uri", ev.Evidence)
}
