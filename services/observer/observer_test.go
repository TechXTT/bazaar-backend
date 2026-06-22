package observer

import (
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
