package wxchannels

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"wx_channel/internal/database/model"
	utilpkg "wx_channel/pkg/util"
)

var ErrUnsupportedURL = errors.New("unsupported channels url")

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

type ChannelsMusicInfo struct {
	DocId             string `json:"docId"`
	DocType           int    `json:"docType"`
	Name              string `json:"name"`
	Artist            string `json:"artist"`
	MediaStreamingUrl string `json:"mediaStreamingUrl"`
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
	Picinfo         []interface{} `json:"picInfo"`
	Description     string        `json:"description"`
	Favcountfmt     string        `json:"favCountFmt"`
	Likecountfmt    string        `json:"likeCountFmt"`
	Forwardcountfmt string        `json:"forwardCountFmt"`
	Commentcountfmt string        `json:"commentCountFmt"`
	Createtime      int           `json:"createtime"`
	Ishardad        bool          `json:"isHardAd"`
	Coverurl        string        `json:"coverUrl"`
}
type SharedFeedAuthorinfo struct {
	Nickname    string `json:"nickname"`
	Headimgurl  string `json:"headImgUrl"`
	Authiconurl string `json:"authIconUrl"`
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
		files := feed.Files
		if len(files) == 0 {
			files = feed.ObjectDesc.Media
		}
		if len(files) == 0 {
			return nil, errors.New("picture 类型缺少 files 数据")
		}
		return &ChannelsFeedProfile{
			ObjectId:  feed.ID,
			NonceId:   feed.ObjectNonceId,
			SourceURL: feed.SourceURL,
			URL:       "",
			Title:     buildTitle(feed.ObjectDesc.Description, false),
			CoverURL:  files[0].CoverUrl,
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
		spec := make([]ChannelsMediaSpec, 0)
		if len(media.Spec) > 0 {
			spec = media.Spec
		} else if len(feed.Spec) > 0 {
			spec = feed.Spec
		}
		return &ChannelsFeedProfile{
			ObjectId:    feed.ID,
			NonceId:     feed.ObjectNonceId,
			SourceURL:   feed.SourceURL,
			URL:         media.URL + media.URLToken,
			Title:       buildTitle(feed.ObjectDesc.Description, false),
			DecryptKey:  media.DecodeKey,
			CoverURL:    media.CoverUrl,
			CoverWidth:  int(media.Width),
			CoverHeight: int(media.Height),
			Duration:    media.VideoPlayLen,
			FileSize:    media.FileSize,
			CreatedAt:   feed.CreateTime,
			Spec:        spec,
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
	return prof, nil
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
		Commentinfo       []FeedCommentInfo      `json:"commentInfo"` // 评论列表
		Countinfo         FeedCommentCountInfo   `json:"countInfo"`
		Lastbuffer        string                 `json:"lastBuffer"` // 分页参数
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
	Commentcount int         `json:"commentCount"` // 评论总数
	Likecount    int         `json:"likeCount"`    // 点赞总数
	Forwardcount int         `json:"forwardCount"` // 转发总数
	Favcount     int         `json:"favCount"`     // 喜欢总数
	Versiondata  Versiondata `json:"versionData"`
}
type Versiondata struct {
	Dataversion int `json:"dataVersion"`
}
type FeedCommentInfo struct {
	Username             string        `json:"username"`
	Nickname             string        `json:"nickname"`  // 评论人昵称
	Content              string        `json:"content"`   // 评论内容
	Commentid            string        `json:"commentId"` // 评论id
	Replycommentid       string        `json:"replyCommentId"`
	Headurl              string        `json:"headUrl"` // 评论人头像
	Leveltwocomment      []interface{} `json:"levelTwoComment"`
	Createtime           string        `json:"createtime"`
	Likeflag             int           `json:"likeFlag"`
	Likecount            int           `json:"likeCount"` // 该评论点赞数
	Expandcommentcount   int           `json:"expandCommentCount"`
	Lastbuffer           string        `json:"lastBuffer"`
	Continueflag         int           `json:"continueFlag"`
	Displayflag          int           `json:"displayFlag"`
	Replycontent         string        `json:"replyContent"`
	Upcontinueflag       int           `json:"upContinueFlag"`
	Extflag              int           `json:"extFlag"`
	Authorcontact        Authorcontact `json:"authorContact"` // 评论人信息
	Contenttype          int           `json:"contentType"`   // 评论内容类型
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
	URL     FeedURLParts
	Resp    *ChannelsFeedProfileResp
	Object  ChannelsObject
	Profile ChannelsFeedProfile
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

func BuildJumpURL(feed *ChannelsFeedProfile) string {
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
		u += "?username=" + url.QueryEscape(username)
	} else {
		u += "?"
	}

	if oid != "" {
		encodedOid := utilpkg.EncodeUint64ToBase64(oid)
		if encodedOid != "" {
			u += "&oid=" + url.QueryEscape(encodedOid)
		}
	}

	if nid != "" {
		encodedNid := utilpkg.EncodeUint64ToBase64(nid)
		if encodedNid != "" {
			u += "&nid=" + url.QueryEscape(encodedNid)
		}
	}

	return strings.TrimSuffix(strings.Replace(u, "?&", "?", 1), "?")
}

func BuildJumpUrl(feed *ChannelsFeedProfile) string {
	return BuildJumpURL(feed)
}

// ToAccount converts ChannelsObject to a model.Account.
func (r *ChannelsObject) ToAccount() (*model.Account, error) {
	profile, err := ChannelsObjectToChannelsFeedProfile(r)
	if err != nil {
		return nil, err
	}

	contact := profile.Contact
	now := utilpkg.NowMillis()
	return &model.Account{
		Id:         BuildAccountID(contact.Username),
		PlatformId: PlatformID,
		ExternalId: contact.Username,
		Username:   contact.Username,
		Nickname:   contact.Nickname,
		AvatarURL:  contact.AvatarURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ToContent converts ChannelsObject to a model.Content.
func (r *ChannelsObject) ToContent() (*model.Content, error) {
	profile, err := ChannelsObjectToChannelsFeedProfile(r)
	if err != nil {
		return nil, err
	}

	now := utilpkg.NowMillis()

	// Determine content type from raw object fields
	contentType := "video"
	if r.LiveInfo != nil {
		contentType = "live"
	} else if r.Type == "picture" || r.ObjectDesc.MediaType == 2 {
		contentType = "picture"
	}

	pub := int64(profile.CreatedAt)

	return &model.Content{
		Id:          BuildContentID(profile.ObjectId),
		PlatformId:  PlatformID,
		ContentType: contentType,
		Title:       profile.Title,
		Description: profile.Title,
		ExternalId:  profile.ObjectId,
		ExternalId2: profile.NonceId,
		ExternalId3: profile.DecryptKey,
		SourceURL:   profile.SourceURL,
		ContentURL:  profile.URL,
		URL:         profile.URL,
		CoverURL:    profile.CoverURL,
		CoverWidth:  strconv.Itoa(profile.CoverWidth),
		CoverHeight: strconv.Itoa(profile.CoverHeight),
		Duration:    int64(profile.Duration),
		Size:        int64(profile.FileSize),
		PublishTime: &pub,
		Metadata:    fmt.Sprintf(`{"key":"%s"}`, profile.DecryptKey),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

const PlatformID = "wx_channels"

// BuildContentID 构建 content 表主键 ID
func BuildContentID(externalID string) string {
	return PlatformID + ":" + externalID
}

// BuildAccountID 构建 account 表主键 ID
func BuildAccountID(externalID string) string {
	return PlatformID + ":" + externalID
}
