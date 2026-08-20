package bridge

import (
	_ "embed"
)

//go:embed index.js
var bridge_worker_javascript string

// BridgeWorkerJavaScript returns the native JavaScript module deployed by `deploy bridge`.
func BridgeWorkerJavaScript() string {
	return bridge_worker_javascript
}
