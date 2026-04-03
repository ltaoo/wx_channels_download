package channels

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	utilpkg "wx_channel/pkg/util"
)

type BaseResponse struct {
	Ret    int        `json:"Ret"`
	ErrMsg BaseErrMsg `json:"ErrMsg"`
}

type BaseErrMsg struct {
	String string `json:"String"`
}

type ChannelsCodecInfo struct {
	VideoScore        int  `json:"videoScore"`
	VideoCoverScore   int  `json:"videoCoverScore"`
	VideoAudioScore   int  `json:"videoAudioScore"`
	ThumbScore        int  `json:"thumbScore"`
	HdimgScore        int  `json:"hdimgScore"`
	HasStickers       bool `json:"hasStickers"`
	UseAlgorithmCover bool `json:"useAlgorithmCover"`
}

type ChannelsMediaSpec struct {
	FileFormat string `json:"fileFormat"`
	// FirstLoadBytes   int    `json:"firstLoadBytes"`
	// BitRate          int    `json:"bitRate"`
	// CodingFormat     string `json:"codingFormat"`
	// DynamicRangeType int    `json:"dynamicRangeType"`
	// Vfps             int    `json:"vfps"`
	Width      int `json:"width"`
	Height     int `json:"height"`
	DurationMs int `json:"durationMs"`
	// QualityScore     int    `json:"qualityScore"`
	// VideoBitrate     int    `json:"videoBitrate"`
	// AudioBitrate     int    `json:"audioBitrate"`
	// LevelOrder       int    `json:"levelOrder"`
	// Bypass           string `json:"bypass"`
	// Is3az            int    `json:"is3az"`
}

type ChannelsScalingInfo struct {
	Version             string `json:"version"`
	IsSplitScreen       bool   `json:"isSplitScreen"`
	IsDisableFollow     bool   `json:"isDisableFollow"`
	UpPercentPosition   int    `json:"upPercentPosition"`
	DownPercentPosition int    `json:"downPercentPosition"`
}

type ChannelsMediaCdnInfo struct {
	IsUsePcdn                       bool `json:"isUsePcdn"`
	BeginUsePcdnBufferSeconds       int  `json:"beginUsePcdnBufferSeconds"`
	ExitUsePcdnBufferSeconds        int  `json:"exitUsePcdnBufferSeconds"`
	PreloadBeginUsePcdnBufferKbytes int  `json:"preloadBeginUsePcdnBufferKbytes"`
	PcdnTimeoutRetryCount           int  `json:"pcdnTimeoutRetryCount"`
	MarsPreDownloadKbytes           int  `json:"marsPreDownloadKbytes"`
	IsUseUgcWhenNoPreload           bool `json:"isUseUgcWhenNoPreload"`
}

type ChannelsMediaItem struct {
	URL          string              `json:"url"`
	MediaType    int                 `json:"mediaType"`
	VideoPlayLen int                 `json:"videoPlayLen"`
	Width        int                 `json:"width"`
	Height       int                 `json:"height"`
	FileSize     int                 `json:"fileSize"`
	Spec         []ChannelsMediaSpec `json:"spec"`
	CoverUrl     string              `json:"coverUrl"`
	DecodeKey    string              `json:"decodeKey"`
	URLToken     string              `json:"urlToken"`
}

type ChannelsLocationLang struct {
	Lang     string `json:"lang"`
	Country  string `json:"country"`
	Province string `json:"province"`
	City     string `json:"city"`
	Region   string `json:"region"`
}

type ChannelsLocation struct {
	Longitude     float64                `json:"longitude"`
	Latitude      float64                `json:"latitude"`
	City          string                 `json:"city"`
	PoiClassifyId string                 `json:"poiClassifyId"`
	Country       string                 `json:"country"`
	ProductId     []any                  `json:"productId"`
	MultiLangInfo []ChannelsLocationLang `json:"multiLangInfo"`
	CountryCode   string                 `json:"countryCode"`
}

type ChannelsLiveMicSetting struct {
	SettingFlag       int `json:"settingFlag"`
	SettingSwitchFlag int `json:"settingSwitchFlag"`
}

type ChannelsLiveInfo struct {
	AnchorStatusFlag string                  `json:"anchorStatusFlag"`
	SwitchFlag       int                     `json:"switchFlag"`
	SourceType       int                     `json:"sourceType"`
	MicSetting       *ChannelsLiveMicSetting `json:"micSetting,omitempty"`
	LotterySetting   map[string]any          `json:"lotterySetting"`
	LiveCoverImgs    []any                   `json:"liveCoverImgs"`
}

type ChannelsContactExtInfo struct {
	Sex      int    `json:"sex,omitempty"`
	Country  string `json:"country,omitempty"`
	Province string `json:"province,omitempty"`
	City     string `json:"city,omitempty"`
}

type ChannelsContact struct {
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	HeadUrl     string `json:"headUrl"`
	Signature   string `json:"signature"`
	CoverImgUrl string `json:"coverImgUrl"`
}

type ShortTitle struct {
	ShortTitle string `json:"shortTitle"`
}

type ChannelsObjectDesc struct {
	Description string              `json:"description"`
	Media       []ChannelsMediaItem `json:"media"`
	MediaType   int                 `json:"mediaType"`
}

type InfoListItem struct {
	Contact             ChannelsContact `json:"contact"`
	HighlightNickname   string          `json:"highlightNickname"`
	HighlightSignature  string          `json:"highlightSignature"`
	FriendFollowCount   int             `json:"friendFollowCount"`
	HighlightProfession string          `json:"highlightProfession"`
	ReqIndex            int             `json:"reqIndex"`
}

type ObjectExtend struct {
	FavInfo                    json.RawMessage `json:"favInfo"`
	PreloadConfig              json.RawMessage `json:"preloadConfig"`
	AdvertisementInfo          json.RawMessage `json:"advertisementInfo"`
	MonotonicData              json.RawMessage `json:"monotonicData"`
	PostScene                  int             `json:"postScene"`
	FinderNewlifeInfo          json.RawMessage `json:"finderNewlifeInfo"`
	GlobalfavFlag              int             `json:"globalfavFlag,omitempty"`
	FriendRecommendCommentInfo json.RawMessage `json:"friendRecommendCommentInfo"`
	AdInternalFeedbackUrl      string          `json:"adInternalFeedbackUrl,omitempty"`
	ExportID                   string          `json:"exportId,omitempty"`
	OriginalInfo               json.RawMessage `json:"originalInfo,omitempty"`
}

type IPRegionInfo struct {
	RegionText string `json:"regionText"`
}

type ChannelsObject struct {
	ID            string              `json:"id"`
	Contact       ChannelsContact     `json:"contact"`
	ObjectDesc    ChannelsObjectDesc  `json:"objectDesc"`
	ObjectNonceId string              `json:"objectNonceId"`
	SourceURL     string              `json:"source_url"`
	CreateTime    int                 `json:"createtime"`
	Type          string              `json:"type"`
	Spec          []ChannelsMediaSpec `json:"spec"`
	LiveInfo      *ChannelsLiveInfo   `json:"liveInfo,omitempty"`
	Files         []ChannelsMediaItem `json:"files"`
	AnchorContact *ChannelsContact    `json:"anchorContact,omitempty"`
}

type ChannelsSearchResp struct {
	BaseResponse    BaseResponse     `json:"BaseResponse"`
	InfoList        []InfoListItem   `json:"infoList"`
	ContinueFlag    int              `json:"continueFlag"`
	ObjectList      []ChannelsObject `json:"objectList"`
	LastBuff        string           `json:"lastBuff"`
	TopicInfoList   []any            `json:"topicInfoList"`
	MusicInfoList   []any            `json:"musicInfoList"`
	MultiFeedStream []any            `json:"multiFeedStream"`
}

type ChannelsFeedListOfAccountResp struct {
	BaseResponse   BaseResponse     `json:"BaseResponse"`
	Object         []ChannelsObject `json:"object"`
	FinderUserInfo struct {
		CoverImgUrl string `json:"coverImgUrl"`
	} `json:"finderUserInfo"`
	Contact      ChannelsContact `json:"contact"`
	FeedsCount   int             `json:"feedsCount"`
	ContinueFlag int             `json:"continueFlag"`
	LastBuffer   string          `json:"lastBuffer"`
	UserTags     []string        `json:"userTags"`
	PreloadInfo  json.RawMessage `json:"preloadInfo"`
	LiveObjects  []any           `json:"liveObjects"`
	UsualTopics  []struct {
		Topic   string `json:"topic"`
		TopicId string `json:"topicId"`
	} `json:"usualTopics"`
	LiveDurationHours int   `json:"liveDurationHours"`
	EventInfoList     []any `json:"eventInfoList"`
	JustWatch         struct {
		ShowJustWatch bool `json:"showJustWatch"`
		AllowPrefetch bool `json:"allowPrefetch"`
	} `json:"justWatch"`
	JumpInfo           []any `json:"jumpInfo"`
	DeprecatedClubInfo []any `json:"deprecatedClubInfo"`
	AnchorStat         struct {
		TotalLiveCount  int `json:"totalLiveCount"`
		RecentLiveCount int `json:"recentLiveCount"`
	} `json:"anchorStat"`
	IPRegionInfo *IPRegionInfo `json:"ipRegionInfo,omitempty"`
	OriginalInfo struct {
		OriginalCount int `json:"originalCount"`
	} `json:"originalInfo"`
	LayoutConfig   json.RawMessage `json:"layoutConfig"`
	ProfileBanner  json.RawMessage `json:"profileBanner"`
	ShowInfo       json.RawMessage `json:"showInfo"`
	MemberStatus   int             `json:"memberStatus"`
	UpContinueFlag int             `json:"upContinueFlag"`
	UpLastbuffer   string          `json:"upLastbuffer"`
}

type MediaProfileResp struct {
	BaseResponse          BaseResponse    `json:"BaseResponse"`
	CommentInfo           []any           `json:"commentInfo"`
	Object                ChannelsObject  `json:"object"`
	CommentCount          int             `json:"commentCount"`
	NextCheckObjectStatus int             `json:"nextCheckObjectStatus"`
	BarrageCommentInfo    []any           `json:"barrageCommentInfo"`
	RefObjectList         []any           `json:"refObjectList"`
	PreloadInfo           json.RawMessage `json:"preloadInfo"`
	TraceBuffer           string          `json:"traceBuffer"`
	LiveAliasInfo         []any           `json:"liveAliasInfo"`
	DescCommentInfo       []any           `json:"descCommentInfo"`
	UserTags              []string        `json:"userTags"`
	ReportBypass          string          `json:"reportBypass"`
	Payload               struct {
		NeedObject        int    `json:"needObject"`
		LastBuffer        string `json:"lastBuffer"`
		Scene             int    `json:"scene"`
		Direction         int    `json:"direction"`
		IdentityScene     int    `json:"identityScene"`
		PullScene         int    `json:"pullScene"`
		ObjectID          string `json:"objectid"`
		ObjectNonceId     string `json:"objectNonceId"`
		EncryptedObjectID string `json:"encrypted_objectid"`
	} `json:"payload"`
}

type ChannelsAccountSearchResultResp struct {
	ErrCode int                         `json:"err_code"`
	ErrMsg  string                      `json:"err_msg"`
	Data    ChannelsAccountSearchResult `json:"data"`
}
type ChannelsAccountSearchResult struct {
}

type ChannelsAccountSearchBody struct {
	Keyword string `json:"keyword"`
}
type ChannelsFeedListBody struct {
	Username   string `json:"username"`
	NextMarker string `json:"next_marker"`
}
type ChannelsFeedProfileBody struct {
	URL      string `json:"url"`
	ObjectId string `json:"oid"`
	NonceId  string `json:"nid"`
}
type ChannelsFeedListResp struct {
	ErrCode int              `json:"err_code"`
	ErrMsg  string           `json:"err_msg"`
	Data    ChannelsFeedList `json:"data"`
}
type ChannelsFeedList struct {
	List       []ChannelsFeedProfile `json:"list"`
	NextMarker string                `json:"next_marker"`
}

type ChannelsFeedAccount struct {
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}
type ChannelsFeedProfile struct {
	ObjectId    string              `json:"id"`
	NonceId     string              `json:"nonce_id"`
	SourceURL   string              `json:"source_url"`
	URL         string              `json:"url"`
	Title       string              `json:"title"`
	DecryptKey  string              `json:"decrypt_key"`
	CoverURL    string              `json:"cover_url"`
	CoverWidth  int                 `json:"cover_width"`
	CoverHeight int                 `json:"cover_height"`
	Duration    int                 `json:"duration"`
	FileSize    int                 `json:"file_size"`
	CreatedAt   int                 `json:"created_at"`
	Spec        []ChannelsMediaSpec `json:"spec"`
	Contact     ChannelsFeedAccount `json:"contact"`
}

type ChannelsContactSearchResp struct {
	ErrCode int                 `json:"errCode"`
	ErrMsg  string              `json:"errMsg"`
	Data    ChannelsFeedProfile `json:"data"`
}

type ChannelsFeedProfileResp struct {
	ErrCode int                 `json:"errCode"`
	ErrMsg  string              `json:"errMsg"`
	Data    ChannelsFeedProfile `json:"data"`
}
type RequestResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func ChannelsObjectToChannelsFeedProfile(r *ChannelsObject) (*ChannelsFeedProfile, error) {
	if r == nil {
		return nil, errors.New("channels object 为空")
	}
	feed := r

	if strings.TrimSpace(feed.ID) == "" {
		return nil, errors.New("缺少 id 字段")
	}

	// 标题处理：优先使用 Description，其次 ID，最后使用时间戳
	buildTitle := func(description string, isLive bool) string {
		if isLive {
			return "直播"
		}
		if strings.TrimSpace(description) != "" {
			return description
		}
		if strings.TrimSpace(feed.ID) != "" {
			return feed.ID
		}
		return strconv.FormatInt(time.Now().Unix(), 10)
	}

	if feed.LiveInfo != nil {
		coverURL := ""
		if feed.AnchorContact != nil {
			coverURL = feed.AnchorContact.CoverImgUrl
		} else if len(feed.ObjectDesc.Media) > 0 && feed.ObjectDesc.Media[0].CoverUrl != "" {
			coverURL = feed.ObjectDesc.Media[0].CoverUrl
		}
		contact := ChannelsFeedAccount{}
		if feed.AnchorContact != nil {
			contact = ChannelsFeedAccount{
				Username:  feed.AnchorContact.Username,
				Nickname:  feed.AnchorContact.Nickname,
				AvatarURL: feed.AnchorContact.HeadUrl,
			}
		} else {
			contact = ChannelsFeedAccount{
				Username:  feed.Contact.Username,
				Nickname:  feed.Contact.Nickname,
				AvatarURL: feed.Contact.HeadUrl,
			}
		}
		return &ChannelsFeedProfile{
			ObjectId:  feed.ID,
			NonceId:   feed.ObjectNonceId,
			SourceURL: feed.SourceURL,
			URL:       "",
			Title:     buildTitle(feed.ObjectDesc.Description, true),
			Contact:   contact,
			CoverURL:  coverURL,
			CreatedAt: feed.CreateTime,
		}, nil
	}

	if feed.Type == "picture" || feed.ObjectDesc.MediaType == 2 {
		if len(feed.Files) == 0 {
			return nil, errors.New("picture 类型缺少 files 数据")
		}
		return &ChannelsFeedProfile{
			ObjectId:  feed.ID,
			NonceId:   feed.ObjectNonceId,
			SourceURL: feed.SourceURL,
			URL:       "",
			Title:     buildTitle(feed.ObjectDesc.Description, false),
			CoverURL:  feed.Files[0].CoverUrl,
			CreatedAt: feed.CreateTime,
			Contact: ChannelsFeedAccount{
				Username:  feed.Contact.Username,
				Nickname:  feed.Contact.Nickname,
				AvatarURL: feed.Contact.HeadUrl,
			},
		}, nil
	}

	if feed.Type == "media" || feed.ObjectDesc.MediaType == 4 {
		if len(feed.ObjectDesc.Media) == 0 {
			return nil, errors.New("media 类型缺少 media 数据")
		}
		media := feed.ObjectDesc.Media[0]
		return &ChannelsFeedProfile{
			ObjectId:    feed.ID,
			NonceId:     feed.ObjectNonceId,
			SourceURL:   feed.SourceURL,
			URL:         media.URL + media.URLToken,
			Title:       buildTitle(feed.ObjectDesc.Description, false),
			DecryptKey:  media.DecodeKey,
			CoverURL:    media.CoverUrl,
			CoverWidth:  media.Width,
			CoverHeight: media.Height,
			Duration:    media.VideoPlayLen,
			FileSize:    media.FileSize,
			CreatedAt:   feed.CreateTime,
			Spec:        media.Spec,
			Contact: ChannelsFeedAccount{
				Username:  feed.Contact.Username,
				Nickname:  feed.Contact.Nickname,
				AvatarURL: feed.Contact.HeadUrl,
			},
		}, nil
	}

	if feed.ObjectDesc.MediaType == 9 {
		return nil, errors.New("不支持直播回放（mediaType=9）")
	}

	if len(feed.ObjectDesc.Media) == 0 {
		return nil, errors.New("objectDesc.media 为空")
	}
	media := feed.ObjectDesc.Media[0]
	prof := &ChannelsFeedProfile{
		ObjectId:    feed.ID,
		NonceId:     feed.ObjectNonceId,
		SourceURL:   feed.SourceURL,
		URL:         media.URL + media.URLToken,
		Title:       buildTitle(feed.ObjectDesc.Description, false),
		DecryptKey:  media.DecodeKey,
		CoverURL:    media.CoverUrl,
		CoverWidth:  media.Width,
		CoverHeight: media.Height,
		Duration:    media.VideoPlayLen,
		FileSize:    media.FileSize,
		CreatedAt:   feed.CreateTime,
		Spec:        media.Spec,
		Contact: ChannelsFeedAccount{
			Username:  feed.Contact.Username,
			Nickname:  feed.Contact.Nickname,
			AvatarURL: feed.Contact.HeadUrl,
		},
	}
	return prof, nil
}

func BuildJumpUrl(feed *ChannelsFeedProfile) string {
	origin := "https://channels.weixin.qq.com"
	if feed == nil {
		return origin + "/web/pages/feed"
	}

	if feed.SourceURL != "" {
		return feed.SourceURL
	}

	oid := feed.ObjectId
	nid := feed.NonceId

	username := ""
	if feed.Contact.Username != "" {
		username = feed.Contact.Username
	}

	u := origin + "/web/pages/feed"
	if username != "" {
		u += "?username=" + username
	} else {
		u += "?"
	}

	if oid != "" {
		encodedOid := utilpkg.EncodeUint64ToBase64(oid)
		if encodedOid != "" {
			u += "&oid=" + encodedOid
		}
	}

	if nid != "" {
		encodedNid := utilpkg.EncodeUint64ToBase64(nid)
		if encodedNid != "" {
			u += "&nid=" + encodedNid
		}
	}

	return strings.TrimPrefix(u, "?")
}
