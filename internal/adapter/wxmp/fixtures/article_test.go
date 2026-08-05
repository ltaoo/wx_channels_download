package wxmp_test

import (
	"encoding/json"
	"testing"

	wxmp "wx_channel/internal/adapter/wxmp"
	"wx_channel/internal/database/model"
	"wx_channel/internal/adapter"
	"wx_channel/pkg/testui"
	example "wx_channel/scraper_examples"
)

var (
	expectedArticleContentID   = "wxmp:MzkyNjQ2NjI2NA=="
	expectedArticlePublishTime = int64(1785123529)
)

func articleFixture(t *testing.T) ([]byte, wxmp.ArticleCgiDataNew) {
	t.Helper()

	raw, err := example.Load("mp/260728/article.json")
	if err != nil {
		t.Fatalf("read article fixture: %v", err)
	}

	var data wxmp.ArticleCgiDataNew
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("Unmarshal article fixture: %v", err)
	}
	return raw, data
}

func TestToAccount_FromArticleFixture(t *testing.T) {
	_, data := articleFixture(t)

	account, err := wxmp.ToAccount(&data)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}
	account.Timestamps = model.Timestamps{}

	expected := model.Account{
		Id:         "wxmp:gh_10597bfef9cf",
		PlatformId: "wxmp",
		ExternalId: "gh_10597bfef9cf",
		Nickname:   "猫猫的理想国度",
		Alias:      "catyaoyao18",
		Signature:  "猫小贱的独家记忆",
		AvatarURL:  "http://mmbiz.qpic.cn/mmbiz_png/wfz3GtheTU0vN63OTFCU1NIu9TFlsnzc44XW31w5CWEoRMX0UHApPrPNaI8cXIX62U16JY9iagqN7Y2Ldc1EkpOQrhUBXhOAsJZufOZrlic2A/0?wx_fmt=png",
	}

	testui.AssertStrictEqual(t, "Account", expected, account)
}

func TestToContent_FromArticleFixture(t *testing.T) {
	_, data := articleFixture(t)

	content, contentArticle, err := wxmp.ToContent(&data)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	normalizeArticleOutput(t, content, contentArticle)

	expected := model.Content{
		Id:          "wxmp:MzkyNjQ2NjI2NA==",
		PlatformId:  "wxmp",
		Type:        "article",
		ExternalId:  "MzkyNjQ2NjI2NA==",
		ExternalId2: "2247484478",
		Title:       "fingerprintjs/fingerprintjs，我用了两周，说说真实感受",
		Description: "两周实测FingerprintJS，分享浏览器指纹识别工具的真实效果、优缺点和适用场景\n\n这个工具我劝你别",
		URL:         "https://mp.weixin.qq.com/s/z17N2Twe7pnt7UW5hJGHiQ",
		CoverURL:    "https://mmbiz.qpic.cn/mmbiz_jpg/wfz3GtheTU1beBeFmQGgoEPus7qt4zRcE0mt6ibWSOZzxKQeicR8HiapGWUBkrRCbCNATtKH6oS9t0ene3UH1gkzcBicEmVXQ3NHYprLDD970xw/0?wx_fmt=jpeg",
		PublishTime: &expectedArticlePublishTime,
	}

	testui.AssertStrictEqual(t, "Content", expected, content)
	testui.AssertStrictEqual(t, "ContentArticle", &model.ContentArticle{
		Id:         "wxmp:MzkyNjQ2NjI2NA==",
		Type:       model.ContentArticleTypeHTML,
		WordCount:  1363,
		IsOriginal: 1,
	}, contentArticle)
}

func TestBuildDownloadTask_FromContent(t *testing.T) {
	raw, data := articleFixture(t)
	contentJSON := json.RawMessage(raw)

	h := adapter.Get(wxmp.PlatformID)
	if h == nil {
		t.Fatal("handler not registered for platform wxmp")
	}

	config := map[string]any{}
	configJSON, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	info, err := h.BuildDownloadTask(contentJSON, json.RawMessage(configJSON))
	if err != nil {
		t.Fatalf("BuildDownloadTask: %v", err)
	}
	content := info.Content
	account := info.Account
	account.Timestamps = model.Timestamps{}
	normalizeArticleOutput(t, content, info.ContentDetail)

	// --- verify account ---
	expectedAccount := model.Account{
		Id:         "wxmp:gh_10597bfef9cf",
		PlatformId: "wxmp",
		ExternalId: "gh_10597bfef9cf",
		Nickname:   "猫猫的理想国度",
		Alias:      "catyaoyao18",
		Signature:  "猫小贱的独家记忆",
		AvatarURL:  "http://mmbiz.qpic.cn/mmbiz_png/wfz3GtheTU0vN63OTFCU1NIu9TFlsnzc44XW31w5CWEoRMX0UHApPrPNaI8cXIX62U16JY9iagqN7Y2Ldc1EkpOQrhUBXhOAsJZufOZrlic2A/0?wx_fmt=png",
	}
	testui.AssertStrictEqual(t, "Account", expectedAccount, account)

	// --- verify content ---
	expectedContent := model.Content{
		Id:          "wxmp:MzkyNjQ2NjI2NA==",
		PlatformId:  "wxmp",
		Type:        "article",
		ExternalId:  "MzkyNjQ2NjI2NA==",
		ExternalId2: "2247484478",
		Title:       "fingerprintjs/fingerprintjs，我用了两周，说说真实感受",
		Description: "两周实测FingerprintJS，分享浏览器指纹识别工具的真实效果、优缺点和适用场景\n\n这个工具我劝你别",
		URL:         "https://mp.weixin.qq.com/s/z17N2Twe7pnt7UW5hJGHiQ",
		CoverURL:    "https://mmbiz.qpic.cn/mmbiz_jpg/wfz3GtheTU1beBeFmQGgoEPus7qt4zRcE0mt6ibWSOZzxKQeicR8HiapGWUBkrRCbCNATtKH6oS9t0ene3UH1gkzcBicEmVXQ3NHYprLDD970xw/0?wx_fmt=jpeg",
		PublishTime: &expectedArticlePublishTime,
	}
	testui.AssertStrictEqual(t, "Content", expectedContent, content)

	// --- verify DownloadInfo ---
	const imageURL = `https://mmbiz.qpic.cn/mmbiz_jpg/wfz3GtheTU3Leslhe27eP6af4PIZOvdics3IwBCtKPy3pviczVWed3nribsUlxCISLqUw0rlKBrunWBhuI1eEnLJzruKmyiaUaoIfGiaknWBLrmc/640?wx_fmt=jpeg`
	const imageURL2 = `https://mmbiz.qpic.cn/mmbiz_jpg/wfz3GtheTU3msa1awM8pAkrhBXylo6tVbl4nJhbiaic8h3UjtBPQn1cpX0Jfib9atibSib8bqwAYAbicxbn6LG6OY69azNOjsqCoicNLproWWLSq5Y/640?wx_fmt=jpeg`
	const coverURL = `https://mmbiz.qpic.cn/mmbiz_jpg/wfz3GtheTU1beBeFmQGgoEPus7qt4zRcE0mt6ibWSOZzxKQeicR8HiapGWUBkrRCbCNATtKH6oS9t0ene3UH1gkzcBicEmVXQ3NHYprLDD970xw/0?wx_fmt=jpeg`
	const wechatHeaderJSON = `{"Referer":"https://mp.weixin.qq.com/","User-Agent":"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.50(0x1800322f) NetType/WIFI Language/zh_CN"}`
	const extraJSON = `{"author":"猫猫的理想国度","created_at":"1785123529","id":"MzkyNjQ2NjI2NA==","title":"fingerprintjs/fingerprintjs，我用了两周，说说真实感受"}`
	const metadataJSON = `{"author":"猫猫的理想国度","biz_type":1,"created_at":1785123529,"external_id":"MzkyNjQ2NjI2NA==","platform":"wxmp"}`
	const coverName = "fingerprintjs/fingerprintjs，我用了两周，说说真实感受"

	expectedInfo := adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &expectedArticleContentID,
			Name:         "fingerprintjs/fingerprintjs，我用了两周，说说真实感受",
			UniqueID:     "MzkyNjQ2NjI2NA==_html",
			PlatformId:   "wxmp",
			Status:       model.TaskStatusWaiting,
			SourceURL:    "https://mp.weixin.qq.com/s/z17N2Twe7pnt7UW5hJGHiQ",
			CoverURL:     coverURL,
			ConfigJSON:   "{}",
			MetadataJSON: metadataJSON,
		},
		Resources: []*adapter.ResourceInfo{
			{
				DownloadResource: model.DownloadResource{
					Name:       "fingerprintjs/fingerprintjs，我用了两周，说说真实感受",
					Kind:       "html",
					UniqueID:   "MzkyNjQ2NjI2NA==_html",
					MergeOrder: 0,
					Extra:      extraJSON,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: "inline",
					URL:      data.ContentNoencode,
					Enabled:  1,
				}},
			},
			{
				DownloadResource: model.DownloadResource{
					Name:       "ddab01c616c40d6512a2ac0fec54b2fc",
					Kind:       "image",
					UniqueID:   "MzkyNjQ2NjI2NA==_img_0",
					MergeOrder: 100,
					Extra:      extraJSON,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: "https",
					URL:      imageURL,
					Enabled:  1,
					Headers:  wechatHeaderJSON,
				}},
			},
			{
				DownloadResource: model.DownloadResource{
					Name:       "eda58626e16f6fc49b1f37cd9da1bae0",
					Kind:       "image",
					UniqueID:   "MzkyNjQ2NjI2NA==_img_1",
					MergeOrder: 101,
					Extra:      extraJSON,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: "https",
					URL:      imageURL2,
					Enabled:  1,
					Headers:  wechatHeaderJSON,
				}},
			},
			{
				DownloadResource: model.DownloadResource{
					Name:       coverName,
					Kind:       "image",
					UniqueID:   "MzkyNjQ2NjI2NA==_cover",
					MergeOrder: 102,
					Extra:      extraJSON,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: "https",
					URL:      coverURL,
					Enabled:  1,
					Headers:  wechatHeaderJSON,
				}},
			},
		},
		ContentDetail: &model.ContentArticle{
			Id:         "wxmp:MzkyNjQ2NjI2NA==",
			Type:       model.ContentArticleTypeHTML,
			WordCount:  1363,
			IsOriginal: 1,
		},
		Account: &model.Account{
			Id:         "wxmp:gh_10597bfef9cf",
			PlatformId: "wxmp",
			ExternalId: "gh_10597bfef9cf",
			Nickname:   "猫猫的理想国度",
			Alias:      "catyaoyao18",
			Signature:  "猫小贱的独家记忆",
			AvatarURL:  "http://mmbiz.qpic.cn/mmbiz_png/wfz3GtheTU0vN63OTFCU1NIu9TFlsnzc44XW31w5CWEoRMX0UHApPrPNaI8cXIX62U16JY9iagqN7Y2Ldc1EkpOQrhUBXhOAsJZufOZrlic2A/0?wx_fmt=png",
		},
		Content: &model.Content{
			Id:          "wxmp:MzkyNjQ2NjI2NA==",
			PlatformId:  "wxmp",
			Type:        "article",
			ExternalId:  "MzkyNjQ2NjI2NA==",
			ExternalId2: "2247484478",
			Title:       "fingerprintjs/fingerprintjs，我用了两周，说说真实感受",
			Description: "两周实测FingerprintJS，分享浏览器指纹识别工具的真实效果、优缺点和适用场景\n\n这个工具我劝你别",
			URL:         "https://mp.weixin.qq.com/s/z17N2Twe7pnt7UW5hJGHiQ",
			CoverURL:    "https://mmbiz.qpic.cn/mmbiz_jpg/wfz3GtheTU1beBeFmQGgoEPus7qt4zRcE0mt6ibWSOZzxKQeicR8HiapGWUBkrRCbCNATtKH6oS9t0ene3UH1gkzcBicEmVXQ3NHYprLDD970xw/0?wx_fmt=jpeg",
			PublishTime: &expectedArticlePublishTime,
		},
	}
	for _, resource := range expectedInfo.Resources {
		resource.ContentId = &expectedArticleContentID
	}
	testui.AssertStrictEqual(t, "DownloadTaskResult", expectedInfo, info)
}

func TestBuildDownloadTask_WithCustomConfig(t *testing.T) {
	raw, data := articleFixture(t)
	contentJSON := json.RawMessage(raw)

	h := adapter.Get(wxmp.PlatformID)
	if h == nil {
		t.Fatal("handler not registered for platform wxmp")
	}

	config := map[string]any{
		"filename":  "自定义文件名",
		"suffix":    ".html",
		"overwrite": true,
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	info, err := h.BuildDownloadTask(contentJSON, json.RawMessage(configJSON))
	if err != nil {
		t.Fatalf("BuildDownloadTask: %v", err)
	}
	info.Account.Timestamps = model.Timestamps{}
	normalizeArticleOutput(t, info.Content, info.ContentDetail)

	const extraJSON = `{"author":"猫猫的理想国度","created_at":"1785123529","id":"MzkyNjQ2NjI2NA==","title":"自定义文件名"}`
	const wechatHeaderJSON = `{"Referer":"https://mp.weixin.qq.com/","User-Agent":"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.50(0x1800322f) NetType/WIFI Language/zh_CN"}`
	const imageURL = `https://mmbiz.qpic.cn/mmbiz_jpg/wfz3GtheTU3Leslhe27eP6af4PIZOvdics3IwBCtKPy3pviczVWed3nribsUlxCISLqUw0rlKBrunWBhuI1eEnLJzruKmyiaUaoIfGiaknWBLrmc/640?wx_fmt=jpeg`
	const imageURL2 = `https://mmbiz.qpic.cn/mmbiz_jpg/wfz3GtheTU3msa1awM8pAkrhBXylo6tVbl4nJhbiaic8h3UjtBPQn1cpX0Jfib9atibSib8bqwAYAbicxbn6LG6OY69azNOjsqCoicNLproWWLSq5Y/640?wx_fmt=jpeg`
	const coverURL = `https://mmbiz.qpic.cn/mmbiz_jpg/wfz3GtheTU1beBeFmQGgoEPus7qt4zRcE0mt6ibWSOZzxKQeicR8HiapGWUBkrRCbCNATtKH6oS9t0ene3UH1gkzcBicEmVXQ3NHYprLDD970xw/0?wx_fmt=jpeg`

	expectedInfo := adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &expectedArticleContentID,
			Name:         "自定义文件名",
			UniqueID:     "MzkyNjQ2NjI2NA==_html",
			PlatformId:   "wxmp",
			Status:       model.TaskStatusWaiting,
			SourceURL:    "https://mp.weixin.qq.com/s/z17N2Twe7pnt7UW5hJGHiQ",
			CoverURL:     coverURL,
			ConfigJSON:   `{"filename":"自定义文件名","overwrite":true,"suffix":".html"}`,
			MetadataJSON: `{"author":"猫猫的理想国度","biz_type":1,"created_at":1785123529,"external_id":"MzkyNjQ2NjI2NA==","platform":"wxmp"}`,
		},
		Resources: []*adapter.ResourceInfo{
			{
				// 1. content_noencode saved as html file
				DownloadResource: model.DownloadResource{
					Name:       "自定义文件名.html",
					Kind:       "html",
					UniqueID:   "MzkyNjQ2NjI2NA==_html",
					MergeOrder: 0,
					Extra:      extraJSON,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: "inline",
					URL:      data.ContentNoencode,
					Enabled:  1,
				}},
			},
			{
				// 2. images in content_noencode
				DownloadResource: model.DownloadResource{
					Name:       "ddab01c616c40d6512a2ac0fec54b2fc",
					Kind:       "image",
					UniqueID:   "MzkyNjQ2NjI2NA==_img_0",
					MergeOrder: 100,
					Extra:      extraJSON,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: "https",
					URL:      imageURL,
					Enabled:  1,
					Headers:  wechatHeaderJSON,
				}},
			},
			{
				// 3. second image in content_noencode
				DownloadResource: model.DownloadResource{
					Name:       "eda58626e16f6fc49b1f37cd9da1bae0",
					Kind:       "image",
					UniqueID:   "MzkyNjQ2NjI2NA==_img_1",
					MergeOrder: 101,
					Extra:      extraJSON,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: "https",
					URL:      imageURL2,
					Enabled:  1,
					Headers:  wechatHeaderJSON,
				}},
			},
			{
				// 4. cover image
				DownloadResource: model.DownloadResource{
					Name:       "自定义文件名",
					Kind:       "image",
					UniqueID:   "MzkyNjQ2NjI2NA==_cover",
					MergeOrder: 102,
					Extra:      extraJSON,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: "https",
					URL:      coverURL,
					Enabled:  1,
					Headers:  wechatHeaderJSON,
				}},
			},
		},
		ContentDetail: &model.ContentArticle{
			Id:         "wxmp:MzkyNjQ2NjI2NA==",
			Type:       model.ContentArticleTypeHTML,
			WordCount:  1363,
			IsOriginal: 1,
		},
		Account: &model.Account{
			Id:         "wxmp:gh_10597bfef9cf",
			PlatformId: "wxmp",
			ExternalId: "gh_10597bfef9cf",
			Nickname:   "猫猫的理想国度",
			Alias:      "catyaoyao18",
			Signature:  "猫小贱的独家记忆",
			AvatarURL:  "http://mmbiz.qpic.cn/mmbiz_png/wfz3GtheTU0vN63OTFCU1NIu9TFlsnzc44XW31w5CWEoRMX0UHApPrPNaI8cXIX62U16JY9iagqN7Y2Ldc1EkpOQrhUBXhOAsJZufOZrlic2A/0?wx_fmt=png",
		},
		Content: &model.Content{
			Id:          "wxmp:MzkyNjQ2NjI2NA==",
			PlatformId:  "wxmp",
			Type:        "article",
			ExternalId:  "MzkyNjQ2NjI2NA==",
			ExternalId2: "2247484478",
			Title:       "fingerprintjs/fingerprintjs，我用了两周，说说真实感受",
			Description: "两周实测FingerprintJS，分享浏览器指纹识别工具的真实效果、优缺点和适用场景\n\n这个工具我劝你别",
			URL:         "https://mp.weixin.qq.com/s/z17N2Twe7pnt7UW5hJGHiQ",
			CoverURL:    "https://mmbiz.qpic.cn/mmbiz_jpg/wfz3GtheTU1beBeFmQGgoEPus7qt4zRcE0mt6ibWSOZzxKQeicR8HiapGWUBkrRCbCNATtKH6oS9t0ene3UH1gkzcBicEmVXQ3NHYprLDD970xw/0?wx_fmt=jpeg",
			PublishTime: &expectedArticlePublishTime,
		},
	}

	for _, resource := range expectedInfo.Resources {
		resource.ContentId = &expectedArticleContentID
	}
	testui.AssertStrictEqual(t, "DownloadTaskResult", expectedInfo, info)
}

func normalizeArticleOutput(t *testing.T, content *model.Content, detail any) {
	t.Helper()
	content.Timestamps = model.Timestamps{}
	article, ok := detail.(*model.ContentArticle)
	if !ok {
		t.Fatalf("unexpected content detail type: %T", detail)
	}
	if len(article.HTML) != 6705 {
		t.Fatalf("unexpected article HTML length: %d", len(article.HTML))
	}
	article.HTML = ""
}
