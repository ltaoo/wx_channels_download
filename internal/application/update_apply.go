package application

import (
	"github.com/ltaoo/velo/updater/applier"
	"github.com/rs/zerolog"
)

func apply_update_archive_now_with_velo(update_path string, exe_path string) error {
	logger := zerolog.Nop()
	update_applier := applier.NewPlatformUpdater(&logger)
	return update_applier.Apply(update_path, exe_path)
}
