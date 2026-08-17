package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	default_api_base_url = "https://open.feishu.cn"
	max_response_size    = 20 << 20
	max_avatar_size      = 5 << 20
)

type token_response struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
}

type table_record struct {
	Fields map[string]interface{} `json:"fields"`
}

type table_records_response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		HasMore   bool           `json:"has_more"`
		PageToken string         `json:"page_token"`
		Items     []table_record `json:"items"`
	} `json:"data"`
}

type sponsor struct {
	Text   string `json:"text"`
	Image  string `json:"image,omitempty"`
	Href   string `json:"href,omitempty"`
	Time   string `json:"time"`
	Amount string `json:"amount"`
	Note   string `json:"note,omitempty"`
	ID     string `json:"id"`
}

type output_response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		List     []sponsor `json:"list"`
		Total    int       `json:"total"`
		Page     int       `json:"page"`
		PageSize int       `json:"pageSize"`
	} `json:"data"`
}

func main() {
	output_path := flag.String("output", "sponsors.json", "output JSON path")
	flag.Parse()

	if err := run(*output_path); err != nil {
		log.Fatal(err)
	}
}

func run(output_path string) error {
	app_id := os.Getenv("FEISHU_APP_ID")
	app_secret := os.Getenv("FEISHU_APP_SECRET")
	base_token := os.Getenv("FEISHU_BASE_TOKEN")
	table_id := os.Getenv("FEISHU_TABLE_ID")
	view_id := os.Getenv("FEISHU_VIEW_ID")
	sort_value := os.Getenv("FEISHU_SORT")
	api_base_url := strings.TrimRight(os.Getenv("FEISHU_API_BASE_URL"), "/")

	if app_id == "" || app_secret == "" || base_token == "" || table_id == "" {
		return errors.New("missing required Feishu configuration: FEISHU_APP_ID, FEISHU_APP_SECRET, FEISHU_BASE_TOKEN, FEISHU_TABLE_ID")
	}
	if sort_value == "" {
		sort_value = `["赞赏时间 ASC"]`
	}
	if api_base_url == "" {
		api_base_url = default_api_base_url
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	http_client := &http.Client{Timeout: 30 * time.Second}
	access_token, err := fetch_access_token(ctx, http_client, api_base_url, app_id, app_secret)
	if err != nil {
		return err
	}

	records, err := fetch_table_records(ctx, http_client, api_base_url, access_token, base_token, table_id, view_id, sort_value)
	if err != nil {
		return err
	}

	avatar_cache := make(map[string]string)
	resolve_avatar := func(field interface{}) string {
		avatar, avatar_err := resolve_avatar_field(ctx, http_client, access_token, field, avatar_cache)
		if avatar_err != nil {
			log.Printf("skip an unavailable sponsor avatar: %v", avatar_err)
			return ""
		}
		return avatar
	}
	sponsors := records_to_sponsors(records, resolve_avatar)
	if len(sponsors) == 0 {
		return errors.New("Feishu returned no publishable sponsor records; refusing to replace the current data")
	}

	response := output_response{Code: 0, Msg: ""}
	response.Data.List = sponsors
	response.Data.Total = len(records)
	response.Data.Page = 1
	response.Data.PageSize = len(sponsors)

	contents, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sponsor data: %w", err)
	}
	contents = append(contents, '\n')

	if err := os.MkdirAll(filepath.Dir(output_path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	temporary_path := output_path + ".tmp"
	if err := os.WriteFile(temporary_path, contents, 0o644); err != nil {
		return fmt.Errorf("write sponsor data: %w", err)
	}
	if err := os.Rename(temporary_path, output_path); err != nil {
		return fmt.Errorf("publish sponsor data: %w", err)
	}

	log.Printf("generated %d sponsor records", len(sponsors))
	return nil
}

func fetch_access_token(ctx context.Context, http_client *http.Client, api_base_url, app_id, app_secret string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"app_id":     app_id,
		"app_secret": app_secret,
	})
	if err != nil {
		return "", fmt.Errorf("marshal Feishu credentials: %w", err)
	}

	var response token_response
	endpoint := api_base_url + "/open-apis/auth/v3/tenant_access_token/internal"
	if err := do_json_request(ctx, http_client, http.MethodPost, endpoint, "", body, &response); err != nil {
		return "", fmt.Errorf("request Feishu access token: %w", err)
	}
	if response.Code != 0 || response.TenantAccessToken == "" {
		return "", fmt.Errorf("request Feishu access token: code=%d, msg=%s", response.Code, response.Msg)
	}
	return response.TenantAccessToken, nil
}

func fetch_table_records(ctx context.Context, http_client *http.Client, api_base_url, access_token, base_token, table_id, view_id, sort_value string) ([]table_record, error) {
	endpoint := fmt.Sprintf(
		"%s/open-apis/bitable/v1/apps/%s/tables/%s/records",
		api_base_url,
		url.PathEscape(base_token),
		url.PathEscape(table_id),
	)
	page_token := ""
	records := make([]table_record, 0)

	for {
		query := url.Values{}
		query.Set("page_size", "500")
		if sort_value != "" {
			query.Set("sort", sort_value)
		} else if view_id != "" {
			query.Set("view_id", view_id)
		}
		if page_token != "" {
			query.Set("page_token", page_token)
		}

		var response table_records_response
		if err := do_json_request(ctx, http_client, http.MethodGet, endpoint+"?"+query.Encode(), access_token, nil, &response); err != nil {
			return nil, fmt.Errorf("list Feishu sponsor records: %w", err)
		}
		if response.Code != 0 {
			return nil, fmt.Errorf("list Feishu sponsor records: code=%d, msg=%s", response.Code, response.Msg)
		}

		records = append(records, response.Data.Items...)
		if !response.Data.HasMore {
			return records, nil
		}
		if response.Data.PageToken == "" || response.Data.PageToken == page_token {
			return nil, errors.New("list Feishu sponsor records: invalid pagination token")
		}
		page_token = response.Data.PageToken
	}
}

func do_json_request(ctx context.Context, http_client *http.Client, method, endpoint, access_token string, body []byte, destination interface{}) error {
	var last_err error
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}
		request.Header.Set("Accept", "application/json")
		if len(body) > 0 {
			request.Header.Set("Content-Type", "application/json")
		}
		if access_token != "" {
			request.Header.Set("Authorization", "Bearer "+access_token)
		}

		response, err := http_client.Do(request)
		if err != nil {
			last_err = err
		} else {
			response_body, read_err := io.ReadAll(io.LimitReader(response.Body, max_response_size+1))
			response.Body.Close()
			if read_err != nil {
				last_err = read_err
			} else if len(response_body) > max_response_size {
				return errors.New("response exceeds size limit")
			} else if response.StatusCode >= http.StatusInternalServerError || response.StatusCode == http.StatusTooManyRequests {
				last_err = fmt.Errorf("temporary HTTP status %d", response.StatusCode)
			} else if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
				return fmt.Errorf("HTTP status %d", response.StatusCode)
			} else if err := json.Unmarshal(response_body, destination); err != nil {
				return fmt.Errorf("decode response: %w", err)
			} else {
				return nil
			}
		}

		if attempt < 2 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(1<<attempt) * time.Second):
			}
		}
	}
	return last_err
}

func records_to_sponsors(records []table_record, resolve_avatar func(interface{}) string) []sponsor {
	name_to_id := make(map[string]string)
	next_id := 1
	result := make([]sponsor, 0, len(records))

	for _, record := range records {
		name := field_string(record.Fields["赞赏者名称"])
		if name == "" {
			continue
		}

		id := ""
		if name != "匿名" {
			id = name_to_id[name]
		}
		if id == "" {
			id = strconv.Itoa(next_id)
			next_id++
			if name != "匿名" {
				name_to_id[name] = id
			}
		}

		href := link_from_field(record.Fields["赞赏者个人主页链接"])
		display_name := name
		if href == "" {
			display_name = mask_name(name)
		}

		result = append(result, sponsor{
			Text:   display_name,
			Image:  resolve_avatar(record.Fields["赞赏者头像链接"]),
			Href:   href,
			Time:   parse_time(record.Fields["赞赏时间"]),
			Amount: field_string(record.Fields["赞赏金额"]),
			Note:   field_string(record.Fields["备注"]),
			ID:     id,
		})
	}

	return result
}

func field_string(field interface{}) string {
	switch value := field.(type) {
	case nil:
		return ""
	case string:
		return value
	case json.Number:
		return value.String()
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 32)
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case map[string]interface{}:
		for _, key := range []string{"text", "link", "url"} {
			if text, ok := value[key].(string); ok {
				return text
			}
		}
	case []interface{}:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if text := field_string(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	}
	return ""
}

func link_from_field(field interface{}) string {
	switch value := field.(type) {
	case string:
		return value
	case map[string]interface{}:
		if link, ok := value["link"].(string); ok {
			return link
		}
	}
	return ""
}

func resolve_avatar_field(ctx context.Context, http_client *http.Client, access_token string, field interface{}, avatar_cache map[string]string) (string, error) {
	if field == nil {
		return "", nil
	}
	if value, ok := field.(string); ok {
		return value, nil
	}

	var attachment map[string]interface{}
	switch value := field.(type) {
	case []interface{}:
		if len(value) > 0 {
			attachment, _ = value[0].(map[string]interface{})
		}
	case map[string]interface{}:
		attachment = value
	}
	if attachment == nil {
		return "", nil
	}
	if link, ok := attachment["link"].(string); ok && link != "" {
		return link, nil
	}

	download_url := ""
	if temporary_url, ok := attachment["tmp_url"].(string); ok {
		download_url = temporary_url
	}
	if download_url == "" {
		download_url, _ = attachment["url"].(string)
	}
	if download_url == "" {
		return "", nil
	}
	if cached, ok := avatar_cache[download_url]; ok {
		return cached, nil
	}

	avatar, err := download_avatar(ctx, http_client, access_token, download_url)
	if err != nil {
		return "", err
	}
	avatar_cache[download_url] = avatar
	return avatar, nil
}

func download_avatar(ctx context.Context, http_client *http.Client, access_token, download_url string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, download_url, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+access_token)

	response, err := http_client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("avatar HTTP status %d", response.StatusCode)
	}

	contents, err := io.ReadAll(io.LimitReader(response.Body, max_avatar_size+1))
	if err != nil {
		return "", err
	}
	if len(contents) > max_avatar_size {
		return "", errors.New("avatar exceeds size limit")
	}
	content_type := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	if content_type == "" || content_type == "application/octet-stream" {
		content_type = mime.TypeByExtension(filepath.Ext(download_url))
	}
	if !strings.HasPrefix(content_type, "image/") {
		return "", fmt.Errorf("avatar has unsupported content type %q", content_type)
	}

	return "data:" + content_type + ";base64," + base64.StdEncoding.EncodeToString(contents), nil
}

func parse_time(field interface{}) string {
	if text, ok := field.(string); ok {
		return text
	}

	var milliseconds int64
	switch value := field.(type) {
	case float64:
		milliseconds = int64(value)
	case int64:
		milliseconds = value
	case int:
		milliseconds = int64(value)
	case json.Number:
		milliseconds, _ = value.Int64()
	}
	if milliseconds == 0 {
		return ""
	}
	return time.UnixMilli(milliseconds).UTC().Format("2006-01-02 15:04")
}

func mask_name(name string) string {
	if name == "匿名" {
		return name
	}
	runes := []rune(name)
	length := len(runes)
	if length == 0 {
		return ""
	}
	if length == 1 {
		return "*"
	}
	if length == 2 {
		return string(runes[0]) + "*"
	}

	mask_count := 1
	if length >= 5 {
		mask_count = 2
	}
	prefix_length := (length - mask_count) / 2
	suffix_length := length - mask_count - prefix_length
	return string(runes[:prefix_length]) + strings.Repeat("*", mask_count) + string(runes[length-suffix_length:])
}
