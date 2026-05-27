//go:build linux

package socket

import (
	"fmt"

	"github.com/go-gost/x/nftables"
)

func (w *WebSocketReporter) initNftablesManager() {
	if supported, _ := nftables.CheckNftablesSupport(); supported {
		mgr, err := nftables.NewManager()
		if err != nil {
			fmt.Printf("nftables manager initialization failed: %v\n", err)
			return
		}
		w.nftablesMgr = mgr
		fmt.Println("nftables manager initialized successfully")
	}
}
