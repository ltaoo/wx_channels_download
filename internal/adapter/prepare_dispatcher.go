package adapter

import (
	"context"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
	"wx_channel/pkg/hermes"
)

type PlatformPreprocessor struct{ deps PostprocessDeps }

func NewPlatformPreprocessor(db *gorm.DB, logger zerolog.Logger, base_path string) *PlatformPreprocessor {
	return &PlatformPreprocessor{deps: PostprocessDeps{DB: db, Logger: logger, BasePath: base_path}}
}

func (p *PlatformPreprocessor) Process(ctx context.Context, task *hermes.TaskJob) error {
	if task == nil {
		return nil
	}
	if handler, ok := Get(task.Platform).(DownloadPreparer); ok {
		return handler.PrepareDownload(ctx, task, p.deps)
	}
	return nil
}
