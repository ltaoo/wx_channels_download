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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/inconshreveable/go-update"
	"github.com/pterm/pterm"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "检查并更新到最新版本",
	Run: func(cmd *cobra.Command, args []string) {
		do_update()
	},
}

func init() {
	root_cmd.AddCommand(updateCmd)
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

	currentVer, err := semver.ParseTolerant(Version)
	if err != nil {
		pterm.Warning.Printf("当前版本号(%s)格式不正确，无法比较版本\n", Version)
	}

	latestVer, err := semver.ParseTolerant(latest.TagName)
	if err != nil {
		pterm.Error.Printf("最新版本号(%s)格式不正确，无法比较版本\n", latest.TagName)
		return
	}

	if currentVer.GE(latestVer) {
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

	assetURL, assetName := find_asset(latest)
	if assetURL == "" {
		pterm.Error.Printf("未找到适用于当前系统 (%s/%s) 的安装包\n", runtime.GOOS, runtime.GOARCH)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		pterm.Error.Println("无法获取当前程序路径:", err)
		return
	}

	pterm.Info.Println("正在下载并更新...")

	// If it's a compressed file, we need to handle it manually
	if strings.HasSuffix(assetName, ".zip") || strings.HasSuffix(assetName, ".tar.gz") || strings.HasSuffix(assetName, ".tgz") {
		if err := update_from_compressed(assetURL, assetName, exe); err != nil {
			pterm.Error.Println("更新失败:", err)
			return
		}
	} else {
		// If it's a binary directly (unlikely based on description but possible)
		spinner, _ := pterm.DefaultSpinner.Start("正在下载并应用更新...")
		if err := selfupdate.UpdateTo(assetURL, exe); err != nil {
			spinner.Fail(fmt.Sprintf("更新失败: %v", err))
			return
		}
		spinner.Success("更新完成")
	}

	pterm.Success.Printf("成功更新至版本 %s\n", latest.TagName)
}

func fetch_releases(slug string) ([]GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases", slug)
	resp, err := http.Get(url)
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

	targetArch := arch
	if arch == "amd64" {
		targetArch = "x86_64"
	} else if arch == "386" {
		targetArch = "x86"
	}

	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, os) && strings.Contains(name, targetArch) {
			return asset.BrowserDownloadURL, asset.Name
		}
	}
	return "", ""
}

// ProgressReader wraps an io.Reader and updates a pterm.ProgressbarPrinter
type ProgressReader struct {
	Reader io.Reader
	Bar    *pterm.ProgressbarPrinter
}

func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	if n > 0 {
		pr.Bar.Add(n)
	}
	return
}

func update_from_compressed(url, filename, exePath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Create progress bar
	bar, _ := pterm.DefaultProgressbar.WithTotal(int(resp.ContentLength)).WithTitle("正在下载").Start()
	defer bar.Stop()

	proxyReader := &ProgressReader{
		Reader: resp.Body,
		Bar:    bar,
	}

	// Read content into memory to avoid temp files if possible,
	// or stream to decompressor.
	// Since we need to find the binary inside, we stream.

	var binaryReader io.Reader

	if strings.HasSuffix(filename, ".zip") {
		// For zip, we need random access, so we have to read it all or save to temp file.
		// Reading to memory is cleaner if not too huge.
		bodyBytes, err := io.ReadAll(proxyReader)
		if err != nil {
			return err
		}

		zipReader, err := zip.NewReader(bytes.NewReader(bodyBytes), int64(len(bodyBytes)))
		if err != nil {
			return err
		}

		for _, file := range zipReader.File {
			if is_executable_file(file.Name) {
				rc, err := file.Open()
				if err != nil {
					return err
				}
				defer rc.Close()
				binaryReader = rc
				break
			}
		}
	} else if strings.HasSuffix(filename, ".tar.gz") || strings.HasSuffix(filename, ".tgz") {
		gzipReader, err := gzip.NewReader(proxyReader)
		if err != nil {
			return err
		}
		defer gzipReader.Close()

		tarReader := tar.NewReader(gzipReader)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}

			if is_executable_file(header.Name) {
				binaryReader = tarReader
				break
			}
		}
	}

	if binaryReader == nil {
		return fmt.Errorf("executable not found in archive")
	}

	return update.Apply(binaryReader, update.Options{})
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
