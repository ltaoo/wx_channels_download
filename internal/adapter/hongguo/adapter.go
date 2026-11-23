package hongguoadapter

import (
	"context"
	"net/http"
	"wx_channel/internal/adapter"
	"wx_channel/internal/adapter/internal/vod"
	"wx_channel/internal/config"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper"
	"wx_channel/pkg/scraper/hongguo"
)

const PlatformID = "hongguo"

type handler struct{ *vod.Handler }

func init() {
	spec := vod.Specification{VideoStreamType: model.ContentVideoVariantStreamTypeProgressive, PlatformID: PlatformID, Name: "红果短剧", Homepage: "https://hongguoduanju.com/", Parse: parse_result, ParseURL: hongguo.ParseURL, Resolve: func(ctx context.Context, source_url string, episode scraper.Episode, cookie string, http_client *http.Client) (scraper.Video, error) {
		client := hongguo.NewClient(cookie)
		client.SetHTTPClient(http_client)
		return client.ResolveContext(ctx, source_url, episode)
	}, Fetch: func(ctx context.Context, raw_url, cookie string) (any, error) {
		return hongguo.NewClient(cookie).FetchContext(ctx, raw_url)
	}}
	adapter.Register(&handler{Handler: vod.NewHandler(spec)})
	config.RegisterPlugin(vod.NewConfig(spec))

}
