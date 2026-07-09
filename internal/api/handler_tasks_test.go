package api

import "testing"

func TestIsChannelsDownloadURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "stodownload",
			url:  "https://finder.video.qq.com/251/20302/stodownload?encfilekey=Cvvj5Ix3eewQs",
			want: true,
		},
		{
			name: "channels feed page",
			url:  "https://channels.weixin.qq.com/web/pages/feed?oid=z0Qii_kLCBA&nid=2-dNcmWxXdc&context_id=33-9-141-18a2bc728e23eacd62e8fc98e3bbff391780413220152",
			want: true,
		},
		{
			name: "other finder path",
			url:  "https://finder.video.qq.com/251/20302/profile",
			want: false,
		},
		{
			name: "other channels path",
			url:  "https://channels.weixin.qq.com/web/pages/profile?oid=z0Qii_kLCBA",
			want: false,
		},
		{
			name: "other domain",
			url:  "https://example.com/251/20302/stodownload",
			want: false,
		},
		{
			name: "invalid url",
			url:  "%",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isChannelsDownloadURL(tt.url); got != tt.want {
				t.Fatalf("isChannelsDownloadURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
