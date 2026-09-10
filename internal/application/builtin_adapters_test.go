package application_test

import (
	"testing"

	"wx_channel/internal/adapter"
	_ "wx_channel/internal/application"
	"wx_channel/internal/services"
)

func Test_builtin_adapters_register_feishu(t *testing.T) {
	if handler := adapter.Get("feishu"); handler == nil {
		t.Fatal("feishu adapter is not registered")
	}
	platform_id, err := services.DetectScraperPlatform("https://mayfairtech.larkenterprise.com/wiki/LxaDwNLkVizzU0k6hQtc8FI5nZc")
	if err != nil {
		t.Fatalf("DetectScraperPlatform() error = %v", err)
	}
	if platform_id != "feishu" {
		t.Fatalf("DetectScraperPlatform() = %q, want feishu", platform_id)
	}
}
