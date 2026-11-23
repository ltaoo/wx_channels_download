//go:build !darwin && !windows && !linux

package wxchannels

import (
	"context"

	"github.com/rs/zerolog"
)

func StartLiteAppCacheCleanup(ctx context.Context, version string, logger *zerolog.Logger) {}
