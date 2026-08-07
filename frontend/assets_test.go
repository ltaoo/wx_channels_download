package frontend

import (
	"net/url"
	"testing"
)

func TestNewURLBuildQueryOverride(t *testing.T) {
	url_build := NewURLBuild("http://127.0.0.1:2022/__assets/", url.Values{
		"lang": []string{"zh-CN"},
		"v":    []string{"base"},
	})

	got := url_build("/public/app.js?source=path", url.Values{
		"q": []string{"hello world"},
		"v": []string{"override"},
	})
	want := "http://127.0.0.1:2022/__assets/public/app.js?lang=zh-CN&q=hello+world&source=path&v=override"
	if got != want {
		t.Fatalf("NewURLBuild() = %q, want %q", got, want)
	}

	got = url_build("/inject/env.js")
	want = "http://127.0.0.1:2022/__assets/inject/env.js?lang=zh-CN&v=base"
	if got != want {
		t.Fatalf("NewURLBuild() default query = %q, want %q", got, want)
	}
}
