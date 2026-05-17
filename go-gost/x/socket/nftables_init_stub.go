//go:build !linux

package socket

func (w *WebSocketReporter) initNftablesManager() {
	// No-op on non-Linux platforms
}
