package mgtv

import "testing"

func TestTokenAndParseURL(t *testing.T) {
	expected := "wAjN5gjN1MzNx0Ddpx2Y8FDMzAjLz4CM9IXZ2xHMzATM98mbwxHN0QDN0QDN0QDN0QTLzMzM40iMyIDNtETMxETLwADMwADMwATPklGZ"
	if mgtv_token("00000000-1111-4222-8333-444444444444", 1735689600) != expected {
		t.Fatal("token differs from independent reference")
	}
	page, cover, id, err := ParseURL("http://mgtv.com/b/914227/24771585.html?from=test")
	if err != nil || page != test_mgtv_page || cover != test_mgtv_cover || id != test_mgtv_id {
		t.Fatalf("parse: %s %s %s %v", page, cover, id, err)
	}
	for _, raw_url := range []string{"https://www.mgtv.com.evil.example/b/914227/24771585.html", "https://user@www.mgtv.com/b/914227/24771585.html", "https://www.mgtv.com/b/914227/abc.html"} {
		if _, _, _, err := ParseURL(raw_url); err == nil {
			t.Fatalf("accepted invalid URL: %s", raw_url)
		}
	}
}
