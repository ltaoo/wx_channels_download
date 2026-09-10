package feishu

import "testing"

func Test_parse_document_url(t *testing.T) {
	cases := []struct {
		name     string
		raw_url  string
		want_url string
		want_err bool
	}{
		{
			name:     "feishu docx",
			raw_url:  "https://example.feishu.cn/docx/ABC123?from=share",
			want_url: "https://example.feishu.cn/docx/ABC123",
		},
		{
			name:     "larkenterprise wiki",
			raw_url:  "https://mayfairtech.larkenterprise.com/wiki/LxaDwNLkVizzU0k6hQtc8FI5nZc/",
			want_url: "https://mayfairtech.larkenterprise.com/wiki/LxaDwNLkVizzU0k6hQtc8FI5nZc",
		},
		{name: "unsupported host", raw_url: "https://example.com/wiki/ABC123", want_err: true},
		{name: "unsupported path", raw_url: "https://example.feishu.cn/sheets/ABC123", want_err: true},
		{name: "http", raw_url: "http://example.feishu.cn/docx/ABC123", want_err: true},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			document_url, _, _, err := parse_document_url(test_case.raw_url)
			if test_case.want_err {
				if err == nil {
					t.Fatalf("expected error, got %s", document_url)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if document_url != test_case.want_url {
				t.Fatalf("url = %s, want %s", document_url, test_case.want_url)
			}
		})
	}
}

func Test_stream_download_host(t *testing.T) {
	cases := map[string]string{
		"mayfairtech.larkenterprise.com": "internal-api-drive-stream.larkenterprise.com",
		"https://example.feishu.cn/":     "internal-api-drive-stream.feishu.cn",
		"feishu.cn":                      "internal-api-drive-stream.feishu.cn",
	}
	for origin, want_host := range cases {
		if got_host := stream_download_host(origin); got_host != want_host {
			t.Fatalf("stream_download_host(%s) = %s, want %s", origin, got_host, want_host)
		}
	}
}
