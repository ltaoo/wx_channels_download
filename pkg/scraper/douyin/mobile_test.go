package douyin

import (
	"encoding/json"
	"testing"
)

func TestMobileRouterRequiresItems(t *testing.T) {
	for _, test_case := range []struct {
		json       string
		want_error bool
	}{
		{`{"loaderData":{"video_(id)/page":{}}}`, true},
		{`{"loaderData":{"video_(id)/page":{"videoInfoRes":{"item_list":[]}}}}`, true},
		{`{"loaderData":{"video_(id)/page":{"videoInfoRes":{"item_list":[{"aweme_id":"123"}]}}}}`, false},
	} {
		data, err := extract_router_json([]byte("<script>window._ROUTER_DATA = " + test_case.json + "</script>"))
		if (err != nil) != test_case.want_error {
			t.Fatalf("want_error=%t, err=%v", test_case.want_error, err)
		}
		if err == nil && !json.Valid(data) {
			t.Fatal("valid router data must remain JSON")
		}
	}
}
