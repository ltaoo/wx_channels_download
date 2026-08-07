package interceptor

import "testing"

func TestRewriteCSPForLocalAssetsAddsLocalOrigins(t *testing.T) {
	policy := "default-src 'self'; style-src 'self' 'unsafe-inline' http://*.qq.com https://*.qq.com http://*.qpic.cn https://*.qpic.cn; script-src 'nonce-abc'; connect-src https://*.qq.com"

	got := RewriteCSPForLocalAssets(policy, "http://127.0.0.1:2022/__assets")
	want := "default-src 'self'; style-src 'self' 'unsafe-inline' http://*.qq.com https://*.qq.com http://*.qpic.cn https://*.qpic.cn http://127.0.0.1:2022; script-src 'nonce-abc' http://127.0.0.1:2022; connect-src https://*.qq.com http://127.0.0.1:2022 ws://127.0.0.1:2022"
	if got != want {
		t.Fatalf("RewriteCSPForLocalAssets() = %q, want %q", got, want)
	}
}

func TestRewriteCSPForLocalAssetsCreatesFallbackDirectives(t *testing.T) {
	policy := "default-src 'none'; style-src-elem 'self'; script-src-elem 'self'"

	got := RewriteCSPForLocalAssets(policy, "http://127.0.0.1:2022/__assets")
	want := "default-src 'none'; style-src-elem 'self' http://127.0.0.1:2022; script-src-elem 'self' http://127.0.0.1:2022; style-src http://127.0.0.1:2022; script-src http://127.0.0.1:2022; connect-src http://127.0.0.1:2022 ws://127.0.0.1:2022"
	if got != want {
		t.Fatalf("RewriteCSPForLocalAssets() = %q, want %q", got, want)
	}
}

func TestRewriteCSPForLocalAssetsIgnoresRelativeAssetBaseURL(t *testing.T) {
	policy := "default-src 'self'; style-src 'self'"

	got := RewriteCSPForLocalAssets(policy, "/__assets")
	if got != policy {
		t.Fatalf("RewriteCSPForLocalAssets() = %q, want unchanged %q", got, policy)
	}
}
