package wxmp_test

import (
	"encoding/json"
	"testing"

	wxmp "wx_channel/internal/adapter/wxmp"
	"wx_channel/internal/database/model"
	"wx_channel/internal/download/registry"
	"wx_channel/internal/download/types"
	"wx_channel/pkg/testui"
	example "wx_channel/scraper_examples"
)

var (
	expectedAlbumContentID   = "wxmp:MzMwNDA3NDg2MQ=="
	expectedAlbumPublishTime = int64(1785075816)
)

func albumFixture(t *testing.T) ([]byte, wxmp.ArticleCgiDataNew) {
	t.Helper()

	raw, err := example.Load("mp/260728/album.json")
	if err != nil {
		t.Fatalf("read album fixture: %v", err)
	}

	var data wxmp.ArticleCgiDataNew
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("Unmarshal album fixture: %v", err)
	}
	if data.ItemShowType != 8 || len(data.PicturePageInfoList) != 6 {
		t.Fatalf("invalid album fixture: item_show_type=%d, pictures=%d", data.ItemShowType, len(data.PicturePageInfoList))
	}
	return raw, data
}

func TestToAccount_FromAlbumFixture(t *testing.T) {
	_, data := albumFixture(t)

	account, err := wxmp.ToAccount(&data)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}
	account.Timestamps = model.Timestamps{}

	expected := model.Account{
		Id:         "wxmp:phacct_df0605d8b2ba",
		PlatformId: "wxmp",
		ExternalId: "phacct_df0605d8b2ba",
		Nickname:   "神秘的小伊",
		Signature:  "",
		AvatarURL:  "http://mmbiz.qpic.cn/mmbiz_png/7y9QHQ3Lll6eD06cJnibg2DEN8tvBn0aDhiaiczDd8EKVFsckcqLVJD2zzZ15WicLtJRUxxkQn6OKvIq9jZfqN4q4WwbuXGUBS5y7lLK9R0CtGU/0?wx_fmt=png",
	}
	testui.AssertStrictEqual(t, "Account", expected, account)
}

func TestToContent_FromAlbumFixture(t *testing.T) {
	_, data := albumFixture(t)

	content, ext, err := wxmp.ToContent(&data)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	content.Timestamps = model.Timestamps{}

	expectedContent := model.Content{
		Id:          "wxmp:MzMwNDA3NDg2MQ==",
		PlatformId:  "wxmp",
		Type:        "article",
		ExternalId:  "MzMwNDA3NDg2MQ==",
		ExternalId2: "2247483706",
		Title:       "比女孩成熟，比女人天真🖤",
		Description: "比女孩成熟，比女人天真🖤",
		URL:         "https://mp.weixin.qq.com/s/4Oa2ncK1WL4atm_TfHB6xQ",
		CoverURL:    "http://mmbiz.qpic.cn/sz_mmbiz_jpg/7y9QHQ3Lll5s3HelADWsqMMaUDAv9DL0KYuyaz3iaK0XUAhHtQYialpAEDJhWicgib4WRejNP3s2ZgIwrRAvdBbCOtadXtaVeuCQckMUxT5xdaA/0?wx_fmt=jpeg",
		PublishTime: &expectedAlbumPublishTime,
	}
	albumExt := ext.(*wxmp.ContentAlbumExt)
	assertAlbumImages(t, albumExt.Images)
	expectedAlbum := &model.ContentAlbum{
		Id:          "wxmp:MzMwNDA3NDg2MQ==",
		ImageCount:  6,
		CoverWidth:  1924,
		CoverHeight: 3526,
		Format:      "jpeg",
		Description: "比女孩成熟，比女人天真🖤",
	}

	testui.AssertStrictEqual(t, "Content", expectedContent, content)
	testui.AssertStrictEqual(t, "ContentAlbum", expectedAlbum, albumExt.Album)
}

func TestBuildDownloadTask_FromAlbumFixture(t *testing.T) {
	raw, _ := albumFixture(t)

	h := registry.Get(wxmp.PlatformID)
	if h == nil {
		t.Fatal("handler not registered for platform wxmp")
	}
	info, err := h.BuildDownloadTask(json.RawMessage(raw), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("BuildDownloadTask: %v", err)
	}

	testui.AssertStrictEqual(t, "DownloadTaskV1", &model.DownloadTaskV1{
		ContentId:    &expectedAlbumContentID,
		Name:         "比女孩成熟，比女人天真🖤",
		UniqueID:     "MzMwNDA3NDg2MQ==",
		PlatformId:   "wxmp",
		Status:       model.TaskStatusWaiting,
		SourceURL:    "https://mp.weixin.qq.com/s/4Oa2ncK1WL4atm_TfHB6xQ",
		CoverURL:     "http://mmbiz.qpic.cn/sz_mmbiz_jpg/7y9QHQ3Lll5s3HelADWsqMMaUDAv9DL0KYuyaz3iaK0XUAhHtQYialpAEDJhWicgib4WRejNP3s2ZgIwrRAvdBbCOtadXtaVeuCQckMUxT5xdaA/0?wx_fmt=jpeg",
		ConfigJSON:   "{}",
		MetadataJSON: `{"author":"神秘的小伊","biz_type":2,"created_at":1785075816,"external_id":"MzMwNDA3NDg2MQ==","platform":"wxmp"}`,
	}, info.Task)
	assertAlbumDownloadTaskContent(t, info)
}

func TestBuildDownloadTask_FromAlbumFixture_WithSuffix(t *testing.T) {
	raw, _ := albumFixture(t)

	h := registry.Get(wxmp.PlatformID)
	if h == nil {
		t.Fatal("handler not registered for platform wxmp")
	}
	configJSON, err := json.Marshal(wxmp.DownloadConfig{Suffix: ".html"})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	info, err := h.BuildDownloadTask(json.RawMessage(raw), configJSON)
	if err != nil {
		t.Fatalf("BuildDownloadTask: %v", err)
	}

	testui.AssertStrictEqual(t, "DownloadTaskV1", &model.DownloadTaskV1{
		ContentId:    &expectedAlbumContentID,
		Name:         "比女孩成熟，比女人天真🖤",
		UniqueID:     "MzMwNDA3NDg2MQ==_html",
		PlatformId:   "wxmp",
		Status:       model.TaskStatusWaiting,
		SourceURL:    "https://mp.weixin.qq.com/s/4Oa2ncK1WL4atm_TfHB6xQ",
		CoverURL:     "http://mmbiz.qpic.cn/sz_mmbiz_jpg/7y9QHQ3Lll5s3HelADWsqMMaUDAv9DL0KYuyaz3iaK0XUAhHtQYialpAEDJhWicgib4WRejNP3s2ZgIwrRAvdBbCOtadXtaVeuCQckMUxT5xdaA/0?wx_fmt=jpeg",
		ConfigJSON:   `{"suffix":".html"}`,
		MetadataJSON: `{"author":"神秘的小伊","biz_type":2,"created_at":1785075816,"external_id":"MzMwNDA3NDg2MQ==","platform":"wxmp"}`,
	}, info.Task)
	assertAlbumDownloadTaskContent(t, info)
}

func assertAlbumDownloadTaskContent(t *testing.T, info *types.DownloadTaskResult) {
	t.Helper()

	const (
		extraJSON        = `{"author":"神秘的小伊","created_at":"1785075816","id":"MzMwNDA3NDg2MQ==","title":"比女孩成熟，比女人天真🖤"}`
		wechatHeaderJSON = `{"Referer":"https://mp.weixin.qq.com/","User-Agent":"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.50(0x1800322f) NetType/WIFI Language/zh_CN"}`
	)

	resources := []*types.ResourceInfo{
		{
			DownloadResource: model.DownloadResource{
				Name:       "比女孩成熟，比女人天真🖤.html",
				Kind:       "html",
				UniqueID:   "MzMwNDA3NDg2MQ==_html",
				MergeOrder: 0,
				Extra:      extraJSON,
			},
		},
		{
			DownloadResource: model.DownloadResource{
				Name:       "9d800e5446526ee9cf47a0ce2464bba1",
				Kind:       "image",
				UniqueID:   "MzMwNDA3NDg2MQ==_album_0",
				MergeOrder: 100,
				Extra:      extraJSON,
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      "https://mmbiz.qpic.cn/sz_mmbiz_jpg/7y9QHQ3Lll7vjYAdgoiazd4dUc12hHEKialHEApP76bxtVAicBGhnsUsBgtsPxF1xkIIG73PTWQxLa1QpRL4aLAX0hWuvxtYcfJOOmgtJGYfNA/0?wx_fmt=jpeg",
				Enabled:  1,
				Headers:  wechatHeaderJSON,
			}},
		},
		{
			DownloadResource: model.DownloadResource{
				Name:       "f7dd3d40dc46b343aeb1e9b94938715c",
				Kind:       "image",
				UniqueID:   "MzMwNDA3NDg2MQ==_album_1",
				MergeOrder: 101,
				Extra:      extraJSON,
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      "https://mmbiz.qpic.cn/mmbiz_jpg/7y9QHQ3Lll6LT85ASMmHSG3bGX2d22XBXTT0f8tt1HqkdRIhnmWayfuzTlUm0kx7CduaC1bECwQMpQ0djkP92PPiapZO8J39Wp8st1y5alS0/0?wx_fmt=jpeg",
				Enabled:  1,
				Headers:  wechatHeaderJSON,
			}},
		},
		{
			DownloadResource: model.DownloadResource{
				Name:       "c756b8f70bc8a94e6e66af5b3e2b9191",
				Kind:       "image",
				UniqueID:   "MzMwNDA3NDg2MQ==_album_2",
				MergeOrder: 102,
				Extra:      extraJSON,
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      "https://mmbiz.qpic.cn/sz_mmbiz_jpg/7y9QHQ3Lll556wAn8Kxy1RyZ15wd9sOKI4Zr1Zic9iaLkWfgZM04gAfIpRq6A5bRnT5ptFPHqJLUAZzwkqSz1MPBZUJTJz4mAJCjIavwywU9A/0?wx_fmt=jpeg",
				Enabled:  1,
				Headers:  wechatHeaderJSON,
			}},
		},
		{
			DownloadResource: model.DownloadResource{
				Name:       "e2596d6b258bad47aee865f1f57b56b9",
				Kind:       "image",
				UniqueID:   "MzMwNDA3NDg2MQ==_album_3",
				MergeOrder: 103,
				Extra:      extraJSON,
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      "https://mmbiz.qpic.cn/mmbiz_jpg/7y9QHQ3Lll4lmI6qSGhsPlrEWqpeNfhq4Yj2kvEMPkdrmoIozwK1O4OlAX7lxGictpZjTxdjAY4BSeAtGvcbkLGqHy2gydnBfNB0ZvSE32icU/0?wx_fmt=jpeg",
				Enabled:  1,
				Headers:  wechatHeaderJSON,
			}},
		},
		{
			DownloadResource: model.DownloadResource{
				Name:       "cff971181d314b6d46d9c84a1a92f246",
				Kind:       "image",
				UniqueID:   "MzMwNDA3NDg2MQ==_album_4",
				MergeOrder: 104,
				Extra:      extraJSON,
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      "https://mmbiz.qpic.cn/mmbiz_jpg/7y9QHQ3Lll6h07TgibyajbYgDBbmEcdbAAGQkia5W5XA5NvsrvWf5FxoQuBuCOmgh7FicFhIwaPgWVVzpejohiaslj3uXp46fPxgLicGWVWhibqsk/0?wx_fmt=jpeg",
				Enabled:  1,
				Headers:  wechatHeaderJSON,
			}},
		},
		{
			DownloadResource: model.DownloadResource{
				Name:       "c08f47c9e0a8a824fa6f011f9d06b95c",
				Kind:       "image",
				UniqueID:   "MzMwNDA3NDg2MQ==_album_5",
				MergeOrder: 105,
				Extra:      extraJSON,
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      "https://mmbiz.qpic.cn/mmbiz_jpg/7y9QHQ3Lll6gkXeL2vVzibzGZSYCrv1VC9InPYeKY2ibPn2hlwWFTvw0ibPico1sNOpZFeYI00UWpe8lZZetiaPher069fDuqeWs7BFIg5vBF9ms/0?wx_fmt=jpeg",
				Enabled:  1,
				Headers:  wechatHeaderJSON,
			}},
		},
		{
			DownloadResource: model.DownloadResource{
				Name:       "比女孩成熟，比女人天真🖤",
				Kind:       "cover",
				UniqueID:   "MzMwNDA3NDg2MQ==_cover",
				MergeOrder: 106,
				Extra:      extraJSON,
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      "http://mmbiz.qpic.cn/sz_mmbiz_jpg/7y9QHQ3Lll5s3HelADWsqMMaUDAv9DL0KYuyaz3iaK0XUAhHtQYialpAEDJhWicgib4WRejNP3s2ZgIwrRAvdBbCOtadXtaVeuCQckMUxT5xdaA/0?wx_fmt=jpeg",
				Enabled:  1,
				Headers:  wechatHeaderJSON,
			}},
		},
	}

	testui.AssertStrictEqual(t, "DownloadResources", resources, info.Resources)

	// ContentDetail is now *ContentAlbum (no Images field)
	actualAlbum := info.ContentDetail.(*model.ContentAlbum)
	testui.AssertStrictEqual(t, "ContentAlbum", &model.ContentAlbum{
		Id:          "wxmp:MzMwNDA3NDg2MQ==",
		ImageCount:  6,
		CoverWidth:  1924,
		CoverHeight: 3526,
		Format:      "jpeg",
		Description: "比女孩成熟，比女人天真🖤",
	}, actualAlbum)

	// AlbumImages is a separate slice
	assertAlbumImages(t, info.AlbumImages)

	account := *info.Account
	account.Timestamps = model.Timestamps{}
	testui.AssertStrictEqual(t, "Account", model.Account{
		Id:         "wxmp:phacct_df0605d8b2ba",
		PlatformId: "wxmp",
		ExternalId: "phacct_df0605d8b2ba",
		Nickname:   "神秘的小伊",
		Signature:  "",
		AvatarURL:  "http://mmbiz.qpic.cn/mmbiz_png/7y9QHQ3Lll6eD06cJnibg2DEN8tvBn0aDhiaiczDd8EKVFsckcqLVJD2zzZ15WicLtJRUxxkQn6OKvIq9jZfqN4q4WwbuXGUBS5y7lLK9R0CtGU/0?wx_fmt=png",
	}, account)

	content := *info.Content
	content.Timestamps = model.Timestamps{}
	testui.AssertStrictEqual(t, "Content", model.Content{
		Id:          "wxmp:MzMwNDA3NDg2MQ==",
		PlatformId:  "wxmp",
		Type:        "article",
		ExternalId:  "MzMwNDA3NDg2MQ==",
		ExternalId2: "2247483706",
		Title:       "比女孩成熟，比女人天真🖤",
		Description: "比女孩成熟，比女人天真🖤",
		URL:         "https://mp.weixin.qq.com/s/4Oa2ncK1WL4atm_TfHB6xQ",
		CoverURL:    "http://mmbiz.qpic.cn/sz_mmbiz_jpg/7y9QHQ3Lll5s3HelADWsqMMaUDAv9DL0KYuyaz3iaK0XUAhHtQYialpAEDJhWicgib4WRejNP3s2ZgIwrRAvdBbCOtadXtaVeuCQckMUxT5xdaA/0?wx_fmt=jpeg",
		PublishTime: &expectedAlbumPublishTime,
	}, content)
}

func assertAlbumImages(t *testing.T, images []*model.ContentImage) {
	t.Helper()

	// Strip IDs, AlbumId, SortOrder for comparison since they vary by test context
	actual := make([]model.ContentImage, len(images))
	for i, img := range images {
		actual[i] = model.ContentImage{
			URL:    img.URL,
			Width:  img.Width,
			Height: img.Height,
		}
	}
	testui.AssertStrictEqual(t, "ContentAlbum.Images", []model.ContentImage{
		{
			URL:    "https://mmbiz.qpic.cn/sz_mmbiz_jpg/7y9QHQ3Lll7vjYAdgoiazd4dUc12hHEKialHEApP76bxtVAicBGhnsUsBgtsPxF1xkIIG73PTWQxLa1QpRL4aLAX0hWuvxtYcfJOOmgtJGYfNA/0?wx_fmt=jpeg",
			Width:  1924,
			Height: 3526,
		},
		{
			URL:    "https://mmbiz.qpic.cn/mmbiz_jpg/7y9QHQ3Lll6LT85ASMmHSG3bGX2d22XBXTT0f8tt1HqkdRIhnmWayfuzTlUm0kx7CduaC1bECwQMpQ0djkP92PPiapZO8J39Wp8st1y5alS0/0?wx_fmt=jpeg",
			Width:  1920,
			Height: 3397,
		},
		{
			URL:    "https://mmbiz.qpic.cn/sz_mmbiz_jpg/7y9QHQ3Lll556wAn8Kxy1RyZ15wd9sOKI4Zr1Zic9iaLkWfgZM04gAfIpRq6A5bRnT5ptFPHqJLUAZzwkqSz1MPBZUJTJz4mAJCjIavwywU9A/0?wx_fmt=jpeg",
			Width:  1920,
			Height: 3474,
		},
		{
			URL:    "https://mmbiz.qpic.cn/mmbiz_jpg/7y9QHQ3Lll4lmI6qSGhsPlrEWqpeNfhq4Yj2kvEMPkdrmoIozwK1O4OlAX7lxGictpZjTxdjAY4BSeAtGvcbkLGqHy2gydnBfNB0ZvSE32icU/0?wx_fmt=jpeg",
			Width:  1920,
			Height: 3250,
		},
		{
			URL:    "https://mmbiz.qpic.cn/mmbiz_jpg/7y9QHQ3Lll6h07TgibyajbYgDBbmEcdbAAGQkia5W5XA5NvsrvWf5FxoQuBuCOmgh7FicFhIwaPgWVVzpejohiaslj3uXp46fPxgLicGWVWhibqsk/0?wx_fmt=jpeg",
			Width:  1920,
			Height: 2560,
		},
		{
			URL:    "https://mmbiz.qpic.cn/mmbiz_jpg/7y9QHQ3Lll6gkXeL2vVzibzGZSYCrv1VC9InPYeKY2ibPn2hlwWFTvw0ibPico1sNOpZFeYI00UWpe8lZZetiaPher069fDuqeWs7BFIg5vBF9ms/0?wx_fmt=jpeg",
			Width:  1183,
			Height: 1920,
		},
	}, actual)
}
