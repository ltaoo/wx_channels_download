package hub

import (
	_ "embed"
)

//go:embed index.js
var hub_worker_javascript string

// HubWorkerJavaScript returns the native JavaScript module deployed by `deploy hub`.
func HubWorkerJavaScript() string {
	return hub_worker_javascript
}
