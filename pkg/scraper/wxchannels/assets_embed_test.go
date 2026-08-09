//go:build embed_frontend_inject

package wxchannels

import (
	"testing"

	"wx_channel/internal/webassets"
)

func TestEmbeddedStaticAssetsRegister(t *testing.T) {
	if err := RegisterStaticAssets(webassets.NewRegistry()); err != nil {
		t.Fatalf("RegisterStaticAssets() error = %v", err)
	}
}
