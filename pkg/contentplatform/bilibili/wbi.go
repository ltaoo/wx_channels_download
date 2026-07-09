package bilibili

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

var wbiMixinKeyEncTab = []int{
	46, 47, 18, 2, 53, 8, 23, 32,
	15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19,
	29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61,
	26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63,
	57, 62, 11, 36, 20, 34, 44, 52,
}

type navResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		WBIImg struct {
			ImgURL string `json:"img_url"`
			SubURL string `json:"sub_url"`
		} `json:"wbi_img"`
	} `json:"data"`
}

func (c *Client) signWBIQuery(ctx context.Context, values url.Values) (url.Values, error) {
	imgKey, subKey, err := c.fetchWBIKeys(ctx)
	if err != nil {
		return nil, err
	}
	mixinKey := mixinKey(imgKey + subKey)
	if mixinKey == "" {
		return nil, fmt.Errorf("empty bilibili wbi mixin key")
	}

	signed := url.Values{}
	for key, vals := range values {
		for _, value := range vals {
			signed.Add(key, filterWBIValue(value))
		}
	}
	signed.Set("wts", fmt.Sprint(time.Now().Unix()))
	encoded := encodeSortedValues(signed)
	sum := md5.Sum([]byte(encoded + mixinKey))
	signed.Set("w_rid", hex.EncodeToString(sum[:]))
	return signed, nil
}

func (c *Client) fetchWBIKeys(ctx context.Context) (string, string, error) {
	navURL, err := c.apiURL("/x/web-interface/nav", nil)
	if err != nil {
		return "", "", err
	}
	req, err := c.newAPIRequest(ctx, navURL)
	if err != nil {
		return "", "", err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("bilibili nav request failed: %s", resp.Status)
	}
	var nav navResponse
	if err := json.NewDecoder(resp.Body).Decode(&nav); err != nil {
		return "", "", err
	}
	if nav.Code != 0 {
		return "", "", fmt.Errorf("bilibili nav failed: %s", firstNonEmpty(nav.Message, fmt.Sprint(nav.Code)))
	}
	imgKey := keyFromWBIURL(nav.Data.WBIImg.ImgURL)
	subKey := keyFromWBIURL(nav.Data.WBIImg.SubURL)
	if imgKey == "" || subKey == "" {
		return "", "", fmt.Errorf("bilibili nav missing wbi keys")
	}
	return imgKey, subKey, nil
}

func mixinKey(raw string) string {
	var b strings.Builder
	for _, idx := range wbiMixinKeyEncTab {
		if idx >= 0 && idx < len(raw) {
			b.WriteByte(raw[idx])
		}
	}
	key := b.String()
	if len(key) > 32 {
		key = key[:32]
	}
	return key
}

func keyFromWBIURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	base := path.Base(u.Path)
	ext := path.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func filterWBIValue(value string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '!', '\'', '(', ')', '*':
			return -1
		default:
			return r
		}
	}, value)
}

func encodeSortedValues(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, key := range keys {
		vals := append([]string(nil), values[key]...)
		sort.Strings(vals)
		for _, value := range vals {
			if b.Len() > 0 {
				b.WriteByte('&')
			}
			b.WriteString(url.QueryEscape(key))
			b.WriteByte('=')
			b.WriteString(url.QueryEscape(value))
		}
	}
	return b.String()
}
