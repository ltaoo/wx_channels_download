package cmd

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blang/semver"
	"github.com/inconshreveable/go-update"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"wx_channel/pkg/hermes"
)

var update_cmd = &cobra.Command{
	Use:   "update",
	Short: "检查并更新到最新版本",
	Run: func(cmd *cobra.Command, args []string) {
		do_update()
	},
}

func init() {
	root_cmd.AddCommand(update_cmd)
}

type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	Body        string    `json:"body"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func do_update() {
	spinner, _ := pterm.DefaultSpinner.Start("正在检查更新...")

	releases, err := fetch_releases("ltaoo/wx_channels_download")
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

	current_ver, err := semver.ParseTolerant(Version)
	if err != nil {
		pterm.Warning.Printf("当前版本号(%s)格式不正确，无法比较版本\n", Version)
	}

	latest_ver, err := semver.ParseTolerant(latest.TagName)
	if err != nil {
		pterm.Error.Printf("最新版本号(%s)格式不正确，无法比较版本\n", latest.TagName)
		return
	}

	if current_ver.GE(latest_ver) {
		pterm.Info.Printf("当前已是最新版本: %s\n", Version)
		return
	}

	pterm.DefaultSection.Println("发现新版本")
	pterm.Info.Printf("最新版本: %s (当前版本: %s)\n", latest.TagName, Version)
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

func fetch_releases(slug string) ([]GitHubRelease, error) {
	raw_url := fmt.Sprintf("https://api.github.com/repos/%s/releases", slug)
	req_url := apply_mirror(raw_url)
	client := create_update_http_client()
	resp, err := client.Get(req_url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	return releases, nil
}

func find_asset(release GitHubRelease) (string, string) {
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
			return asset.BrowserDownloadURL, asset.Name
		}
	}
	return "", ""
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
	req_url := apply_mirror(raw_url)
	progress := &update_hermes_progress_bar{}
	asset_path, cleanup, err := download_update_asset_with_hermes(
		req_url,
		filename,
		hermes.ProxyServer{Address: viper.GetString("update.proxy")},
		progress.update,
	)
	progress.stop()
	if err != nil {
		return err
	}
	defer cleanup()

	source, err := os.Open(asset_path)
	if err != nil {
		return fmt.Errorf("open downloaded update: %w", err)
	}
	defer source.Close()

	return apply_downloaded_update(source, filename, exe_path)
}

func apply_downloaded_update(source io.Reader, filename, exe_path string) error {
	var binary_reader io.Reader
	lower_filename := strings.ToLower(filename)

	if strings.HasSuffix(lower_filename, ".zip") {
		// For zip, we need random access, so we have to read it all or save to temp file.
		// Reading to memory is cleaner if not too huge.
		body_bytes, err := io.ReadAll(source)
		if err != nil {
			return fmt.Errorf("read zip update: %w", err)
		}

		zip_reader, err := zip.NewReader(bytes.NewReader(body_bytes), int64(len(body_bytes)))
		if err != nil {
			return fmt.Errorf("open zip update: %w", err)
		}

		for _, file := range zip_reader.File {
			if is_executable_file(file.Name) {
				rc, err := file.Open()
				if err != nil {
					return fmt.Errorf("open executable in zip update: %w", err)
				}
				defer rc.Close()
				binary_reader = rc
				break
			}
		}
	} else if strings.HasSuffix(lower_filename, ".tar.gz") || strings.HasSuffix(lower_filename, ".tgz") {
		gzip_reader, err := gzip.NewReader(source)
		if err != nil {
			return fmt.Errorf("open gzip update: %w", err)
		}
		defer gzip_reader.Close()

		tar_reader := tar.NewReader(gzip_reader)
		for {
			header, err := tar_reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("read tar update: %w", err)
			}

			if is_executable_file(header.Name) {
				binary_reader = tar_reader
				break
			}
		}
	} else {
		binary_reader = source
	}

	if binary_reader == nil {
		return fmt.Errorf("executable not found in archive")
	}

	if err := update.Apply(binary_reader, update.Options{TargetPath: exe_path}); err != nil {
		return fmt.Errorf("apply update: %w", err)
	}
	return nil
}

func is_executable_file(name string) bool {
	// Simple check: name matches our binary name "wx_channels_download" or "wx_video_download"
	// or ends with .exe on windows
	base := filepath.Base(name)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(base, "wx_channels_download.exe") || strings.EqualFold(base, "wx_video_download.exe")
	}
	return base == "wx_channels_download" || base == "wx_video_download"
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
