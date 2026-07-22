package wxmp

import (
	"encoding/xml"
	"html"
	"strings"
	"unicode"
)

type AtomAuthor struct {
	Name string `xml:"name"`
	URI  string `xml:"uri"`
}
type AtomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}
type AtomContent struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}
type MediaThumbnail struct {
	XMLName    xml.Name `xml:"media:thumbnail"`
	XMLNSMedia string   `xml:"xmlns:media,attr"`
	URL        string   `xml:"url,attr"`
	Width      int      `xml:"width,attr,omitempty"`
	Height     int      `xml:"height,attr,omitempty"`
}
type AtomEntry struct {
	ID             string          `xml:"id"`
	Title          string          `xml:"title"`
	Updated        string          `xml:"updated"`
	Published      string          `xml:"published"`
	Author         AtomAuthor      `xml:"author"`
	Link           []AtomLink      `xml:"link"`
	Content        AtomContent     `xml:"content"`
	Summary        AtomContent     `xml:"summary"`
	MediaThumbnail *MediaThumbnail `xml:"media:thumbnail"`
}
type AtomCategory struct {
	Term string `xml:"term,attr"`
}
type AtomFeed struct {
	XMLName   xml.Name       `xml:"http://www.w3.org/2005/Atom feed"`
	Title     string         `xml:"title"`
	ID        string         `xml:"id"`
	Updated   string         `xml:"updated"`
	Generator string         `xml:"generator"`
	Icon      string         `xml:"icon"`
	Category  []AtomCategory `xml:"category"`
	Link      []AtomLink     `xml:"link"`
	Author    AtomAuthor     `xml:"author"`
	Entry     []AtomEntry    `xml:"entry"`
}

type officialAccountArticleVariable struct {
	Title string `json:"title,omitempty"`
}

type officialAccountPublisherVariable struct {
	AvatarURL string `json:"avatar_url,omitempty"`
	Nickname  string `json:"nickname,omitempty"`
	Biz       string `json:"biz,omitempty"`
	Username  string `json:"username,omitempty"`
}

type officialAccountPageVariable struct {
	Publisher officialAccountPublisherVariable `json:"publisher,omitempty"`
	Article   officialAccountArticleVariable   `json:"article,omitempty"`
}

func buildOfficialAccountVariables(htmlText string) map[string]interface{} {
	page := extractOfficialAccountPageVariable(htmlText)
	variables := map[string]interface{}{}
	if page.Publisher.AvatarURL == "" &&
		page.Publisher.Nickname == "" &&
		page.Publisher.Biz == "" &&
		page.Publisher.Username == "" &&
		page.Article.Title == "" {
		return variables
	}
	variables["officialAccount"] = page
	return variables
}

func extractOfficialAccountPageVariable(htmlText string) officialAccountPageVariable {
	block, ok := extractWindowObjectBlock(htmlText, "cgiDataNew")
	if !ok {
		return officialAccountPageVariable{}
	}

	nickname := decodeWechatJSString(topLevelStringProperty(block, "nick_name"))
	avatarURL := firstNonEmpty(
		decodeWechatJSString(topLevelStringProperty(block, "round_head_img")),
		decodeWechatJSString(topLevelStringProperty(block, "ori_head_img_url")),
		decodeWechatJSString(topLevelStringProperty(block, "hd_head_img")),
	)

	return officialAccountPageVariable{
		Publisher: officialAccountPublisherVariable{
			AvatarURL: avatarURL,
			Nickname:  nickname,
			Biz:       decodeWechatJSString(topLevelStringProperty(block, "bizuin")),
			Username:  decodeWechatJSString(topLevelStringProperty(block, "user_name")),
		},
		Article: officialAccountArticleVariable{
			Title: decodeWechatJSString(topLevelStringProperty(block, "title")),
		},
	}
}

func extractWindowObjectBlock(source string, name string) (string, bool) {
	marker := "window." + name
	idx := strings.Index(source, marker)
	if idx < 0 {
		return "", false
	}
	rest := source[idx+len(marker):]
	assignIdx := strings.Index(rest, "=")
	if assignIdx < 0 {
		return "", false
	}
	rest = rest[assignIdx+1:]
	startRel := strings.Index(rest, "{")
	if startRel < 0 {
		return "", false
	}
	start := idx + len(marker) + assignIdx + 1 + startRel

	var quote rune
	escaped := false
	depth := 0
	for i, r := range source[start:] {
		pos := start + i
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '{' {
			depth++
			continue
		}
		if r == '}' {
			depth--
			if depth == 0 {
				return source[start+1 : pos], true
			}
		}
	}

	return "", false
}

func topLevelStringProperty(objectBody string, key string) string {
	var quote rune
	escaped := false
	depth := 0
	for i := 0; i < len(objectBody); i++ {
		ch := rune(objectBody[i])
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		switch ch {
		case '{', '[':
			depth++
			continue
		case '}', ']':
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth != 0 || !isIdentStart(ch) {
			continue
		}
		end := i + 1
		for end < len(objectBody) && isIdentPart(rune(objectBody[end])) {
			end++
		}
		if objectBody[i:end] != key {
			i = end - 1
			continue
		}
		pos := skipSpace(objectBody, end)
		if pos >= len(objectBody) || objectBody[pos] != ':' {
			i = end - 1
			continue
		}
		return readStringLikeExpression(objectBody, skipSpace(objectBody, pos+1))
	}
	return ""
}

func readStringLikeExpression(source string, start int) string {
	if start >= len(source) {
		return ""
	}
	if strings.HasPrefix(source[start:], "htmlDecode(") {
		innerStart := skipSpace(source, start+len("htmlDecode("))
		return readQuotedStringLiteral(source, innerStart)
	}
	return readQuotedStringLiteral(source, start)
}

func readQuotedStringLiteral(source string, start int) string {
	if start >= len(source) || (source[start] != '\'' && source[start] != '"') {
		return ""
	}
	quote := source[start]
	escaped := false
	for i := start + 1; i < len(source); i++ {
		if escaped {
			escaped = false
			continue
		}
		if source[i] == '\\' {
			escaped = true
			continue
		}
		if source[i] == quote {
			return source[start : i+1]
		}
	}
	return ""
}

func decodeWechatJSString(literal string) string {
	if len(literal) < 2 {
		return ""
	}
	quote := literal[0]
	if quote != '\'' && quote != '"' {
		return ""
	}
	var b strings.Builder
	for i := 1; i < len(literal)-1; i++ {
		ch := literal[i]
		if ch != '\\' || i+1 >= len(literal)-1 {
			b.WriteByte(ch)
			continue
		}
		i++
		switch literal[i] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '0':
			b.WriteByte(0)
		case 'x':
			if i+2 < len(literal)-1 {
				if value, ok := parseHexByte(literal[i+1 : i+3]); ok {
					b.WriteByte(value)
					i += 2
					break
				}
			}
			b.WriteByte('x')
		case '\\', '\'', '"':
			b.WriteByte(literal[i])
		default:
			b.WriteByte(literal[i])
		}
	}
	return strings.ReplaceAll(html.UnescapeString(b.String()), "\u00a0", " ")
}

func parseHexByte(s string) (byte, bool) {
	if len(s) != 2 {
		return 0, false
	}
	hi, ok := hexValue(s[0])
	if !ok {
		return 0, false
	}
	lo, ok := hexValue(s[1])
	if !ok {
		return 0, false
	}
	return hi<<4 | lo, true
}

func hexValue(ch byte) (byte, bool) {
	switch {
	case ch >= '0' && ch <= '9':
		return ch - '0', true
	case ch >= 'a' && ch <= 'f':
		return ch - 'a' + 10, true
	case ch >= 'A' && ch <= 'F':
		return ch - 'A' + 10, true
	default:
		return 0, false
	}
}

func skipSpace(source string, start int) int {
	for start < len(source) && unicode.IsSpace(rune(source[start])) {
		start++
	}
	return start
}

func isIdentStart(ch rune) bool {
	return ch == '_' || ch == '$' || unicode.IsLetter(ch)
}

func isIdentPart(ch rune) bool {
	return isIdentStart(ch) || unicode.IsDigit(ch)
}

// ArticleCgiData represents the window.cgiDataNew object on an official account article page.
type ArticleCgiData struct {
	BaseResp                 BaseResp            `json:"base_resp"`
	UserName                 string              `json:"user_name"`
	NickName                 string              `json:"nick_name"`
	RoundHeadImg             string              `json:"round_head_img"`
	Title                    string              `json:"title"`
	Desc                     string              `json:"desc"`
	ContentNoencode          string              `json:"content_noencode"`
	CreateTime               string              `json:"create_time"`
	CdnURL                   string              `json:"cdn_url"`
	Link                     string              `json:"link"`
	SourceURL                string              `json:"source_url"`
	CanShare                 int                 `json:"can_share"`
	Alias                    string              `json:"alias"`
	Type                     int                 `json:"type"`
	Author                   string              `json:"author"`
	IsLimitUser              int                 `json:"is_limit_user"`
	ShowCoverPic             int                 `json:"show_cover_pic"`
	AdvertisementNum         int                 `json:"advertisement_num"`
	AdvertisementInfo        []interface{}       `json:"advertisement_info"`
	OriCreateTime            int                 `json:"ori_create_time"`
	UserUin                  string              `json:"user_uin"`
	TotalItemNum             int                 `json:"total_item_num"`
	IsAsync                  int                 `json:"is_async"`
	CommentID                string              `json:"comment_id"`
	ImgFormat                string              `json:"img_format"`
	SvrTime                  int                 `json:"svr_time"`
	CopyrightInfo            CopyrightInfo       `json:"copyright_info"`
	CanReward                int                 `json:"can_reward"`
	Signature                string              `json:"signature"`
	InMm                     int                 `json:"in_mm"`
	AppID                    string              `json:"app_id"`
	ShowComment              int                 `json:"show_comment"`
	CanUsePage               int                 `json:"can_use_page"`
	HdHeadImg                string              `json:"hd_head_img"`
	DelReasonID              int                 `json:"del_reason_id"`
	Srcid                    string              `json:"srcid"`
	IsWxgStuffUin            int                 `json:"is_wxg_stuff_uin"`
	NeedReportCost           int                 `json:"need_report_cost"`
	Bizuin                   string              `json:"bizuin"`
	Mid                      int                 `json:"mid"`
	Idx                      int                 `json:"idx"`
	Sn                       string              `json:"sn"`
	UseTxVideoPlayer         int                 `json:"use_tx_video_player"`
	IsOnlyRead               int                 `json:"is_only_read"`
	ReqID                    string              `json:"req_id"`
	UseOuterLink             int                 `json:"use_outer_link"`
	BanScene                 int                 `json:"ban_scene"`
	CspNonceStr              int                 `json:"csp_nonce_str"`
	MsgDailyIdx              int                 `json:"msg_daily_idx"`
	OriHeadImgURL            string              `json:"ori_head_img_url"`
	FilterTime               int                 `json:"filter_time"`
	AppmsgFeFilter           string              `json:"appmsg_fe_filter"`
	IsLogin                  int                 `json:"is_login"`
	ItemShowType             int                 `json:"item_show_type"`
	VoiceInAppmsg            []interface{}       `json:"voice_in_appmsg"`
	VideoPageInfo            VideoPageInfo       `json:"video_page_info"`
	MaliciousTitleReasonID   int                 `json:"malicious_title_reason_id"`
	PicturePageInfoList      []PicturePageInfo   `json:"picture_page_info_list"`
	ShowMsgVoice             int                 `json:"show_msg_voice"`
	Locationlist             []interface{}       `json:"locationlist"`
	Hotspotinfolist          []interface{}       `json:"hotspotinfolist"`
	Isnew                    int                 `json:"isnew"`
	MaliciousContentType     int                 `json:"malicious_content_type"`
	FasttmplVersion          int                 `json:"fasttmpl_version"`
	IsTopStories             int                 `json:"is_top_stories"`
	VideoIDs                 []interface{}       `json:"video_ids"`
	Isprofileblock           int                 `json:"isprofileblock"`
	CdnURL2351               string              `json:"cdn_url_235_1"`
	CdnURL11                 string              `json:"cdn_url_1_1"`
	MoreReadType             int                 `json:"more_read_type"`
	AppmsgLikeType           int                 `json:"appmsg_like_type"`
	OriSendTime              int                 `json:"ori_send_time"`
	ShowTopBar               int                 `json:"show_top_bar"`
	RelatedTag               []interface{}       `json:"related_tag"`
	UserInfo                 UserInfo            `json:"user_info"`
	Ainfos                   []Ainfo             `json:"ainfos"`
	RelatedArticleInfo       RelatedArticleInfo  `json:"related_article_info"`
	HasRedPacketCover        int                 `json:"has_red_packet_cover"`
	IsPaySubscribe           int                 `json:"is_pay_subscribe"`
	PaySubscribeInfo         PaySubscribeInfo    `json:"pay_subscribe_info"`
	VideoInArticle           []interface{}       `json:"video_in_article"`
	IsAreaShield             int                 `json:"is_area_shield"`
	ShieldAreaids            []interface{}       `json:"shield_areaids"`
	AppmsgExtGet             AppmsgExtGet        `json:"appmsg_ext_get"`
	AnchorTree               []interface{}       `json:"anchor_tree"`
	VoiceInAppmsgListJSON    string              `json:"voice_in_appmsg_list_json"`
	LiveInfo                 []interface{}       `json:"live_info"`
	Lang                     string              `json:"lang"`
	CdnURL169                string              `json:"cdn_url_16_9"`
	BizCard                  BizCard             `json:"biz_card"`
	RealItemShowType         int                 `json:"real_item_show_type"`
	URLItemShowType          int                 `json:"url_item_show_type"`
	VideoPageInfos           []interface{}       `json:"video_page_infos"`
	CanUseWecoin             int                 `json:"can_use_wecoin"`
	WecoinTips               int                 `json:"wecoin_tips"`
	FrontEndAdditionalFields FrontEndAddFields   `json:"front_end_additional_fields"`
	OpenFansmsg              int                 `json:"open_fansmsg"`
	IsCoolingAppmsg          int                 `json:"is_cooling_appmsg"`
	IPWording                IPWording           `json:"ip_wording"`
	ShowIPWording            int                 `json:"show_ip_wording"`
	IsAcctAreaShield         int                 `json:"is_acct_area_shield"`
	ShieldAcctAreaids        []interface{}       `json:"shield_acct_areaids"`
	StyleType                int                 `json:"style_type"`
	ShieldAreasInfo          []interface{}       `json:"shield_areas_info"`
	CreateTimestamp          int                 `json:"create_timestamp"`
	PictureListInPictext     []interface{}       `json:"picture_list_in_pictext"`
	Servicetype              int                 `json:"servicetype"`
	SegmentCommentID         string              `json:"segment_comment_id"`
	AdMarkStatus             int                 `json:"ad_mark_status"`
	HideAdMarkOnCps          int                 `json:"hide_ad_mark_on_cps"`
	FinderAudioCard          string              `json:"finder_audio_card"`
	ClaimSource              ClaimSource         `json:"claim_source"`
	AtBizList                AtBizList           `json:"at_biz_list"`
	ExtraCommentID           string              `json:"extra_comment_id"`
	LastText                 []interface{}       `json:"last_text"`
	WashStatus               int                 `json:"wash_status"`
	Enterid                  int                 `json:"enterid"`
	ZhugeQaIDList            []interface{}       `json:"zhuge_qa_id_list"`
	SecControlInfo           SecControlInfo      `json:"sec_control_info"`
	CdnURL34                 string              `json:"cdn_url_3_4"`
	WindowProductList        []interface{}       `json:"window_product_list"`
	FinderMusicCard          string              `json:"finder_music_card"`
	FinderAudioCardList      FinderAudioCardList `json:"finder_audio_card_list"`
	FinderMusicCardList      FinderMusicCardList `json:"finder_music_card_list"`
	NewServiceType           int                 `json:"new_service_type"`
	ProductActivity          struct{}            `json:"product_activity"`
	RtBizInfo                struct{}            `json:"rt_biz_info"`
	RedpacketCoverList       []interface{}       `json:"redpacket_cover_list"`
	FooterGiftActivity       struct{}            `json:"footer_gift_activity"`
	VerifyStatus             int                 `json:"verify_status"`
	IsPhacctVerify           int                 `json:"is_phacct_verify"`
	WatermarkSetting         int                 `json:"watermark_setting"`
	TitleGenType             int                 `json:"title_gen_type"`
	AppmsgListenID           string              `json:"appmsg_listen_id"`
	TransAppmsgInfo          struct{}            `json:"trans_appmsg_info"`
	Location                 struct{}            `json:"location"`
	TopicInfos               []interface{}       `json:"topic_infos"`
	FooterCommonShops        []interface{}       `json:"footer_common_shops"`
	FooterProductCard        struct{}            `json:"footer_product_card"`
	DescEmpty                bool                `json:"desc_empty"`
	Hashtags                 Hashtags            `json:"hashtags"`
	AigcPictures             []interface{}       `json:"aigc_pictures"`
	PrivateInfo              struct{}            `json:"private_info"`
	BizType                  int                 `json:"biz_type"`
	AIChatInfo               AIChatInfo          `json:"ai_chat_info"`
	SpecialBiz               bool                `json:"special_biz"`
	PreloadCommentItemList   []interface{}       `json:"preload_comment_item_list"`
}

// BaseResp represents the base_resp field in article CGI data.
type BaseResp struct {
	Ret         int    `json:"ret"`
	ErrMsg      string `json:"errmsg"`
	WxToken     int    `json:"wxtoken"`
	CookieCount int    `json:"cookie_count"`
	SessionID   string `json:"sessionid"`
}

// CopyrightInfo represents the copyright_info field.
type CopyrightInfo struct {
	CopyrightStat      int `json:"copyright_stat"`
	IsCartoonCopyright int `json:"is_cartoon_copyright"`
}

// VideoPageInfo represents the video_page_info field.
type VideoPageInfo struct {
	MpVideoTransInfo []interface{} `json:"mp_video_trans_info"`
	DramaVideoInfo   struct{}      `json:"drama_video_info"`
	DramaInfo        struct{}      `json:"drama_info"`
}

// PicturePageInfo represents an item in picture_page_info_list.
// type PicturePageInfo struct {
// 	CdnURL                string         `json:"cdn_url"`
// 	Width                 int            `json:"width"`
// 	Height                int            `json:"height"`
// 	PoiInfo               []interface{}  `json:"poi_info"`
// 	WxaInfo               []interface{}  `json:"wxa_info"`
// 	BindAdInfo            []interface{}  `json:"bind_ad_info"`
// 	CpsAdInfo             []interface{}  `json:"cps_ad_info"`
// 	SpotProductInfo       []interface{}  `json:"spot_product_info"`
// 	ShowWatermark         bool           `json:"show_watermark,omitempty"`
// 	BottomRightBrightness int            `json:"bottom_right_brightness,omitempty"`
// 	WatermarkInfo         *WatermarkInfo `json:"watermark_info,omitempty"`
// }

// WatermarkInfo represents the watermark_info nested in PicturePageInfo.
type WatermarkInfo struct {
	CdnURL     string `json:"cdn_url"`
	IsUploader bool   `json:"is_uploader"`
}

// UserInfo represents the user_info field.
type UserInfo struct {
	IsPaid                   int               `json:"is_paid"`
	ClientVersion            string            `json:"clientversion"`
	Ckeys                    []interface{}     `json:"ckeys"`
	FasttmplInfos            []FasttmplInfo    `json:"fasttmpl_infos"`
	IsOversea                int               `json:"isoversea"`
	SearchKeyword            SearchKeyword     `json:"search_keyword"`
	TransferConfig           []TransferConfig  `json:"transfer_config"`
	AppmsgBarData            struct{}          `json:"appmsg_bar_data"`
	PicRelatedRecInfo        struct{}          `json:"pic_related_rec_info"`
	QuoteList                []interface{}     `json:"quote_list"`
	RedFlowerLikeInfo        RedFlowerLikeInfo `json:"red_flower_like_info"`
	GetSearchKeywordRealtime int               `json:"get_search_keyword_realtime"`
}

// FasttmplInfo represents an item in user_info.fasttmpl_infos.
type FasttmplInfo struct {
	Type         int    `json:"type"`
	Version      int    `json:"version"`
	Lang         string `json:"lang"`
	FullVersion  string `json:"fullversion"`
	VersionGroup string `json:"versiongroup"`
}

// SearchKeyword represents the search_keyword field in user_info.
type SearchKeyword struct {
	ItemList         []SearchKeywordItem `json:"item_list"`
	ExpInfo          string              `json:"exp_info"`
	NeedBaikePreload bool                `json:"need_baike_preload"`
	ShowAdKeyword    bool                `json:"show_ad_keyword"`
	AdItemList       []interface{}       `json:"ad_item_list"`
}

// SearchKeywordItem represents an item in search_keyword.item_list.
type SearchKeywordItem struct {
	Keyword        string         `json:"keyword"`
	IdxRangeList   []IdxRangeItem `json:"idx_range_list"`
	S1sStatInfo    string         `json:"s1s_stat_info"`
	S1sContextInfo string         `json:"s1s_context_info"`
	S1sJsapiName   string         `json:"s1s_jsapi_name"`
	S1sJsapiParas  string         `json:"s1s_jsapi_paras"`
	Tags           []interface{}  `json:"tags"`
}

// IdxRangeItem represents an item in search_keyword.idx_range_list.
type IdxRangeItem struct {
	BeginIdx   int `json:"begin_idx"`
	EndIdx     int `json:"end_idx"`
	SectionIdx int `json:"section_idx"`
}

// TransferConfig represents an item in user_info.transfer_config.
type TransferConfig struct {
	Scope string   `json:"scope"`
	Cgis  []string `json:"cgis"`
}

// RedFlowerLikeInfo represents the red_flower_like_info field.
type RedFlowerLikeInfo struct {
	IsRedFlowerLike int `json:"is_red_flower_like"`
}

// Ainfo represents an item in ainfos.
type Ainfo struct {
	LinkType     *int   `json:"link_type"`
	Title        string `json:"title"`
	SubjectName  string `json:"subject_name"`
	ItemShowType int    `json:"item_show_type"`
	URL          string `json:"url"`
	ServiceType  int    `json:"service_type"`
}

// RelatedArticleInfo represents the related_article_info field.
type RelatedArticleInfo struct {
	HasRelatedArticleInfo int `json:"has_related_article_info"`
}

// PaySubscribeInfo represents the pay_subscribe_info field.
type PaySubscribeInfo struct {
	PreviewPercent int    `json:"preview_percent"`
	Desc           string `json:"desc"`
	Fee            int    `json:"fee"`
	GiftsCount     int    `json:"gifts_count"`
	WecoinAmount   int    `json:"wecoin_amount"`
}

// AppmsgExtGet represents the appmsg_ext_get field.
type AppmsgExtGet struct {
	FuncFlag int `json:"func_flag"`
}

// BizCard represents the biz_card field.
type BizCard struct {
	List  []BizCardItem `json:"list"`
	Total int           `json:"total"`
}

// BizCardItem represents an item in biz_card.list.
type BizCardItem struct {
	Fakeid           string `json:"fakeid"`
	Nickname         string `json:"nickname"`
	Alias            string `json:"alias"`
	RoundHeadImg     string `json:"round_head_img"`
	ServiceType      int    `json:"service_type"`
	Signature        string `json:"signature"`
	OrignalNum       int    `json:"orignal_num"`
	IsBizBan         int    `json:"is_biz_ban"`
	Username         string `json:"username"`
	BizAccountStatus int    `json:"biz_account_status"`
	VerifyStatus     int    `json:"verify_status"`
}

// FrontEndAddFields represents the front_end_additional_fields.
type FrontEndAddFields struct {
	IsAutoTypeSetting int    `json:"is_auto_type_setting"`
	SaveType          int    `json:"save_type"`
	TemplateVersion   string `json:"template_version"`
}

// IPWording represents the ip_wording field.
type IPWording struct {
	CountryName  string `json:"country_name"`
	CountryID    string `json:"country_id"`
	ProvinceName string `json:"province_name"`
}

// ClaimSource represents the claim_source field.
type ClaimSource struct {
	IsUserNoClaimSource int `json:"is_user_no_claim_source"`
}

// AtBizList represents the at_biz_list field.
type AtBizList struct {
	List  []interface{} `json:"list"`
	Total int           `json:"total"`
}

// SecControlInfo represents the sec_control_info field.
type SecControlInfo struct {
	List []interface{} `json:"list"`
}

// FinderAudioCardList represents finder_audio_card_list / finder_music_card_list.
type FinderAudioCardList struct {
	List []interface{} `json:"list"`
}

// FinderMusicCardList is an alias for the same shape.
type FinderMusicCardList = FinderAudioCardList

// Hashtags represents the hashtags field.
type Hashtags struct {
	Hashtag []interface{} `json:"hashtag"`
}

// AIChatInfo represents the ai_chat_info field.
type AIChatInfo struct {
	AIChatStatus int    `json:"ai_chat_status"`
	RoomInfo     string `json:"room_info"`
}
