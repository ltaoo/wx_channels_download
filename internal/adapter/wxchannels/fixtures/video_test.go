package wxchannels_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	wxchannels "wx_channel/internal/adapter/wxchannels"
	"wx_channel/internal/database/model"
	"wx_channel/internal/adapter"
	"wx_channel/pkg/testui"
	"wx_channel/pkg/testui/assert"
	"wx_channel/pkg/testui/require"
	example "wx_channel/scraper_examples"
)

const expectedVideoURL = "https://finder.video.qq.com/251/20302/stodownload?encfilekey=Cvvj5Ix3eewQsFvYyicia1J4vPZhKwibibyibAO6BVb6JtHx7sfjtTfmCnIib4dtTeSl2Skialoibjc4ia6VtH3tyOo2Sbfhz1vNa4lmBoRG3uapCVhgnZfcJBou7lg&token=2lt8WBSnjTkTjXXRcWF576SLtqb9LdRn1Cliaa0icf5zFjCLyBFNe1e3eKzhzzEc5h05O81ibb3hwbVTVywYQAQbSQzZkHicCqabpEdwBzhTgdyPiakaMMw7n96CtNxoPbKkQxiaYOzPImgS9ZG3kDzKcLjMEyIIVGYuibzdHECVIOFibOQGL4pWibDRRD6VcpGApwhugo6k9Mq48YAov7zg751dO260H5iaGeEkJZWhKhib0hib4W0&basedata=CAMSBnhXVDE1OCJaCgoKBnhXVDE1OBAACgoKBnhXVDE1NxAACgoKBnhXVDE1NhAACgoKBnhXVDExMxAACgoKBnhXVDExMhAACgoKBnhXVDExMRAACgcKA3hBMBAACgcKA3hBMhAA&sign=AgZzkYT5vBvSWwKe5MpufA75x2T3Xnnz7PtuTK98WxdVbZm4Grpnyl52sDN4W6CI562FVgGaZ-_tYlBjCRLdIQ&web=1&extg=10f0000&svrbypass=AAuL%2FQsFAAABAAAAAABRfl4aFfX8vo5XJgBRahAAAADnaHZTnGbFfAj9RgZXfw6ViUCWOt8LYujr%2BrkpCHNy7PD375%2BDqLzGDCk8ibQxWRl9tKOjUKAhiL4%3D&svrnonce=1783693350"

const expectedVideoCoverURL = "https://finder.video.qq.com/251/20350/stodownload?encfilekey=2fG3V4WwQPnQr0gxUocFa2h6q3eoq4hXzG39ub5SWukSZAsfOaRiadTuuGIYouJicfpVpzk12gN6RJ2mlOl26YUBWWTVupMcpSIhJDGZaKiaRI&token=ic1n0xDG6awibhOHyNxbvz6nLNtsL3qg5UrFPrz5Jj4TMUicLBbchc6FxnZm5WybqCJGmyeCPokfKqLKqgia6PpXIc7oxANHcCfUGvZ2tkcIfe9Gnz8pKU6G2fVsHnRmVYqPkoqyLdic9MrwTdQWmCLTamzeQ40lL8sTUiaaMgr0QibWm7wQAbtMvUalYywFOoiaotMxjeEHU4mg8GLIS33rP8iaUwuyIrBiandouT&hy=SZ&idx=1&m=7b022855f315b6aa0a3dd30f631d1d4a&picformat=200&wxampicformat=503"

var (
	expectedVideoContentID   = "wxchannels:14962486294771997060"
	expectedVideoPublishTime = int64(1783667361)
)

const expectedXWT111VideoURL = "https://finder.video.qq.com/251/20302/stodownload?encfilekey=Cvvj5Ix3eewQsFvYyicia1J4vPZhKwibibyibAO6BVb6JtHx7sfjtTfmCnIib4dtTeSl2Skialoibjc4ia6VtH3tyOo2Sbfhz1vNa4lmBoRG3uapCVhgnZfcJBou7lg&token=2lt8WBSnjTkTjXXRcWF576SLtqb9LdRn1Cliaa0icf5zFjCLyBFNe1e3eKzhzzEc5h05O81ibb3hwbVTVywYQAQbSQzZkHicCqabpEdwBzhTgdyPiakaMMw7n96CtNxoPbKkQxiaYOzPImgS9ZG3kDzKcLjMEyIIVGYuibzdHECVIOFibOQGL4pWibDRRD6VcpGApwhugo6k9Mq48YAov7zg751dO260H5iaGeEkJZWhKhib0hib4W0&basedata=CAMSBnhXVDE1OCJaCgoKBnhXVDE1OBAACgoKBnhXVDE1NxAACgoKBnhXVDE1NhAACgoKBnhXVDExMxAACgoKBnhXVDExMhAACgoKBnhXVDExMRAACgcKA3hBMBAACgcKA3hBMhAA&sign=AgZzkYT5vBvSWwKe5MpufA75x2T3Xnnz7PtuTK98WxdVbZm4Grpnyl52sDN4W6CI562FVgGaZ-_tYlBjCRLdIQ&web=1&extg=10f0000&svrbypass=AAuL%2FQsFAAABAAAAAABRfl4aFfX8vo5XJgBRahAAAADnaHZTnGbFfAj9RgZXfw6ViUCWOt8LYujr%2BrkpCHNy7PD375%2BDqLzGDCk8ibQxWRl9tKOjUKAhiL4%3D&svrnonce=1783693350&X-snsvideoflag=xWT111"

const expectedVideoResourceCoverURL = "https://finder.video.qq.com/251/20304/stodownload?encfilekey=2fG3V4WwQPluPpjb46OTKMXHc112k4G2oJic38N7rnuA86EibU1Y76s8oA7ibJ2icEheVFXiah55XOtQTzMnAsGIe2IWYOSogJ0DHQGv97AFZePM&token=AxricY7RBHdVdU7Gn7iczBDOyqkPzEiazv6slYib62vrPnRWLdajxdDW6L5750WibUCk6R96RGUJ3MAHbTqSV90lo9nH8Wn7JShFsWZgr68VIDPoEYFqYLakd4tDgsE26h00sXkjVy5cSHmf6aCEbjhuJYGRaQ3eZISKiatbry08Ugw1R9B6zzeWxvqJ2hNlojz1GCPcpNq8j85OXOWGlicSBmVd3kQGj5vTzx7&hy=SZ&idx=1&m=73a9ef1bc335f9c43d800208ddc42f09&uzid=1&picformat=200&wxampicformat=503"

const expectedAudioOnlyURL = "https://finder.video.qq.com/251/20302/stodownload?encfilekey=Cvvj5Ix3eewQsFvYyicia1J4vPZhKwibibyibAO6BVb6JtHx7sfjtTfmCnIib4dtTeSl2Skialoibjc4ia6VtH3tyOo2Sbfhz1vNa4lmBoRG3uapCVhgnZfcJBou7lg&hy=SZ&idx=1&m=414c8b10462c8fa97a904c3d999a0476&uzid=7a206&X-snsvideoflag=xWT111&audio_only=1"

func TestToAccount_FromVideoFeedJSON(t *testing.T) {
	raw, err := example.Load("wxchannels/260710/video.json")
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
	expectedAccount := model.Account{
		Id:         "wxchannels:v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
		PlatformId: "wxchannels",
		ExternalId: "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
		Nickname:   "迷人的大嘴猴",
		Signature:  "谢谢观看\n只是爱分享一些大哥爱看的視頻 仅此而已\n懂点规矩 蠢狗不要发私信",
		AvatarURL:  "https://wx.qlogo.cn/finderhead/ver_1/6Tb4IdXSgHeMiaInfddhMkcUpPVnibc60ofHpia1hSUfepsmeuFibGSicicTDN3r8cU4LG9Ef73YyfY3X1mibOGtNgpBKTficKq9tEgaBZTtnNMaviam6JySau4JCnYIibcK9aMicWsJC6IqJCU7gjKwsniaNRlncw/132",
	}
	testui.AssertStrictEqual(t, "Account", expectedAccount, account)
}

func TestToContent_FromVideoFeedJSON(t *testing.T) {
	raw, err := example.Load("wxchannels/260710/video.json")
	if err != nil {
		t.Fatalf("load video fixture: %v", err)
	}

	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	content, contentVideo, err := wxchannels.ToContent(&obj)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	normalizeVideoContent(t, content)
	expectedContent := model.Content{
		Id:          "wxchannels:14962486294771997060",
		PlatformId:  "wxchannels",
		Type:        "video",
		ExternalId:  "14962486294771997060",
		ExternalId2: "4390481592474233535_0_146_0_0",
		Title:       "讨厌我有什么用 有本事弄死我",
		Description: "讨厌我有什么用 有本事弄死我",
		URL:         expectedVideoURL,
		SourceURL:   "https://channels.weixin.qq.com/web/pages/feed?username=v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665%40finder&oid=z6VuAqyJGYQ&nid=PO4fvyBRar8",
		CoverURL:    expectedVideoCoverURL,
		CoverWidth:  "1080",
		CoverHeight: "2128",
		PublishTime: &expectedVideoPublishTime,
	}
	testui.AssertStrictEqual(t, "Content", expectedContent, content)

	expectedVideo := model.ContentVideo{
		Id:       "wxchannels:14962486294771997060",
		Duration: 9,
		Width:    1080,
		Height:   2128,
		Size:     9613487,
		URL:      expectedVideoURL,
	}
	testui.AssertStrictEqual(t, "ContentVideo", expectedVideo, contentVideo)
}

func TestBuildBrowseRecord_FromVideoFeedJSON(t *testing.T) {
	raw, err := example.Load("wxchannels/260710/video.json")
	if err != nil {
		t.Fatalf("load video fixture: %v", err)
	}

	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	r := wxchannels.BuildBrowseRecordFromObject(&obj)
	if r.PublishTime == nil || *r.PublishTime != 1783667361 {
		t.Fatalf("unexpected publish time: %v", r.PublishTime)
	}
	r.PublishTime = nil
	r.Timestamps = model.Timestamps{}

	expected := model.BrowseHistory{
		Id:           "wxchannels:14962486294771997060",
		PlatformId:   "wxchannels",
		VisitedTimes: 1,
		Type:         "video",
		ExternalId:   "14962486294771997060",
		Title:             "讨厌我有什么用 有本事弄死我",
		URL:               "https://finder.video.qq.com/251/20302/stodownload?encfilekey=Cvvj5Ix3eewQsFvYyicia1J4vPZhKwibibyibAO6BVb6JtHx7sfjtTfmCnIib4dtTeSl2Skialoibjc4ia6VtH3tyOo2Sbfhz1vNa4lmBoRG3uapCVhgnZfcJBou7lg&token=2lt8WBSnjTkTjXXRcWF576SLtqb9LdRn1Cliaa0icf5zFjCLyBFNe1e3eKzhzzEc5h05O81ibb3hwbVTVywYQAQbSQzZkHicCqabpEdwBzhTgdyPiakaMMw7n96CtNxoPbKkQxiaYOzPImgS9ZG3kDzKcLjMEyIIVGYuibzdHECVIOFibOQGL4pWibDRRD6VcpGApwhugo6k9Mq48YAov7zg751dO260H5iaGeEkJZWhKhib0hib4W0&basedata=CAMSBnhXVDE1OCJaCgoKBnhXVDE1OBAACgoKBnhXVDE1NxAACgoKBnhXVDE1NhAACgoKBnhXVDExMxAACgoKBnhXVDExMhAACgoKBnhXVDExMRAACgcKA3hBMBAACgcKA3hBMhAA&sign=AgZzkYT5vBvSWwKe5MpufA75x2T3Xnnz7PtuTK98WxdVbZm4Grpnyl52sDN4W6CI562FVgGaZ-_tYlBjCRLdIQ&web=1&extg=10f0000&svrbypass=AAuL%2FQsFAAABAAAAAABRfl4aFfX8vo5XJgBRahAAAADnaHZTnGbFfAj9RgZXfw6ViUCWOt8LYujr%2BrkpCHNy7PD375%2BDqLzGDCk8ibQxWRl9tKOjUKAhiL4%3D&svrnonce=1783693350",
		SourceURL:         "https://channels.weixin.qq.com/web/pages/feed?username=v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665%40finder&oid=z6VuAqyJGYQ&nid=PO4fvyBRar8",
		CoverURL:          "https://finder.video.qq.com/251/20350/stodownload?encfilekey=2fG3V4WwQPnQr0gxUocFa2h6q3eoq4hXzG39ub5SWukSZAsfOaRiadTuuGIYouJicfpVpzk12gN6RJ2mlOl26YUBWWTVupMcpSIhJDGZaKiaRI&token=ic1n0xDG6awibhOHyNxbvz6nLNtsL3qg5UrFPrz5Jj4TMUicLBbchc6FxnZm5WybqCJGmyeCPokfKqLKqgia6PpXIc7oxANHcCfUGvZ2tkcIfe9Gnz8pKU6G2fVsHnRmVYqPkoqyLdic9MrwTdQWmCLTamzeQ40lL8sTUiaaMgr0QibWm7wQAbtMvUalYywFOoiaotMxjeEHU4mg8GLIS33rP8iaUwuyIrBiandouT&hy=SZ&idx=1&m=7b022855f315b6aa0a3dd30f631d1d4a&picformat=200&wxampicformat=503",
		CoverWidth:        "1080",
		CoverHeight:       "2128",
		ExtraData:         `{"decode_key":"1522886121","id":"14962486294771997060","nonce_id":"4390481592474233535_0_146_0_0"}`,
	}

	testui.AssertStrictEqual(t, "BrowseHistory", expected, r)
}

// TestBuildDownloadTask_FromContent verifies the model conversion and linkage
// from a raw video feed JSON: Account, Content, BrowseHistory, and DownloadTask
// are correctly built and cross-referenced.
func TestBuildDownloadTask_FromContent(t *testing.T) {
	contentJSON, err := example.Load("wxchannels/260710/video.json")
	if err != nil {
		t.Fatalf("load video fixture: %v", err)
	}
	var obj wxchannels.ChannelsObject
	require.NoError(t, json.Unmarshal(contentJSON, &obj))

	h := adapter.Get(wxchannels.PlatformID)
	if h == nil {
		t.Fatal("handler not registered for platform wxchannels")
	}

	config := map[string]any{}
	cfgJSON, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	info, err := h.BuildDownloadTask(contentJSON, json.RawMessage(cfgJSON))
	if err != nil {
		t.Fatalf("build download task failed: %v", err)
	}
	content := info.Content
	account := info.Account
	contentVideo := info.ContentDetail
	account.Timestamps = model.Timestamps{}
	normalizeVideoContent(t, content)

	testui.AssertStrictEqual(t, "Account", model.Account{
		Id:         "wxchannels:v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
		PlatformId: "wxchannels",
		ExternalId: "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
		Nickname:   "迷人的大嘴猴",
		Signature:  "谢谢观看\n只是爱分享一些大哥爱看的視頻 仅此而已\n懂点规矩 蠢狗不要发私信",
		AvatarURL:  "https://wx.qlogo.cn/finderhead/ver_1/6Tb4IdXSgHeMiaInfddhMkcUpPVnibc60ofHpia1hSUfepsmeuFibGSicicTDN3r8cU4LG9Ef73YyfY3X1mibOGtNgpBKTficKq9tEgaBZTtnNMaviam6JySau4JCnYIibcK9aMicWsJC6IqJCU7gjKwsniaNRlncw/132",
	}, *account)

	expectedContent := model.Content{
		Id:          "wxchannels:14962486294771997060",
		PlatformId:  "wxchannels",
		Type:        "video",
		ExternalId:  "14962486294771997060",
		ExternalId2: "4390481592474233535_0_146_0_0",
		Title:       "讨厌我有什么用 有本事弄死我",
		Description: "讨厌我有什么用 有本事弄死我",
		URL:         expectedVideoURL,
		SourceURL:   "https://channels.weixin.qq.com/web/pages/feed?username=v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665%40finder&oid=z6VuAqyJGYQ&nid=PO4fvyBRar8",
		CoverURL:    expectedVideoCoverURL,
		CoverWidth:  "1080",
		CoverHeight: "2128",
		PublishTime: &expectedVideoPublishTime,
	}
	testui.AssertStrictEqual(t, "Content", expectedContent, content)
	testui.AssertStrictEqual(t, "ContentVideo", model.ContentVideo{
		Id:       "wxchannels:14962486294771997060",
		Duration: 9,
		Width:    1080,
		Height:   2128,
		Size:     9613487,
		URL:      expectedVideoURL,
	}, contentVideo)

	// ---- V1 DownloadTask: task-level container ----
	expectedTask := model.DownloadTask{
		ContentId:   &expectedVideoContentID,
		Name:        "讨厌我有什么用 有本事弄死我",
		PlatformId:  "wxchannels",
		UniqueID:    "14962486294771997060",
		Status:      model.TaskStatusWaiting,
		SourceURL:   "https://channels.weixin.qq.com/web/pages/feed?username=v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665%40finder&oid=z6VuAqyJGYQ&nid=PO4fvyBRar8",
		CoverURL:    expectedVideoCoverURL,
		CoverWidth:  "1080",
		CoverHeight: "2128",
		ConfigJSON:  "{}",
	}
	testui.AssertStrictEqual(t, "DownloadTask", expectedTask, info.Task)

	require.Len(t, info.Resources, 1)
	expectedResource := model.DownloadResource{
		ContentId: &expectedVideoContentID,
		Name:      "讨厌我有什么用 有本事弄死我",
		Kind:      "video/mp4",
		UniqueID:  "14962486294771997060",
		Size:      9613487,
		Extra:     `{"id":"14962486294771997060","title":"讨厌我有什么用 有本事弄死我","created_at":"1783667361","author":"迷人的大嘴猴","decode_key":"1522886121"}`,
	}
	testui.AssertStrictEqual(t, "DownloadResource", expectedResource, info.Resources[0].DownloadResource)

	require.Len(t, info.Resources[0].Endpoints, 1)
	expectedEndpoint := model.DownloadEndpoint{
		Protocol: "https",
		URL:      expectedXWT111VideoURL,
		Enabled:  1,
	}
	testui.AssertStrictEqual(t, "DownloadEndpoint", expectedEndpoint, info.Resources[0].Endpoints[0])
}

// TestBuildDownloadTask_FromContent_WithSpecAndSuffix verifies that when both
// Spec and Suffix are set in the download config, the generated UniqueID and download
// URL correctly reflect the configuration.
func TestBuildDownloadTask_FromContent_WithSpecAndSuffix(t *testing.T) {
	contentJSON, err := example.Load("wxchannels/260710/video.json")
	if err != nil {
		t.Fatalf("load video fixture: %v", err)
	}
	var obj wxchannels.ChannelsObject
	require.NoError(t, json.Unmarshal(contentJSON, &obj))

	h := adapter.Get(wxchannels.PlatformID)
	if h == nil {
		t.Fatal("handler not registered for platform wxchannels")
	}

	config := map[string]any{"spec": "xWT111", "suffix": ".mp3"}
	cfgJSON, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	info, err := h.BuildDownloadTask(contentJSON, json.RawMessage(cfgJSON))
	if err != nil {
		t.Fatalf("build download task failed: %v", err)
	}
	content := info.Content
	account := info.Account
	contentVideo := info.ContentDetail
	account.Timestamps = model.Timestamps{}
	normalizeVideoContent(t, content)

	testui.AssertStrictEqual(t, "Account", model.Account{
		Id:         "wxchannels:v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
		PlatformId: "wxchannels",
		ExternalId: "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
		Nickname:   "迷人的大嘴猴",
		Signature:  "谢谢观看\n只是爱分享一些大哥爱看的視頻 仅此而已\n懂点规矩 蠢狗不要发私信",
		AvatarURL:  "https://wx.qlogo.cn/finderhead/ver_1/6Tb4IdXSgHeMiaInfddhMkcUpPVnibc60ofHpia1hSUfepsmeuFibGSicicTDN3r8cU4LG9Ef73YyfY3X1mibOGtNgpBKTficKq9tEgaBZTtnNMaviam6JySau4JCnYIibcK9aMicWsJC6IqJCU7gjKwsniaNRlncw/132",
	}, *account)

	expectedContent := model.Content{
		Id:          "wxchannels:14962486294771997060",
		PlatformId:  "wxchannels",
		Type:        "video",
		ExternalId:  "14962486294771997060",
		ExternalId2: "4390481592474233535_0_146_0_0",
		Title:       "讨厌我有什么用 有本事弄死我",
		Description: "讨厌我有什么用 有本事弄死我",
		URL:         expectedVideoURL,
		SourceURL:   "https://channels.weixin.qq.com/web/pages/feed?username=v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665%40finder&oid=z6VuAqyJGYQ&nid=PO4fvyBRar8",
		CoverURL:    expectedVideoCoverURL,
		CoverWidth:  "1080",
		CoverHeight: "2128",
		PublishTime: &expectedVideoPublishTime,
	}
	testui.AssertStrictEqual(t, "Content", expectedContent, content)
	testui.AssertStrictEqual(t, "ContentVideo", model.ContentVideo{
		Id:       "wxchannels:14962486294771997060",
		Duration: 9,
		Width:    1080,
		Height:   2128,
		Size:     9613487,
		URL:      expectedVideoURL,
	}, contentVideo)

	// ---- V1 DownloadTask: UniqueID reflects both spec and suffix ----
	testui.AssertStrictEqual(t, "DownloadTask", model.DownloadTask{
		ContentId:   &expectedVideoContentID,
		Name:        "讨厌我有什么用 有本事弄死我",
		PlatformId:  "wxchannels",
		UniqueID:    "14962486294771997060_xWT111_mp3",
		Status:      model.TaskStatusWaiting,
		SourceURL:   "https://channels.weixin.qq.com/web/pages/feed?username=v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665%40finder&oid=z6VuAqyJGYQ&nid=PO4fvyBRar8",
		CoverURL:    expectedVideoCoverURL,
		CoverWidth:  "1080",
		CoverHeight: "2128",
		ConfigJSON:  `{"spec":"xWT111","suffix":".mp3"}`,
	}, info.Task)

	require.Len(t, info.Resources, 1)
	testui.AssertStrictEqual(t, "DownloadResource", model.DownloadResource{
		ContentId: &expectedVideoContentID,
		Name:      "讨厌我有什么用 有本事弄死我",
		Kind:      "audio/mpeg",
		UniqueID:  "14962486294771997060_xWT111_mp3",
		Size:      9613487,
		Extra:     `{"id":"14962486294771997060","title":"讨厌我有什么用 有本事弄死我","created_at":"1783667361","author":"迷人的大嘴猴","decode_key":"1522886121"}`,
	}, info.Resources[0].DownloadResource)

	require.Len(t, info.Resources[0].Endpoints, 1)
	testui.AssertStrictEqual(t, "DownloadEndpoint", model.DownloadEndpoint{
		Protocol: "https",
		URL:      expectedXWT111VideoURL,
		Enabled:  1,
	}, info.Resources[0].Endpoints[0])
}

func TestBuildDownloadTaskWithMultiResource_FromContent(t *testing.T) {
	contentJSON, err := example.Load("wxchannels/260710/video.json")
	if err != nil {
		t.Fatalf("load video fixture: %v", err)
	}

	h := adapter.Get(wxchannels.PlatformID)
	if h == nil {
		t.Fatal("handler not registered for platform wxchannels")
	}

	config := map[string]any{}
	cfgJSON, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	info, err := h.BuildDownloadTask(contentJSON, json.RawMessage(cfgJSON))
	if err != nil {
		t.Fatalf("build download task failed: %v", err)
	}
	content := info.Content
	account := info.Account
	account.Timestamps = model.Timestamps{}
	normalizeVideoContent(t, content)

	testui.AssertStrictEqual(t, "Account", model.Account{
		Id:         "wxchannels:v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
		PlatformId: "wxchannels",
		ExternalId: "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder",
		Nickname:   "迷人的大嘴猴",
		Signature:  "谢谢观看\n只是爱分享一些大哥爱看的視頻 仅此而已\n懂点规矩 蠢狗不要发私信",
		AvatarURL:  "https://wx.qlogo.cn/finderhead/ver_1/6Tb4IdXSgHeMiaInfddhMkcUpPVnibc60ofHpia1hSUfepsmeuFibGSicicTDN3r8cU4LG9Ef73YyfY3X1mibOGtNgpBKTficKq9tEgaBZTtnNMaviam6JySau4JCnYIibcK9aMicWsJC6IqJCU7gjKwsniaNRlncw/132",
	}, *account)

	testui.AssertStrictEqual(t, "Content", model.Content{
		Id:          "wxchannels:14962486294771997060",
		PlatformId:  "wxchannels",
		Type:        "video",
		ExternalId:  "14962486294771997060",
		ExternalId2: "4390481592474233535_0_146_0_0",
		Title:       "讨厌我有什么用 有本事弄死我",
		Description: "讨厌我有什么用 有本事弄死我",
		URL:         expectedVideoURL,
		SourceURL:   "https://channels.weixin.qq.com/web/pages/feed?username=v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665%40finder&oid=z6VuAqyJGYQ&nid=PO4fvyBRar8",
		CoverURL:    expectedVideoCoverURL,
		CoverWidth:  "1080",
		CoverHeight: "2128",
		PublishTime: &expectedVideoPublishTime,
	}, content)

	// ---- V1 DownloadTask: task-level container ----
	require.NotNil(t, info.Task.ContentId)
	assert.Equal(t, "wxchannels:14962486294771997060", *info.Task.ContentId)
	assert.Equal(t, "14962486294771997060", info.Task.UniqueID)
	assert.Equal(t, "wxchannels", info.Task.PlatformId)
	assert.Equal(t, "讨厌我有什么用 有本事弄死我", info.Task.Name)
	assert.Equal(t, model.TaskStatusWaiting, info.Task.Status)
	assert.Equal(t, "{}", info.Task.ConfigJSON)
	require.Len(t, info.Resources, 1)
	assert.Equal(t, "video/mp4", info.Resources[0].DownloadResource.Kind)
	assert.Equal(t, "14962486294771997060", info.Resources[0].DownloadResource.UniqueID)
	assert.Equal(t, int64(9613487), info.Resources[0].DownloadResource.Size)
	assert.Equal(t, "https", info.Resources[0].Endpoints[0].Protocol)
	assert.Equal(t, 1, info.Resources[0].Endpoints[0].Enabled)
}

// TestBuildDownloadTask_FromContent_Lifecycle simulates the download lifecycle state machine:
// Start → 10% pause → Resume → 100% complete → Merging → Finished.
// Validates state transitions and data consistency across task/resource/endpoint/segment/conn entities.
func TestBuildDownloadTask_FromContent_Lifecycle(t *testing.T) {
	task := model.DownloadTask{
		Id:           1,
		Name:         "讨厌我有什么用 有本事弄死我",
		UniqueID:     "14962486294771997060",
		PlatformId:   "wxchannels",
		Status:       model.TaskStatusWaiting,
		ConfigJSON:   `{"download_cover":false,"overwrite":false}`,
		MetadataJSON: `{"platform":"wxchannels","external_id":"14962486294771997060"}`,
	}

	resource := model.DownloadResource{
		Id:     1,
		TaskId: 1,
		Name:   "讨厌我有什么用 有本事弄死我.mp4",
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

	const tenPercent = 961348
	type logEntry struct {
		Message string
	}
	logs := make([]logEntry, 0, 4)

	// Stage 1: Start download → Preparing → Downloading
	t.Run("Stage1_StartDownloading", func(t *testing.T) {
		task.Status = model.TaskStatusPreparing
		if task.Status != model.TaskStatusPreparing {
			t.Errorf("Preparing.Status = %d, want %d", task.Status, model.TaskStatusPreparing)
		}

		task.Status = model.TaskStatusDownloading
		conn.Status = 1
		logs = append(logs, logEntry{
			Message: "download started",
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
		segment.Downloaded = tenPercent
		conn.Bytes = tenPercent
		conn.Speed = 1024 * 1024 // 1 MB/s

		task.Status = model.TaskStatusPaused
		resource.Status = 1
		segment.Status = 1
		conn.Status = 2
		logs = append(logs, logEntry{
			Message: "download paused at 10%",
		})

		if segment.Downloaded != tenPercent {
			t.Errorf("Stage2 segment.Downloaded = %d, want %d (10%%)", segment.Downloaded, tenPercent)
		}
		if conn.Bytes != tenPercent {
			t.Errorf("Stage2 conn.Bytes = %d, want %d", conn.Bytes, tenPercent)
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
		logs = append(logs, logEntry{
			Message: "download resumed",
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
		logs = append(logs, logEntry{
			Message: "download finished",
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
	expectedLogLevels := []string{"started", "paused", "resumed", "finished"}
	for i, l := range logs {
		if !strings.Contains(l.Message, expectedLogLevels[i]) {
			t.Errorf("log[%d].Message = %q, should contain %q", i, l.Message, expectedLogLevels[i])
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
	raw, err := example.Load("wxchannels/260710/video.json")
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
	content, rawExt, err := wxchannels.ToContent(&obj)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	ext, ok := rawExt.(*model.ContentVideo)
	if !ok {
		t.Fatalf("extension is not *ContentVideo, got %T", rawExt)
	}
	const platformId = "wxchannels"
	const spec = "xWT111"

	// ---- Create V1 DownloadTask: task-level container ----
	const taskId = 100
	task := model.DownloadTask{
		Id:           taskId,
		Name:         "讨厌我有什么用 有本事弄死我",
		UniqueID:     "14962486294771997060",
		PlatformId:   "wxchannels",
		Status:       model.TaskStatusWaiting,
		ConfigJSON:   `{"convert_mp3":false,"download_cover":false,"duplicate":false,"overwrite":false,"upload_cloud":false}`,
		MetadataJSON: `{"external_id":"14962486294771997060","nonce_id":"4390481592474233535_0_146_0_0","platform":"wxchannels","spec":"xWT111"}`,
	}

	// ---- Create three V1 DownloadResources (one per file type) ----
	resources := []model.DownloadResource{
		{Id: 101, TaskId: 100, Name: "讨厌我有什么用 有本事弄死我.mp4", Kind: "video/mp4", Size: 9613487, Status: 0, MergeOrder: 0},
		{Id: 102, TaskId: 100, Name: "讨厌我有什么用 有本事弄死我.jpg", Kind: "image/jpeg", Size: 0, Status: 0, MergeOrder: 1},
		{Id: 103, TaskId: 100, Name: "讨厌我有什么用 有本事弄死我.mp3", Kind: "audio/mpeg", Size: 0, Status: 0, MergeOrder: 2},
	}

	// ---- Create three V1 DownloadEndpoints (one per resource) ----
	endpoints := []model.DownloadEndpoint{
		{Id: 201, ResourceId: 101, Protocol: "https", URL: expectedXWT111VideoURL, Priority: 0, Enabled: 1, Status: 0},
		{Id: 202, ResourceId: 102, Protocol: "https", URL: expectedVideoResourceCoverURL, Priority: 0, Enabled: 1, Status: 0},
		{Id: 203, ResourceId: 103, Protocol: "https", URL: expectedAudioOnlyURL, Priority: 0, Enabled: 1, Status: 0},
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
	if account.PlatformId != platformId {
		t.Errorf("Account.PlatformId = %q, want %q", account.PlatformId, platformId)
	}
	if account.ExternalId != "v2_060000231003b20faec8c5e78f19c4d7ca0dee30b077a7a4527af7236dbe740f76db287e7665@finder" {
		t.Errorf("Account.ExternalId mismatch")
	}
	if content.PlatformId != platformId {
		t.Errorf("Content.PlatformId = %q, want %q", content.PlatformId, platformId)
	}
	if content.ExternalId != "14962486294771997060" {
		t.Errorf("Content.ExternalId = %q, want %q", content.ExternalId, "14962486294771997060")
	}
	if content.Type != "video" {
		t.Errorf("Content.ContentType = %q, want %q", content.Type, "video")
	}

	// 2. Content → Account linkage
	if ca.Role != "owner" {
		t.Errorf("ContentAccount.Role = %q, want 'owner'", ca.Role)
	}

	// 3. DownloadTask shape
	if task.Id != taskId {
		t.Errorf("task.Id = %d, want %d", task.Id, taskId)
	}
	if task.Name != "讨厌我有什么用 有本事弄死我" {
		t.Errorf("task.Name = %q, want %q", task.Name, "讨厌我有什么用 有本事弄死我")
	}
	if task.Status != model.TaskStatusWaiting {
		t.Errorf("task.Status = %v, want %v", task.Status, model.TaskStatusWaiting)
	}
	// 4. Each resource is correctly linked to the task
	expectedResourceIDs := []int{101, 102, 103}
	expectedKinds := []string{"video/mp4", "image/jpeg", "audio/mpeg"}
	expectedNames := []string{
		"讨厌我有什么用 有本事弄死我.mp4",
		"讨厌我有什么用 有本事弄死我.jpg",
		"讨厌我有什么用 有本事弄死我.mp3",
	}
	for i, r := range resources {
		sub := fmt.Sprintf("resource[%d]", i)

		if r.TaskId != 100 {
			t.Errorf("%s.TaskId = %d, want 100", sub, r.TaskId)
		}
		if r.Id != expectedResourceIDs[i] {
			t.Errorf("%s.Id = %d, want %d", sub, r.Id, expectedResourceIDs[i])
		}
		if r.Kind != expectedKinds[i] {
			t.Errorf("%s.Kind = %q, want %q", sub, r.Kind, expectedKinds[i])
		}
		if r.Name != expectedNames[i] {
			t.Errorf("%s.Name = %q, want %q", sub, r.Name, expectedNames[i])
		}
		if r.MergeOrder != i {
			t.Errorf("%s.MergeOrder = %d, want %d", sub, r.MergeOrder, i)
		}
	}

	// 5. Each endpoint is correctly linked to its resource
	expectedEndpointIDs := []int{201, 202, 203}
	expectedURLs := []string{expectedXWT111VideoURL, expectedVideoResourceCoverURL, expectedAudioOnlyURL}
	for i, ep := range endpoints {
		sub := fmt.Sprintf("endpoint[%d]", i)

		if ep.ResourceId != expectedResourceIDs[i] {
			t.Errorf("%s.ResourceId = %d, want %d", sub, ep.ResourceId, expectedResourceIDs[i])
		}
		if ep.Id != expectedEndpointIDs[i] {
			t.Errorf("%s.Id = %d, want %d", sub, ep.Id, expectedEndpointIDs[i])
		}
		if ep.Protocol != "https" {
			t.Errorf("%s.Protocol = %q, want %q", sub, ep.Protocol, "https")
		}
		if ep.URL != expectedURLs[i] {
			t.Errorf("%s.URL mismatch:\n  got  %s\n  want %s", sub, ep.URL, expectedURLs[i])
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
	if meta["platform"] != platformId {
		t.Errorf("MetadataJSON.platform = %v, want %q", meta["platform"], platformId)
	}
	if meta["external_id"] != "14962486294771997060" {
		t.Errorf("MetadataJSON.external_id = %v, want %q", meta["external_id"], "14962486294771997060")
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
		{Id: 301, ResourceId: 101, Index: 0, URL: expectedXWT111VideoURL, OffsetStart: 0,
			OffsetEnd: 9613486, Size: 9613487, Downloaded: 0, Status: 0, Retry: 0},
		{Id: 302, ResourceId: 102, Index: 0, URL: expectedVideoResourceCoverURL, OffsetStart: 0,
			OffsetEnd: 0, Size: 0, Downloaded: 0, Status: 0, Retry: 0},
		{Id: 303, ResourceId: 103, Index: 0, URL: expectedAudioOnlyURL, OffsetStart: 0,
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

	type logEntry struct {
		Message string
	}
	logs := make([]logEntry, 0, 5)

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
		logs = append(logs, logEntry{
			Message: "multi-file download started (video+cover+audio)",
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
	const tenPctVideo = 961348
	t.Run("Stage2_Progress10PercentAndPause", func(t *testing.T) {
		// video at 10%
		segments[0].Downloaded = tenPctVideo
		connections[0].Bytes = tenPctVideo
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

		logs = append(logs, logEntry{
			Message: "download paused at video 10%, cover completed, audio partial",
		})

		if segments[0].Downloaded != tenPctVideo {
			t.Errorf("Stage2 segment[0].Downloaded = %d, want %d", segments[0].Downloaded, tenPctVideo)
		}
		if connections[0].Bytes != tenPctVideo {
			t.Errorf("Stage2 conn[0].Bytes = %d, want %d", connections[0].Bytes, tenPctVideo)
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

		logs = append(logs, logEntry{
			Message: "all resources resumed and fully downloaded",
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
		logs = append(logs, logEntry{
			Message: "multi-file download finished and merged",
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
	expectedKeywords := []string{"started", "paused", "resumed", "finished"}
	for i, l := range logs {
		if !strings.Contains(l.Message, expectedKeywords[i]) {
			t.Errorf("log[%d].Message = %q, should contain %q", i, l.Message, expectedKeywords[i])
		}
	}

	// Verify primary resource segment integrity
	mainSeg := segments[0] // video
	if mainSeg.ResourceId != 101 {
		t.Errorf("mainSeg.ResourceId = %d, want 101", mainSeg.ResourceId)
	}
	if mainSeg.Size != 9613487 {
		t.Errorf("mainSeg.Size = %d, want %d", mainSeg.Size, int64(9613487))
	}
	if mainSeg.OffsetStart != 0 {
		t.Errorf("mainSeg.OffsetStart = %d, want 0", mainSeg.OffsetStart)
	}
	if mainSeg.OffsetEnd != 9613486 {
		t.Errorf("mainSeg.OffsetEnd = %d, want %d", mainSeg.OffsetEnd, int64(9613486))
	}

	// Verify three-table association chain: endpoint → resource → task
	for i := range resources {
		if endpoints[i].ResourceId != expectedResourceIDs[i] {
			t.Errorf("chain: endpoint[%d].ResourceId(%d) != expected ResourceId(%d)",
				i, endpoints[i].ResourceId, expectedResourceIDs[i])
		}
		if resources[i].TaskId != 100 {
			t.Errorf("chain: resource[%d].TaskId(%d) != 100", i, resources[i].TaskId)
		}
		if segments[i].ResourceId != expectedResourceIDs[i] {
			t.Errorf("chain: segment[%d].ResourceId(%d) != expected ResourceId(%d)",
				i, segments[i].ResourceId, expectedResourceIDs[i])
		}
		if connections[i].EndpointId != expectedEndpointIDs[i] {
			t.Errorf("chain: conn[%d].EndpointId(%d) != expected EndpointId(%d)",
				i, connections[i].EndpointId, expectedEndpointIDs[i])
		}
	}

	// Verify final task status
	if task.Status != model.TaskStatusFinished {
		t.Errorf("final task.Status = %d, want %d (Finished)", task.Status, model.TaskStatusFinished)
	}
	// All resources completed
	totalCompleted := 0
	for _, r := range resources {
		if r.Status == 2 {
			totalCompleted++
		}
	}
	if totalCompleted != 3 {
		t.Errorf("total completed resources = %d, want 3", totalCompleted)
	}
}

func normalizeVideoContent(t *testing.T, content *model.Content) {
	t.Helper()
	content.Timestamps = model.Timestamps{}
}
