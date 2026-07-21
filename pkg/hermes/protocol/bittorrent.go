package protocol

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"

	"wx_channel/pkg/hermes"
)

// TorrentConfig 是 BitTorrent 驱动的可选配置。
type TorrentConfig struct {
	// DataDir 是 torrent 客户端存放下载数据的目录，为空时使用系统临时目录。
	DataDir string
	// NoUpload 禁止上传数据（吸血模式），默认 true。
	NoUpload bool
}

// TorrentDriver 实现 hermes.ProtocolDriver 接口，支持通过磁力链接或 .torrent 文件下载。
type TorrentDriver struct {
	client  *torrent.Client
	mu      sync.Mutex
	torrents map[metainfo.Hash]*torrent.Torrent
	config  TorrentConfig
}

// NewTorrentDriver 创建 BitTorrent 协议驱动。需要事先通过 go get 安装依赖。
func NewTorrentDriver(config TorrentConfig) (*TorrentDriver, error) {
	cfg := torrent.NewDefaultClientConfig()
	if config.DataDir == "" {
		config.DataDir = os.TempDir()
	}
	cfg.DataDir = config.DataDir
	cfg.NoUpload = config.NoUpload
	cfg.DisableIPv6 = true
	cfg.Seed = false

	client, err := torrent.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建 torrent 客户端失败: %w", err)
	}
	return &TorrentDriver{
		client:   client,
		torrents: make(map[metainfo.Hash]*torrent.Torrent),
		config:   config,
	}, nil
}

// Protocols 返回该驱动支持的协议标识符。
func (d *TorrentDriver) Protocols() []string {
	return []string{"bittorrent", "magnet", "bt", "torrent"}
}

// Prepare 解析磁力链接或 .torrent 文件地址，获取 torrent 元数据。
func (d *TorrentDriver) Prepare(ctx context.Context, endpoint hermes.Endpoint) (hermes.PreparedResource, error) {
	url := strings.TrimSpace(endpoint.URL)
	if url == "" {
		return hermes.PreparedResource{}, fmt.Errorf("torrent: 端点 URL 为空")
	}

	t, err := d.getOrAddTorrent(ctx, url)
	if err != nil {
		return hermes.PreparedResource{}, fmt.Errorf("torrent: %w", err)
	}

	select {
	case <-ctx.Done():
		return hermes.PreparedResource{}, ctx.Err()
	case <-t.GotInfo():
	}

	info := t.Info()
	if info == nil {
		return hermes.PreparedResource{}, fmt.Errorf("torrent: 未能获取元数据")
	}

	return hermes.PreparedResource{
		Size:          info.TotalLength(),
		ContentType:   "application/octet-stream",
		SupportsRange: true,
	}, nil
}

// Open 创建 torrent 数据读取流，支持断点续传。
func (d *TorrentDriver) Open(ctx context.Context, endpoint hermes.Endpoint, request hermes.ReadRequest) (io.ReadCloser, error) {
	url := strings.TrimSpace(endpoint.URL)
	if url == "" {
		return nil, fmt.Errorf("torrent: 端点 URL 为空")
	}

	t, err := d.getOrAddTorrent(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("torrent: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.GotInfo():
	}

	reader := t.NewReader()
	if request.OffsetStart > 0 {
		if _, err := reader.Seek(request.OffsetStart, io.SeekStart); err != nil {
			reader.Close()
			return nil, fmt.Errorf("torrent: seek 失败: %w", err)
		}
	}

	return reader, nil
}

func (d *TorrentDriver) getOrAddTorrent(ctx context.Context, url string) (*torrent.Torrent, error) {
	infoHash, isMagnet := parseInfoHash(url)
	d.mu.Lock()
	defer d.mu.Unlock()

	if isMagnet {
		if t, ok := d.torrents[infoHash]; ok {
			return t, nil
		}
		t, err := d.client.AddMagnet(url)
		if err != nil {
			return nil, fmt.Errorf("添加磁力链接失败: %w", err)
		}
		d.torrents[infoHash] = t
		return t, nil
	}

	// .torrent 文件：先通过 HTTP 下载，再解析并添加
	if t, ok := d.torrents[infoHash]; ok {
		return t, nil
	}

	metaInfo, err := fetchTorrentFile(ctx, url)
	if err != nil {
		return nil, err
	}

	hash := metaInfo.HashInfoBytes()
	t, err := d.client.AddTorrent(metaInfo)
	if err != nil {
		return nil, fmt.Errorf("添加 torrent 文件失败: %w", err)
	}
	d.torrents[hash] = t
	return t, nil
}

// parseInfoHash 尝试从 URL 中解析 info hash。如果是磁力链接则返回其 info hash；
// 否则返回零值 hash。
func parseInfoHash(url string) (metainfo.Hash, bool) {
	if strings.HasPrefix(url, "magnet:") {
		m, err := metainfo.ParseMagnetURI(url)
		if err != nil {
			return metainfo.Hash{}, false
		}
		return m.InfoHash, true
	}
	return metainfo.Hash{}, false
}

// fetchTorrentFile 通过 HTTP GET 下载 .torrent 文件并解析为 metainfo.MetaInfo。
func fetchTorrentFile(ctx context.Context, url string) (*metainfo.MetaInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 torrent 文件请求失败: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载 torrent 文件失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("下载 torrent 文件返回状态码 %d", resp.StatusCode)
	}
	metaInfo, err := metainfo.Load(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("解析 torrent 文件失败: %w", err)
	}
	return metaInfo, nil
}
