package officialaccountdownload

import (
	"encoding/json"
	"testing"
)

func TestExtractArticleID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "short path",
			url:  "https://mp.weixin.qq.com/s/SXyNocq1-K4WkFcI-0aD6w",
			want: "SXyNocq1-K4WkFcI-0aD6w",
		},
		{
			name: "biz sn query",
			url:  "https://mp.weixin.qq.com/s?__biz=xz&sn=abc",
			want: "xz_abc",
		},
		{
			name: "officialaccount scheme",
			url:  "officialaccount://https://mp.weixin.qq.com/s/SXyNocq1-K4WkFcI-0aD6w",
			want: "SXyNocq1-K4WkFcI-0aD6w",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractArticleID(tt.url); got != tt.want {
				t.Fatalf("ExtractArticleID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFlexibleIntUnmarshalString(t *testing.T) {
	var data CgiDataNew
	if err := json.Unmarshal([]byte(`{"user_uin":"69477998648217","mid":"123","idx":"1"}`), &data); err != nil {
		t.Fatal(err)
	}
	if data.UserUin != FlexibleInt(69477998648217) || data.Mid != 123 || data.Idx != 1 {
		t.Fatalf("unexpected flexible ints: user_uin=%d mid=%d idx=%d", data.UserUin, data.Mid, data.Idx)
	}
}
