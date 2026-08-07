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

	"wx_channel/internal/config"
	"wx_channel/pkg/hermes"
)

type update_command_config struct {
	proxy  string
	mirror string
}

func run_update(version string, args []string) error {
	flags := new_command_flag_set("update", "检查并安装最新版本")
	var config_filepath string
	add_config_flags(flags, &config_filepath)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := reject_command_args(flags); err != nil {
		return err
	}
	cfg := config.New(config_filepath, nil)
	if err := cfg.LoadConfig(); err != nil {
		return err
	}
	settings := update_command_config{
		proxy:  cfg.GetString("update.proxy"),
		mirror: cfg.GetString("update.mirror"),
	}
	do_update(version, settings)
	return nil
}

type github_release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	Body        string    `json:"body"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func do_update(version string, settings update_command_config) {
	spinner, _ := pterm.DefaultSpinner.Start("Checking for updates...")

	releases, err := fetch_releases("ltaoo/wx_channels_download", settings)
	if err != nil {
		spinner.Fail(fmt.Sprintf("Failed to check for updates: %v", err))
		return
	}

	if len(releases) == 0 {
		spinner.Warning("No releases found")
		return
	}

	spinner.Success("Update check completed")

	latest := releases[0]

	current_ver, err := semver.ParseTolerant(version)
	if err != nil {
		pterm.Warning.Printf("Current version (%s) is invalid and cannot be compared\n", version)
	}

	latest_ver, err := semver.ParseTolerant(latest.TagName)
	if err != nil {
		pterm.Error.Printf("Latest version (%s) is invalid and cannot be compared\n", latest.TagName)
		return
	}

	if current_ver.GE(latest_ver) {
		pterm.Info.Printf("You are already using the latest version: %s\n", version)
		return
	}

	pterm.DefaultSection.Println("New version available")
	pterm.Info.Printf("Latest version: %s (current version: %s)\n", latest.TagName, version)
	pterm.Info.Printf("Published at: %s\n", latest.PublishedAt.Format("2006-01-02 15:04:05"))
	pterm.Println()
	pterm.Println(pterm.Yellow("Release notes:"))
	pterm.Println(latest.Body)
	pterm.Println()

	result, _ := pterm.DefaultInteractiveConfirm.Show("Start the update?")
	if !result {
		pterm.Info.Println("Update canceled")
		return
	}

	asset_url, asset_name := find_asset(latest)
	if asset_url == "" {
		pterm.Error.Printf("No package found for the current system (%s/%s)\n", runtime.GOOS, runtime.GOARCH)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		pterm.Error.Println("Unable to locate the current executable:", err)
		return
	}

	pterm.Info.Println("Downloading and installing the update...")

	if err := download_and_apply_update(asset_url, asset_name, exe, settings); err != nil {
		pterm.Error.Println("Update failed:", err)
		return
	}

	pterm.Success.Printf("Successfully updated to version %s\n", latest.TagName)
}

func fetch_releases(slug string, settings update_command_config) ([]github_release, error) {
	raw_url := fmt.Sprintf("https://api.github.com/repos/%s/releases", slug)
	req_url := apply_mirror(raw_url, settings.mirror)
	client := create_update_http_client(settings.proxy)
	resp, err := client.Get(req_url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var releases []github_release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	return releases, nil
}

func find_asset(release github_release) (string, string) {
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
	downloader.OnEvent(func(_ int, event hermes.EventType, progress *hermes.TaskProgress) {
		if event == hermes.EventProgress && !terminal.Load() && progress != nil && on_progress != nil {
			on_progress(progress)
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
			WithTitle("Downloading").
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

func download_and_apply_update(raw_url, filename, exe_path string, settings update_command_config) error {
	req_url := apply_mirror(raw_url, settings.mirror)
	progress := &update_hermes_progress_bar{}
	asset_path, cleanup, err := download_update_asset_with_hermes(
		req_url,
		filename,
		hermes.ProxyServer{Address: settings.proxy},
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

func create_update_http_client(proxy_url string) *http.Client {
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

func apply_mirror(raw_url string, mirror string) string {
	if mirror == "" {
		return raw_url
	}
	mirror = strings.TrimSuffix(mirror, "/")
	return mirror + "/" + raw_url
}
