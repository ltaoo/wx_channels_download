// Package adapterctx provides AdapterContext, a shared dependency container
// used by platform adapters (database, logger, event bus).
package adapterctx

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/events"
)

// AdapterContext bundles the shared dependencies that platform adapters need:
// database access, structured logging, an event bus, and the absolute download root.
type AdapterContext struct {
	DB       *gorm.DB
	Logger   zerolog.Logger
	Bus      *events.Bus
	BasePath string // absolute download root directory
}
