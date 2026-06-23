package observer

import (
	"math/big"
	"testing"

	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestReconcileOrder covers the BE-18 reconciliation groundwork: a clear
// DB/on-chain divergence increments the mismatch metric, while a consistent
// order does not.
func TestReconcileOrder(t *testing.T) {
	o := &observer{}

	cases := []struct {
		name        string
		order       db.Orders
		amount      *big.Int
		fee         *big.Int
		wantTotal   float64 // expected increment of the "total" mismatch counter
		wantFeeIncr float64 // expected increment of the "fee" mismatch counter
	}{
		{
			name:      "zero DB total against non-zero on-chain amount",
			order:     db.Orders{Total: 0, Fee: ""},
			amount:    big.NewInt(1000),
			fee:       big.NewInt(10),
			wantTotal: 1,
		},
		{
			name:        "recorded fee disagrees with on-chain fee",
			order:       db.Orders{Total: 5, Fee: "10"},
			amount:      big.NewInt(1000),
			fee:         big.NewInt(25),
			wantFeeIncr: 1,
		},
		{
			name:   "consistent order, no mismatch",
			order:  db.Orders{Total: 5, Fee: "10"},
			amount: big.NewInt(1000),
			fee:    big.NewInt(10),
		},
		{
			name:   "nil amount/fee are ignored",
			order:  db.Orders{Total: 0, Fee: ""},
			amount: nil,
			fee:    nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			beforeTotal := testutil.ToFloat64(metrics.ObserverReconcileMismatchTotal.WithLabelValues("total", "OrderCompleted"))
			beforeFee := testutil.ToFloat64(metrics.ObserverReconcileMismatchTotal.WithLabelValues("fee", "OrderCompleted"))

			o.reconcileOrder("OrderCompleted", "order-1", tc.order, tc.amount, tc.fee)

			afterTotal := testutil.ToFloat64(metrics.ObserverReconcileMismatchTotal.WithLabelValues("total", "OrderCompleted"))
			afterFee := testutil.ToFloat64(metrics.ObserverReconcileMismatchTotal.WithLabelValues("fee", "OrderCompleted"))

			if got := afterTotal - beforeTotal; got != tc.wantTotal {
				t.Fatalf("total-mismatch delta: got %v want %v", got, tc.wantTotal)
			}
			if got := afterFee - beforeFee; got != tc.wantFeeIncr {
				t.Fatalf("fee-mismatch delta: got %v want %v", got, tc.wantFeeIncr)
			}
		})
	}
}
