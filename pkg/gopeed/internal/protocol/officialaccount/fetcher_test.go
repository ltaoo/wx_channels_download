package officialaccountdownload

import "testing"

func TestFetcherManagerParseNameUsesArticleID(t *testing.T) {
	fm := &FetcherManager{}
	got := fm.ParseName("officialaccount://https://mp.weixin.qq.com/s/VmGgwr4-8O71LO-MivK3Qg")
	want := "VmGgwr4-8O71LO-MivK3Qg.html"
	if got != want {
		t.Fatalf("ParseName() = %q, want %q", got, want)
	}
}

func TestFetcherManagerParseNameFallback(t *testing.T) {
	fm := &FetcherManager{}
	got := fm.ParseName("officialaccount://not-weixin.example/path")
	want := "article.html"
	if got != want {
		t.Fatalf("ParseName() = %q, want %q", got, want)
	}
}
