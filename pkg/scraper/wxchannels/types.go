package wxchannels

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	utilpkg "wx_channel/pkg/util"
)

var ErrUnsupportedURL = errors.New("unsupported channels url")

// MediaType constants for ChannelsObjectDesc.MediaType and ChannelsMediaItem.MediaType.
const (
	MediaTypePicture = 2 // picture album
	MediaTypeVideo   = 4 // video
	MediaTypeLive    = 9 // live stream
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
	ThumbUrl     string              `json:"thumbUrl"`
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

func (m *ChannelsMediaItem) UnmarshalJSON(data []byte) error {
	type alias ChannelsMediaItem
	aux := &struct {
		DecodeKey flexibleString `json:"decodeKey"`
		*alias
	}{
		alias: (*alias)(m),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	m.DecodeKey = string(aux.DecodeKey)
	return nil
}

type ChannelsMusicInfo struct {
	DocId             string `json:"docId"`
	DocType           int    `json:"docType"`
	Name              string `json:"name"`
	Artist            string `json:"artist"`
	MediaStreamingUrl string `json:"mediaStreamingUrl"`
}

func (m *ChannelsMusicInfo) UnmarshalJSON(data []byte) error {
	type alias ChannelsMusicInfo
	aux := &struct {
		DocId flexibleString `json:"docId"`
		*alias
	}{
		alias: (*alias)(m),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	m.DocId = string(aux.DocId)
	return nil
}

type ChannelsFollowPostInfo struct {
	MusicInfo ChannelsMusicInfo `json:"musicInfo"`
	HasBgm    int               `json:"hasBgm"`
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

func (i *ChannelsLiveInfo) UnmarshalJSON(data []byte) error {
	type alias ChannelsLiveInfo
	aux := &struct {
		AnchorStatusFlag flexibleString `json:"anchorStatusFlag"`
		*alias
	}{
		alias: (*alias)(i),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	i.AnchorStatusFlag = string(aux.AnchorStatusFlag)
	return nil
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
	Description    string                 `json:"description"`
	Media          []ChannelsMediaItem    `json:"media"`
	MediaType      int                    `json:"mediaType"`
	FollowPostInfo ChannelsFollowPostInfo `json:"followPostInfo"`
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

func (o *ChannelsObject) UnmarshalJSON(data []byte) error {
	type alias ChannelsObject
	aux := &struct {
		ID flexibleString `json:"id"`
		*alias
	}{
		alias: (*alias)(o),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	o.ID = string(aux.ID)
	return nil
}

type flexibleString string

func (s *flexibleString) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*s = ""
		return nil
	}
	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*s = flexibleString(value)
		return nil
	}
	*s = flexibleString(raw)
	return nil
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
		Query      string `json:"query"`
		Scene      int    `json:"scene"`
		LastBuffer string `json:"lastBuff"`
		RequestId  string `json:"requestId"`
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

type ChannelsSharedFeedProfileResp struct {
	Data    SharedFeedProfileData `json:"data"`
	Errcode int                   `json:"errCode"`
	Errmsg  string                `json:"errMsg"`
}
type SharedFeedProfileData struct {
	Authorinfo SharedFeedAuthorinfo `json:"authorInfo"`
	Feedinfo   SharedFeedinfo       `json:"feedInfo"`
	Errmsg     Errmsg               `json:"errMsg"`
	Sceneinfo  SharedFeedSceneinfo  `json:"sceneInfo"`
}
type SharedFeedSceneinfo struct {
	Dynamicexportid string `json:"dynamicExportId"`
	Commentscene    int    `json:"commentScene"`
	Expiredtime     int    `json:"expiredTime"`
	Requestscene    int    `json:"requestScene"`
	Entryscene      int    `json:"entryScene"`
	Entrycardtype   int    `json:"entryCardType"`
}
type Errmsg struct {
	Type int `json:"type"`
}
type SharedFeedinfo struct {
	Picinfo         []SharedFeedPicInfo `json:"picInfo"`
	VideoURL        string              `json:"videoUrl"`
	H264VideoInfo   SharedFeedVideoInfo `json:"h264VideoInfo"`
	H265VideoInfo   SharedFeedVideoInfo `json:"h265VideoInfo"`
	Description     string              `json:"description"`
	MediaType       int                 `json:"mediaType"`
	Favcountfmt     string              `json:"favCountFmt"`
	Likecountfmt    string              `json:"likeCountFmt"`
	Forwardcountfmt string              `json:"forwardCountFmt"`
	Commentcountfmt string              `json:"commentCountFmt"`
	Bgminfo         SharedFeedBGMInfo   `json:"bgmInfo"`
	Createtime      int                 `json:"createtime"`
	Ishardad        bool                `json:"isHardAd"`
	Coverurl        string              `json:"coverUrl"`
}

type SharedFeedVideoInfo struct {
	VideoURL string `json:"videoUrl"`
}

type SharedFeedPicInfo struct {
	URL      string  `json:"url"`
	Width    float32 `json:"width"`
	Height   float32 `json:"height"`
	FileSize int     `json:"fileSize"`
}

type SharedFeedBGMInfo struct {
	BGMURL            string `json:"bgmUrl"`
	MediaStreamingURL string `json:"mediaStreamingUrl"`
	Name              string `json:"name"`
	Artist            string `json:"artist"`
	DocID             string `json:"docId"`
	DocType           int    `json:"docType"`
}
type SharedFeedAuthorinfo struct {
	Nickname    string `json:"nickname"`
	Headimgurl  string `json:"headImgUrl"`
	Authiconurl string `json:"authIconUrl"`
}

type ChannelsFeedProfileData struct {
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
}

type ChannelsFeedProfileResp struct {
	ErrCode int                     `json:"errCode"`
	ErrMsg  string                  `json:"errMsg"`
	Data    ChannelsFeedProfileData `json:"data"`
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
	Keyword    string `json:"keyword"`
	NextMarker string `json:"next_marker"`
}
type ChannelsFeedListBody struct {
	Username   string `json:"username"`
	NextMarker string `json:"next_marker"`
}
type ChannelsLiveReplayListBody struct {
	Username   string `json:"username"`
	NextMarker string `json:"next_marker"`
}
type ChannelsLiveInfoBody struct {
	Username      string `json:"username"`
	ObjectId      string `json:"oid"`
	ObjectNonceId string `json:"nid"`
	LiveId        string `json:"id"`
}
type ChannelsInteractionedFeedListBody struct {
	Flag       string `json:"flag"`
	NextMarker string `json:"next_marker"`
}
type ChannelsFeedProfileBody struct {
	URL               string `json:"url"`
	ObjectId          string `json:"oid"`
	NonceId           string `json:"nid"`
	EncryptedObjectId string `json:"eid"`
}
type ChannelsSharedFeedProfileBody struct {
	URL string `json:"url"`
}
type RequestResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

type ChannelsFeedCommentListBody struct {
	ObjectId      string `json:"oid"`
	ObjectNonceId string `json:"nid"`
	CommentId     string `json:"comment_id"`
	NextMarker    string `json:"next_marker"`
}

type ChannelsFeedCommentListResp struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	Data    struct {
		Baseresponse      Baseresponse           `json:"BaseResponse"`
		Commentinfo       []FeedCommentInfo      `json:"commentInfo"` // comment list
		Countinfo         FeedCommentCountInfo   `json:"countInfo"`
		Lastbuffer        string                 `json:"lastBuffer"` // pagination cursor
		Upcontinueflag    int                    `json:"upContinueFlag"`
		Downcontinueflag  int                    `json:"downContinueFlag"`
		Monotonicdata     Monotonicdata          `json:"monotonicData"`
		Newlifeinfo       FeedCommentNewlifeinfo `json:"newlifeInfo"`
		Requestid         int                    `json:"requestId"`
		Emojidesccomments []interface{}          `json:"emojiDescComments"`
		Desccomments      []interface{}          `json:"descComments"`
		Payload           struct {
			FinderBasereq struct {
				Scene   int `json:"scene"`
				CtxInfo struct {
					ClientReportBuff string `json:"clientReportBuff"`
				} `json:"ctxInfo"`
				ObjectBaseInfos []struct {
					SessionBuffer string `json:"sessionBuffer"`
				} `json:"objectBaseInfos"`
			} `json:"finderBasereq"`
			ObjectId       string `json:"objectId"`
			ObjectNonceId  string `json:"objectNonceId"`
			Direction      int    `json:"direction"`
			IdentityScene  int    `json:"identityScene"`
			LastBuffer     string `json:"lastBuffer"`
			EnterSessionId string `json:"enterSessionId"`
		} `json:"payload"`
	} `json:"data"`
}
type FeedCommentNewlifeinfo struct {
	Commentflag int `json:"commentFlag"`
}
type Monotonicdata struct {
	Countinfo    FeedCommentCountInfo `json:"countInfo"`
	Commentcount FeedCommentCount     `json:"commentCount"`
}
type FeedCommentCount struct {
	Commentcount      int         `json:"commentCount"`
	Imagecommentcount int         `json:"imageCommentCount"`
	Versiondata       Versiondata `json:"versionData"`
}
type FeedCommentCountInfo struct {
	Commentcount int         `json:"commentCount"` // total comments
	Likecount    int         `json:"likeCount"`    // total likes
	Forwardcount int         `json:"forwardCount"` // total forwards
	Favcount     int         `json:"favCount"`     // total favorites
	Versiondata  Versiondata `json:"versionData"`
}
type Versiondata struct {
	Dataversion int `json:"dataVersion"`
}
type FeedCommentInfo struct {
	Username             string        `json:"username"`
	Nickname             string        `json:"nickname"`  // commenter nickname
	Content              string        `json:"content"`   // comment content
	Commentid            string        `json:"commentId"` // comment id
	Replycommentid       string        `json:"replyCommentId"`
	Headurl              string        `json:"headUrl"` // commenter avatar
	Leveltwocomment      []interface{} `json:"levelTwoComment"`
	Createtime           string        `json:"createtime"`
	Likeflag             int           `json:"likeFlag"`
	Likecount            int           `json:"likeCount"` // likes for this comment
	Expandcommentcount   int           `json:"expandCommentCount"`
	Lastbuffer           string        `json:"lastBuffer"`
	Continueflag         int           `json:"continueFlag"`
	Displayflag          int           `json:"displayFlag"`
	Replycontent         string        `json:"replyContent"`
	Upcontinueflag       int           `json:"upContinueFlag"`
	Extflag              int           `json:"extFlag"`
	Authorcontact        Authorcontact `json:"authorContact"` // commenter info
	Contenttype          int           `json:"contentType"`   // comment content type
	Reportjson           string        `json:"reportJson"`
	Dislikecount         int           `json:"dislikeCount"`
	Ipregioninfo         Ipregioninfo  `json:"ipRegionInfo"`
	Searchkeywordinfo    []interface{} `json:"searchKeywordInfo"`
	Mentioneduserinfo    []interface{} `json:"mentionedUserInfo"`
	Interactionlabellist []interface{} `json:"interactionLabelList"`
}
type Ipregioninfo struct {
	Regiontext string `json:"regionText"`
}
type Authorcontact struct {
	Username      string        `json:"username"`
	Nickname      string        `json:"nickname"`
	Headurl       string        `json:"headUrl"`
	Bindinfo      []interface{} `json:"bindInfo"`
	Menu          []interface{} `json:"menu"`
	Referenceinfo []interface{} `json:"referenceInfo"`
}
type Baseresponse struct {
	Ret    int    `json:"Ret"`
	Errmsg Errmsg `json:"ErrMsg"`
}

type ChannelsFollowListBody struct {
	NextMarker string `json:"next_marker"`
}

type ChannelsPlayHistoryBody struct {
	NextMarker string `json:"next_marker"`
}

type ChannelsFeedShareUrlResp struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	Data    struct {
		Baseresponse Baseresponse  `json:"BaseResponse"`
		FeedH5Url    string        `json:"feedH5Url"`
		UrlList      []interface{} `json:"urlList"`
	} `json:"data"`
	Payload struct {
		ObjectId string `json:"objectId"`
	} `json:"payload"`
}

type ChannelsFeedShareUrlBody struct {
	ObjectId string `json:"oid"`
}

type ChannelsFollowReferenceInfo struct {
	Type   int    `json:"type"`
	Name   string `json:"name"`
	Status int    `json:"status"`
}

type ChannelsFollowLiveReplaySetting struct {
	CanUseIntelligentlyGenReplayHighlight bool `json:"canUseIntelligentlyGenReplayHighlight"`
}

type ChannelsFollowLiveInfo struct {
	AnchorStatusFlag string                           `json:"anchorStatusFlag"`
	SwitchFlag       int                              `json:"switchFlag"`
	SourceType       int                              `json:"sourceType"`
	MicSetting       *ChannelsLiveMicSetting          `json:"micSetting,omitempty"`
	LotterySetting   map[string]any                   `json:"lotterySetting"`
	LiveCoverImgs    []any                            `json:"liveCoverImgs"`
	ReplaySetting    *ChannelsFollowLiveReplaySetting `json:"replaySetting,omitempty"`
}

func (i *ChannelsFollowLiveInfo) UnmarshalJSON(data []byte) error {
	type alias ChannelsFollowLiveInfo
	aux := &struct {
		AnchorStatusFlag flexibleString `json:"anchorStatusFlag"`
		*alias
	}{
		alias: (*alias)(i),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	i.AnchorStatusFlag = string(aux.AnchorStatusFlag)
	return nil
}

type ChannelsFollowContact struct {
	Username        string                        `json:"username"`
	Nickname        string                        `json:"nickname"`
	HeadUrl         string                        `json:"headUrl"`
	Signature       string                        `json:"signature"`
	FollowFlag      int                           `json:"followFlag"`
	FollowTime      int                           `json:"followTime"`
	CoverImgUrl     string                        `json:"coverImgUrl"`
	SpamStatus      int                           `json:"spamStatus"`
	ExtFlag         int                           `json:"extFlag"`
	ExtInfo         ChannelsContactExtInfo        `json:"extInfo"`
	LiveStatus      int                           `json:"liveStatus"`
	LiveCoverImgUrl string                        `json:"liveCoverImgUrl"`
	LiveInfo        ChannelsFollowLiveInfo        `json:"liveInfo"`
	BindInfo        []any                         `json:"bindInfo"`
	Menu            []any                         `json:"menu"`
	Status          string                        `json:"status"`
	AdditionalFlag  string                        `json:"additionalFlag"`
	ReferenceInfo   []ChannelsFollowReferenceInfo `json:"referenceInfo"`
}

type ChannelsFollowListResp struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	Data    struct {
		BaseResponse BaseResponse            `json:"BaseResponse"`
		ContactList  []ChannelsFollowContact `json:"contactList"`
		LastBuffer   string                  `json:"lastBuffer"`
		ContinueFlag int                     `json:"continueFlag"`
		FollowCount  int                     `json:"followCount"`
	} `json:"data"`
	Payload struct {
		LastBuffer string `json:"lastBuffer"`
	} `json:"payload"`
}

type ChannelsPlayHistoryResp struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	Data    struct {
		BaseResponse BaseResponse     `json:"baseresponse"`
		Object       []ChannelsObject `json:"objects"`
		LastBuffer   string           `json:"lastBuffer"`
		ContinueFlag int              `json:"continueFlag"`
		RecentNDays  int              `json:"recentNDays"`
	} `json:"data"`
	Payload struct {
		LastBuffer string `json:"lastBuffer"`
	} `json:"payload"`
}

type FeedURLParts struct {
	URL string
	Oid string
	Nid string
	Eid string
}

type SphURLParts struct {
	URL string
	ID  string
}

type FeedPage struct {
	URL    FeedURLParts
	Resp   *ChannelsFeedProfileResp
	Object ChannelsObject
}

type SphProfile struct {
	ShareURL        string `json:"share_url,omitempty"`
	SphID           string `json:"sph_id,omitempty"`
	ExportID        string `json:"export_id,omitempty"`
	VideoURL        string `json:"video_url,omitempty"`
	OriginVideoURL  string `json:"origin_video_url,omitempty"`
	Description     string `json:"description,omitempty"`
	CoverURL        string `json:"cover_url,omitempty"`
	MediaType       int    `json:"media_type,omitempty"`
	CreateTime      int64  `json:"create_time,omitempty"`
	AuthorNickname  string `json:"author_nickname,omitempty"`
	AuthorAvatarURL string `json:"author_avatar_url,omitempty"`
	ErrCode         int    `json:"err_code,omitempty"`
	ErrMsg          string `json:"err_msg,omitempty"`
}

// JoinLivePayload is the structure used for joinLive response detection.
// The frontend merges joinLive data with feed profile info before sending.
type JoinLivePayload struct {
	LiveSdkInfo *struct {
		LiveCdnUrl string `json:"liveCdnUrl"`
	} `json:"liveSdkInfo"`
	LiveInfo *struct {
		LiveId    string `json:"liveId"`
		StartTime int    `json:"startTime"`
	} `json:"liveInfo"`
	LiveDescription string           `json:"liveDescription"`
	Nickname        string           `json:"nickname"`
	Username        string           `json:"username"`
	AnchorContact   *ChannelsContact `json:"anchorContact"`
	Contact         *ChannelsContact `json:"contact"`
}

func ParseFeedURL(rawURL string) (*FeedURLParts, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(parsed.Hostname(), "channels.weixin.qq.com") || parsed.EscapedPath() != "/web/pages/feed" {
		return nil, ErrUnsupportedURL
	}
	q := parsed.Query()
	oid := q.Get("oid")
	nid := q.Get("nid")
	if oid != "" {
		if decoded := utilpkg.DecodeBase64ToUint64String(oid); decoded != "" {
			oid = decoded
		}
	}
	if nid != "" {
		if decoded := utilpkg.DecodeBase64ToUint64String(nid); decoded != "" {
			nid = decoded
		}
	}
	return &FeedURLParts{
		URL: rawURL,
		Oid: oid,
		Nid: nid,
		Eid: q.Get("eid"),
	}, nil
}

// ======================== Interceptor Types ========================

type InterceptorMediaSpec struct {
	FileFormat       string  `json:"file_format"`
	FirstLoadBytes   int     `json:"first_load_bytes"`
	BitRate          int     `json:"bit_rate"`
	CodingFormat     string  `json:"coding_format"`
	DynamicRangeType int     `json:"dynamic_range_type"`
	Vfps             float64 `json:"vfps"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	DurationMs       int     `json:"duration_ms"`
	QualityScore     float64 `json:"quality_score"`
	VideoBitrate     int     `json:"video_bitrate"`
	AudioBitrate     int     `json:"audio_bitrate"`
	LevelOrder       int     `json:"level_order"`
	Bypass           string  `json:"bypass"`
	Is3az            int     `json:"is3az"`
}

type InterceptorPicture struct {
	URL string `json:"url"`
}

type InterceptorContact struct {
	Id        string `json:"id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type MediaProfile struct {
	Type     string             `json:"type"` // media | picture | live
	Id       string             `json:"id"`
	NonceId  string             `json:"nonce_id"`
	Title    string             `json:"title"`
	URL      string             `json:"url"`
	Key      string             `json:"key"`
	CoverURL string             `json:"cover_url"`
	Contact  InterceptorContact `json:"contact"`
	Spec     json.RawMessage    `json:"spec"`
	Files    json.RawMessage    `json:"files"`
	Pageurl  string             `json:"pageurl"`
}

// aliases
type ChannelMediaSpec = InterceptorMediaSpec
type ChannelPicture = InterceptorPicture
type ChannelContact = InterceptorContact
type ChannelMediaProfile = MediaProfile

func ParseSphShareURL(rawURL string) (*SphURLParts, error) {
	rawURL = strings.TrimSpace(rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	host := parsed.Hostname()
	path := parsed.EscapedPath()
	var id string
	if strings.EqualFold(host, "weixin.qq.com") && strings.HasPrefix(path, "/sph/") {
		id = strings.Trim(strings.TrimPrefix(parsed.Path, "/sph/"), "/")
	} else if strings.EqualFold(host, "channels.weixin.qq.com") && path == "/finder-preview/pages/sph" {
		id = strings.TrimSpace(parsed.Query().Get("id"))
	} else {
		return nil, ErrUnsupportedURL
	}
	if id == "" {
		return nil, ErrUnsupportedURL
	}
	return &SphURLParts{URL: rawURL, ID: id}, nil
}
