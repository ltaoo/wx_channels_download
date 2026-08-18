package application

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ltaoo/velo/updater/applier"
	update_util "github.com/ltaoo/velo/updater/util"
	"github.com/pterm/pterm"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"wx_channel/internal/config"
	"wx_channel/internal/services"
	"wx_channel/pkg/hermes"
)

const update_repository = "ltaoo/wx_channels_download"

var apply_update_archive_fn = apply_update_archive_with_velo

type github_release struct {
	TagName     string                 `json:"tag_name"`
	Name        string                 `json:"name"`
	PublishedAt time.Time              `json:"published_at"`
	Body        string                 `json:"body"`
	Assets      []github_release_asset `json:"assets"`
}

type github_release_asset struct {
	URL                string `json:"url"`
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type update_release_manifest struct {
	Version      string                           `json:"version"`
	PublishedAt  string                           `json:"published_at"`
	ReleaseNotes string                           `json:"release_notes"`
	Assets       map[string]update_manifest_asset `json:"assets"`
}

type update_manifest_asset struct {
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
	Name     string `json:"name"`
}

func new_update_service(current_version string, restart_service *services.ApplicationRestartService) *services.UpdateService {
	return services.NewUpdateService(services.UpdateServiceOptions{
		CurrentVersion: current_version,
		Repository:     update_repository,
		FetchReleases:  fetch_service_update_releases,
		DownloadUpdate: download_and_apply_update_with_progress,
		RestartService: restart_service,
	})
}

func fetch_service_update_releases(ctx context.Context, repository string) ([]services.UpdateRelease, error) {
	releases, err := fetch_releases_with_context(ctx, repository)
	if err != nil {
		return nil, err
	}
	result := make([]services.UpdateRelease, 0, len(releases))
	for _, release := range releases {
		assets := make([]services.UpdateAsset, 0, len(release.Assets))
		for _, asset := range release.Assets {
			assets = append(assets, services.UpdateAsset{
				Name:        asset.Name,
				DownloadURL: asset.BrowserDownloadURL,
				Size:        asset.Size,
			})
		}
		result = append(result, services.UpdateRelease{
			Version:     release.TagName,
			Name:        release.Name,
			PublishedAt: release.PublishedAt,
			Body:        release.Body,
			Assets:      assets,
		})
	}
	return result, nil
}

// Update checks for the latest release and replaces the current executable when confirmed.
func Update(current_version string) {
	spinner, _ := pterm.DefaultSpinner.Start("正在检查更新...")

	releases, err := fetch_releases(update_repository)
	if err != nil {
		spinner.Fail(fmt.Sprintf("检查更新失败: %v", err))
		return
	}

	if len(releases) == 0 {
		spinner.Warning("未找到发布版本")
		return
	}

	spinner.Success("检查完成")

	latest := releases[0]

	is_newer, err := update_util.CompareVersions(current_version, latest.TagName)
	if err != nil {
		pterm.Error.Printf("版本号格式不正确，无法比较版本: %v\n", err)
		return
	}

	if !is_newer {
		pterm.Info.Printf("当前已是最新版本: %s\n", current_version)
		return
	}

	pterm.DefaultSection.Println("发现新版本")
	pterm.Info.Printf("最新版本: %s (当前版本: %s)\n", latest.TagName, current_version)
	pterm.Info.Printf("发布时间: %s\n", latest.PublishedAt.Format("2006-01-02 15:04:05"))
	pterm.Println()
	pterm.Println(pterm.Yellow("发布说明:"))
	pterm.Println(latest.Body)
	pterm.Println()

	result, _ := pterm.DefaultInteractiveConfirm.Show("是否开始更新?")
	if !result {
		pterm.Info.Println("已取消更新")
		return
	}

	asset_url, asset_name := find_asset(latest)
	if asset_url == "" {
		pterm.Error.Printf("未找到适用于当前系统 (%s/%s) 的安装包\n", runtime.GOOS, runtime.GOARCH)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		pterm.Error.Println("无法获取当前程序路径:", err)
		return
	}

	pterm.Info.Println("正在下载并更新...")

	if err := download_and_apply_update(asset_url, asset_name, exe); err != nil {
		pterm.Error.Println("更新失败:", err)
		return
	}

	pterm.Success.Printf("成功更新至版本 %s\n", latest.TagName)
}

func fetch_releases(slug string) ([]github_release, error) {
	return fetch_releases_with_context(context.Background(), slug)
}

func fetch_releases_with_context(ctx context.Context, slug string) ([]github_release, error) {
	sources, err := config.LoadUpdateSources()
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		sources = []config.UpdateSource{{
			Type:       "github",
			Priority:   1,
			GitHubRepo: slug,
			Enabled:    true,
		}}
	}
	sort.SliceStable(sources, func(first int, second int) bool {
		return sources[first].Priority < sources[second].Priority
	})

	source_errors := make([]string, 0, len(sources))
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		var releases []github_release
		switch strings.ToLower(strings.TrimSpace(source.Type)) {
		case "github":
			releases, err = fetch_github_releases(ctx, source)
		case "http":
			releases, err = fetch_http_release_manifest(ctx, source)
		default:
			err = fmt.Errorf("unsupported update source type %q", source.Type)
		}
		if err == nil {
			return releases, nil
		}
		if ctx_err := ctx.Err(); ctx_err != nil {
			return nil, ctx_err
		}
		source_errors = append(source_errors, err.Error())
	}
	if len(source_errors) == 0 {
		return nil, fmt.Errorf("no enabled update sources configured")
	}
	return nil, fmt.Errorf("all update sources failed: %s", strings.Join(source_errors, "; "))
}

func fetch_github_releases(ctx context.Context, source config.UpdateSource) ([]github_release, error) {
	raw_url, err := github_releases_url(source)
	if err != nil {
		return nil, err
	}
	var releases []github_release
	if err := fetch_update_json(ctx, raw_url, source.GitHubToken, &releases); err != nil {
		return nil, err
	}
	return releases, nil
}

func github_releases_url(source config.UpdateSource) (string, error) {
	repository := strings.Trim(strings.TrimSpace(source.GitHubRepo), "/")
	repository_parts := strings.Split(repository, "/")
	if len(repository_parts) != 2 || repository_parts[0] == "" || repository_parts[1] == "" {
		return "", fmt.Errorf("invalid GitHub update repository %q", source.GitHubRepo)
	}
	self_url := strings.TrimSpace(source.SelfURL)
	if self_url == "" {
		self_url = "https://api.github.com"
	}
	parsed_url, err := parse_update_source_url(self_url)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(strings.TrimRight(parsed_url.Path, "/"), "/releases") {
		return strings.TrimRight(self_url, "/"), nil
	}
	joined_url, err := url.JoinPath(self_url, "repos", repository_parts[0], repository_parts[1], "releases")
	if err != nil {
		return "", fmt.Errorf("build GitHub release URL: %w", err)
	}
	return joined_url, nil
}

func fetch_http_release_manifest(ctx context.Context, source config.UpdateSource) ([]github_release, error) {
	manifest_url := strings.TrimSpace(source.ManifestURL)
	if manifest_url == "" {
		return nil, fmt.Errorf("HTTP update source is missing manifest_url")
	}
	if _, err := parse_update_source_url(manifest_url); err != nil {
		return nil, err
	}
	var manifest update_release_manifest
	if err := fetch_update_json(ctx, manifest_url, "", &manifest); err != nil {
		return nil, err
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return nil, fmt.Errorf("update manifest is missing version")
	}
	published_at := time.Time{}
	if manifest.PublishedAt != "" {
		parsed_time, err := time.Parse(time.RFC3339, manifest.PublishedAt)
		if err != nil {
			return nil, fmt.Errorf("invalid update manifest published_at: %w", err)
		}
		published_at = parsed_time
	}
	assets := make([]github_release_asset, 0, len(manifest.Assets))
	for platform_name, manifest_asset := range manifest.Assets {
		asset_name := strings.TrimSpace(manifest_asset.Name)
		if asset_name == "" {
			asset_name = platform_name
		}
		assets = append(assets, github_release_asset{
			Name:               asset_name,
			BrowserDownloadURL: manifest_asset.URL,
			Size:               manifest_asset.Size,
		})
	}
	return []github_release{{
		TagName:     manifest.Version,
		Name:        manifest.Version,
		PublishedAt: published_at,
		Body:        manifest.ReleaseNotes,
		Assets:      assets,
	}}, nil
}

func fetch_update_json(ctx context.Context, raw_url string, token string, target any) error {
	if _, err := parse_update_source_url(raw_url); err != nil {
		return err
	}
	req_url := apply_mirror(raw_url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, req_url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	client := create_update_http_client()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update source %s returned status: %s", raw_url, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return err
	}
	return nil
}

func parse_update_source_url(raw_url string) (*url.URL, error) {
	raw_url = strings.TrimSpace(raw_url)
	parsed_url, err := url.Parse(raw_url)
	if err != nil || parsed_url.Scheme == "" {
		return nil, fmt.Errorf("invalid update release URL %q", raw_url)
	}
	protocol_name := strings.ToLower(parsed_url.Scheme)
	if protocol_name != "http" && protocol_name != "https" {
		return nil, fmt.Errorf("unsupported update release URL protocol %q", protocol_name)
	}
	if parsed_url.Host == "" {
		return nil, fmt.Errorf("invalid update release URL %q", raw_url)
	}
	return parsed_url, nil
}

func find_asset(release github_release) (string, string) {
	asset, ok := find_release_asset(release)
	if !ok {
		return "", ""
	}
	return asset.BrowserDownloadURL, asset.Name
}

func find_release_asset(release github_release) (github_release_asset, bool) {
	os := runtime.GOOS
	arch := runtime.GOARCH

	target_arch := arch
	if arch == "amd64" {
		target_arch = "x86_64"
	} else if arch == "386" {
		target_arch = "x86"
	}

	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, os) && strings.Contains(name, target_arch) {
			return asset, true
		}
	}
	return github_release_asset{}, false
}

func download_update_asset_with_hermes(
	raw_url string,
	filename string,
	proxy_server hermes.ProxyServer,
	on_progress func(*hermes.TaskProgress),
) (string, func(), error) {
	parsed_url, err := url.Parse(raw_url)
	if err != nil || parsed_url.Scheme == "" || parsed_url.Host == "" {
		return "", nil, fmt.Errorf("invalid update download URL")
	}
	protocol_name := strings.ToLower(parsed_url.Scheme)
	if protocol_name != "http" && protocol_name != "https" {
		return "", nil, fmt.Errorf("unsupported update download protocol %q", protocol_name)
	}

	asset_name := filepath.Base(strings.TrimSpace(filename))
	if asset_name == "" || asset_name == "." || asset_name == string(filepath.Separator) {
		return "", nil, fmt.Errorf("invalid update asset filename")
	}

	temp_dir, err := os.MkdirTemp("", "wx-channels-update-*")
	if err != nil {
		return "", nil, fmt.Errorf("create update download directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(temp_dir) }
	succeeded := false
	defer func() {
		if !succeeded {
			cleanup()
		}
	}()

	downloader := hermes.New(hermes.HermesNewConfig{
		Config: hermes.HermesEngineConfig{
			MaxConcurrent: 1,
			BasePath:      temp_dir,
		},
	})

	var terminal atomic.Bool
	downloader.OnEvent(func(event hermes.EventType, data hermes.EventData) {
		if event == hermes.EventProgress && !terminal.Load() && on_progress != nil {
			event_data, ok := data.(hermes.TaskProgressEventData)
			if ok && event_data.Progress != nil {
				on_progress(event_data.Progress)
			}
		}
		if event == hermes.EventFinished || event == hermes.EventFailed {
			terminal.Store(true)
		}
	})

	task := downloader.CreateTask(
		raw_url,
		hermes.WithFilename(asset_name),
		hermes.WithProxyServer(proxy_server),
	)
	if err := task.Wait(); err != nil {
		terminal.Store(true)
		return "", nil, fmt.Errorf("download update: %w", err)
	}
	terminal.Store(true)

	asset_path := task.FilePath()
	info, err := os.Stat(asset_path)
	if err != nil {
		return "", nil, fmt.Errorf("open downloaded update: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("downloaded update is not a regular file")
	}

	succeeded = true
	return asset_path, cleanup, nil
}

type update_hermes_progress_bar struct {
	mu         sync.Mutex
	bar        *pterm.ProgressbarPrinter
	downloaded int64
	stopped    bool
}

func (p *update_hermes_progress_bar) update(progress *hermes.TaskProgress) {
	if progress == nil || progress.TotalSize <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return
	}
	if p.bar == nil {
		bar, err := pterm.DefaultProgressbar.
			WithTotal(int(progress.TotalSize)).
			WithTitle("正在下载").
			Start()
		if err != nil {
			return
		}
		p.bar = bar
	}
	delta := progress.Downloaded - p.downloaded
	if delta > 0 {
		p.bar.Add(int(delta))
		p.downloaded = progress.Downloaded
	}
}

func (p *update_hermes_progress_bar) stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = true
	if p.bar != nil {
		_, _ = p.bar.Stop()
	}
}

func download_and_apply_update(raw_url, filename, exe_path string) error {
	progress := &update_hermes_progress_bar{}
	err := download_and_apply_update_with_progress(
		raw_url,
		filename,
		exe_path,
		progress.update,
	)
	progress.stop()
	return err
}

func download_and_apply_update_with_progress(
	raw_url string,
	filename string,
	exe_path string,
	on_progress func(*hermes.TaskProgress),
) error {
	req_url := apply_mirror(raw_url)
	asset_path, cleanup, err := download_update_asset_with_hermes(
		req_url,
		filename,
		hermes.ProxyServer{Address: viper.GetString("update.proxy")},
		on_progress,
	)
	if err != nil {
		return err
	}
	defer cleanup()

	return apply_downloaded_update(asset_path, exe_path)
}

func apply_downloaded_update(update_path string, exe_path string) error {
	if err := apply_update_archive_fn(update_path, exe_path); err != nil {
		return fmt.Errorf("apply update with Velo: %w", err)
	}
	return nil
}

func apply_update_archive_with_velo(update_path string, exe_path string) error {
	logger := zerolog.Nop()
	update_applier := applier.NewPlatformUpdater(&logger)
	return update_applier.Apply(update_path, exe_path)
}

func create_update_http_client() *http.Client {
	proxy_url := viper.GetString("update.proxy")
	if proxy_url == "" {
		return http.DefaultClient
	}
	proxy, err := url.Parse(proxy_url)
	if err != nil {
		return http.DefaultClient
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxy),
		},
	}
}

func apply_mirror(raw_url string) string {
	mirror := viper.GetString("update.mirror")
	if mirror == "" {
		return raw_url
	}
	mirror = strings.TrimSuffix(mirror, "/")
	return mirror + "/" + raw_url
}
