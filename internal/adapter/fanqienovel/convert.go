package fanqienoveladapter

import (
	"encoding/json"
	"fmt"
	"strings"

	"wx_channel/pkg/scraper/fanqienovel"
)

func fanqie_result_from_fetch(data any) (*fanqienovel.FanqieFetchResult, error) {
	switch value := data.(type) {
	case *fanqienovel.FanqieFetchResult:
		return validate_fanqie_result(value)
	case fanqienovel.FanqieFetchResult:
		return validate_fanqie_result(&value)
	case *fanqienovel.FanqieBookProfile:
		if value == nil {
			return nil, fmt.Errorf("fanqienovel profile is nil")
		}
		return validate_fanqie_result(&fanqienovel.FanqieFetchResult{Profile: value})
	case fanqienovel.FanqieBookProfile:
		return validate_fanqie_result(&fanqienovel.FanqieFetchResult{Profile: &value})
	case *fanqienovel.FanqieResp[*fanqienovel.FanqieBookProfile]:
		if value == nil {
			return nil, fmt.Errorf("fanqienovel response is nil")
		}
		return validate_fanqie_result(&fanqienovel.FanqieFetchResult{Profile: value.Data})
	case fanqienovel.FanqieResp[*fanqienovel.FanqieBookProfile]:
		return validate_fanqie_result(&fanqienovel.FanqieFetchResult{Profile: value.Data})
	case json.RawMessage:
		return fanqie_result_from_json(value)
	case []byte:
		return fanqie_result_from_json(value)
	case string:
		return fanqie_result_from_json([]byte(value))
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encode fanqienovel fetch data: %w", err)
	}
	return fanqie_result_from_json(encoded)
}

func fanqie_result_from_json(content_json []byte) (*fanqienovel.FanqieFetchResult, error) {
	if len(strings.TrimSpace(string(content_json))) == 0 {
		return nil, fmt.Errorf("fanqienovel content is empty")
	}

	var result fanqienovel.FanqieFetchResult
	if err := json.Unmarshal(content_json, &result); err == nil && result.Profile != nil {
		return validate_fanqie_result(&result)
	}

	var profile fanqienovel.FanqieBookProfile
	if err := json.Unmarshal(content_json, &profile); err == nil && strings.TrimSpace(profile.Title) != "" {
		return validate_fanqie_result(&fanqienovel.FanqieFetchResult{Profile: &profile})
	}

	var response fanqienovel.FanqieResp[*fanqienovel.FanqieBookProfile]
	if err := json.Unmarshal(content_json, &response); err == nil && response.Data != nil {
		return validate_fanqie_result(&fanqienovel.FanqieFetchResult{Profile: response.Data})
	}
	return nil, fmt.Errorf("unsupported fanqienovel fetch data")
}

func validate_fanqie_result(result *fanqienovel.FanqieFetchResult) (*fanqienovel.FanqieFetchResult, error) {
	if result == nil || result.Profile == nil {
		return nil, fmt.Errorf("fanqienovel profile is nil")
	}
	if strings.TrimSpace(result.Profile.URL) == "" {
		return nil, fmt.Errorf("fanqienovel profile url is empty")
	}
	if strings.TrimSpace(result.Profile.Title) == "" {
		return nil, fmt.Errorf("fanqienovel profile title is empty")
	}
	return result, nil
}
