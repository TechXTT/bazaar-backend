package disputes

import (
	"testing"

	"github.com/gofrs/uuid/v5"
)

// TestOrderAccessibleBy covers BE-11: the dispute-read IDOR guard must admit
// only the order's buyer or the store owner, and reject any unrelated user.
func TestOrderAccessibleBy(t *testing.T) {
	buyer := uuid.Must(uuid.NewV4())
	seller := uuid.Must(uuid.NewV4())
	stranger := uuid.Must(uuid.NewV4())

	order := &Orders{BuyerID: buyer}
	order.Product.Store.OwnerID = seller

	cases := []struct {
		name      string
		requester uuid.UUID
		want      bool
	}{
		{"buyer can access", buyer, true},
		{"seller can access", seller, true},
		{"stranger cannot access", stranger, false},
		{"nil uuid cannot access", uuid.Nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := orderAccessibleBy(order, tc.requester); got != tc.want {
				t.Fatalf("orderAccessibleBy(%s) = %v, want %v", tc.requester, got, tc.want)
			}
		})
	}
}
