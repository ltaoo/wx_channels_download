package api

import (
	"context"
	"fmt"

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
func (pp *PlatformPostprocessor) Process(ctx context.Context, info *hermes.PostprocessInfo) error {
	platform, _ := info.Metadata["platform"].(string)
	pp.deps.Logger.Info().Msg(fmt.Sprintf(
		"Postprocessor.Process: task_id=%d platform=%q resources=%d",
		info.TaskID, platform, len(info.Resources),
	))
	handler := registry.Get(platform)
	if handler == nil {
		pp.deps.Logger.Info().Msg(fmt.Sprintf(
			"Postprocessor.Process: task_id=%d no adapter for platform=%q, skipping",
			info.TaskID, platform,
		))
		return nil
	}
	postprocessor, ok := handler.(registry.Postprocessor)
	if !ok {
		return nil
	}
	return postprocessor.Postprocess(ctx, info, pp.deps)
}
