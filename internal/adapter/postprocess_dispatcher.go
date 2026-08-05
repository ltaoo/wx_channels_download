package adapter

import (
	"context"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/pkg/hermes"
)

// PlatformPostprocessor dispatches postprocessing through the registered
// adapter instead of importing platform implementations.
type PlatformPostprocessor struct {
	deps PostprocessDeps
}

func NewPlatformPostprocessor(db *gorm.DB, logger zerolog.Logger, basePath string) *PlatformPostprocessor {
	return &PlatformPostprocessor{deps: PostprocessDeps{DB: db, Logger: logger, BasePath: basePath}}
}

// Process implements hermes.Postprocessor.
func (pp *PlatformPostprocessor) Process(ctx context.Context, info *hermes.TaskJob) error {
	platform := info.Platform
	pp.deps.Logger.Info().
		Int("task_id", info.ID).
		Str("platform", platform).
		Int("resource_count", len(info.Resources)).
		Msg("starting platform postprocessing")
	handler := Get(platform)
	if handler == nil {
		pp.deps.Logger.Info().
			Int("task_id", info.ID).
			Str("platform", platform).
			Msg("no postprocessing adapter registered, skipping")
		return nil
	}
	postprocessor, ok := handler.(Postprocessor)
	if !ok {
		return nil
	}
	return postprocessor.Postprocess(ctx, info, pp.deps)
}
