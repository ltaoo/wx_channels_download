package youku

import "testing"

func TestSign(t *testing.T) {
	got := Sign("932c0584a1e938afe486173318c72189", 1701224407059, `{"a":1}`)
	if got != "e5437ce57760f4dd0bc05ac264020ec7" {
		t.Fatalf("unexpected sign %s", got)
	}
}

func TestFormatSeasonProfile(t *testing.T) {
	profile := ProfileData{
		Data: ProfileDataPayload{Extra: ProfileExtra{
			ShowID:          "sid",
			ShowName:        "节目",
			ShowImgV:        "poster.jpg",
			ShowReleaseTime: "20231126",
			ShowCategory:    "综艺",
			VideoCategory:   "综艺",
		}},
		Nodes: []Node{{
			Type: 10001,
			Nodes: []Node{
				{Type: 20009, Nodes: []Node{
					{Type: 20010, Data: NodeData{Desc: "简介", IntroSubTitle: "大陆·综艺"}},
					{Type: 10011, Data: NodeData{PersonID: "p1", Title: "嘉宾A", Subtitle: "嘉宾", Img: "a.jpg"}},
				}},
				{Type: 10013, Data: NodeData{Series: []SeriesItem{{Title: "节目", ShowID: "sid"}}}, Nodes: []Node{
					{ID: 1, Data: NodeData{Title: "第1期", Stage: 20231126, Img: "e.jpg", Action: &Action{Value: "vid1"}}},
				}},
			},
		}},
	}
	got, err := FormatSeasonProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "sid" || got.Type != "season" || len(got.Seasons) != 1 || len(got.Seasons[0].Episodes) != 1 {
		t.Fatalf("unexpected profile %#v", got)
	}
	if got.Seasons[0].Episodes[0].AirDate != "2023-11-26" {
		t.Fatalf("unexpected air date %q", got.Seasons[0].Episodes[0].AirDate)
	}
}

func TestParseProfilePageModuleListInitialData(t *testing.T) {
	html := `<html><body><script>
window.__INITIAL_DATA__ = {
	"pageMap": {
		"extra": {
			"videoCategory": "电视剧",
			"showId": "ffefdd76a5b344e0a48b",
			"showCategory": "电视剧",
			"episodeTotal": 24,
			"showName": "翘楚",
			"showImgV": "http://m.ykimg.com/poster.jpg",
			"showReleaseTime": "2026-06-02 12:00:00"
		}
	},
	"moduleList": [{
		"type": 10001,
		"components": [{
			"type": 20009,
			"itemList": [{
				"id": 740501,
				"type": 20010,
				"introTitle": "翘楚",
				"desc": "将军之女楚朝主动出击；护皇长孙进宫面圣。",
				"introSubTitle": "中国·2026·古装"
			}, {
				"id": 882245,
				"type": 10011,
				"title": "杨龙",
				"subtitle": "导演",
				"img": "http://m.ykimg.com/person.jpg"
			}]
		}, {
			"type": 10013,
			"itemList": [{
				"id": 1634383054,
				"type": 10013,
				"title": "翘楚 第1集",
				"img": "https://m.ykimg.com/e1.jpg",
				"stage": 1,
				"rank": 1,
				"videoType": "正片",
				"action_value": "XNjUzNzUzMjIxNg=="
			}, {
				"id": 1632519726,
				"type": 10013,
				"title": "翘楚 第2集",
				"img": "https://m.ykimg.com/e2.jpg",
				"stage": 2,
				"rank": 2,
				"videoType": "正片",
				"action": {"value": "XNjUzMDA3ODkwNA=="}
			}, {
				"id": 1630000000,
				"type": 10013,
				"title": "翘楚 预告",
				"stage": 3,
				"rank": 3,
				"videoType": "预告片",
				"action_value": "trailer"
			}]
		}]
	}]
};
</script></body></html>`
	profile, err := ParseProfilePage(html)
	if err != nil {
		t.Fatal(err)
	}
	got, err := FormatSeasonProfile(*profile)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "ffefdd76a5b344e0a48b" || got.Name != "翘楚" || got.Type != "season" {
		t.Fatalf("unexpected profile basics: %#v", got)
	}
	if got.Overview != "将军之女楚朝主动出击；护皇长孙进宫面圣。" {
		t.Fatalf("unexpected overview %q", got.Overview)
	}
	if len(got.Seasons) != 1 {
		t.Fatalf("unexpected seasons: %#v", got.Seasons)
	}
	season := got.Seasons[0]
	if len(season.Episodes) != 2 {
		t.Fatalf("unexpected episodes: %#v", season.Episodes)
	}
	if season.Episodes[0].ID != "XNjUzNzUzMjIxNg==" || season.Episodes[0].AirDate != "" {
		t.Fatalf("unexpected first episode: %#v", season.Episodes[0])
	}
	if len(season.Persons) != 1 || season.Persons[0].ID != "882245" || season.Persons[0].Department != "director" {
		t.Fatalf("unexpected persons: %#v", season.Persons)
	}
	if len(season.OriginCountry) != 1 || season.OriginCountry[0] != "CN" {
		t.Fatalf("unexpected origin country: %#v", season.OriginCountry)
	}
}
