package zhihu

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/internal/interceptor"
)

func TestFormatProfileUsesZhihuRawFields(t *testing.T) {
	rawPayload := map[string]any{
		"zhihu_content_kind":          "answer",
		"zhihu_content_token":         "1972224920428742006",
		"zhihu_author_member_hash_id": "5c8e0c7af4ce477275c6f6aac45c5200",
	}
	raw, err := json.Marshal(rawPayload)
	if err != nil {
		t.Fatal(err)
	}

	profile := FormatProfile(&interceptor.PlatformBrowserProfile{
		PlatformId:        PlatformId,
		PlatformName:      PlatformName,
		ContentType:       "answer",
		ContentExternalId: "https://www.zhihu.com/question/638094850/answer/1972224920428742006",
		ContentURL:        "https://www.zhihu.com/question/638094850/answer/1972224920428742006",
		AccountNickname:   "不写瓜娃的码农",
		Raw:               raw,
	})

	if profile.ContentType != "answer" {
		t.Fatalf("ContentType = %q, want answer", profile.ContentType)
	}
	if profile.ContentExternalId != "zhihu:answer:1972224920428742006" {
		t.Fatalf("ContentExternalId = %q", profile.ContentExternalId)
	}
	if profile.AccountExternalId != "5c8e0c7af4ce477275c6f6aac45c5200" {
		t.Fatalf("AccountExternalId = %q", profile.AccountExternalId)
	}
}

func TestFetchPeopleProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/people/e40c4495e66937d3e085ff1b5f2d03a6" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("user-agent"); got != "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36" {
			t.Fatalf("user-agent = %q", got)
		}
		if got := r.Header.Get("accept"); got != "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7" {
			t.Fatalf("accept = %q", got)
		}
		if got := r.Header.Get("sec-fetch-mode"); got != "navigate" {
			t.Fatalf("sec-fetch-mode = %q", got)
		}
		if got := r.Header.Get("sec-ch-ua-platform"); got != `"macOS"` {
			t.Fatalf("sec-ch-ua-platform = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"e40c4495e66937d3e085ff1b5f2d03a6",
			"url_token":"li-hai-yang-14-43",
			"name":"工叔",
			"avatar_url":"https://pic1.zhimg.com/v2-avatar_xl.jpg",
			"follower_count":82751
		}`))
	}))
	defer server.Close()

	oldBase := zhihuPeopleAPIBase
	zhihuPeopleAPIBase = server.URL + "/people/"
	defer func() {
		zhihuPeopleAPIBase = oldBase
	}()

	profile, err := FetchPeopleProfile(server.Client(), "e40c4495e66937d3e085ff1b5f2d03a6")
	if err != nil {
		t.Fatal(err)
	}
	if profile.URLToken != "li-hai-yang-14-43" {
		t.Fatalf("URLToken = %q", profile.URLToken)
	}
	if profile.Name != "工叔" {
		t.Fatalf("Name = %q", profile.Name)
	}
	if profile.AvatarURL != "https://pic1.zhimg.com/v2-avatar_xl.jpg" {
		t.Fatalf("AvatarURL = %q", profile.AvatarURL)
	}
	if profile.FollowerCount != 82751 {
		t.Fatalf("FollowerCount = %d", profile.FollowerCount)
	}
}

func TestPeopleProfileUpdates(t *testing.T) {
	updates := PeopleProfileUpdates(&PeopleProfile{
		URLToken:      "li-hai-yang-14-43",
		Name:          "工叔",
		AvatarURL:     "https://pic1.zhimg.com/v2-avatar_xl.jpg",
		FollowerCount: 82751,
	})

	if updates["username"] != "li-hai-yang-14-43" {
		t.Fatalf("username = %v", updates["username"])
	}
	if updates["profile_url"] != "https://www.zhihu.com/people/li-hai-yang-14-43" {
		t.Fatalf("profile_url = %v", updates["profile_url"])
	}
	if updates["nickname"] != "工叔" {
		t.Fatalf("nickname = %v", updates["nickname"])
	}
	if updates["avatar_url"] != "https://pic1.zhimg.com/v2-avatar_xl.jpg" {
		t.Fatalf("avatar_url = %v", updates["avatar_url"])
	}
	if updates["follower_count"] != int64(82751) {
		t.Fatalf("follower_count = %v", updates["follower_count"])
	}
}

func TestBrowseHistoryPeopleProfileUpdates(t *testing.T) {
	updates := BrowseHistoryPeopleProfileUpdates(&PeopleProfile{
		URLToken:  "li-hai-yang-14-43",
		Name:      "工叔",
		AvatarURL: "https://pic1.zhimg.com/v2-avatar_xl.jpg",
	})

	if updates["account_username"] != "li-hai-yang-14-43" {
		t.Fatalf("account_username = %v", updates["account_username"])
	}
	if updates["account_nickname"] != "工叔" {
		t.Fatalf("account_nickname = %v", updates["account_nickname"])
	}
	if updates["account_avatar_url"] != "https://pic1.zhimg.com/v2-avatar_xl.jpg" {
		t.Fatalf("account_avatar_url = %v", updates["account_avatar_url"])
	}
}

func TestEnrichAccountUpdatesAccountAndBrowseHistory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Account{}, &model.BrowseHistory{}); err != nil {
		t.Fatal(err)
	}

	accountID := "e40c4495e66937d3e085ff1b5f2d03a6"
	if err := db.Create(&model.Account{
		PlatformId: PlatformId,
		ExternalId: accountID,
		Username:   accountID,
		Nickname:   "旧昵称",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.BrowseHistory{
		PlatformId:        PlatformId,
		ContentExternalId: "zhihu:answer:1",
		AccountExternalId: accountID,
		AccountUsername:   accountID,
		AccountNickname:   "旧昵称",
		AccountAvatarURL:  "https://old.example/avatar.jpg",
		VisitedTimes:      1,
		ContentType:       "answer",
		ContentTitle:      "title",
		ContentURL:        "https://www.zhihu.com/question/1/answer/1",
		ContentSourceURL:  "https://www.zhihu.com/question/1/answer/1",
		ContentCoverURL:   "",
		ExtraData:         "",
		ContentId:         nil,
		AccountId:         nil,
		InfluencerId:      nil,
		Timestamps:        model.Timestamps{CreatedAt: 1, UpdatedAt: 1},
	}).Error; err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/people/"+accountID {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"e40c4495e66937d3e085ff1b5f2d03a6",
			"url_token":"li-hai-yang-14-43",
			"name":"工叔",
			"avatar_url":"https://pic1.zhimg.com/v2-avatar_xl.jpg",
			"follower_count":82751
		}`))
	}))
	defer server.Close()

	oldBase := zhihuPeopleAPIBase
	zhihuPeopleAPIBase = server.URL + "/people/"
	defer func() {
		zhihuPeopleAPIBase = oldBase
	}()

	if err := EnrichAccount(db, server.Client(), accountID); err != nil {
		t.Fatal(err)
	}

	var account model.Account
	if err := db.Where("platform_id = ? AND external_id = ?", PlatformId, accountID).First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if account.Username != "li-hai-yang-14-43" {
		t.Fatalf("account.Username = %q", account.Username)
	}
	if account.Nickname != "工叔" {
		t.Fatalf("account.Nickname = %q", account.Nickname)
	}
	if account.AvatarURL != "https://pic1.zhimg.com/v2-avatar_xl.jpg" {
		t.Fatalf("account.AvatarURL = %q", account.AvatarURL)
	}
	if account.FollowerCount != 82751 {
		t.Fatalf("account.FollowerCount = %d", account.FollowerCount)
	}

	var browse model.BrowseHistory
	if err := db.Where("platform_id = ? AND account_external_id = ?", PlatformId, accountID).First(&browse).Error; err != nil {
		t.Fatal(err)
	}
	if browse.AccountUsername != "li-hai-yang-14-43" {
		t.Fatalf("browse.AccountUsername = %q", browse.AccountUsername)
	}
	if browse.AccountNickname != "工叔" {
		t.Fatalf("browse.AccountNickname = %q", browse.AccountNickname)
	}
	if browse.AccountAvatarURL != "https://pic1.zhimg.com/v2-avatar_xl.jpg" {
		t.Fatalf("browse.AccountAvatarURL = %q", browse.AccountAvatarURL)
	}
}

func TestFormatRecordsBuildsAccountAndBrowse(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"zhihu_content_kind":          "answer",
		"zhihu_content_token":         "1972224920428742006",
		"zhihu_author_member_hash_id": "5c8e0c7af4ce477275c6f6aac45c5200",
		"zhihu_date_created":          "2026-06-03T02:11:43.000Z",
		"zhihu_date_modified":         "2026-06-03T03:28:06.000Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	records := FormatRecords(&interceptor.PlatformBrowserProfile{
		PlatformId:       PlatformId,
		ContentType:      "answer",
		ContentTitle:     "用 node 写后端存在什么问题？",
		ContentURL:       "https://www.zhihu.com/question/638094850/answer/1972224920428742006",
		AccountNickname:  "不写瓜娃的码农",
		AccountAvatarURL: "https://picx.zhimg.com/avatar.jpg",
		Raw:              raw,
	}, 1234)

	if records == nil {
		t.Fatal("records is nil")
	}
	if records.ContentID != "zhihu:answer:1972224920428742006" {
		t.Fatalf("ContentID = %q", records.ContentID)
	}
	if records.Account.ExternalId != "5c8e0c7af4ce477275c6f6aac45c5200" {
		t.Fatalf("Account.ExternalId = %q", records.Account.ExternalId)
	}
	if records.Browse.ContentType != "answer" {
		t.Fatalf("Browse.ContentType = %q", records.Browse.ContentType)
	}
	if records.ZhihuRawFields.ZhihuDateCreated != "2026-06-03T02:11:43.000Z" {
		t.Fatalf("ZhihuDateCreated = %q", records.ZhihuRawFields.ZhihuDateCreated)
	}
	if records.ZhihuRawFields.ZhihuDateModified != "2026-06-03T03:28:06.000Z" {
		t.Fatalf("ZhihuDateModified = %q", records.ZhihuRawFields.ZhihuDateModified)
	}
}
