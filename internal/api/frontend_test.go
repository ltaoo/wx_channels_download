package api

import "testing"

func TestShouldServeByAPIIncludesImageProxies(t *testing.T) {
	for _, path := range []string{
		"/xiaohongshu/proxy",
		"/bilibili/proxy",
		"/douban/proxy",
		"/instagram/proxy",
		"/weibo/proxy",
	} {
		if !shouldServeByAPI(path) {
			t.Fatalf("%s route should be served by API handler", path)
		}
	}
}
