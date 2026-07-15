package weibo

import "testing"

func TestMergeCookieHeadersOverwritesExistingValues(t *testing.T) {
	got := mergeCookieHeaders("SINAGLOBAL=old; XSRF-TOKEN=old", "XSRF-TOKEN=new; SUB=session")
	want := "SINAGLOBAL=old; XSRF-TOKEN=new; SUB=session"
	if got != want {
		t.Fatalf("merged cookie = %q, want %q", got, want)
	}
}

func TestCookiesFromSetCookieHeaders(t *testing.T) {
	got := cookiesFromSetCookieHeaders([]string{
		"XSRF-TOKEN=abc; Path=/; Domain=.weibo.com",
		"SUB=session; Path=/; HttpOnly",
	})
	want := []string{"XSRF-TOKEN=abc", "SUB=session"}
	if len(got) != len(want) {
		t.Fatalf("cookies len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cookie[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestIsWeiboHost(t *testing.T) {
	for _, host := range []string{"weibo.com", "www.weibo.com", "api.weibo.com"} {
		if !isWeiboHost(host) {
			t.Fatalf("expected %q to be weibo host", host)
		}
	}
	if isWeiboHost("notweibo.com") {
		t.Fatal("unexpected host match")
	}
}
