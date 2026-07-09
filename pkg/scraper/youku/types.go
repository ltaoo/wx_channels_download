package youku

import (
	"encoding/json"
	"time"
)

type Token struct {
	Cookie  string
	Token   string
	Expired time.Time
}

type SearchResult struct {
	List []SearchItem `json:"list"`
}

type SearchItem struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	OriginalName  string   `json:"original_name"`
	Overview      string   `json:"overview"`
	PosterPath    string   `json:"poster_path"`
	BackdropPath  string   `json:"backdrop_path"`
	FirstAirDate  string   `json:"first_air_date"`
	OriginCountry []string `json:"origin_country"`
	Type          string   `json:"type"`
	Source        string   `json:"source"`
}

type ProfileWithSeasons struct {
	Platform     string   `json:"platform"`
	Type         string   `json:"type"`
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Overview     string   `json:"overview"`
	PosterPath   string   `json:"poster_path"`
	BackdropPath string   `json:"backdrop_path"`
	OriginalName string   `json:"original_name"`
	Seasons      []Season `json:"seasons"`
}

type SeasonProfile struct {
	Type          string    `json:"type"`
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Overview      string    `json:"overview"`
	PosterPath    string    `json:"poster_path"`
	AirDate       string    `json:"air_date"`
	BackdropPath  string    `json:"backdrop_path"`
	OriginalName  string    `json:"original_name"`
	Episodes      []Episode `json:"episodes"`
	Genres        []string  `json:"genres"`
	OriginCountry []string  `json:"origin_country"`
	Persons       []Person  `json:"persons"`
}

type Season struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Overview      string    `json:"overview"`
	PosterPath    string    `json:"poster_path"`
	AirDate       string    `json:"air_date"`
	Episodes      []Episode `json:"episodes,omitempty"`
	Genres        []string  `json:"genres"`
	OriginCountry []string  `json:"origin_country"`
	Persons       []Person  `json:"persons"`
}

type Episode struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Thumbnail     string `json:"thumbnail"`
	EpisodeNumber int    `json:"episode_number"`
	AirDate       string `json:"air_date"`
}

type Person struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Avatar     string   `json:"avatar"`
	Character  []string `json:"character"`
	Department string   `json:"department"`
}

type PageInfo struct {
	Data struct {
		Data ProfileData `json:"data"`
	} `json:"data"`
	PageMap struct {
		Extra ProfileExtra `json:"extra"`
	} `json:"pageMap"`
	ModuleList []Node `json:"moduleList"`
}

func (p PageInfo) ProfileData() ProfileData {
	profile := p.Data.Data
	if profile.Data.Extra.ShowID == "" && p.PageMap.Extra.ShowID != "" {
		profile.Data.Extra = p.PageMap.Extra
	}
	if len(profile.Nodes) == 0 && len(p.ModuleList) > 0 {
		profile.Nodes = p.ModuleList
	}
	return profile
}

type ProfileData struct {
	Data  ProfileDataPayload `json:"data"`
	Nodes []Node             `json:"nodes"`
}

type ProfileDataPayload struct {
	Extra ProfileExtra `json:"extra"`
	Title string       `json:"title"`
}

type ProfileExtra struct {
	VideoID           string `json:"videoId"`
	VideoCategory     string `json:"videoCategory"`
	VideoImg          string `json:"videoImg"`
	VideoImgV         string `json:"videoImgV"`
	VideoTitle        string `json:"videoTitle"`
	ShowID            string `json:"showId"`
	ShowCategory      string `json:"showCategory"`
	EpisodeTotal      int    `json:"episodeTotal"`
	ShowName          string `json:"showName"`
	Completed         bool   `json:"completed"`
	ShowImg           string `json:"showImg"`
	ShowImgV          string `json:"showImgV"`
	ShowReleaseTime   string `json:"showReleaseTime"`
	EpisodeFinalStage int    `json:"episodeFinalStage"`
}

type Node struct {
	ID    int64    `json:"id"`
	Type  int      `json:"type"`
	Data  NodeData `json:"data"`
	Nodes []Node   `json:"nodes"`
}

func (n *Node) UnmarshalJSON(data []byte) error {
	type nodeAlias struct {
		ID         int64    `json:"id"`
		Type       int      `json:"type"`
		Data       NodeData `json:"data"`
		Nodes      []Node   `json:"nodes"`
		Components []Node   `json:"components"`
		ItemList   []Node   `json:"itemList"`
	}
	var raw nodeAlias
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var inline NodeData
	_ = json.Unmarshal(data, &inline)
	nodeData := mergeNodeData(inline, raw.Data)
	nodes := raw.Nodes
	if len(nodes) == 0 {
		nodes = raw.Components
	}
	if len(nodes) == 0 {
		nodes = raw.ItemList
	}
	*n = Node{
		ID:    raw.ID,
		Type:  raw.Type,
		Data:  nodeData,
		Nodes: nodes,
	}
	return nil
}

type NodeData struct {
	Title         string       `json:"title"`
	Subtitle      string       `json:"subtitle"`
	Desc          string       `json:"desc"`
	IntroTitle    string       `json:"introTitle"`
	IntroSubTitle string       `json:"introSubTitle"`
	PersonID      any          `json:"personId"`
	Img           string       `json:"img"`
	Stage         any          `json:"stage"`
	Rank          int          `json:"rank"`
	VideoType     string       `json:"videoType"`
	Series        []SeriesItem `json:"series"`
	Action        *Action      `json:"action"`
	ActionValue   string       `json:"action_value"`
}

func mergeNodeData(base NodeData, override NodeData) NodeData {
	if override.Title != "" {
		base.Title = override.Title
	}
	if override.Subtitle != "" {
		base.Subtitle = override.Subtitle
	}
	if override.Desc != "" {
		base.Desc = override.Desc
	}
	if override.IntroTitle != "" {
		base.IntroTitle = override.IntroTitle
	}
	if override.IntroSubTitle != "" {
		base.IntroSubTitle = override.IntroSubTitle
	}
	if override.PersonID != nil {
		base.PersonID = override.PersonID
	}
	if override.Img != "" {
		base.Img = override.Img
	}
	if override.Stage != nil {
		base.Stage = override.Stage
	}
	if override.Rank != 0 {
		base.Rank = override.Rank
	}
	if override.VideoType != "" {
		base.VideoType = override.VideoType
	}
	if len(override.Series) > 0 {
		base.Series = override.Series
	}
	if override.Action != nil {
		base.Action = override.Action
	}
	if override.ActionValue != "" {
		base.ActionValue = override.ActionValue
	}
	return base
}

type SeriesItem struct {
	Title              string `json:"title"`
	ShowID             string `json:"showId"`
	LastEpisodeVideoID string `json:"lastEpisodeVideoId"`
	Current            bool   `json:"current"`
}

type Action struct {
	Value string `json:"value"`
}

type mtopResponse struct {
	Ret  []string                  `json:"ret"`
	Data map[string]mtopDataHolder `json:"data"`
}

type mtopDataHolder struct {
	Success bool        `json:"success"`
	Data    ProfileData `json:"data"`
}
