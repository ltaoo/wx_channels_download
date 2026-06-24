package interceptor

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const defaultChannelInjectDir = "internal/interceptor/inject"

type ChannelInjectedFiles struct {
	InjectDir               string
	LibFS                   fs.FS
	SrcFS                   fs.FS
	JSFileSaver             []byte
	JSZip                   []byte
	JSRecorder              []byte
	JSPageSpy               []byte
	JSBox                   []byte
	JSMitt                  []byte
	JSAxios                 []byte
	JSGetFeedInfo           []byte
	JSDebug                 []byte
	JSEventBus              []byte
	JSEnv                   []byte
	JSEnvChannels           []byte
	JSEnvMock               []byte
	JSTimeless              []byte
	JSTimelessUtils         []byte
	CSSTimelessShadcn       []byte
	JSTimelessShadcn        []byte
	JSTimelessDOM           []byte
	JSTimelessWeb           []byte
	CSSComponents           []byte
	JSComponents            []byte
	JSChannels              []byte
	JSDownloadCore          []byte
	JSDownloadPanel         []byte
	JSDownloadIndex         []byte
	JSDownloader            []byte
	JSUtils                 []byte
	JSError                 []byte
	JSHomePage              []byte
	JSFeedProfilePage       []byte
	JSLiveProfilePage       []byte
	JSContactPage           []byte
	JSWechatOfficialAccount []byte
}

var Assets = NewChannelInjectedFiles("")

func NewChannelInjectedFiles(injectDir string) *ChannelInjectedFiles {
	if injectDir == "" {
		injectDir = findChannelInjectDir()
	}
	if abs, err := filepath.Abs(injectDir); err == nil {
		injectDir = abs
	}
	files := &ChannelInjectedFiles{
		InjectDir: injectDir,
		LibFS:     os.DirFS(filepath.Join(injectDir, "lib")),
		SrcFS:     os.DirFS(filepath.Join(injectDir, "src")),
	}
	files.JSFileSaver = files.readLib("FileSaver.min.js")
	files.JSZip = files.readLib("jszip.min.js")
	files.JSRecorder = files.readLib("recorder.min.js")
	files.JSPageSpy = files.readLib("pagespy.min.js")
	files.JSMitt = files.readLib("mitt.umd.js")
	files.JSAxios = files.readLib("axios.min.js")
	files.JSGetFeedInfo = files.readLib("getFeedInfo.js")
	files.JSTimeless = files.readLib("timeless/0.26.3/timeless.umd.min.js")
	files.JSTimelessUtils = files.readLib("timeless/0.26.3/timeless.utils.umd.min.js")
	files.CSSTimelessShadcn = files.readLib("timeless/0.26.3/timeless.shadcn.css")
	files.JSTimelessShadcn = files.readLib("timeless/0.26.3/timeless.shadcn.umd.min.js")
	files.JSTimelessDOM = files.readLib("timeless/0.26.3/timeless.dom.umd.min.js")
	files.JSTimelessWeb = files.readLib("timeless/0.26.3/timeless.web.umd.min.js")
	files.CSSComponents = files.readSrc("components.css")
	files.JSDebug = files.readSrc("pagespy.js")
	files.JSError = files.readSrc("error.js")
	files.JSEventBus = files.readSrc("eventbus.js")
	files.JSEnv = files.readSrc("env.js")
	files.JSEnvChannels = files.readSrc("env.channels.js")
	files.JSEnvMock = files.readSrc("env.mock.js")
	files.JSComponents = files.readSrc("components.js")
	files.JSUtils = files.readSrc("utils.js")
	files.JSChannels = files.readSrc("channels.js")
	files.JSDownloadCore = files.readSrc("download/core.js")
	files.JSDownloadPanel = files.readSrc("download/panel.js")
	files.JSDownloadIndex = files.readSrc("download/index.js")
	files.JSDownloader = files.JSDownloadPanel
	files.JSWechatOfficialAccount = files.readSrc("officialaccount.js")
	files.JSHomePage = files.readSrc("home.js")
	files.JSFeedProfilePage = files.readSrc("feed.js")
	files.JSLiveProfilePage = files.readSrc("live.js")
	files.JSContactPage = files.readSrc("profile.js")
	return files
}

func (files *ChannelInjectedFiles) ReadLib(rel string) ([]byte, error) {
	return readChannelAsset(files.LibFS, rel)
}

func (files *ChannelInjectedFiles) ReadSrc(rel string) ([]byte, error) {
	return readChannelAsset(files.SrcFS, rel)
}

func (files *ChannelInjectedFiles) readLib(rel string) []byte {
	data, _ := files.ReadLib(rel)
	return data
}

func (files *ChannelInjectedFiles) readSrc(rel string) []byte {
	data, _ := files.ReadSrc(rel)
	return data
}

func readChannelAsset(assetFS fs.FS, rel string) ([]byte, error) {
	clean, ok := cleanChannelAssetRel(rel)
	if !ok {
		return nil, fs.ErrInvalid
	}
	return fs.ReadFile(assetFS, clean)
}

func cleanChannelAssetRel(rel string) (string, bool) {
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.Contains(rel, "..") || strings.ContainsRune(rel, 0) {
		return "", false
	}
	clean := path.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", false
	}
	return clean, true
}

func findChannelInjectDir() string {
	candidates := []string{defaultChannelInjectDir}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "inject"),
			filepath.Join(exeDir, defaultChannelInjectDir),
		)
	}
	for _, candidate := range candidates {
		if stat, err := os.Stat(filepath.Join(candidate, "lib")); err == nil && stat.IsDir() {
			return candidate
		}
	}
	return defaultChannelInjectDir
}
