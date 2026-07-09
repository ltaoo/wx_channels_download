package assetsync

import "testing"

func TestNormalizeAppliesLocalDeviceStorageRoot(t *testing.T) {
	cfg := Config{
		Version:  1,
		Manifest: ".asset-sync.lock.json",
		Roots: []RootConfig{
			{Path: "scraper_examples"},
		},
		Storage: StorageConfig{
			Type:      "local",
			LocalPath: "/fallback",
			Prefix:    "repo",
		},
		Devices: map[string]DeviceConfig{
			"macbook": {StorageRoot: "/Users/example/SynologyDrive/asset-sync-store"},
		},
	}

	if err := cfg.Normalize("macbook"); err != nil {
		t.Fatal(err)
	}
	if cfg.ResolvedDevice != "macbook" {
		t.Fatalf("unexpected resolved device: %q", cfg.ResolvedDevice)
	}
	if cfg.Storage.LocalPath != "/Users/example/SynologyDrive/asset-sync-store" {
		t.Fatalf("unexpected local path: %q", cfg.Storage.LocalPath)
	}
}

func TestNormalizeAppliesRcloneDeviceStorageRoot(t *testing.T) {
	cfg := Config{
		Version:  1,
		Manifest: ".asset-sync.lock.json",
		Roots: []RootConfig{
			{Path: "scraper_examples"},
		},
		Storage: StorageConfig{
			Type:         "rclone",
			RcloneBinary: "rclone",
			RcloneRemote: "fallback:bucket",
			Prefix:       "repo",
		},
		Devices: map[string]DeviceConfig{
			"ci": {StorageRoot: "r2:asset-bucket"},
		},
	}

	if err := cfg.Normalize("ci"); err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.RcloneRemote != "r2:asset-bucket" {
		t.Fatalf("unexpected rclone remote: %q", cfg.Storage.RcloneRemote)
	}
}

func TestNormalizeRejectsUnknownDevice(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Devices = map[string]DeviceConfig{
		"known": {StorageRoot: "/tmp/assets"},
	}
	if err := cfg.Normalize("missing"); err == nil {
		t.Fatal("expected unknown device to fail")
	}
}

func TestNormalizeAutoDeviceRequiresMatchWhenDevicesConfigured(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Device = "auto"
	cfg.Devices = map[string]DeviceConfig{
		"unlikely-hostname-for-asset-sync-test": {StorageRoot: "/tmp/assets"},
	}
	if err := cfg.Normalize(""); err == nil {
		t.Fatal("expected auto device without hostname match to fail")
	}
}

func TestNormalizeAutoDeviceUsesDefaultFallback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Device = "auto"
	cfg.Devices = map[string]DeviceConfig{
		"default": {StorageRoot: "/tmp/default-assets"},
	}
	if err := cfg.Normalize(""); err != nil {
		t.Fatal(err)
	}
	if cfg.ResolvedDevice != "default" {
		t.Fatalf("unexpected resolved device: %q", cfg.ResolvedDevice)
	}
	if cfg.Storage.LocalPath != "/tmp/default-assets" {
		t.Fatalf("unexpected local path: %q", cfg.Storage.LocalPath)
	}
}
