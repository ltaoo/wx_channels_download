package wxchannelsadapter

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/scraper/wxchannels"
)

const sharedPictureFeedJSON = `{
  "data": {
    "authorInfo": {
      "nickname": "一帆读书",
      "headImgUrl": "https://wx.qlogo.cn/finderhead/avatar/0"
    },
    "feedInfo": {
      "picInfo": [
        {"url": "https://finder.video.qq.com/251/20304/stodownload?encfilekey=pic1&token=token1"},
        {"url": "https://finder.video.qq.com/251/20304/stodownload?encfilekey=pic2&token=token2"}
      ],
      "description": "王朔谈于丹",
      "mediaType": 2,
      "bgmInfo": {
        "bgmUrl": "https://finder.video.qq.com/251/20305/stodownload?encfilekey=bgm&token=token3"
      },
      "createtime": 1783650796,
      "coverUrl": "https://finder.video.qq.com/251/20350/stodownload?encfilekey=cover&token=token4"
    },
    "errMsg": {"type": 0},
    "sceneInfo": {
      "dynamicExportId": "export/UzFfBgAAxIynPFo0GGG3"
    }
  },
  "errCode": 0,
  "errMsg": ""
}`

const videoFeedJSON = `{
  "id": "feed123",
  "objectNonceId": "nonce123",
  "createtime": 1783650796,
  "contact": {
    "username": "finder_user",
    "nickname": "作者"
  },
  "objectDesc": {
    "description": "视频标题",
    "mediaType": 4,
    "media": [{
      "url": "https://finder.video.qq.com/video.mp4?encfilekey=abc&token=tok",
      "urlToken": "",
      "thumbUrl": "https://example.com/thumb.jpg",
      "width": 1920,
      "height": 1080,
      "fileSize": 12345,
      "videoPlayLen": 60000,
      "decodeKey": "123",
      "spec": [{"fileFormat": "1080p"}]
    }]
  }
}`

func TestBuildDownloadTaskSharedPictureFeedCreatesZipResources(t *testing.T) {
	info, err := NewChannelsAdapter().BuildDownloadTask(json.RawMessage(sharedPictureFeedJSON), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("BuildDownloadTask() error = %v", err)
	}
	if info == nil || info.Task == nil {
		t.Fatal("BuildDownloadTask() returned nil task info")
	}
	if got, want := info.Task.UniqueID, "export_UzFfBgAAxIynPFo0GGG3"; got != want {
		t.Fatalf("task UniqueID = %q, want %q", got, want)
	}
	if got, want := len(info.Resources), 3; got != want {
		t.Fatalf("resource count = %d, want %d", got, want)
	}
	if got, want := info.Resources[0].Endpoints[0].URL, "https://finder.video.qq.com/251/20304/stodownload?encfilekey=pic1&token=token1"; got != want {
		t.Fatalf("first picture URL = %q, want %q", got, want)
	}
	if got, want := info.Resources[2].Name, "bgm"; got != want {
		t.Fatalf("bgm resource name = %q, want %q", got, want)
	}
	if got, want := info.Resources[2].Endpoints[0].Protocol, "https"; got != want {
		t.Fatalf("bgm endpoint protocol = %q, want %q", got, want)
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(info.Task.ConfigJSON), &cfg); err != nil {
		t.Fatalf("ConfigJSON unmarshal error = %v", err)
	}
	if got, want := cfg["suffix"], ".zip"; got != want {
		t.Fatalf("config suffix = %v, want %v", got, want)
	}
	if got, want := int(cfg["type"].(float64)), wxchannels.MediaTypePicture; got != want {
		t.Fatalf("config type = %d, want %d", got, want)
	}
	if info.Content == nil || info.Content.Type != "album" {
		t.Fatalf("content type = %v, want album", info.Content)
	}
	if got, want := len(info.AlbumImages), 2; got != want {
		t.Fatalf("album image count = %d, want %d", got, want)
	}
}

func TestBuildDownloadTaskSharedPictureFeedWrappedResponse(t *testing.T) {
	raw := json.RawMessage(`{"code":0,"msg":"成功","data":` + sharedPictureFeedJSON + `}`)
	info, err := NewChannelsAdapter().BuildDownloadTask(raw, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("BuildDownloadTask() wrapped response error = %v", err)
	}
	if got, want := len(info.Resources), 3; got != want {
		t.Fatalf("wrapped response resource count = %d, want %d", got, want)
	}
}

func TestBuildDownloadTaskPreconstructsSingleResourceTaskNameWithoutTemplate(t *testing.T) {
	info, err := NewChannelsAdapter().BuildDownloadTask(
		json.RawMessage(videoFeedJSON),
		json.RawMessage(`{"spec":"1080p"}`),
	)
	if err != nil {
		t.Fatalf("BuildDownloadTask() error = %v", err)
	}
	if info == nil || info.Task == nil {
		t.Fatal("BuildDownloadTask() returned nil task info")
	}
	if got, want := info.Task.Name, "视频标题.mp4"; got != want {
		t.Fatalf("task name = %q, want %q", got, want)
	}
}

func TestBuildDownloadTaskPreconstructsSingleResourceTaskName(t *testing.T) {
	info, err := NewChannelsAdapter().BuildDownloadTask(
		json.RawMessage(videoFeedJSON),
		json.RawMessage(`{"spec":"1080p","filename_template":"{{author}}/{{filename}}_{{spec}}"}`),
	)
	if err != nil {
		t.Fatalf("BuildDownloadTask() error = %v", err)
	}
	if info == nil || info.Task == nil {
		t.Fatal("BuildDownloadTask() returned nil task info")
	}
	if got, want := info.Task.Name, "作者/视频标题_1080p.mp4"; got != want {
		t.Fatalf("task name = %q, want %q", got, want)
	}
	if got, want := len(info.Resources), 1; got != want {
		t.Fatalf("resource count = %d, want %d", got, want)
	}
	if got, want := info.Resources[0].Name, "视频标题"; got != want {
		t.Fatalf("resource name = %q, want unchanged %q", got, want)
	}
}

func TestPostprocessZipSuffixArchivesResources(t *testing.T) {
	basePath := t.TempDir()
	savePath := "downloads"
	saveDir := filepath.Join(basePath, savePath)
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	imagePath := filepath.Join(saveDir, "image_1")
	audioPath := filepath.Join(saveDir, "bgm")
	if err := os.WriteFile(imagePath, []byte("image"), 0644); err != nil {
		t.Fatalf("WriteFile(image) error = %v", err)
	}
	if err := os.WriteFile(audioPath, []byte("audio"), 0644); err != nil {
		t.Fatalf("WriteFile(audio) error = %v", err)
	}

	task := &hermes.TaskJob{
		ID:       1,
		Name:     "album",
		UniqueID: "shared_picture",
		Platform: PlatformID,
		SavePath: savePath,
		Config: map[string]any{
			"type":   wxchannels.MediaTypePicture,
			"suffix": ".zip",
		},
		Resources: []hermes.ResourceJob{
			{ID: 11, Name: "image_1", Kind: "image/jpeg", Type: "FILE", UniqueID: "image_1", FilePath: imagePath, Extra: map[string]string{}},
			{ID: 12, Name: "bgm", Kind: "audio/mpeg", Type: "FILE", UniqueID: "bgm", FilePath: audioPath, Extra: map[string]string{}},
		},
	}
	logger := zerolog.Nop()
	if err := NewChannelsAdapter().Postprocess(context.Background(), task, adapter.PostprocessDeps{Logger: logger, BasePath: basePath}); err != nil {
		t.Fatalf("Postprocess() error = %v", err)
	}
	if got, want := len(task.Resources), 1; got != want {
		t.Fatalf("resource count after postprocess = %d, want %d", got, want)
	}
	archivePath := task.Resources[0].FilePath
	if archivePath == "" {
		t.Fatal("archive FilePath is empty")
	}
	if _, err := os.Stat(imagePath); !os.IsNotExist(err) {
		t.Fatalf("image source stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(audioPath); !os.IsNotExist(err) {
		t.Fatalf("audio source stat error = %v, want not exist", err)
	}
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("OpenReader(zip) error = %v", err)
	}
	defer reader.Close()
	if got, want := len(reader.File), 2; got != want {
		t.Fatalf("zip entry count = %d, want %d", got, want)
	}
	if got, want := reader.File[0].Name, "image_1.jpg"; got != want {
		t.Fatalf("first zip entry = %q, want %q", got, want)
	}
	if got, want := reader.File[1].Name, "bgm.mp3"; got != want {
		t.Fatalf("second zip entry = %q, want %q", got, want)
	}
}
