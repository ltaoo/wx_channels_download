package douban

type MediaType string

const (
	MediaTypeMovie  MediaType = "movie"
	MediaTypeTV     MediaType = "tv"
	MediaTypeSeason MediaType = "season"
	MediaTypeBook   MediaType = "book"
)

type SearchResult struct {
	List []SearchItem `json:"list"`
}

type SearchItem struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	OriginalName string        `json:"origin_name"`
	Overview     string        `json:"overview"`
	PosterPath   string        `json:"poster_path"`
	AirDate      string        `json:"air_date"`
	VoteAverage  float64       `json:"vote_average"`
	Type         string        `json:"type"`
	Genres       []Genre       `json:"genres"`
	Raw          RawSearchItem `json:"-"`
}

type RawSearchItem struct {
	ID           any           `json:"id"`
	Topics       []any         `json:"topics"`
	Title        string        `json:"title"`
	Abstract     string        `json:"abstract"`
	Abstract2    string        `json:"abstract_2"`
	CoverURL     string        `json:"cover_url"`
	LabelActions []any         `json:"label_actions"`
	TLPName      string        `json:"tlp_name"`
	URL          string        `json:"url"`
	ExtraActions []any         `json:"extra_actions"`
	Labels       []DoubanLabel `json:"labels"`
	Rating       *DoubanRating `json:"rating"`
}

type DoubanLabel struct {
	Color string `json:"color"`
	Text  string `json:"text"`
}

type DoubanRating struct {
	Count     int     `json:"count"`
	Value     float64 `json:"value"`
	StarCount int     `json:"star_count"`
}

type Genre struct {
	ID    int    `json:"id,omitempty"`
	Text  string `json:"text,omitempty"`
	Value int    `json:"value,omitempty"`
	Label string `json:"label,omitempty"`
}

type MediaProfile struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	Name          string   `json:"name"`
	OriginalName  string   `json:"original_name"`
	PosterPath    string   `json:"poster_path"`
	CoverURL      string   `json:"cover_url"`
	AirDate       string   `json:"air_date"`
	Overview      string   `json:"overview"`
	SourceCount   int      `json:"source_count"`
	Alias         string   `json:"alias"`
	Actors        []Person `json:"actors"`
	Director      []Person `json:"director"`
	Author        []Person `json:"author"`
	VoteAverage   float64  `json:"vote_average"`
	Genres        []Genre  `json:"genres"`
	OriginCountry string   `json:"origin_country"`
	IMDB          string   `json:"imdb"`
}

type GroupTopicProfile struct {
	ID              string `json:"id"`
	GroupID         string `json:"group_id"`
	GroupName       string `json:"group_name"`
	Title           string `json:"title"`
	BodyHTML        string `json:"body_html"`
	BodyText        string `json:"body_text"`
	URL             string `json:"url"`
	SourceURL       string `json:"source_url"`
	AuthorID        string `json:"author_id"`
	AuthorName      string `json:"author_name"`
	AuthorURL       string `json:"author_url"`
	AuthorAvatarURL string `json:"author_avatar_url"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	CommentCount    int    `json:"comment_count"`
}

type Person struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
}

type RankResult struct {
	List []RankItem `json:"list"`
}

type RankItem struct {
	Name      string  `json:"name"`
	Order     int     `json:"order"`
	Rate      float64 `json:"rate"`
	ExtraText string  `json:"extra_text"`
	DoubanID  string  `json:"douban_id"`
}

type MatchMedia struct {
	Type         MediaType
	Name         string
	OriginalName string
	Order        int
	AirDate      string
}

var genreTextToValue = map[string]int{
	"纪录片":  1,
	"传记":   2,
	"犯罪":   3,
	"历史":   4,
	"动作":   5,
	"情色":   6,
	"歌舞":   7,
	"儿童":   8,
	"悬疑":   9,
	"剧情":   10,
	"灾难":   11,
	"爱情":   12,
	"音乐":   13,
	"冒险":   14,
	"奇幻":   15,
	"科幻":   16,
	"运动":   17,
	"惊悚":   18,
	"恐怖":   19,
	"战争":   20,
	"短片":   21,
	"喜剧":   24,
	"动画":   25,
	"西部":   27,
	"家庭":   28,
	"武侠":   29,
	"古装":   30,
	"黑色电影": 31,
}
