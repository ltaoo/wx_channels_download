package douyin

import (
	"strings"

	"github.com/rs/zerolog"
)

const log_preview_limit = 1024

func new_component_logger(parent_logger *zerolog.Logger, component string) zerolog.Logger {
	if parent_logger == nil {
		return zerolog.Nop()
	}
	return parent_logger.With().Str("component", component).Logger()
}

func log_text_preview(value string) string {
	value = strings.TrimSpace(strings.ToValidUTF8(value, "�"))
	value_runes := []rune(value)
	if len(value_runes) <= log_preview_limit {
		return value
	}
	return string(value_runes[:log_preview_limit]) + "…"
}

func log_body_preview(body []byte) string {
	return log_text_preview(string(body))
}
