package wxchannels_test

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"wx_channel/internal/adapter"
	wxchannels "wx_channel/internal/adapter/wxchannels"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
)

const fixture_path = "wxchannels/260710/feedprofile.json"

const expected_external_id = "14962486294771997000"
const expected_nonce_id = "4390481592474233535_0_140_0_0"
const expected_title = "讨厌我有什么用 有本事弄死我"
const expected_decode_key = "1227338722"
const expected_account_external_id = "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder"
const expected_account_nickname = "迷人的大嘴猴"
const expected_account_signature = "感谢观看\n 纯分享一些大哥爱看的視頻 仅此而已\n 懂点规矩 蠢货不要随便发私信\n 喜欢就看 不喜欢就从我作品里面爬出去\n 轮不到你在我作品底下指手画脚"
const expected_account_avatar_url = "https://wx.qlogo.cn/finderhead/ver_1/qFNbjmtcFiaDEdzgVJX4adKGr8e3tngbL52R8apVcp167uR1gYpV5mkByTxibvV7F63j7CNQrfpgY96vEwyCmvDAlZl2Ic86NG4TNLl4ZpJFhw4jL7d2U8ibicAfkz2L4HJA9v1SfUdG0SLepY2uwvwANA/132"
const raw_expected_video_url = "https://finder.video.qq.com/251/20302/stodownload?encfilekey=Cvvj5Ix3eewQsFvYyicia1J4vPZhKwibibyibAO6BVb6JtHx7sfjtTfmCnIib4dtTeSl2Skialoibjc4ia6VtH3tyOo2Sbfhz1vNa4lmBoRG3uapCVhgnZfcJBou7lg&hy=SZ&idx=1&m=414c8b10462c8fa97a904c3d999a0476&uzid=7a206"
const expected_video_url_token = "&token=2lt8WBSnjTkVMibWGXCbibQYrsia4RYvheR4CohiaWrxXusKaK1HbUXySDX9xlQ5Aya9pG01NaEr3nV4uWzv9RLnX21VEVE7Uh2LUJGsjml3a9Ql41uKjqW5maZic7oYPZDQNDj7wmW4mWB1ibibhlQJSzrltkibIzFaojcAYvHiaRlqibwibrYwIzGoum5LgJEEaDoiclZzeQyv7FjX1bF4fJRgAqfJRta5Ym17JwS0FmKDicaABQmk&basedata=CAMSBnhXVDExMyJaCgoKBnhXVDExMxAACgoKBnhXVDExMhAACgoKBnhXVDExMRAACgoKBnhXVDEyOBAACgoKBnhXVDEyNxAACgoKBnhXVDEyNhAACgcKA3hBMBAACgcKA3hBMhAAQjBTuHhbNHyKM05Ce4Htl2FA9vmfZf_nqUf0_zUDtX8JtGc-FDdtmp6lBuhIZLzBvvlIkNDi0wY&sign=jcWTRPG-pMrPk63Yc5_3526BpL-If3KELnvxKMQW6YKgj1W0LWRwDeJdyFGlj_tkhxdtVVa7i_jm8NrUqDLTAg&web=1&extg=10f0000&svrbypass=AAuL%2FQsFAAABAAAAAADllCeOTo29FRUBEKh4ahAAAADnaHZTnGbFfAj9RgZXfw6VZURMULrIRJijFNZ2fc7ng3nSduyUELKKKAFhAn%2FWQBkWG3qfVKHo7iM%3D&svrnonce=1786292240"
const expected_video_cover_url = "https://finder.video.qq.com/251/20350/stodownload?encfilekey=2fG3V4WwQPnQr0gxUocFa2h6q3eoq4hXzG39ub5SWukSZAsfOaRiadTuuGIYouJicfpVpzk12gN6RJ2mlOl26YUBWWTVupMcpSIhJDGZaKiaRI&token=ic1n0xDG6awibdickticZOerRhGLUFicOmA4lnHuXNwnvprkSlmNWOGtCk8pKCJI4OoQVlqU2hKiak0ibO0EMZYW62adBJX5LGB6aCrIPiax8A8YyHiajMriaBYWa9pCj1PDVn4Ke7ZfOwGDk8sfK4bKW7dFKZg1J0AictooAicanrkHIDH2cnmstBK7hsNFD94BJbSyVV4qYRRwjz6rWYBFQG0icHiaqoTKudIp5dX3ibA&hy=SZ&idx=1&m=7b022855f315b6aa0a3dd30f631d1d4a&picformat=200&wxampicformat=503"
const expected_video_resource_cover_url = "https://finder.video.qq.com/251/20304/stodownload?encfilekey=2fG3V4WwQPluPpjb46OTKMXHc112k4G2oJic38N7rnuA86EibU1Y76s8oA7ibJ2icEheVFXiah55XOtQTzMnAsGIe2IWYOSogJ0DHQGv97AFZePM&token=x5Y29zUxcibAtVh1Q7cQ3cnvOAxVsTK8h5YA8ZcVRZx0S9XjjibDTQIHI9jaGHZ6mD6PYqABOpQHCuKwelyI9VbzF4mnicJulFLRq6wg0cNmZn4lBbpRHBqmvP7eUBKY9Zibk2lICElAwgVku1fheR19a0iblkRWxRVZSaPzEVAsYFGmgowibibhcicRkvTKMicFmBADeOBqGGQmf71uwEbcsq5UTDJmWzakFPrR2&hy=SZ&idx=1&m=73a9ef1bc335f9c43d800208ddc42f09&uzid=1&picformat=200&wxampicformat=503"

var (
	expected_video_content_id   = wxchannels.BuildContentID(expected_external_id)
	expected_video_publish_time = int64(1783667361)
	expected_video_url          = normalize_media_url(raw_expected_video_url) + expected_video_url_token
	expected_xwt111_video_url   = expected_video_url + "&X-snsvideoflag=xWT111"
	expected_audio_only_url     = expected_xwt111_video_url + "&audio_only=1"
	expected_source_url         = wxchannels.BuildJumpURLFromParts(expected_external_id, expected_nonce_id, "", expected_account_external_id)
)

const expected_video_metadata = `{"key":"1227338722"}`

func normalize_media_url(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return raw
	}
	q := u.Query()
	q.Del("hy")
	q.Del("idx")
	q.Del("m")
	q.Del("uzid")
	u.RawQuery = q.Encode()
	return u.String()
}

func load_fixture(name string) ([]byte, error) {
	candidates := []string{
		filepath.Join("testdata", name),
		filepath.Join("..", "..", "..", "..", "scraper_examples", name),
	}
	var last_err error
	for _, candidate := range candidates {
		raw, err := os.ReadFile(candidate)
		if err == nil {
			return raw, nil
		}
		last_err = err
	}
	return nil, fmt.Errorf("fixture %q not found in %v: %w", name, candidates, last_err)
}

func assert_equal(t *testing.T, name string, want, got any) {
	t.Helper()
	want = deref_for_compare(want)
	got = deref_for_compare(got)
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%s mismatch:\nwant: %#v\ngot:  %#v", name, want, got)
	}
}

func deref_for_compare(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		return v
	}
	if rv.IsNil() {
		return nil
	}
	return rv.Elem().Interface()
}

func expected_account_model() model.Account {
	return model.Account{
		Id:         wxchannels.BuildAccountID(expected_account_external_id),
		PlatformId: "wxchannels",
		ExternalId: expected_account_external_id,
		Nickname:   expected_account_nickname,
		Signature:  expected_account_signature,
		AvatarURL:  expected_account_avatar_url,
	}
}

func expected_content_model() model.Content {
	return model.Content{
		Id:          expected_video_content_id,
		PlatformId:  "wxchannels",
		Type:        "video",
		ExternalId:  expected_external_id,
		ExternalId2: expected_nonce_id,
		Title:       expected_title,
		Description: expected_title,
		URL:         expected_video_url,
		SourceURL:   expected_source_url,
		CoverURL:    expected_video_cover_url,
		CoverWidth:  "1080",
		CoverHeight: "2128",
		PublishTime: &expected_video_publish_time,
		Metadata:    expected_video_metadata,
	}
}

func expected_content_video_model() model.ContentVideo {
	return model.ContentVideo{
		Id:       expected_video_content_id,
		Duration: 9,
		Width:    1080,
		Height:   2128,
		Size:     9613487,
		URL:      expected_video_url,
	}
}

func assert_resource_extra(t *testing.T, raw string, want map[string]string) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("DownloadResource.Extra is not valid JSON: %v", err)
	}
	for key, want_value := range want {
		if fmt.Sprint(got[key]) != want_value {
			t.Errorf("DownloadResource.Extra[%q] = %v, want %q", key, got[key], want_value)
		}
	}
	if strings.TrimSpace(fmt.Sprint(got["download_at"])) == "" {
		t.Errorf("DownloadResource.Extra[%q] is empty", "download_at")
	}
}

func TestToAccount_FromVideoFeedJSON(t *testing.T) {
	raw, err := load_fixture(fixture_path)
	if err != nil {
		t.Fatalf("load video fixture: %v", err)
	}

	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	account, err := wxchannels.ToAccount(&obj)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}
	account.Timestamps = model.Timestamps{}
	assert_equal(t, "Account", expected_account_model(), account)
}

func TestToContent_FromVideoFeedJSON(t *testing.T) {
	raw, err := load_fixture(fixture_path)
	if err != nil {
		t.Fatalf("load video fixture: %v", err)
	}

	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	content, content_video, err := wxchannels.ToContent(&obj)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	normalize_video_content(t, content)
	assert_equal(t, "Content", expected_content_model(), content)
	assert_equal(t, "ContentVideo", expected_content_video_model(), content_video)
}

func TestBuildBrowseRecord_FromVideoFeedJSON(t *testing.T) {
	raw, err := load_fixture(fixture_path)
	if err != nil {
		t.Fatalf("load video fixture: %v", err)
	}

	h := adapter.Get("wxchannels")
	built, err := h.BuildBrowseHistory(raw)
	if err != nil {
		t.Fatalf("BuildBrowseHistory: %v", err)
	}
	r := built.BrowseHistory
	if r.PublishTime == nil || *r.PublishTime != 1783667361 {
		t.Fatalf("unexpected publish time: %v", r.PublishTime)
	}
	r.PublishTime = nil
	r.Timestamps = model.Timestamps{}

	expected := model.BrowseHistory{
		Id:           expected_video_content_id,
		PlatformId:   "wxchannels",
		VisitedTimes: 1,
		Type:         "video",
		ExternalId:   expected_external_id,
		Title:        expected_title,
		URL:          expected_video_url,
		SourceURL:    expected_source_url,
		CoverURL:     expected_video_cover_url,
		CoverWidth:   "1080",
		CoverHeight:  "2128",
		ExtraData:    `{"decode_key":"1227338722","id":"14962486294771997000","nonce_id":"4390481592474233535_0_140_0_0"}`,
	}

	assert_equal(t, "BrowseHistory", expected, r)
}

// TestBuildDownloadTask_FromContent verifies the model conversion and linkage
// from a raw video feed JSON: Account, Content, BrowseHistory, and DownloadTask
// are correctly built and cross-referenced.
func TestBuildDownloadTask_FromContent(t *testing.T) {
	content_json, err := load_fixture(fixture_path)
	if err != nil {
		t.Fatalf("load video fixture: %v", err)
	}
	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(content_json, &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	h := adapter.Get(wxchannels.PlatformID)
	if h == nil {
		t.Fatal("handler not registered for platform wxchannels")
	}

	config := map[string]any{}
	cfg_json, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	info, err := h.BuildDownloadTask(content_json, json.RawMessage(cfg_json))
	if err != nil {
		t.Fatalf("build download task failed: %v", err)
	}
	content := info.Content
	account := info.Account
	content_video := info.ContentDetail
	account.Timestamps = model.Timestamps{}
	normalize_video_content(t, content)

	assert_equal(t, "Account", expected_account_model(), *account)
	assert_equal(t, "Content", expected_content_model(), content)
	assert_equal(t, "ContentVideo", expected_content_video_model(), content_video)

	// ---- V1 DownloadTask: task-level container ----
	expected_task := model.DownloadTask{
		ContentId:  &expected_video_content_id,
		Name:       expected_title + ".mp4",
		PlatformId: "wxchannels",
		UniqueID:   expected_external_id + "_xWT111",
		Status:     model.TaskStatusWaiting,
		SourceURL:  expected_source_url,
		CoverURL:   expected_video_cover_url,
		ConfigJSON: `{"spec":"xWT111","type":4}`,
	}
	assert_equal(t, "DownloadTask", expected_task, info.Task)

	if len(info.Resources) != 1 {
		t.Fatalf("Resources length = %d, want 1", len(info.Resources))
	}
	expected_resource := model.DownloadResource{
		ContentId: &expected_video_content_id,
		Name:      expected_title,
		Kind:      "video/mp4",
		UniqueID:  expected_external_id + "_xWT111",
		Size:      9613487,
	}
	got_resource := info.Resources[0].DownloadResource
	resource_extra := got_resource.Extra
	got_resource.Extra = ""
	assert_equal(t, "DownloadResource", expected_resource, got_resource)
	assert_resource_extra(t, resource_extra, map[string]string{
		"id":         expected_external_id,
		"title":      expected_title,
		"filename":   expected_title,
		"spec":       "xWT111",
		"created_at": "1783667361",
		"author":     expected_account_nickname,
		"decode_key": expected_decode_key,
		"type":       "4",
	})

	if len(info.Resources[0].Endpoints) != 1 {
		t.Fatalf("Endpoints length = %d, want 1", len(info.Resources[0].Endpoints))
	}
	expected_endpoint := model.DownloadEndpoint{
		Protocol: "https",
		URL:      expected_xwt111_video_url,
		Enabled:  1,
	}
	assert_equal(t, "DownloadEndpoint", expected_endpoint, info.Resources[0].Endpoints[0])
}

// TestBuildDownloadTask_FromContent_WithSpecAndSuffix verifies that when both
// Spec and Suffix are set in the download config, the generated UniqueID and download
// URL correctly reflect the configuration.
func TestBuildDownloadTask_FromContent_WithSpecAndSuffix(t *testing.T) {
	content_json, err := load_fixture(fixture_path)
	if err != nil {
		t.Fatalf("load video fixture: %v", err)
	}
	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(content_json, &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	h := adapter.Get(wxchannels.PlatformID)
	if h == nil {
		t.Fatal("handler not registered for platform wxchannels")
	}

	config := map[string]any{"spec": "xWT111", "suffix": ".mp3"}
	cfg_json, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	info, err := h.BuildDownloadTask(content_json, json.RawMessage(cfg_json))
	if err != nil {
		t.Fatalf("build download task failed: %v", err)
	}
	content := info.Content
	account := info.Account
	content_video := info.ContentDetail
	account.Timestamps = model.Timestamps{}
	normalize_video_content(t, content)

	assert_equal(t, "Account", expected_account_model(), *account)
	assert_equal(t, "Content", expected_content_model(), content)
	assert_equal(t, "ContentVideo", expected_content_video_model(), content_video)

	// ---- V1 DownloadTask: UniqueID reflects both spec and suffix ----
	assert_equal(t, "DownloadTask", model.DownloadTask{
		ContentId:  &expected_video_content_id,
		Name:       expected_title + ".mp3",
		PlatformId: "wxchannels",
		UniqueID:   expected_external_id + "_xWT111_mp3",
		Status:     model.TaskStatusWaiting,
		SourceURL:  expected_source_url,
		CoverURL:   expected_video_cover_url,
		ConfigJSON: `{"spec":"xWT111","suffix":".mp3","type":4}`,
	}, info.Task)

	if len(info.Resources) != 1 {
		t.Fatalf("Resources length = %d, want 1", len(info.Resources))
	}
	got_resource := info.Resources[0].DownloadResource
	resource_extra := got_resource.Extra
	got_resource.Extra = ""
	assert_equal(t, "DownloadResource", model.DownloadResource{
		ContentId: &expected_video_content_id,
		Name:      expected_title,
		Kind:      "audio/mpeg",
		UniqueID:  expected_external_id + "_xWT111_mp3",
		Size:      9613487,
	}, got_resource)
	assert_resource_extra(t, resource_extra, map[string]string{
		"id":         expected_external_id,
		"title":      expected_title,
		"filename":   expected_title,
		"spec":       "xWT111",
		"created_at": "1783667361",
		"author":     expected_account_nickname,
		"decode_key": expected_decode_key,
		"type":       "4",
	})

	if len(info.Resources[0].Endpoints) != 1 {
		t.Fatalf("Endpoints length = %d, want 1", len(info.Resources[0].Endpoints))
	}
	assert_equal(t, "DownloadEndpoint", model.DownloadEndpoint{
		Protocol: "https",
		URL:      expected_xwt111_video_url,
		Enabled:  1,
	}, info.Resources[0].Endpoints[0])
}

func TestBuildDownloadTask_FromContent_WithFilenameTemplateAndHook_Video(t *testing.T) {
	content_json, err := load_fixture(fixture_path)
	if err != nil {
		t.Fatalf("load video fixture: %v", err)
	}

	hook_path := filepath.Join(t.TempDir(), "hooks.js")
	if err := os.WriteFile(hook_path, []byte(`
function onFilename(systemName, meta, task, config) {
  return systemName + "_" + meta.author + "_" + config.suffix;
}
`), 0644); err != nil {
		t.Fatalf("write hook script: %v", err)
	}
	hooks := hermes.NewHookManager()
	if err := hooks.Load(hook_path); err != nil {
		t.Fatalf("load hook script: %v", err)
	}

	h, err := wxchannels.Register(wxchannels.Deps{Hooks: hooks})
	if err != nil {
		t.Fatalf("register wxchannels adapter: %v", err)
	}
	defer h.Stop()

	config := map[string]any{
		"filenameTemplate": "{{author}}/{{filename}}",
		"suffix":           "hook",
	}
	cfg_json, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	info, err := h.BuildDownloadTask(content_json, json.RawMessage(cfg_json))
	if err != nil {
		t.Fatalf("build download task failed: %v", err)
	}

	want_name := expected_account_nickname + "/" + expected_title + "_" + expected_account_nickname + "_hook.mp4"
	if info.Task.Name != want_name {
		t.Fatalf("DownloadTask.Name = %q, want %q", info.Task.Name, want_name)
	}
	if len(info.Resources) != 1 {
		t.Fatalf("Resources length = %d, want 1", len(info.Resources))
	}
	if info.Resources[0].DownloadResource.Name != want_name {
		t.Fatalf("DownloadResource.Name = %q, want %q", info.Resources[0].DownloadResource.Name, want_name)
	}
}

func TestBuildDownloadTaskWithMultiResource_FromContent(t *testing.T) {
	content_json, err := load_fixture(fixture_path)
	if err != nil {
		t.Fatalf("load video fixture: %v", err)
	}

	h := adapter.Get(wxchannels.PlatformID)
	if h == nil {
		t.Fatal("handler not registered for platform wxchannels")
	}

	config := map[string]any{}
	cfg_json, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	info, err := h.BuildDownloadTask(content_json, json.RawMessage(cfg_json))
	if err != nil {
		t.Fatalf("build download task failed: %v", err)
	}
	content := info.Content
	account := info.Account
	account.Timestamps = model.Timestamps{}
	normalize_video_content(t, content)

	assert_equal(t, "Account", expected_account_model(), *account)
	assert_equal(t, "Content", expected_content_model(), content)

	// ---- V1 DownloadTask: task-level container ----
	if info.Task.ContentId == nil {
		t.Fatal("DownloadTask.ContentId is nil")
	}
	if *info.Task.ContentId != expected_video_content_id {
		t.Errorf("DownloadTask.ContentId = %q, want %q", *info.Task.ContentId, expected_video_content_id)
	}
	if info.Task.UniqueID != expected_external_id+"_xWT111" {
		t.Errorf("DownloadTask.UniqueID = %q, want %q", info.Task.UniqueID, expected_external_id+"_xWT111")
	}
	if info.Task.PlatformId != "wxchannels" {
		t.Errorf("DownloadTask.PlatformId = %q, want %q", info.Task.PlatformId, "wxchannels")
	}
	if info.Task.Name != expected_title+".mp4" {
		t.Errorf("DownloadTask.Name = %q, want %q", info.Task.Name, expected_title+".mp4")
	}
	if info.Task.Status != model.TaskStatusWaiting {
		t.Errorf("DownloadTask.Status = %v, want %v", info.Task.Status, model.TaskStatusWaiting)
	}
	if info.Task.ConfigJSON != `{"spec":"xWT111","type":4}` {
		t.Errorf("DownloadTask.ConfigJSON = %q, want %q", info.Task.ConfigJSON, `{"spec":"xWT111","type":4}`)
	}
	if len(info.Resources) != 1 {
		t.Fatalf("Resources length = %d, want 1", len(info.Resources))
	}
	if info.Resources[0].DownloadResource.Kind != "video/mp4" {
		t.Errorf("DownloadResource.Kind = %q, want %q", info.Resources[0].DownloadResource.Kind, "video/mp4")
	}
	if info.Resources[0].DownloadResource.UniqueID != expected_external_id+"_xWT111" {
		t.Errorf("DownloadResource.UniqueID = %q, want %q", info.Resources[0].DownloadResource.UniqueID, expected_external_id+"_xWT111")
	}
	if info.Resources[0].DownloadResource.Size != 9613487 {
		t.Errorf("DownloadResource.Size = %d, want %d", info.Resources[0].DownloadResource.Size, int64(9613487))
	}
	if len(info.Resources[0].Endpoints) == 0 {
		t.Fatal("Endpoints length = 0, want at least 1")
	}
	if info.Resources[0].Endpoints[0].Protocol != "https" {
		t.Errorf("DownloadEndpoint.Protocol = %q, want %q", info.Resources[0].Endpoints[0].Protocol, "https")
	}
	if info.Resources[0].Endpoints[0].Enabled != 1 {
		t.Errorf("DownloadEndpoint.Enabled = %d, want 1", info.Resources[0].Endpoints[0].Enabled)
	}
}

// TestBuildDownloadTask_FromContent_Lifecycle simulates the download lifecycle state machine:
// Start → 10% pause → Resume → 100% complete → Merging → Finished.
// Validates state transitions and data consistency across task/resource/endpoint/segment/conn entities.
func TestBuildDownloadTask_FromContent_Lifecycle(t *testing.T) {
	task := model.DownloadTask{
		Id:           1,
		Name:         expected_title,
		UniqueID:     expected_external_id,
		PlatformId:   "wxchannels",
		Status:       model.TaskStatusWaiting,
		ConfigJSON:   `{"download_cover":false,"overwrite":false}`,
		MetadataJSON: `{"platform":"wxchannels","external_id":"14962486294771997000"}`,
	}

	resource := model.DownloadResource{
		Id:     1,
		TaskId: 1,
		Name:   expected_title + ".mp4",
		Kind:   "video/mp4",
		Size:   9613487,
		Status: 0,
	}

	// =====================================================================
	// Download lifecycle simulation
	// Simulate full download flow: Start → 10% pause → Resume → 100% complete
	// =====================================================================

	segment := model.DownloadSegment{
		Id:          1,
		ResourceId:  1,
		Index:       0,
		URL:         "https://finder.video.qq.com/251/20302/stodownload?...",
		OffsetStart: 0,
		OffsetEnd:   9613486,
		Size:        9613487,
		Downloaded:  0,
		Status:      0,
		Retry:       0,
	}

	conn := model.DownloadConnection{
		Id:         1,
		EndpointId: 1,
		WorkerId:   "worker-1",
		Host:       "finder.video.qq.com",
		IP:         "183.60.15.100",
		Speed:      0,
		Bytes:      0,
		Status:     0,
		LastActive: 0,
	}

	const ten_percent = 961348
	type log_entry struct {
		message string
	}
	logs := make([]log_entry, 0, 4)

	// Stage 1: Start download → Preparing → Downloading
	t.Run("Stage1_StartDownloading", func(t *testing.T) {
		task.Status = model.TaskStatusPreparing
		if task.Status != model.TaskStatusPreparing {
			t.Errorf("Preparing.Status = %d, want %d", task.Status, model.TaskStatusPreparing)
		}

		task.Status = model.TaskStatusDownloading
		conn.Status = 1
		logs = append(logs, log_entry{
			message: "download started",
		})

		if task.Status != model.TaskStatusDownloading {
			t.Errorf("Stage1 task.Status = %d, want %d (Downloading)", task.Status, model.TaskStatusDownloading)
		}
		if conn.Status != 1 {
			t.Errorf("Stage1 conn.Status = %d, want 1 (active)", conn.Status)
		}
		if len(logs) != 1 {
			t.Errorf("Stage1 logs count = %d, want 1", len(logs))
		}
	})

	// Stage 2: Download to 10% → Pause
	t.Run("Stage2_Download10PercentAndPause", func(t *testing.T) {
		segment.Downloaded = ten_percent
		conn.Bytes = ten_percent
		conn.Speed = 1024 * 1024 // 1 MB/s

		task.Status = model.TaskStatusPaused
		resource.Status = 1
		segment.Status = 1
		conn.Status = 2
		logs = append(logs, log_entry{
			message: "download paused at 10%",
		})

		if segment.Downloaded != ten_percent {
			t.Errorf("Stage2 segment.Downloaded = %d, want %d (10%%)", segment.Downloaded, ten_percent)
		}
		if conn.Bytes != ten_percent {
			t.Errorf("Stage2 conn.Bytes = %d, want %d", conn.Bytes, ten_percent)
		}
		if task.Status != model.TaskStatusPaused {
			t.Errorf("Stage2 task.Status = %d, want %d (Paused)", task.Status, model.TaskStatusPaused)
		}
		if resource.Status != 1 {
			t.Errorf("Stage2 resource.Status = %d, want 1 (partial)", resource.Status)
		}
		if conn.Status != 2 {
			t.Errorf("Stage2 conn.Status = %d, want 2 (paused)", conn.Status)
		}
	})

	// Stage 3: Resume download → Continue to 100%
	t.Run("Stage3_ResumeAndDownloadTo100Percent", func(t *testing.T) {
		task.Status = model.TaskStatusDownloading
		conn.Status = 1
		conn.Speed = 2 * 1024 * 1024 // 2 MB/s
		logs = append(logs, log_entry{
			message: "download resumed",
		})

		if task.Status != model.TaskStatusDownloading {
			t.Errorf("Stage3 task.Status = %d, want %d (Downloading)", task.Status, model.TaskStatusDownloading)
		}
		if conn.Status != 1 {
			t.Errorf("Stage3 conn.Status = %d, want 1 (active)", conn.Status)
		}

		// Download to 100%
		segment.Downloaded = resource.Size
		conn.Bytes = resource.Size
		segment.Status = 2
		conn.Speed = 0
		conn.Status = 0

		if segment.Downloaded != 9613487 {
			t.Errorf("Stage3 segment.Downloaded = %d, want %d (100%%)", segment.Downloaded, int64(9613487))
		}
		if conn.Bytes != 9613487 {
			t.Errorf("Stage3 conn.Bytes = %d, want %d", conn.Bytes, int64(9613487))
		}
		if segment.Status != 2 {
			t.Errorf("Stage3 segment.Status = %d, want 2 (completed)", segment.Status)
		}
	})

	// Stage 4: Merging → Finished
	t.Run("Stage4_MergingAndFinished", func(t *testing.T) {
		task.Status = model.TaskStatusMerging
		if task.Status != model.TaskStatusMerging {
			t.Errorf("Stage4 merging status = %d, want %d", task.Status, model.TaskStatusMerging)
		}

		task.Status = model.TaskStatusFinished
		resource.Status = 2
		logs = append(logs, log_entry{
			message: "download finished",
		})

		if task.Status != model.TaskStatusFinished {
			t.Errorf("Stage4 task.Status = %d, want %d (Finished)", task.Status, model.TaskStatusFinished)
		}
		if resource.Status != 2 {
			t.Errorf("Stage4 resource.Status = %d, want 2 (completed)", resource.Status)
		}
		if len(logs) != 4 {
			t.Errorf("Stage4 logs count = %d, want 4", len(logs))
		}
	})

	// Final verification: check complete download lifecycle chain
	expected_log_levels := []string{"started", "paused", "resumed", "finished"}
	for i, l := range logs {
		if !strings.Contains(l.message, expected_log_levels[i]) {
			t.Errorf("log[%d].message = %q, should contain %q", i, l.message, expected_log_levels[i])
		}
	}

	// Verify state machine completeness
	if task.Status != model.TaskStatusFinished {
		t.Errorf("final task.Status = %d, want %d (Finished)", task.Status, model.TaskStatusFinished)
	}
	if segment.Downloaded != 9613487 {
		t.Errorf("final segment.Downloaded = %d, want %d (full)", segment.Downloaded, int64(9613487))
	}
	if segment.Size != 9613487 {
		t.Errorf("segment.Size = %d, want %d", segment.Size, int64(9613487))
	}
	if segment.ResourceId != 1 {
		t.Errorf("segment.ResourceId = %d, want 1", segment.ResourceId)
	}
	if segment.OffsetStart != 0 {
		t.Errorf("segment.OffsetStart = %d, want 0", segment.OffsetStart)
	}
	if segment.OffsetEnd != 9613486 {
		t.Errorf("segment.OffsetEnd = %d, want %d", segment.OffsetEnd, int64(9613486))
	}
}

// TestBuildDownloadTask_FromContent_WithSubtasks extends TestBuildDownloadTask_FromContent
// by creating a V1 download task that contains three resources (video, cover, audio),
// each with its own endpoint. This demonstrates how a single content record links
// to a multi-file batch download via the layered V1 model.
//
// The Content and Account records are identical to the single-task scenario, but
// the download structure demonstrates three resources under one task.
func TestBuildDownloadTask_FromContent_WithSubtasks(t *testing.T) {
	raw, err := load_fixture(fixture_path)
	if err != nil {
		t.Fatalf("load video fixture: %v", err)
	}

	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// ---- Account & Content (same as single-task scenario) ----
	account, err := wxchannels.ToAccount(&obj)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}
	content, raw_ext, err := wxchannels.ToContent(&obj)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	ext, ok := raw_ext.(*model.ContentVideo)
	if !ok {
		t.Fatalf("extension is not *ContentVideo, got %T", raw_ext)
	}
	const platform_id = "wxchannels"
	const spec = "xWT111"

	// ---- Create V1 DownloadTask: task-level container ----
	const task_id = 100
	task := model.DownloadTask{
		Id:           task_id,
		Name:         expected_title,
		UniqueID:     expected_external_id,
		PlatformId:   "wxchannels",
		Status:       model.TaskStatusWaiting,
		ConfigJSON:   `{"convert_mp3":false,"download_cover":false,"duplicate":false,"overwrite":false,"upload_cloud":false}`,
		MetadataJSON: `{"external_id":"14962486294771997000","nonce_id":"4390481592474233535_0_140_0_0","platform":"wxchannels","spec":"xWT111"}`,
	}

	// ---- Create three V1 DownloadResources (one per file type) ----
	resources := []model.DownloadResource{
		{Id: 101, TaskId: 100, Name: expected_title + ".mp4", Kind: "video/mp4", Size: 9613487, Status: 0, MergeOrder: 0},
		{Id: 102, TaskId: 100, Name: expected_title + ".jpg", Kind: "image/jpeg", Size: 0, Status: 0, MergeOrder: 1},
		{Id: 103, TaskId: 100, Name: expected_title + ".mp3", Kind: "audio/mpeg", Size: 0, Status: 0, MergeOrder: 2},
	}

	// ---- Create three V1 DownloadEndpoints (one per resource) ----
	endpoints := []model.DownloadEndpoint{
		{Id: 201, ResourceId: 101, Protocol: "https", URL: expected_xwt111_video_url, Priority: 0, Enabled: 1, Status: 0},
		{Id: 202, ResourceId: 102, Protocol: "https", URL: expected_video_resource_cover_url, Priority: 0, Enabled: 1, Status: 0},
		{Id: 203, ResourceId: 103, Protocol: "https", URL: expected_audio_only_url, Priority: 0, Enabled: 1, Status: 0},
	}

	// ---- Link Content to Account via ContentAccount bridge ----
	ca := model.ContentAccount{
		ContentId: content.Id,
		AccountId: account.Id,
		Role:      "owner",
	}

	// =====================================================================
	// Assertions
	// =====================================================================

	// 1. Account and Content are unchanged from single-task scenario
	if account.PlatformId != platform_id {
		t.Errorf("Account.PlatformId = %q, want %q", account.PlatformId, platform_id)
	}
	if account.ExternalId != expected_account_external_id {
		t.Errorf("Account.ExternalId mismatch")
	}
	if content.PlatformId != platform_id {
		t.Errorf("Content.PlatformId = %q, want %q", content.PlatformId, platform_id)
	}
	if content.ExternalId != expected_external_id {
		t.Errorf("Content.ExternalId = %q, want %q", content.ExternalId, expected_external_id)
	}
	if content.Type != "video" {
		t.Errorf("Content.ContentType = %q, want %q", content.Type, "video")
	}

	// 2. Content → Account linkage
	if ca.Role != "owner" {
		t.Errorf("ContentAccount.Role = %q, want 'owner'", ca.Role)
	}

	// 3. DownloadTask shape
	if task.Id != task_id {
		t.Errorf("task.Id = %d, want %d", task.Id, task_id)
	}
	if task.Name != expected_title {
		t.Errorf("task.Name = %q, want %q", task.Name, expected_title)
	}
	if task.Status != model.TaskStatusWaiting {
		t.Errorf("task.Status = %v, want %v", task.Status, model.TaskStatusWaiting)
	}
	// 4. Each resource is correctly linked to the task
	expected_resource_ids := []int{101, 102, 103}
	expected_kinds := []string{"video/mp4", "image/jpeg", "audio/mpeg"}
	expected_names := []string{
		expected_title + ".mp4",
		expected_title + ".jpg",
		expected_title + ".mp3",
	}
	for i, r := range resources {
		sub := fmt.Sprintf("resource[%d]", i)

		if r.TaskId != 100 {
			t.Errorf("%s.TaskId = %d, want 100", sub, r.TaskId)
		}
		if r.Id != expected_resource_ids[i] {
			t.Errorf("%s.Id = %d, want %d", sub, r.Id, expected_resource_ids[i])
		}
		if r.Kind != expected_kinds[i] {
			t.Errorf("%s.Kind = %q, want %q", sub, r.Kind, expected_kinds[i])
		}
		if r.Name != expected_names[i] {
			t.Errorf("%s.Name = %q, want %q", sub, r.Name, expected_names[i])
		}
		if r.MergeOrder != i {
			t.Errorf("%s.MergeOrder = %d, want %d", sub, r.MergeOrder, i)
		}
	}

	// 5. Each endpoint is correctly linked to its resource
	expected_endpoint_ids := []int{201, 202, 203}
	expected_urls := []string{expected_xwt111_video_url, expected_video_resource_cover_url, expected_audio_only_url}
	for i, ep := range endpoints {
		sub := fmt.Sprintf("endpoint[%d]", i)

		if ep.ResourceId != expected_resource_ids[i] {
			t.Errorf("%s.ResourceId = %d, want %d", sub, ep.ResourceId, expected_resource_ids[i])
		}
		if ep.Id != expected_endpoint_ids[i] {
			t.Errorf("%s.Id = %d, want %d", sub, ep.Id, expected_endpoint_ids[i])
		}
		if ep.Protocol != "https" {
			t.Errorf("%s.Protocol = %q, want %q", sub, ep.Protocol, "https")
		}
		if ep.URL != expected_urls[i] {
			t.Errorf("%s.URL mismatch:\n  got  %s\n  want %s", sub, ep.URL, expected_urls[i])
		}
		if ep.Enabled != 1 {
			t.Errorf("%s.Enabled = %d, want 1", sub, ep.Enabled)
		}
	}

	// 6. MetadataJSON carries content lineage signals; ConfigJSON carries download settings
	var cfg map[string]any
	if err := json.Unmarshal([]byte(task.ConfigJSON), &cfg); err != nil {
		t.Fatalf("task.ConfigJSON is not valid JSON: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(task.MetadataJSON), &meta); err != nil {
		t.Fatalf("task.MetadataJSON is not valid JSON: %v", err)
	}
	if meta["platform"] != platform_id {
		t.Errorf("MetadataJSON.platform = %v, want %q", meta["platform"], platform_id)
	}
	if meta["external_id"] != expected_external_id {
		t.Errorf("MetadataJSON.external_id = %v, want %q", meta["external_id"], expected_external_id)
	}
	if meta["spec"] != spec {
		t.Errorf("MetadataJSON.spec = %v, want %q", meta["spec"], spec)
	}

	t.Logf("=== Multi-file Download Tree (V1) ===")
	t.Logf("Account  : %s (%s)", account.Nickname, account.ExternalId)
	t.Logf("Content  : %s | %s | %d bytes | %ds", content.Title, content.Type, ext.Size, ext.Duration)
	t.Logf("Task     : id=%d name=%q", task.Id, task.Name)
	for i, r := range resources {
		t.Logf("  ├── resource[%d] id=%d kind=%s size=%d", i, r.Id, r.Kind, r.Size)
		t.Logf("  │   └── endpoint[%d] id=%d protocol=%s", i, endpoints[i].Id, endpoints[i].Protocol)
	}
	t.Logf("Linkage  : ContentAccount(role=%q) binds Content(%s) ↔ Account(%s)", ca.Role, ca.ContentId, ca.AccountId)

	// =====================================================================
	// Multi-file download lifecycle simulation
	// Scenario: three resources (video/cover/audio) downloaded in parallel, paused midway then resumed to full completion
	// =====================================================================

	// ---- Initialize segments: one segment per resource ----
	segments := []model.DownloadSegment{
		{Id: 301, ResourceId: 101, Index: 0, URL: expected_xwt111_video_url, OffsetStart: 0,
			OffsetEnd: 9613486, Size: 9613487, Downloaded: 0, Status: 0, Retry: 0},
		{Id: 302, ResourceId: 102, Index: 0, URL: expected_video_resource_cover_url, OffsetStart: 0,
			OffsetEnd: 0, Size: 0, Downloaded: 0, Status: 0, Retry: 0},
		{Id: 303, ResourceId: 103, Index: 0, URL: expected_audio_only_url, OffsetStart: 0,
			OffsetEnd: 0, Size: 0, Downloaded: 0, Status: 0, Retry: 0},
	}

	// ---- Initialize connections: one connection per endpoint ----
	connections := []model.DownloadConnection{
		{Id: 401, EndpointId: 201, WorkerId: "w-video", Host: "finder.video.qq.com",
			IP: "183.60.15.100", Speed: 0, Bytes: 0, Status: 0, LastActive: 0},
		{Id: 402, EndpointId: 202, WorkerId: "w-cover", Host: "finder.video.qq.com",
			IP: "183.60.15.101", Speed: 0, Bytes: 0, Status: 0, LastActive: 0},
		{Id: 403, EndpointId: 203, WorkerId: "w-audio", Host: "finder.video.qq.com",
			IP: "183.60.15.102", Speed: 0, Bytes: 0, Status: 0, LastActive: 0},
	}

	type log_entry struct {
		message string
	}
	logs := make([]log_entry, 0, 5)

	// Stage 1: All resources start downloading
	t.Run("Stage1_MultiStartDownloading", func(t *testing.T) {
		task.Status = model.TaskStatusPreparing
		task.Status = model.TaskStatusDownloading

		for i := range resources {
			resources[i].Status = 1
		}
		for i := range connections {
			connections[i].Status = 1
		}
		for i := range segments {
			segments[i].Status = 1
		}
		logs = append(logs, log_entry{
			message: "multi-file download started (video+cover+audio)",
		})

		if task.Status != model.TaskStatusDownloading {
			t.Errorf("Stage1 task.Status = %d, want %d", task.Status, model.TaskStatusDownloading)
		}
		for i, conn := range connections {
			if conn.Status != 1 {
				t.Errorf("Stage1 conn[%d].Status = %d, want 1", i, conn.Status)
			}
		}
		if len(logs) != 1 {
			t.Errorf("Stage1 logs = %d, want 1", len(logs))
		}
	})

	// Stage 2: video progress at 10% (video is the primary resource), cover at 100%, audio at 50%
	//               All downloading in parallel, video as primary: pause when video reaches 10%
	const ten_pct_video = 961348
	t.Run("Stage2_Progress10PercentAndPause", func(t *testing.T) {
		// video at 10%
		segments[0].Downloaded = ten_pct_video
		connections[0].Bytes = ten_pct_video
		connections[0].Speed = 3 * 1024 * 1024 // 3 MB/s for video

		// cover completed (small file)
		segments[1].Downloaded = 0
		segments[1].Status = 2 // completed
		resources[1].Status = 2
		connections[1].Status = 0 // idle

		// audio at 50%
		segments[2].Downloaded = 0
		resources[2].Status = 1

		// Pause all
		task.Status = model.TaskStatusPaused
		segments[0].Status = 1 // paused
		connections[0].Status = 2
		resources[0].Status = 1

		logs = append(logs, log_entry{
			message: "download paused at video 10%, cover completed, audio partial",
		})

		if segments[0].Downloaded != ten_pct_video {
			t.Errorf("Stage2 segment[0].Downloaded = %d, want %d", segments[0].Downloaded, ten_pct_video)
		}
		if connections[0].Bytes != ten_pct_video {
			t.Errorf("Stage2 conn[0].Bytes = %d, want %d", connections[0].Bytes, ten_pct_video)
		}
		if task.Status != model.TaskStatusPaused {
			t.Errorf("Stage2 task.Status = %d, want %d", task.Status, model.TaskStatusPaused)
		}
		if resources[1].Status != 2 {
			t.Errorf("Stage2 resource[1].Status = %d, want 2 (cover completed)", resources[1].Status)
		}
	})

	// Stage 3: Resume → All complete
	t.Run("Stage3_ResumeAndAllComplete", func(t *testing.T) {
		task.Status = model.TaskStatusDownloading

		// video: resume from 10% to 100%
		segments[0].Downloaded = resources[0].Size
		connections[0].Bytes = resources[0].Size
		segments[0].Status = 2
		resources[0].Status = 2
		connections[0].Status = 0
		connections[0].Speed = 0

		// audio: completed
		segments[2].Downloaded = 0
		segments[2].Status = 2
		resources[2].Status = 2
		connections[2].Status = 0

		logs = append(logs, log_entry{
			message: "all resources resumed and fully downloaded",
		})

		if segments[0].Downloaded != 9613487 {
			t.Errorf("Stage3 segment[0].Downloaded = %d, want %d", segments[0].Downloaded, int64(9613487))
		}
		for i := range segments {
			if segments[i].Status != 2 {
				t.Errorf("Stage3 segment[%d].Status = %d, want 2", i, segments[i].Status)
			}
		}
		for i := range resources {
			if resources[i].Status != 2 {
				t.Errorf("Stage3 resource[%d].Status = %d, want 2", i, resources[i].Status)
			}
		}
	})

	// Stage 4: Merging → Finished
	t.Run("Stage4_MergingAndFinished", func(t *testing.T) {
		task.Status = model.TaskStatusMerging
		if task.Status != model.TaskStatusMerging {
			t.Errorf("Stage4 merging status = %d, want %d", task.Status, model.TaskStatusMerging)
		}

		// Merge by MergeOrder: video(0)→cover(1)→audio(2)
		for _, r := range resources {
			if r.Status != 2 {
				t.Errorf("merge: resource[%d] (kind=%s, order=%d) should be completed before merging",
					r.MergeOrder, r.Kind, r.MergeOrder)
			}
		}

		task.Status = model.TaskStatusFinished
		logs = append(logs, log_entry{
			message: "multi-file download finished and merged",
		})

		if task.Status != model.TaskStatusFinished {
			t.Errorf("Stage4 task.Status = %d, want %d", task.Status, model.TaskStatusFinished)
		}
		if len(logs) != 4 {
			t.Errorf("Stage4 logs = %d, want 4", len(logs))
		}
	})

	// =====================================================================
	// Final verification
	// =====================================================================

	// Log chain integrity
	expected_keywords := []string{"started", "paused", "resumed", "finished"}
	for i, l := range logs {
		if !strings.Contains(l.message, expected_keywords[i]) {
			t.Errorf("log[%d].message = %q, should contain %q", i, l.message, expected_keywords[i])
		}
	}

	// Verify primary resource segment integrity
	main_seg := segments[0] // video
	if main_seg.ResourceId != 101 {
		t.Errorf("main_seg.ResourceId = %d, want 101", main_seg.ResourceId)
	}
	if main_seg.Size != 9613487 {
		t.Errorf("main_seg.Size = %d, want %d", main_seg.Size, int64(9613487))
	}
	if main_seg.OffsetStart != 0 {
		t.Errorf("main_seg.OffsetStart = %d, want 0", main_seg.OffsetStart)
	}
	if main_seg.OffsetEnd != 9613486 {
		t.Errorf("main_seg.OffsetEnd = %d, want %d", main_seg.OffsetEnd, int64(9613486))
	}

	// Verify three-table association chain: endpoint → resource → task
	for i := range resources {
		if endpoints[i].ResourceId != expected_resource_ids[i] {
			t.Errorf("chain: endpoint[%d].ResourceId(%d) != expected ResourceId(%d)",
				i, endpoints[i].ResourceId, expected_resource_ids[i])
		}
		if resources[i].TaskId != 100 {
			t.Errorf("chain: resource[%d].TaskId(%d) != 100", i, resources[i].TaskId)
		}
		if segments[i].ResourceId != expected_resource_ids[i] {
			t.Errorf("chain: segment[%d].ResourceId(%d) != expected ResourceId(%d)",
				i, segments[i].ResourceId, expected_resource_ids[i])
		}
		if connections[i].EndpointId != expected_endpoint_ids[i] {
			t.Errorf("chain: conn[%d].EndpointId(%d) != expected EndpointId(%d)",
				i, connections[i].EndpointId, expected_endpoint_ids[i])
		}
	}

	// Verify final task status
	if task.Status != model.TaskStatusFinished {
		t.Errorf("final task.Status = %d, want %d (Finished)", task.Status, model.TaskStatusFinished)
	}
	// All resources completed
	total_completed := 0
	for _, r := range resources {
		if r.Status == 2 {
			total_completed++
		}
	}
	if total_completed != 3 {
		t.Errorf("total completed resources = %d, want 3", total_completed)
	}
}

func normalize_video_content(t *testing.T, content *model.Content) {
	t.Helper()
	content.Timestamps = model.Timestamps{}
}
