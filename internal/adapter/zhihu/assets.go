package zhihuadapter

import (
	"fmt"
	"io/fs"

	"wx_channel/internal/webassets"
	"wx_channel/pkg/scraper/zhihu"
)

func register_static_assets(registry *webassets.Registry) error {
	assets := zhihu.InjectAssets()
	if err := registry.Register(zhihu.InjectAssetsPath, assets); err != nil {
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
