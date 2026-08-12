package shuba69

import (
	"testing"
)

func TestNormalizeNovelProfileURL(t *testing.T) {
	tests := []struct {
		name     string
		raw_url  string
		want_url string
	}{
		{name: "www directory", raw_url: "https://www.69shuba.com/book/34567/", want_url: "https://www.69shuba.com/book/34567.htm"},
		{name: "directory without www", raw_url: "https://69shuba.com/book/34567/", want_url: "https://69shuba.com/book/34567.htm"},
		{name: "http directory", raw_url: "http://www.69shuba.com/book/34567/", want_url: "http://www.69shuba.com/book/34567.htm"},
		{name: "surrounding spaces", raw_url: " https://www.69shuba.com/book/34567/ ", want_url: "https://www.69shuba.com/book/34567.htm"},
		{name: "directory without trailing slash", raw_url: "https://www.69shuba.com/book/34567", want_url: "https://www.69shuba.com/book/34567"},
		{name: "directory with query", raw_url: "https://www.69shuba.com/book/34567/?from=search", want_url: "https://www.69shuba.com/book/34567/?from=search"},
		{name: "wrong host", raw_url: "https://example.com/book/34567/", want_url: "https://example.com/book/34567/"},
		{name: "non-numeric book id", raw_url: "https://www.69shuba.com/book/abc/", want_url: "https://www.69shuba.com/book/abc/"},
		{name: "profile", raw_url: "https://www.69shuba.com/book/34567.htm", want_url: "https://www.69shuba.com/book/34567.htm"},
		{name: "chapter", raw_url: "https://www.69shuba.com/book/34567/123456.htm", want_url: "https://www.69shuba.com/book/34567/123456.htm"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got_url := normalize_novel_profile_url(test.raw_url)
			if got_url != test.want_url {
				t.Fatalf("normalize_novel_profile_url(%q) = %q, want %q", test.raw_url, got_url, test.want_url)
			}
		})
	}
}
