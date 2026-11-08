package vqq

import "testing"

func TestParseURL(t *testing.T) {
	for _, item := range []struct{ url, cover, id string }{
		{"https://v.qq.com/x/cover/mzc002001w361jz.html", "mzc002001w361jz", ""},
		{"https://v.qq.com/detail/m/mzc002001w361jz.html", "mzc002001w361jz", ""},
		{"https://v.qq.com/x/cover/mzc002001w361jz/n3371za9efh.html?from=test", "mzc002001w361jz", "n3371za9efh"},
		{"v.qq.com/x/page/n3371za9efh.html", "", "n3371za9efh"},
	} {
		_, cover, id, err := ParseURL(item.url)
		if err != nil || cover != item.cover || id != item.id {
			t.Fatalf("parse: %s %s %v", cover, id, err)
		}
	}
	if _, _, _, err := ParseURL("https://v.qq.com.evil.example/x/page/n3371za9efh.html"); err == nil {
		t.Fatal("accepted spoofed host")
	}
}
