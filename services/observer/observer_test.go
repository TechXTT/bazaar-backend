package observer

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// TestHandleLog_MalformedLogsDoNotPanic covers BE-2: an anonymous/malformed log
// (no topics, or an unknown signature) must not panic the observer goroutine.
func TestHandleLog_MalformedLogsDoNotPanic(t *testing.T) {
	o := &observer{}
	contractABI := abi.ABI{}

	cases := []struct {
		name string
		log  types.Log
	}{
		{name: "no topics", log: types.Log{}},
		{name: "single unknown topic", log: types.Log{Topics: []common.Hash{common.HexToHash("0xdeadbeef")}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("handleLog panicked on %s: %v", tc.name, r)
				}
			}()
			o.handleLog(tc.log, contractABI)
		})
	}
}

// TestOrderStatusForRuling covers BE-9: orders must leave `disputed` on
// resolution, mapping the ruling to a terminal order status that matches the
// escrow contract's settlement.
func TestOrderStatusForRuling(t *testing.T) {
	cases := []struct {
		name   string
		ruling uint8
		want   string
	}{
		{"buyer wins refunds order", 1, "cancelled"},
		{"receiver wins releases order", 2, "released"},
		{"refused settles to receiver", 0, "released"},
		{"unknown ruling settles to receiver", 99, "released"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(orderStatusForRuling(tc.ruling)); got != tc.want {
				t.Fatalf("orderStatusForRuling(%d) = %q, want %q", tc.ruling, got, tc.want)
			}
		})
	}
}

// TestBytes32ToUUID covers BE-11: the 32-hex-char packed form and the 36-char
// dashed form both round-trip, and bad lengths error.
func TestBytes32ToUUID(t *testing.T) {
	// A UUID with dashes stripped is 32 lowercase hex chars; the contract packs
	// those 32 ASCII bytes into bytes32.
	const dashed = "0123456789abcdef0123456789abcdef"
	var packed [32]uint8
	copy(packed[:], dashed)

	got, err := bytes32ToUUID(packed)
	if err != nil {
		t.Fatalf("bytes32ToUUID(packed): %v", err)
	}
	want := "01234567-89ab-cdef-0123-456789abcdef"
	if got != want {
		t.Fatalf("packed: got %q want %q", got, want)
	}

	// Round-trip a real UUID: strip dashes, pack into bytes32, convert back.
	const realUUID = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	var packed2 [32]uint8
	copy(packed2[:], strings.ReplaceAll(realUUID, "-", ""))
	rt, err := bytes32ToUUID(packed2)
	if err != nil {
		t.Fatalf("round-trip: %v", err)
	}
	if rt != realUUID {
		t.Fatalf("round-trip: got %q want %q", rt, realUUID)
	}

	// Too-short content errors.
	var short [32]uint8
	copy(short[:], "abc")
	if _, err := bytes32ToUUID(short); err == nil {
		t.Fatal("expected error for unexpected length")
	}
}

// TestComputeScore covers BE-11 edge cases for the reputation score.
func TestComputeScore(t *testing.T) {
	deref := func(p *int) int {
		if p == nil {
			t.Fatal("expected non-nil score")
		}
		return *p
	}

	if computeScore(0, 0, 0) != nil {
		t.Fatal("zero total must yield nil score")
	}
	if got := deref(computeScore(10, 10, 0)); got != 80 {
		t.Fatalf("all completed, no disputes: got %d want 80", got)
	}
	if got := deref(computeScore(0, 10, 10)); got != 0 {
		t.Fatalf("none completed, all disputed: got %d want 0 (clamped)", got)
	}
	// Score is clamped to [0,100].
	s := deref(computeScore(10, 10, 10))
	if s < 0 || s > 100 {
		t.Fatalf("score out of range: %d", s)
	}
}
