package wxmpadapter

import (
	"testing"

	"wx_channel/pkg/scraper/wxmp"
)

func TestArticleExternalID(t *testing.T) {
	data := &wxmp.ArticleCgiDataNew{
		Bizuin: "MzExample==",
		Mid:    2247506133,
		Idx:    2,
	}

	got := ArticleExternalID(data)
	want := "MzExample==_2247506133_2"
	if got != want {
		t.Fatalf("ArticleExternalID() = %q, want %q", got, want)
	}
}

func TestArticleExternalIDRequiresArticleCoordinates(t *testing.T) {
	tests := []struct {
		name string
		data *wxmp.ArticleCgiDataNew
	}{
		{name: "nil", data: nil},
		{name: "missing biz", data: &wxmp.ArticleCgiDataNew{Mid: 1, Idx: 1}},
		{name: "missing mid", data: &wxmp.ArticleCgiDataNew{Bizuin: "biz", Idx: 1}},
		{name: "missing idx", data: &wxmp.ArticleCgiDataNew{Bizuin: "biz", Mid: 1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ArticleExternalID(test.data); got != "" {
				t.Fatalf("ArticleExternalID() = %q, want empty string", got)
			}
		})
	}
}
