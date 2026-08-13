package wxchannels_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"wx_channel/internal/adapter"
	wxchannels "wx_channel/internal/adapter/wxchannels"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
)

const album_fixture_path = "wxchannels/260810/album.json"

const expected_album_external_id = "14936583787284728000"
const expected_album_nonce_id = "8189655580179332358_0_146_0_0"
const expected_album_title = "锦鲤上岸，事事皆如意！#锦鲤#锦妆阁汉服旅拍#杭州#锦妆阁#汉服"
const expected_album_account_external_id = "v2_060000231003b20faec8c5eb8c1dc3d5cc02ec34b0777fde89138921d31325e189f6ac106494@finder"
const expected_album_account_nickname = "锦妆阁汉服旅拍"
const expected_album_account_signature = "📍杭州西湖国贸中心424 锦妆阁\n📞️19560433888\n🫘锦妆阁官方号（杭州西湖店）\n🍠锦妆阁汉服旅拍（杭州西湖店）\n📩💗预约可加V\n其他暂未入驻～💓"
const expected_album_account_avatar_url = "https://wx.qlogo.cn/finderhead/ver_1/Dg9Z4iclgTs8np2IEBAqiam2pdiaNtpDfWUVIH5ffr64mQXufDcNboc8E1ComYbMnYj5HVqdD1PfFUPGwnHjnlQTuofZ2OqtalnqOUibV3EcZj9r6DvCwh27OFO42JXfKYicC6IrIuqTzar3o3ib4Kl3Dyqw/132"
const expected_album_metadata = `{"key":""}`
const expected_album_bgm_name = "洛春赋"
const expected_album_bgm_url = "http://wx.music.tc.qq.com/RS0400168uEl18VckC.mp3?guid=2000000354&vkey=32B820E93D0A17EA583C29732AB5F7E1D966B92069385F89B034B39F4E0B67B6EE5B2851CE1DEB6F56A652E1D0F59073F4F322F525D4A0D4__v2ba7600b&uin=0&fromtag=99010354&trace=6423019dd5b41dcd"

var (
	expected_album_content_id   = wxchannels.BuildContentID(expected_album_external_id)
	expected_album_publish_time = int64(1780579541)
	expected_album_source_url   = wxchannels.BuildJumpURLFromParts(expected_album_external_id, expected_album_nonce_id, "", expected_album_account_external_id)
)

func load_album_object(t *testing.T) (wxchannels.ChannelsObject, []byte) {
	t.Helper()
	raw, err := load_fixture(album_fixture_path)
	if err != nil {
		t.Fatalf("load album fixture: %v", err)
	}
	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(obj.Files) != 0 {
		t.Fatalf("album fixture Files length = %d, want 0 to exercise objectDesc.media fallback", len(obj.Files))
	}
	if len(obj.ObjectDesc.Media) != 9 {
		t.Fatalf("album fixture media length = %d, want 9", len(obj.ObjectDesc.Media))
	}
	return obj, raw
}

func expected_album_account_model() model.Account {
	return model.Account{
		Id:         wxchannels.BuildAccountID(expected_album_account_external_id),
		PlatformId: "wxchannels",
		ExternalId: expected_album_account_external_id,
		Nickname:   expected_album_account_nickname,
		Signature:  expected_album_account_signature,
		AvatarURL:  expected_album_account_avatar_url,
	}
}

func expected_album_content_model(obj *wxchannels.ChannelsObject) model.Content {
	first := obj.ObjectDesc.Media[0]
	return model.Content{
		Id:          expected_album_content_id,
		PlatformId:  "wxchannels",
		Type:        "album",
		ExternalId:  expected_album_external_id,
		ExternalId2: expected_album_nonce_id,
		Title:       expected_album_title,
		Description: expected_album_title,
		CoverURL:    first.URL + first.URLToken,
		CoverWidth:  "954",
		CoverHeight: "636",
		PublishTime: &expected_album_publish_time,
		Metadata:    expected_album_metadata,
	}
}

func expected_album_images(obj *wxchannels.ChannelsObject) []model.ContentImage {
	files := obj.ObjectDesc.Media
	images := make([]model.ContentImage, 0, len(files))
	for i, file := range files {
		images = append(images, model.ContentImage{
			AlbumId:   expected_album_content_id,
			SortOrder: i,
			URL:       file.URL + file.URLToken,
			Width:     int(file.Width),
			Height:    int(file.Height),
			Size:      int64(file.FileSize),
			ImageType: model.ContentImageTypeStill,
		})
	}
	return images
}

func expected_album_model(obj *wxchannels.ChannelsObject) model.ContentAlbum {
	return model.ContentAlbum{
		Id:          expected_album_content_id,
		ImageCount:  len(obj.ObjectDesc.Media),
		CoverWidth:  954,
		CoverHeight: 636,
		Description: expected_album_title,
		Images:      expected_album_images(obj),
	}
}

func TestToAccount_FromAlbumJSON(t *testing.T) {
	obj, _ := load_album_object(t)

	account, err := wxchannels.ToAccount(&obj)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}
	account.Timestamps = model.Timestamps{}
	assert_equal(t, "Account", expected_album_account_model(), account)
}

func TestToContent_FromAlbumJSON(t *testing.T) {
	obj, _ := load_album_object(t)

	content, raw_ext, err := wxchannels.ToContent(&obj)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	album, ok := raw_ext.(*model.ContentAlbum)
	if !ok {
		t.Fatalf("extension is not *ContentAlbum, got %T", raw_ext)
	}
	content.Timestamps = model.Timestamps{}

	assert_equal(t, "Content", expected_album_content_model(&obj), content)
	assert_equal(t, "ContentAlbum", expected_album_model(&obj), album)
}

func TestBuildBrowseRecord_FromAlbumJSON(t *testing.T) {
	obj, raw := load_album_object(t)
	first := obj.ObjectDesc.Media[0]

	h := adapter.Get(wxchannels.PlatformID)
	built, err := h.BuildBrowseHistory(raw)
	if err != nil {
		t.Fatalf("BuildBrowseHistory: %v", err)
	}
	r := built.BrowseHistory
	if r.PublishTime == nil || *r.PublishTime != expected_album_publish_time {
		t.Fatalf("unexpected publish time: %v", r.PublishTime)
	}
	r.PublishTime = nil
	r.Timestamps = model.Timestamps{}

	expected := model.BrowseHistory{
		Id:           expected_album_content_id,
		PlatformId:   "wxchannels",
		VisitedTimes: 1,
		Type:         "album",
		ExternalId:   expected_album_external_id,
		Title:        expected_album_title,
		SourceURL:    expected_album_source_url,
		CoverURL:     first.URL + first.URLToken,
		CoverWidth:   "954",
		CoverHeight:  "636",
		ExtraData:    `{"decode_key":"","id":"14936583787284728000","nonce_id":"8189655580179332358_0_146_0_0"}`,
	}

	assert_equal(t, "BrowseHistory", expected, r)
}

func TestBuildDownloadTask_FromAlbumJSON(t *testing.T) {
	obj, raw := load_album_object(t)

	h := adapter.Get(wxchannels.PlatformID)
	if h == nil {
		t.Fatal("handler not registered for platform wxchannels")
	}

	cfg_json, err := json.Marshal(map[string]any{})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	info, err := h.BuildDownloadTask(raw, json.RawMessage(cfg_json))
	if err != nil {
		t.Fatalf("build download task failed: %v", err)
	}
	account := info.Account
	content := info.Content
	account.Timestamps = model.Timestamps{}
	content.Timestamps = model.Timestamps{}

	assert_equal(t, "Account", expected_album_account_model(), *account)
	assert_equal(t, "Content", expected_album_content_model(&obj), content)

	album, ok := info.ContentDetail.(*model.ContentAlbum)
	if !ok {
		t.Fatalf("ContentDetail is not *ContentAlbum, got %T", info.ContentDetail)
	}
	expected_album := expected_album_model(&obj)
	assert_equal(t, "ContentAlbum", expected_album, album)

	expected_task := model.DownloadTask{
		ContentId:  &expected_album_content_id,
		Name:       expected_album_title,
		PlatformId: "wxchannels",
		UniqueID:   expected_album_external_id,
		Status:     model.TaskStatusWaiting,
		CoverURL:   obj.ObjectDesc.Media[0].URL + obj.ObjectDesc.Media[0].URLToken,
		ConfigJSON: `{"suffix":".zip","type":2}`,
	}
	assert_equal(t, "DownloadTask", expected_task, info.Task)

	expected_resource_count := len(obj.ObjectDesc.Media) + 1
	if len(info.Resources) != expected_resource_count {
		t.Fatalf("Resources length = %d, want %d", len(info.Resources), expected_resource_count)
	}

	for i, file := range obj.ObjectDesc.Media {
		resource_info := info.Resources[i]
		got_resource := resource_info.Resource
		resource_extra := got_resource.Extra
		got_resource.Extra = ""
		assert_equal(t, fmt.Sprintf("DownloadResource[%d]", i), model.DownloadResource{
			ContentId: &expected_album_content_id,
			Name:      fmt.Sprintf("%s_%d", expected_album_title, i+1),
			Kind:      "image/jpeg",
			UniqueID:  fmt.Sprintf("%s_%d", expected_album_external_id, i),
		}, got_resource)

		want_extra := map[string]string{
			"id":         expected_album_external_id,
			"title":      expected_album_title,
			"filename":   expected_album_title,
			"spec":       "",
			"created_at": "1780579541",
			"author":     expected_album_account_nickname,
			"type":       "2",
		}
		if i > 0 {
			want_extra["idx"] = fmt.Sprint(i)
		}
		assert_resource_extra(t, resource_extra, want_extra)

		if len(resource_info.Endpoints) != 1 {
			t.Fatalf("Endpoints[%d] length = %d, want 1", i, len(resource_info.Endpoints))
		}
		assert_equal(t, fmt.Sprintf("DownloadEndpoint[%d]", i), model.DownloadEndpoint{
			Protocol: "https",
			URL:      file.URL + file.URLToken,
			Enabled:  1,
		}, resource_info.Endpoints[0])
	}

	bgm_info := info.Resources[len(obj.ObjectDesc.Media)]
	got_bgm := bgm_info.Resource
	bgm_extra := got_bgm.Extra
	got_bgm.Extra = ""
	assert_equal(t, "DownloadResource[bgm]", model.DownloadResource{
		ContentId: &expected_album_content_id,
		Name:      expected_album_bgm_name,
		Kind:      "audio/mpeg",
		UniqueID:  expected_album_external_id + "_bgm",
	}, got_bgm)
	assert_resource_extra(t, bgm_extra, map[string]string{
		"id":         expected_album_external_id,
		"title":      expected_album_title,
		"filename":   expected_album_title,
		"spec":       "",
		"created_at": "1780579541",
		"author":     expected_album_account_nickname,
		"type":       "2",
	})

	if len(bgm_info.Endpoints) != 1 {
		t.Fatalf("BGM Endpoints length = %d, want 1", len(bgm_info.Endpoints))
	}
	assert_equal(t, "DownloadEndpoint[bgm]", model.DownloadEndpoint{
		Protocol: "http",
		URL:      expected_album_bgm_url,
		Enabled:  1,
	}, bgm_info.Endpoints[0])
}

func TestBuildDownloadTask_FromContent_WithFilenameTemplateAndHook(t *testing.T) {
	obj, content_json := load_album_object(t)

	hooks := hermes.NewHookManager()
	if err := hooks.Load(`
function onFilename(meta, task, config) {
  return {
    name: meta.name,
    directories: [meta.author]
  };
}
`); err != nil {
		t.Fatalf("load hook script: %v", err)
	}

	h, err := wxchannels.Register(&adapter.AdapterOptions{Hooks: hooks})
	if err != nil {
		t.Fatalf("register wxchannels adapter: %v", err)
	}
	defer h.Stop()

	config := map[string]any{
		"filenameTemplate": "{{filename}}_{{spec}}",
	}
	cfg_json, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	info, err := h.BuildDownloadTask(content_json, json.RawMessage(cfg_json))
	if err != nil {
		t.Fatalf("build download task failed: %v", err)
	}

	want_name := expected_album_account_nickname + "/" + expected_album_title + ".zip"
	if info.Task.Name != want_name {
		t.Fatalf("DownloadTask.Name = %q, want %q", info.Task.Name, want_name)
	}
	if info.Task.UniqueID != expected_album_external_id {
		t.Fatalf("DownloadTask.UniqueID = %q, want %q", info.Task.UniqueID, expected_album_external_id)
	}
	expected_resource_count := len(obj.ObjectDesc.Media) + 1
	if len(info.Resources) != expected_resource_count {
		t.Fatalf("Resources length = %d, want %d", len(info.Resources), expected_resource_count)
	}
	if info.Resources[0].Resource.UniqueID != expected_album_external_id+"_0" {
		t.Fatalf("Resource.UniqueID = %q, want %q", info.Resources[0].Resource.UniqueID, expected_album_external_id+"_0")
	}
	for i := range obj.ObjectDesc.Media {
		want_resource_name := fmt.Sprintf("%s/%s_%d.jpg", expected_album_account_nickname, expected_album_title, i+1)
		if info.Resources[i].Resource.Name != want_resource_name {
			t.Fatalf("Resource[%d].Name = %q, want %q", i, info.Resources[i].Resource.Name, want_resource_name)
		}
	}
	want_bgm_name := expected_album_account_nickname + "/" + expected_album_bgm_name + ".mp3"
	if info.Resources[len(obj.ObjectDesc.Media)].Resource.Name != want_bgm_name {
		t.Fatalf("Resource[bgm].Name = %q, want %q", info.Resources[len(obj.ObjectDesc.Media)].Resource.Name, want_bgm_name)
	}
}

func TestBuildDownloadTask_Cover_WithFilenameTemplate(t *testing.T) {
	obj, content_json := load_album_object(t)

	h := adapter.Get(wxchannels.PlatformID)
	if h == nil {
		t.Fatal("handler not registered for platform wxchannels")
	}

	config := map[string]any{
		"filenameTemplate": "{{author}}/{{filename}}",
		"suffix":           ".jpg",
	}
	cfg_json, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	info, err := h.BuildDownloadTask(content_json, json.RawMessage(cfg_json))
	if err != nil {
		t.Fatalf("build download task failed: %v", err)
	}

	want_name := expected_album_account_nickname + "_" + expected_album_title + ".jpg"
	if info.Task.Name != want_name {
		t.Fatalf("DownloadTask.Name = %q, want %q", info.Task.Name, want_name)
	}
	if info.Task.UniqueID != expected_album_external_id+"_cover" {
		t.Fatalf("DownloadTask.UniqueID = %q, want %q", info.Task.UniqueID, expected_album_external_id+"_cover")
	}
	if len(info.Resources) != 1 {
		t.Fatalf("Resources length = %d, want 1", len(info.Resources))
	}
	got_resource := info.Resources[0].Resource
	resource_extra := got_resource.Extra
	got_resource.Extra = ""
	assert_equal(t, "DownloadResource[cover]", model.DownloadResource{
		ContentId:  &expected_album_content_id,
		Name:       want_name,
		Kind:       "image/jpeg",
		UniqueID:   expected_album_external_id + "_cover",
		MergeOrder: 0,
	}, got_resource)
	assert_resource_extra(t, resource_extra, map[string]string{
		"id":         expected_album_external_id,
		"title":      expected_album_title,
		"filename":   expected_album_title,
		"spec":       "",
		"created_at": "1780579541",
		"author":     expected_album_account_nickname,
		"type":       "2",
	})
	if len(info.Resources[0].Endpoints) != 1 {
		t.Fatalf("Endpoints length = %d, want 1", len(info.Resources[0].Endpoints))
	}
	want_cover_url := obj.ObjectDesc.Media[0].URL + obj.ObjectDesc.Media[0].URLToken
	if obj.ObjectDesc.Media[0].CoverUrl != "" {
		want_cover_url = obj.ObjectDesc.Media[0].CoverUrl
	} else if obj.ObjectDesc.Media[0].ThumbUrl != "" {
		want_cover_url = obj.ObjectDesc.Media[0].ThumbUrl
	}
	assert_equal(t, "DownloadEndpoint[cover]", model.DownloadEndpoint{
		Protocol: "https",
		URL:      want_cover_url,
		Enabled:  1,
	}, info.Resources[0].Endpoints[0])
}
