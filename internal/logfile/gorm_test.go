package logfile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	gorm_logger "gorm.io/gorm/logger"
)

func TestGORMTraceLevels(t *testing.T) {
	original_level := zerolog.GlobalLevel()
	t.Cleanup(func() { zerolog.SetGlobalLevel(original_level) })
	for _, test_case := range []struct {
		name         string
		log_level    gorm_logger.LogLevel
		global_level zerolog.Level
		elapsed      time.Duration
		err          error
		want_level   string
	}{
		{name: "query_error", log_level: gorm_logger.Warn, err: errors.New("constraint failed"), want_level: "error"},
		{name: "record_not_found", log_level: gorm_logger.Warn, err: gorm_logger.ErrRecordNotFound, want_level: "error"},
		{name: "slow_query", log_level: gorm_logger.Warn, elapsed: time.Second, want_level: "warn"},
		{name: "slow_error", log_level: gorm_logger.Warn, elapsed: time.Second, err: errors.New("constraint failed"), want_level: "error"},
		{name: "normal_query", log_level: gorm_logger.Warn},
		{name: "verbose_query", log_level: gorm_logger.Info, want_level: "info"},
		{name: "silent", log_level: gorm_logger.Silent, err: errors.New("constraint failed")},
		{name: "disabled", log_level: gorm_logger.Warn, global_level: zerolog.Disabled, err: errors.New("constraint failed")},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			zerolog.SetGlobalLevel(test_case.global_level)
			var output bytes.Buffer
			logger := zerolog.New(&output)
			adapter := NewGORMLogger(&logger).LogMode(test_case.log_level)
			query_calls := 0
			adapter.Trace(context.Background(), time.Now().Add(-test_case.elapsed), func() (string, int64) {
				query_calls++
				return "SELECT 1", -1
			}, test_case.err)
			if test_case.want_level == "" {
				if output.Len() != 0 || query_calls != 0 {
					t.Fatalf("filtered query logged or formatted SQL: output=%s, calls=%d", output.String(), query_calls)
				}
				return
			}
			var entry map[string]interface{}
			if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if entry["level"] != test_case.want_level || entry["sql"] != "SELECT 1" || entry["rows"] != float64(-1) || query_calls != 1 {
				t.Fatalf("unexpected query log: %v, calls=%d", entry, query_calls)
			}
			if test_case.err != nil && entry["error"] != test_case.err.Error() {
				t.Fatalf("query error lost: %v", entry)
			}
		})
	}
}

func TestGORMLogModeDoesNotChangeOriginal(t *testing.T) {
	original_level := zerolog.GlobalLevel()
	t.Cleanup(func() { zerolog.SetGlobalLevel(original_level) })
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	var output bytes.Buffer
	logger := zerolog.New(&output)
	adapter := NewGORMLogger(&logger)
	adapter.LogMode(gorm_logger.Info).Info(context.Background(), "query %d", 1)
	adapter.Info(context.Background(), "must be filtered")
	adapter.LogMode(gorm_logger.Silent).Error(context.Background(), "must be filtered")
	var entry map[string]interface{}
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["level"] != "info" || entry["message"] != "query 1" {
		t.Fatalf("unexpected log: %v", entry)
	}
}
