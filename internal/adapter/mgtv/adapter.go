package mgtvadapter

import (
	"context"
	"net/http"
	"wx_channel/internal/adapter"
	"wx_channel/internal/adapter/internal/vod"
	"wx_channel/internal/config"
	"wx_channel/pkg/scraper"
	"wx_channel/pkg/scraper/mgtv"
)

const PlatformID = "mgtv"

type handler struct{ *vod.Handler }

func init() {
	spec := vod.Specification{PlatformID: PlatformID, Name: "芒果TV", Homepage: "https://www.mgtv.com/", Parse: parse_result, ParseURL: mgtv.ParseURL, Resolve: func(ctx context.Context, source_url string, episode scraper.Episode, cookie string, http_client *http.Client) (scraper.Video, error) {
		client := mgtv.NewClient(cookie)
		client.SetHTTPClient(http_client)
		return client.ResolveContext(ctx, source_url, episode)
	}, Fetch: func(ctx context.Context, raw_url, cookie string) (any, error) {
		return mgtv.NewClient(cookie).FetchContext(ctx, raw_url)
	}}
	adapter.Register(&handler{Handler: vod.NewHandler(spec)})
	config.RegisterPlugin(vod.NewConfig(spec))

}
