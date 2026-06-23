package products

import (
	"testing"

	"github.com/gofrs/uuid/v5"
)

// TestCanAccessOrder covers BE-11: the GetOrder IDOR guard must admit only the
// buyer or the store owner, and reject any unrelated user.
func TestCanAccessOrder(t *testing.T) {
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
			if got := canAccessOrder(order, tc.requester); got != tc.want {
				t.Fatalf("canAccessOrder(%s) = %v, want %v", tc.requester, got, tc.want)
			}
		})
	}
}
