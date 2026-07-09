package iqiyi

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestSign(t *testing.T) {
	params := map[string]any{
		"entity_id":   2109395369199100,
		"timestamp":   int64(1700978349899),
		"src":         "pcw_tvg",
		"vip_status":  0,
		"vip_type":    "",
		"auth_cookie": "",
		"device_id":   "4798183996645ebf3163434564f5252c",
		"user_id":     "",
		"app_version": "6.1.0",
		"scale":       200,
	}
	if got := Sign(params); got != "FE56903F2C9BD72EC4E65729442139AB" {
		t.Fatalf("Sign() = %s", got)
	}
}

func TestFormatPosterPath(t *testing.T) {
	got := FormatPosterPath("https://pic5.iqiyipic.com/image/20231123/1f/da/v_174083380_m_601_m5.jpg")
	if got["s4"] != "https://pic5.iqiyipic.com/image/20231123/1f/da/v_174083380_m_601_m5_579_772.jpg" {
		t.Fatalf("unexpected s4 %q", got["s4"])
	}
}

func TestParseProfilePage(t *testing.T) {
	html := `<script>window.Q.PageInfo.playPageInfo = {"tvId":123,"albumId":456,"channelId":2,"albumName":"测试","categories":[{"name":"剧情","subType":2}],"people":{"guest":[{"id":1,"name":"甲","image_url":"a.jpg","character":[""]}]}};</script>`
	got, err := ParseProfilePage(html)
	if err != nil {
		t.Fatal(err)
	}
	if got.TVID != 123 || got.ChannelID != 2 || got.AlbumName != "测试" {
		t.Fatalf("unexpected profile %#v", got)
	}
}

func TestParseTVID(t *testing.T) {
	got, ok := ParseTVID("https://www.iqiyi.com/v_2dkhwocyjk4.html")
	if !ok || got != 8631805876282600 {
		t.Fatalf("unexpected tvid: %d ok=%v", got, ok)
	}
	got, ok = ParseTVID("https://www.iqiyi.com/v_x.html?positiveId=MTIz")
	if !ok || got != 123 {
		t.Fatalf("unexpected positiveId tvid: %d ok=%v", got, ok)
	}
}

func TestParseBaseInfoFixture(t *testing.T) {
	body, err := os.ReadFile("../../../scraper_examples/iqiyi/260619/baseinfo.md")
	if err != nil {
		t.Fatal(err)
	}
	rawJSON := markdownCodeBlock(t, string(body), "json")
	var resp baseInfoResponse
	if err := json.Unmarshal([]byte(rawJSON), &resp); err != nil {
		t.Fatal(err)
	}
	got, err := parseBaseInfoResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 3757044018545201 || got.Title != "人生若如初见" || got.TotalEpisode != 40 {
		t.Fatalf("unexpected base info: %#v", got)
	}
	if len(got.Seasons) != 1 {
		t.Fatalf("unexpected seasons: %#v", got.Seasons)
	}
	episodes := got.Seasons[0].Episodes
	if len(episodes) < 40 {
		t.Fatalf("expected at least 40 episodes, got %d", len(episodes))
	}
	if episodes[0].ID != "https://www.iqiyi.com/v_2dkhwocyjk4.html" || episodes[0].EpisodeNumber != 1 {
		t.Fatalf("unexpected first episode: %#v", episodes[0])
	}
	if ep := findEpisode(episodes, 40); ep == nil || ep.ID != "https://www.iqiyi.com/v_13h4cxtntto.html" {
		t.Fatalf("missing episode 40: %#v", ep)
	}
}

func markdownCodeBlock(t *testing.T, text string, lang string) string {
	t.Helper()
	marker := "```" + lang
	start := strings.Index(text, marker)
	if start < 0 {
		t.Fatalf("missing %s code block", marker)
	}
	start += len(marker)
	end := strings.Index(text[start:], "```")
	if end < 0 {
		t.Fatalf("unterminated %s code block", marker)
	}
	return strings.TrimSpace(text[start : start+end])
}

func findEpisode(episodes []Episode, number int) *Episode {
	for i := range episodes {
		if episodes[i].EpisodeNumber == number {
			return &episodes[i]
		}
	}
	return nil
}
