package officialaccount

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

func TestNewOfficialAccountConfigExposesEnabledFromMPEnabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	rootDir := t.TempDir()
	configPath := filepath.Join(rootDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("mp:\n  enabled: true\n  disabled: true\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	viper.SetConfigFile(configPath)
	cfg := &config.Config{
		RootDir:  rootDir,
		Filename: "config.yaml",
		FullPath: configPath,
		Existing: true,
	}
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	got := NewOfficialAccountConfig(cfg, false)
	if !got.Enabled {
		t.Fatal("Enabled = false, want true")
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if payload["officialAccountEnabled"] != true {
		t.Fatalf("officialAccountEnabled = %v, want true in JSON payload", payload["officialAccountEnabled"])
	}
}
