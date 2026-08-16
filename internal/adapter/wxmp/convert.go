package wxmpadapter

import (
	"encoding/json"
	"fmt"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/wxmp"
)

func article_data_from_fetch(data any) (*wxmp.ArticleCgiDataNew, error) {
	switch value := data.(type) {
	case *wxmp.ArticleCgiDataNew:
		if value == nil {
			return nil, fmt.Errorf("wxmp article data is nil")
		}
		return value, nil
	case wxmp.ArticleCgiDataNew:
		return &value, nil
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encode wxmp fetch data: %w", err)
	}
	var article_data wxmp.ArticleCgiDataNew
	if err := json.Unmarshal(encoded, &article_data); err != nil {
		return nil, fmt.Errorf("decode wxmp fetch data: %w", err)
	}
	return &article_data, nil
}

func (a *OfficialAccountAdapter) ToContent(data any) (*model.Content, error) {
	article_data, err := article_data_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content, _, err := ToContent(article_data)
	return content, err
}

func (a *OfficialAccountAdapter) ToAccount(data any) (*model.Account, error) {
	article_data, err := article_data_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToAccount(article_data)
}

func (a *OfficialAccountAdapter) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	article_data, err := article_data_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content, detail, err := ToContent(article_data)
	if err != nil {
		return nil, err
	}
	if album_ext, ok := detail.(*ContentAlbumExt); ok {
		album_ext.Album.Images = content_image_values(album_ext.Images, content.Id)
		detail = album_ext.Album
	}
	if detail == nil {
		return nil, nil
	}
	return []adapter.ContentDetail{{Type: content.Type, Key: content.Id, Data: detail}}, nil
}

// BuildDownloadTaskFromFetch normalizes the wrapper returned by FetchArticle
// and delegates to BuildDownloadTask so the preview and task creation share the
// same task-building implementation.
func (a *OfficialAccountAdapter) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	article_data, err := article_data_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content_json, err := json.Marshal(article_data)
	if err != nil {
		return nil, fmt.Errorf("encode wxmp download task content: %w", err)
	}
	return a.BuildDownloadTask(content_json, config_json)
}
