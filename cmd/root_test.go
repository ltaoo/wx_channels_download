package cmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestParseStartCommandOnlyPublishesExplicitValues(t *testing.T) {
	options, values, err := parse_start_command(nil)
	if err != nil {
		t.Fatalf("parse defaults: %v", err)
	}
	if options.hostname != "127.0.0.1" || options.port != 2023 {
		t.Fatalf("start defaults = %+v", options)
	}
	if len(values) != 0 {
		t.Fatalf("default flags became overrides: %v", values)
	}

	options, values, err = parse_start_command([]string{
		"--config", "custom.yaml",
		"--dev", "en0",
		"--hostname", "0.0.0.0",
		"--port", "4040",
		"--debug=false",
	})
	if err != nil {
		t.Fatalf("parse explicit flags: %v", err)
	}
	if options.config_filepath != "custom.yaml" {
		t.Fatalf("config path = %q", options.config_filepath)
	}
	want := map[string]any{
		"proxy.device":   "en0",
		"proxy.hostname": "0.0.0.0",
		"proxy.port":     4040,
		"debug.error":    false,
	}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("values = %v, want %v", values, want)
	}
}

func TestParseStartCommandSupportsShortConfigFlag(t *testing.T) {
	options, _, err := parse_start_command([]string{"-c", "custom.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if options.config_filepath != "custom.yaml" {
		t.Fatalf("config path = %q", options.config_filepath)
	}
}

func TestExecuteRejectsUnknownCommand(t *testing.T) {
	logger := zerolog.Nop()
	err := Execute("test", "debug", []string{"missing"}, &logger)
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("unknown command error = %v", err)
	}
}
