package xiaohongshu

import "encoding/json"

const (
	PlatformID = "xiaohongshu"
	SourceURL  = "https://www.xiaohongshu.com/"
)

type NoteURL struct {
	NoteID    string
	Canonical string
	XSecToken string
}

type NotePage struct {
	URL              NoteURL
	Source           string
	PageHTML         string
	InitialState     *InitialState
	InitialStateJSON json.RawMessage
	Note             Note
}

type InitialState struct {
	Global json.RawMessage `json:"global,omitempty"`
	Note   NoteState       `json:"note"`
	Raw    json.RawMessage `json:"-"`
}

type NoteState struct {
	FirstNoteID   string                    `json:"firstNoteId"`
	CurrentNoteID string                    `json:"currentNoteId"`
	NoteDetailMap map[string]NoteDetailItem `json:"noteDetailMap"`
}

type NoteDetailItem struct {
	CurrentTime int64           `json:"currentTime"`
	Comments    json.RawMessage `json:"comments,omitempty"`
	Note        Note            `json:"note"`
}

type Note struct {
	XSecToken      string       `json:"xsecToken"`
	NoteID         string       `json:"noteId"`
	Title          string       `json:"title"`
	Desc           string       `json:"desc"`
	User           User         `json:"user"`
	Video          Video        `json:"video,omitempty"`
	Time           int64        `json:"time"`
	ShareInfo      ShareInfo    `json:"shareInfo"`
	Type           string       `json:"type"`
	InteractInfo   InteractInfo `json:"interactInfo"`
	ImageList      []Image      `json:"imageList"`
	TagList        []Tag        `json:"tagList"`
	AtUserList     []User       `json:"atUserList"`
	LastUpdateTime int64        `json:"lastUpdateTime"`
}

type User struct {
	UserID    string `json:"userId"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	XSecToken string `json:"xsecToken"`
}

type ShareInfo struct {
	UnShare bool `json:"unShare"`
}

type InteractInfo struct {
	Liked          bool   `json:"liked"`
	LikedCount     string `json:"likedCount"`
	Collected      bool   `json:"collected"`
	CollectedCount string `json:"collectedCount"`
	CommentCount   string `json:"commentCount"`
	ShareCount     string `json:"shareCount"`
	Followed       bool   `json:"followed"`
	Relation       string `json:"relation"`
}

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type Image struct {
	Width      int         `json:"width"`
	Height     int         `json:"height"`
	URL        string      `json:"url"`
	URLPre     string      `json:"urlPre"`
	URLDefault string      `json:"urlDefault"`
	FileID     string      `json:"fileId"`
	TraceID    string      `json:"traceId"`
	LivePhoto  bool        `json:"livePhoto"`
	InfoList   []ImageInfo `json:"infoList"`
}

type ImageInfo struct {
	URL        string `json:"url"`
	ImageScene string `json:"imageScene"`
}

type Video struct {
	Media   VideoMedia `json:"media"`
	Image   VideoImage `json:"image"`
	Capa    VideoCapa  `json:"capa"`
	MediaV2 string     `json:"mediaV2"`
}

type VideoMedia struct {
	Video  VideoInfo   `json:"video"`
	Stream VideoStream `json:"stream"`
}

type VideoInfo struct {
	VideoID     int64    `json:"videoId"`
	StreamTypes []int    `json:"streamTypes"`
	BizName     int      `json:"bizName"`
	BizID       string   `json:"bizId"`
	Duration    int      `json:"duration"`
	MD5         string   `json:"md5"`
	HDRType     int      `json:"hdrType"`
	DRMType     int      `json:"drmType"`
	Width       int      `json:"width,omitempty"`
	Height      int      `json:"height,omitempty"`
	Bound       []Bounds `json:"bound,omitempty"`
}

type Bounds struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type VideoStream struct {
	H264 []VideoStreamInfo `json:"h264"`
	H265 []VideoStreamInfo `json:"h265"`
	H266 []VideoStreamInfo `json:"h266"`
	AV1  []VideoStreamInfo `json:"av1"`
}

type VideoStreamInfo struct {
	AudioCodec    string   `json:"audioCodec"`
	MasterURL     string   `json:"masterUrl"`
	Format        string   `json:"format"`
	Height        int      `json:"height"`
	Width         int      `json:"width"`
	Size          int64    `json:"size"`
	StreamType    int      `json:"streamType"`
	StreamDesc    string   `json:"streamDesc"`
	QualityType   string   `json:"qualityType"`
	DefaultStream int      `json:"defaultStream"`
	AvgBitrate    int      `json:"avgBitrate"`
	VideoBitrate  int      `json:"videoBitrate"`
	AudioBitrate  int      `json:"audioBitrate"`
	Duration      int64    `json:"duration"`
	VideoDuration int64    `json:"videoDuration"`
	AudioDuration int64    `json:"audioDuration"`
	VideoCodec    string   `json:"videoCodec"`
	BackupURLs    []string `json:"backupUrls"`
	FPS           int      `json:"fps"`
	Rotate        int      `json:"rotate"`
}

type VideoImage struct {
	ThumbnailFileID  string `json:"thumbnailFileid"`
	FirstFrameFileID string `json:"firstFrameFileid"`
}

type VideoCapa struct {
	Duration int64 `json:"duration"`
}
