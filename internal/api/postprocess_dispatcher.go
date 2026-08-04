package api

import (
	"context"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/download/registry"
	"wx_channel/pkg/hermes"
)

// PlatformPostprocessor dispatches postprocessing through the registered
// adapter instead of importing platform implementations.
type PlatformPostprocessor struct {
	deps registry.PostprocessDeps
}

func NewPlatformPostprocessor(db *gorm.DB, logger zerolog.Logger, basePath string) *PlatformPostprocessor {
	return &PlatformPostprocessor{deps: registry.PostprocessDeps{DB: db, Logger: logger, BasePath: basePath}}
}

// Process implements hermes.Postprocessor.
func (pp *PlatformPostprocessor) Process(ctx context.Context, info *hermes.TaskJob) error {
	platform := info.Platform
	pp.deps.Logger.Info().
		Int("task_id", info.ID).
		Str("platform", platform).
		Int("resource_count", len(info.Resources)).
		Msg("starting platform postprocessing")
	handler := registry.Get(platform)
	if handler == nil {
		pp.deps.Logger.Info().
			Int("task_id", info.ID).
			Str("platform", platform).
			Msg("no postprocessing adapter registered, skipping")
		return nil
	}
	postprocessor, ok := handler.(registry.Postprocessor)
	if !ok {
		return nil
	}
	return postprocessor.Postprocess(ctx, info, pp.deps)
}
