package iqiyi

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dop251/goja"
)

const signSecret = "howcuteitis"

func Sign(params map[string]any) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		if key != "sign" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys)+1)
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, params[key]))
	}
	parts = append(parts, "secret_key="+signSecret)
	sum := md5.Sum([]byte(strings.Join(parts, "&")))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func BuildQuery(extra map[string]any) map[string]any {
	query := map[string]any{
		"timestamp":   time.Now().UnixMilli(),
		"src":         "pcw_tvg",
		"vip_status":  0,
		"vip_type":    "",
		"auth_cookie": "",
		"device_id":   "4798183996645ebf3163434564f5252c",
		"user_id":     "",
		"app_version": "6.1.0",
		"scale":       200,
	}
	for key, value := range extra {
		query[key] = value
	}
	query["sign"] = Sign(query)
	return query
}

func BuildLWQuery(extra map[string]any) map[string]any {
	query := map[string]any{
		"timestamp":   time.Now().UnixMilli(),
		"src":         "pca_tvg",
		"vip_status":  0,
		"vip_type":    -1,
		"auth_cookie": "",
		"device_id":   "4798183996645ebf3163434564f5252c",
		"user_id":     0,
		"conduit_id":  "",
		"pcv":         "17.063.25600",
		"app_version": "17.063.25600",
		"ext":         "",
		"app_mode":    "standard",
		"os":          "",
		"scale":       200,
	}
	for key, value := range extra {
		query[key] = value
	}
	query["sign"] = Sign(query)
	return query
}

func withBase64Padding(value string) string {
	if mod := len(value) % 4; mod != 0 {
		value += strings.Repeat("=", 4-mod)
	}
	return value
}

func FormatPosterPath(rawURL string) map[string]string {
	sizes := map[string]string{
		"s2": "_260_360",
		"s3": "_405_540",
		"s4": "_579_772",
	}
	lastDot := strings.LastIndex(rawURL, ".")
	if lastDot < 0 {
		return map[string]string{"s1": rawURL, "s2": rawURL, "s3": rawURL, "s4": rawURL}
	}
	prev := rawURL[:lastDot]
	if !strings.HasSuffix(prev, "m5") {
		prev = regexpImageSize.ReplaceAllString(prev, "")
	}
	suffix := rawURL[lastDot:]
	out := map[string]string{"s1": rawURL}
	for key, size := range sizes {
		out[key] = prev + size + suffix
	}
	return out
}

func FormatPeople(people map[string][]RawPerson) []Person {
	orders := map[string]int{"main_charactor": 1, "director": 2, "screen_writer": 3, "host": 4, "guest": 5}
	keys := make([]string, 0, len(people))
	for key := range people {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return orders[keys[i]] < orders[keys[j]]
	})
	var out []Person
	for _, department := range keys {
		for _, item := range people[department] {
			characters := make([]string, 0, len(item.Character))
			for _, character := range item.Character {
				if strings.TrimSpace(character) != "" {
					characters = append(characters, character)
				}
			}
			out = append(out, Person{
				ID:         item.ID,
				Name:       item.Name,
				Avatar:     item.ImageURL,
				Character:  characters,
				Department: department,
				Order:      len(out) + 1,
			})
		}
	}
	return out
}

func evalJSObjectJSON(expr string) (json.RawMessage, error) {
	vm := goja.New()
	value, err := vm.RunString("(" + expr + ")")
	if err != nil {
		return nil, err
	}
	if err := vm.Set("__iqiyi_value", value); err != nil {
		return nil, err
	}
	result, err := vm.RunString("JSON.stringify(__iqiyi_value)")
	if err != nil {
		return nil, err
	}
	text := result.String()
	if text == "" || text == "undefined" {
		return nil, fmt.Errorf("script object is not JSON serializable")
	}
	return json.RawMessage(text), nil
}
