package types

import (
	"encoding/json"
)

type ChannelsRequestResp[T any] struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	Data    T      `json:"data"`
}

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
	Width      float32 `json:"width"`
	Height     float32 `json:"height"`
	DurationMs int     `json:"durationMs"`
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
	Width        float32             `json:"width"`
	Height       float32             `json:"height"`
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
	ID            string             `json:"id"`
	Contact       ChannelsContact    `json:"contact"`
	ObjectDesc    ChannelsObjectDesc `json:"objectDesc"`
	ObjectNonceId string             `json:"objectNonceId"`
	SourceURL     string             `json:"source_url"`
	CreateTime    int                `json:"createtime"`
}

type ChannelsContactSearchResp struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	Data    struct {
		BaseResponse    BaseResponse   `json:"BaseResponse"`
		InfoList        []InfoListItem `json:"infoList"`
		ContinueFlag    int            `json:"continueFlag"`
		LastBuff        string         `json:"lastBuff"`
		TopicInfoList   []any          `json:"topicInfoList"`
		MusicInfoList   []any          `json:"musicInfoList"`
		MultiFeedStream []any          `json:"multiFeedStream"`
		// ObjectList      []ChannelsObject `json:"objectList"`
	} `json:"data"`
	Payload struct {
		Query     string `json:"query"`
		Scene     int    `json:"scene"`
		RequestId string `json:"requestId"`
	} `json:"payload"`
}

type ChannelsFeedListOfAccountResp struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	Data    struct {
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
	} `json:"data"`
	Payload struct {
		Username       string `json:"username"`
		FinderUsername string `json:"finderUsername"`
		LastBuffer     string `json:"lastBuffer"`
		NeedFansCount  int    `json:"needFansCount"`
		ObjectId       string `json:"objectId"`
	} `json:"payload"`
}

type ChannelsFeedProfileResp struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	Data    struct {
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
	} `json:"data"`
	Payload struct {
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

type ChannelsAccountSearchBody struct {
	Keyword string `json:"keyword"`
}
type ChannelsFeedListBody struct {
	Username   string `json:"username"`
	NextMarker string `json:"next_marker"`
}
type ChannelsLiveReplayListBody struct {
	Username   string `json:"username"`
	NextMarker string `json:"next_marker"`
}
type ChannelsInteractionedFeedListBody struct {
	Flag       string `json:"flag"`
	NextMarker string `json:"next_marker"`
}
type ChannelsFeedProfileBody struct {
	URL      string `json:"url"`
	ObjectId string `json:"oid"`
	NonceId  string `json:"nid"`
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
	Contact     ChannelsFeedAccount `json:"contact"`
}

type RequestResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func ChannelsObjectToChannelsFeedProfile(r *ChannelsObject) *ChannelsFeedProfile {
	if r == nil {
		return nil
	}
	feed := r
	if len(feed.ObjectDesc.Media) == 0 {
		return nil
	}
	media := feed.ObjectDesc.Media[0]
	// file_size, _ := strconv.Atoi(media.FileSize)
	prof := &ChannelsFeedProfile{
		ObjectId:    feed.ID,
		NonceId:     feed.ObjectNonceId,
		SourceURL:   feed.SourceURL,
		URL:         media.URL + media.URLToken,
		Title:       feed.ObjectDesc.Description,
		DecryptKey:  media.DecodeKey,
		CoverURL:    media.CoverUrl,
		CoverWidth:  int(media.Width),
		CoverHeight: int(media.Height),
		Duration:    media.VideoPlayLen,
		FileSize:    media.FileSize,
		CreatedAt:   feed.CreateTime,
		Contact: ChannelsFeedAccount{
			Username:  feed.Contact.Username,
			Nickname:  feed.Contact.Nickname,
			AvatarURL: feed.Contact.HeadUrl,
		},
	}
	return prof
}
