package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

func TestHandleCompatAccountListIncludesContentDisplayFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Platform{}, &model.Account{}, &model.Content{}, &model.ContentAccount{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	account := model.Account{
		Id:         "zhihu:author_1",
		PlatformId: "zhihu",
		ExternalId: "author_1",
		Nickname:   "作者",
		Timestamps: model.Timestamps{CreatedAt: 1, UpdatedAt: 1},
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	content := model.Content{
		Id:           "zhihu:answer_1#mp3",
		PlatformId:   "zhihu",
		ContentType:  "audio",
		ExternalId:   "answer_1#mp3",
		Title:        "回答标题",
		ContentURL:   "https://example.com/answer",
		DownloadPath: "/tmp/answer.mp3",
		Metadata:     `{"source_content_type":"html","output_format":"mp3","mime_type":"audio/mpeg"}`,
		Timestamps:   model.Timestamps{CreatedAt: 2, UpdatedAt: 2},
	}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content: %v", err)
	}
	if err := db.Create(&model.ContentAccount{ContentId: content.Id, AccountId: account.Id, Role: "owner", CreatedAt: 3}).Error; err != nil {
		t.Fatalf("create link: %v", err)
	}

	client := &APIClient{db: db}
	router := gin.New()
	router.POST("/api/account/list", client.handleCompatAccountList)

	req := httptest.NewRequest(http.MethodPost, "/api/account/list", bytes.NewBufferString(`{"content_filter":"with"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			List []struct {
				ContentAccounts []struct {
					Content struct {
						Title             string `json:"title"`
						ContentType       string `json:"content_type"`
						MediaType         string `json:"media_type"`
						SourceContentType string `json:"source_content_type"`
						OutputFormat      string `json:"output_format"`
						DisplayType       string `json:"display_type"`
						TypeLabel         string `json:"type_label"`
					} `json:"content"`
				} `json:"content_accounts"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if resp.Code != 0 || len(resp.Data.List) != 1 || len(resp.Data.List[0].ContentAccounts) != 1 {
		t.Fatalf("unexpected response: %#v body=%s", resp, rec.Body.String())
	}
	got := resp.Data.List[0].ContentAccounts[0].Content
	if got.Title != "回答标题" {
		t.Fatalf("title = %q", got.Title)
	}
	if got.ContentType != "audio" || got.MediaType != "audio" {
		t.Fatalf("content/media type = %q/%q", got.ContentType, got.MediaType)
	}
	if got.SourceContentType != "html" || got.OutputFormat != "mp3" {
		t.Fatalf("source/output type = %q/%q", got.SourceContentType, got.OutputFormat)
	}
	if got.DisplayType != "html mp3" || got.TypeLabel != "html mp3" {
		t.Fatalf("display/type label = %q/%q", got.DisplayType, got.TypeLabel)
	}
}
