package logtime

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

type timestamp_context_key struct{}

// Hook uses an event's source timestamp when present and the write time otherwise.
type Hook struct{}

func (Hook) Run(event *zerolog.Event, _ zerolog.Level, _ string) {
	log_time, ok := event.GetCtx().Value(timestamp_context_key{}).(time.Time)
	if !ok {
		log_time = zerolog.TimestampFunc()
	}
	event.Time(zerolog.TimestampFieldName, log_time)
}

// WithTimestamp attaches a source timestamp without rendering an extra log field.
func WithTimestamp(ctx context.Context, log_time time.Time) context.Context {
	return context.WithValue(ctx, timestamp_context_key{}, log_time)
}
