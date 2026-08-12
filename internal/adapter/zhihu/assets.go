package zhihuadapter

import (
	"fmt"
	"io/fs"
	"net/url"
	"strings"

	"wx_channel/frontend"
	"wx_channel/internal/webassets"
	"wx_channel/pkg/scraper/zhihu"
)

const (
	asset_path_prefix  = "/zhihu"
	static_assets_path = "/__assets" + asset_path_prefix + "/inject"
)

func register_static_assets(registry *webassets.Registry) error {
	assets := zhihu.InjectAssets()
	if err := registry.Register(static_assets_path, assets); err != nil {
		return err
	}

	entries, err := fs.ReadDir(assets, ".")
	if err != nil {
		return fmt.Errorf("read legacy asset aliases: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := registry.RegisterFile("/__assets/inject/"+entry.Name(), assets, entry.Name()); err != nil {
			return fmt.Errorf("register legacy asset %q: %w", entry.Name(), err)
		}
	}
	return nil
}

func asset_url(base_url, name string, query ...url.Values) string {
	return frontend.NewURLBuild(base_url, nil)(asset_path_prefix+"/"+strings.TrimLeft(name, "/"), query...)
}
