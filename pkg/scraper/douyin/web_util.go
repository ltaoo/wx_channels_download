package douyin

import (
	"net/url"
	"strings"
)

var paramOrder = []string{
	"device_platform",
	"aid",
	"channel",
	"pc_client_type",
	"version_code",
	"version_name",
	"cookie_enabled",
	"screen_width",
	"screen_height",
	"browser_language",
	"browser_platform",
	"browser_name",
	"browser_version",
	"browser_online",
	"engine_name",
	"engine_version",
	"os_name",
	"os_version",
	"cpu_core_num",
	"device_memory",
	"platform",
	"downlink",
	"effective_type",
	"from_user_page",
	"locate_query",
	"need_time_list",
	"pc_libra_divert",
	"publish_video_strategy_type",
	"round_trip_time",
	"show_live_replay_strategy",
	"time_list_query",
	"whale_cut_token",
	"update_version_code",
	"msToken",
	"aweme_id",
	"a_bogus",
}

var defaultParams = map[string]string{
	"device_platform":             "webapp",
	"aid":                         "6383",
	"channel":                     "channel_pc_web",
	"pc_client_type":              "1",
	"version_code":                "290100",
	"version_name":                "29.1.0",
	"cookie_enabled":              "true",
	"screen_width":                "1920",
	"screen_height":               "1080",
	"browser_language":            "zh-CN",
	"browser_platform":            "Win32",
	"browser_name":                "Chrome",
	"browser_version":             "130.0.0.0",
	"browser_online":              "true",
	"engine_name":                 "Blink",
	"engine_version":              "130.0.0.0",
	"os_name":                     "Windows",
	"os_version":                  "10",
	"cpu_core_num":                "12",
	"device_memory":               "8",
	"platform":                    "PC",
	"downlink":                    "10",
	"effective_type":              "4g",
	"from_user_page":              "1",
	"locate_query":                "false",
	"need_time_list":              "1",
	"pc_libra_divert":             "Windows",
	"publish_video_strategy_type": "2",
	"round_trip_time":             "0",
	"show_live_replay_strategy":   "1",
	"time_list_query":             "0",
	"whale_cut_token":             "",
	"update_version_code":         "170400",
	"msToken":                     "",
}

func queryStringify(params map[string]string, orders []string) string {
	var builder strings.Builder
	first := true

	for _, key := range orders {
		if value, exists := params[key]; exists {
			if !first {
				builder.WriteByte('&')
			}
			first = false
			builder.WriteString(url.QueryEscape(key))
			builder.WriteByte('=')
			builder.WriteString(url.QueryEscape(value))
		}
	}

	return builder.String()
}
