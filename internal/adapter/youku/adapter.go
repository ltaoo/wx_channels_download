package youkuadapter

import (
	"context"
	"net/http"
	"wx_channel/internal/adapter"
	"wx_channel/internal/adapter/internal/vod"
	"wx_channel/internal/config"
	"wx_channel/pkg/scraper"
	"wx_channel/pkg/scraper/youku"
)

const PlatformID = "youku"

type handler struct{ *vod.Handler }

func init() {
	spec := vod.Specification{PlatformID: PlatformID, Name: "优酷", Homepage: "https://v.youku.com/", Parse: parse_result, ParseURL: youku.ParseURL, Resolve: func(ctx context.Context, source_url string, episode scraper.Episode, cookie string, http_client *http.Client) (scraper.Video, error) {
		client := youku.NewClient(cookie)
		client.SetHTTPClient(http_client)
		return client.ResolveContext(ctx, source_url, episode)
	}, Fetch: func(ctx context.Context, raw_url, cookie string) (any, error) {
		return youku.NewClient(cookie).FetchContext(ctx, raw_url)
	}}
	adapter.Register(&handler{Handler: vod.NewHandler(spec)})
	config.RegisterPlugin(vod.NewConfig(spec))

}
