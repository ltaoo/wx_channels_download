package ttkadapter

import (
	"encoding/json"
	"fmt"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/ttk"
)

func ttk_result_from_fetch(data any) (*ttk.TtkFetchResult, error) {
	switch value := data.(type) {
	case *ttk.TtkFetchResult:
		return validate_ttk_result(value)
	case ttk.TtkFetchResult:
		return validate_ttk_result(&value)
	case *ttk.TtkNovel:
		if value == nil {
			return nil, fmt.Errorf("ttk profile is nil")
		}
		return validate_ttk_result(&ttk.TtkFetchResult{Profile: value})
	case ttk.TtkNovel:
		return validate_ttk_result(&ttk.TtkFetchResult{Profile: &value})
	case *ttk.TtkResp[*ttk.TtkNovel]:
		if value == nil {
			return nil, fmt.Errorf("ttk response is nil")
		}
		return validate_ttk_result(&ttk.TtkFetchResult{Profile: value.Data})
	case ttk.TtkResp[*ttk.TtkNovel]:
		return validate_ttk_result(&ttk.TtkFetchResult{Profile: value.Data})
	case json.RawMessage:
		return ttk_result_from_json(value)
	case []byte:
		return ttk_result_from_json(value)
	case string:
		return ttk_result_from_json([]byte(value))
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encode ttk fetch data: %w", err)
	}
	return ttk_result_from_json(encoded)
}

func ttk_result_from_json(content_json []byte) (*ttk.TtkFetchResult, error) {
	if len(strings.TrimSpace(string(content_json))) == 0 {
		return nil, fmt.Errorf("ttk content is empty")
	}
	var result ttk.TtkFetchResult
	if err := json.Unmarshal(content_json, &result); err == nil && result.Profile != nil {
		return validate_ttk_result(&result)
	}
	var profile ttk.TtkNovel
	if err := json.Unmarshal(content_json, &profile); err == nil && strings.TrimSpace(profile.Title) != "" {
		return validate_ttk_result(&ttk.TtkFetchResult{Profile: &profile})
	}
	var response ttk.TtkResp[*ttk.TtkNovel]
	if err := json.Unmarshal(content_json, &response); err == nil && response.Data != nil {
		return validate_ttk_result(&ttk.TtkFetchResult{Profile: response.Data})
	}
	return nil, fmt.Errorf("unsupported ttk fetch data")
}

func validate_ttk_result(result *ttk.TtkFetchResult) (*ttk.TtkFetchResult, error) {
	if result == nil || result.Profile == nil {
		return nil, fmt.Errorf("ttk profile is nil")
	}
	if strings.TrimSpace(result.Profile.URL) == "" {
		return nil, fmt.Errorf("ttk profile url is empty")
	}
	if strings.TrimSpace(result.Profile.Title) == "" {
		return nil, fmt.Errorf("ttk profile title is empty")
	}
	if _, err := book_id_from_url(result.Profile.URL); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *TTKAdapter) ToContent(data any) (*model.Content, error) {
	result, err := ttk_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToContent(result)
}

func (a *TTKAdapter) ToAccount(data any) (*model.Account, error) {
	result, err := ttk_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToAccount(result)
}

func (a *TTKAdapter) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	result, err := ttk_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToContentDetails(result)
}
