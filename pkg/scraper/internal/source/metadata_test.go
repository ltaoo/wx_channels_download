package source

import (
	"testing"
)

func TestMetadataKindsAndUnknownType(t *testing.T) {
	for _, item := range []struct{ page, kind string }{
		{`<meta property="og:type" content="video.movie"><meta property="og:title" content="电影名">`, "movie"},
		{`<script type="application/ld+json">{"@graph":[{"@type":"WebSite"},{"@type":"Movie","name":"电影名","actor":[{"name":"演员"}]}]}</script>`, "movie"},
		{`<script type="application/ld+json">{"@type":["TVSeries"],"name":"系列","numberOfEpisodes":10}</script>`, "series"},
		{`<script type="application/ld+json">{"@type":"TVEpisode","partOfSeries":{"name":"所属系列"}}</script>`, "series"},
		{`<meta property="og:title" content="单集名称">`, "video"},
	} {
		if got := ParsePageMetadata([]byte(item.page)); got.Kind != item.kind {
			t.Fatalf("kind=%s want=%s", got.Kind, item.kind)
		}
	}
}
