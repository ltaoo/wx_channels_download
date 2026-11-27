package cmd

import (
	"net/url"
	"testing"

	"github.com/spf13/viper"
	"wx_channel/internal/config"
)

func TestConfiguredAPIURLPreservesEncodedCredentials(t *testing.T) {
	previous_config := Cfg
	Cfg = &config.Config{}
	t.Cleanup(func() { Cfg = previous_config })
	for key, value := range map[string]any{"api.protocol": "http", "api.hostname": "0.0.0.0", "api.port": 2022, "api.username": "owner", "api.password": "s:e@cret"} {
		previous_value := viper.Get(key)
		config_key := key
		t.Cleanup(func() { viper.Set(config_key, previous_value) })
		viper.Set(key, value)
	}
	address, err := url.Parse(configured_api_base_url())
	if err != nil {
		t.Fatal(err)
	}
	password, has_password := address.User.Password()
	if address.Host != "127.0.0.1:2022" || address.User.Username() != "owner" || !has_password || password != "s:e@cret" {
		t.Fatal("configured API address lost the local address or credentials")
	}
}
