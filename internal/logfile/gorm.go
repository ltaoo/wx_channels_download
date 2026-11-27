package logfile

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	gorm_logger "gorm.io/gorm/logger"
)

type gorm_log_adapter struct {
	logger    *zerolog.Logger
	log_level gorm_logger.LogLevel
}

// NewGORMLogger preserves GORM severity in structured application logs.
func NewGORMLogger(logger *zerolog.Logger) gorm_logger.Interface {
	return gorm_log_adapter{logger: logger, log_level: gorm_logger.Warn}
}

func (l gorm_log_adapter) LogMode(level gorm_logger.LogLevel) gorm_logger.Interface {
	l.log_level = level
	return l
}

func (l gorm_log_adapter) Info(ctx context.Context, message string, args ...interface{}) {
	if l.log_level >= gorm_logger.Info {
		l.logger.Info().Ctx(ctx).Msgf(message, args...)
	}
}

func (l gorm_log_adapter) Warn(ctx context.Context, message string, args ...interface{}) {
	if l.log_level >= gorm_logger.Warn {
		l.logger.Warn().Ctx(ctx).Msgf(message, args...)
	}
}

func (l gorm_log_adapter) Error(ctx context.Context, message string, args ...interface{}) {
	if l.log_level >= gorm_logger.Error {
		l.logger.Error().Ctx(ctx).Msgf(message, args...)
	}
}

func (l gorm_log_adapter) Trace(ctx context.Context, begin time.Time, query func() (string, int64), err error) {
	if l.log_level <= gorm_logger.Silent {
		return
	}
	elapsed := time.Since(begin)
	var event *zerolog.Event
	switch {
	case err != nil && l.log_level >= gorm_logger.Error:
		event = l.logger.Error().Err(err)
	case elapsed > 200*time.Millisecond && l.log_level >= gorm_logger.Warn:
		event = l.logger.Warn()
	case l.log_level >= gorm_logger.Info:
		event = l.logger.Info()
	default:
		return
	}
	if !event.Enabled() {
		return
	}
	sql, rows_affected := query()
	event.Ctx(ctx).Dur("elapsed_ms", elapsed).Int64("rows", rows_affected).Str("sql", sql).Msg("database query")
}
