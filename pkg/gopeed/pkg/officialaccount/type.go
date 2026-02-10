package officialaccountdownload

type CgiDataNew struct {
	BaseResp struct {
		Ret         int    `json:"ret"`
		ErrMsg      string `json:"errmsg"`
		WxToken     int    `json:"wxtoken"`
		CookieCount int    `json:"cookie_count"`
		SessionID   string `json:"sessionid"`
	} `json:"base_resp"`
	UserName          string `json:"user_name"`
	NickName          string `json:"nick_name"`
	RoundHeadImg      string `json:"round_head_img"`
	Title             string `json:"title"`
	Desc              string `json:"desc"`
	ContentNoEncode   string `json:"content_noencode"`
	CreateTime        string `json:"create_time"`
	CdnUrl            string `json:"cdn_url"`
	Link              string `json:"link"`
	SourceUrl         string `json:"source_url"`
	CanShare          int    `json:"can_share"`
	Alias             string `json:"alias"`
	Type              int    `json:"type"`
	Author            string `json:"author"`
	IsLimitUser       int    `json:"is_limit_user"`
	ShowCoverPic      int    `json:"show_cover_pic"`
	AdvertisementNum  int    `json:"advertisement_num"`
	AdvertisementInfo []any  `json:"advertisement_info"`
	OriCreateTime     int    `json:"ori_create_time"`
	UserUin           int    `json:"user_uin"`
	TotalItemNum      int    `json:"total_item_num"`
	IsAsync           int    `json:"is_async"`
	CommentID         string `json:"comment_id"`
	ImgFormat         string `json:"img_format"`
	SvrTime           int    `json:"svr_time"`
	CopyrightInfo     struct {
		CopyrightStat      int `json:"copyright_stat"`
		IsCartoonCopyright int `json:"is_cartoon_copyright"`
	} `json:"copyright_info"`
	CanReward        int    `json:"can_reward"`
	Signature        string `json:"signature"`
	RewardWording    string `json:"reward_wording"`
	InMm             int    `json:"in_mm"`
	AppID            string `json:"app_id"`
	ShowComment      int    `json:"show_comment"`
	CanUsePage       int    `json:"can_use_page"`
	HdHeadImg        string `json:"hd_head_img"`
	DelReasonID      int    `json:"del_reason_id"`
	SrcID            string `json:"srcid"`
	IsWxgStuffUin    int    `json:"is_wxg_stuff_uin"`
	NeedReportCost   int    `json:"need_report_cost"`
	BizUin           string `json:"bizuin"`
	Mid              int    `json:"mid"`
	Idx              int    `json:"idx"`
	Sn               string `json:"sn"`
	UseTxVideoPlayer int    `json:"use_tx_video_player"`
	IsOnlyRead       int    `json:"is_only_read"`
	ReqID            string `json:"req_id"`
	UseOuterLink     int    `json:"use_outer_link"`
	BanScene         int    `json:"ban_scene"`
	CspNonceStr      int    `json:"csp_nonce_str"`
	MsgDailyIdx      int    `json:"msg_daily_idx"`
	OriHeadImgUrl    string `json:"ori_head_img_url"`
	FilterTime       int    `json:"filter_time"`
	AppmsgFeFilter   string `json:"appmsg_fe_filter"`
	IsLogin          int    `json:"is_login"`
	RewardMoney      int    `json:"reward_money"`
	PageType         int    `json:"page_type"`
	ItemShowType     int    `json:"item_show_type"`
	VoiceInAppmsg    []any  `json:"voice_in_appmsg"`
	VideoPageInfo    struct {
		MpVideoTransInfo []any `json:"mp_video_trans_info"`
		DramaVideoInfo   struct {
		} `json:"drama_video_info"`
		DramaInfo struct {
		} `json:"drama_info"`
	} `json:"video_page_info"`
	MaliciousTitleReasonID int                 `json:"malicious_title_reason_id"`
	VideoPageInfos         []VideoPageInfoItem `json:"video_page_infos"`
	PicturePageInfoList    []PicturePageInfo   `json:"picture_page_info_list"`
}

type VideoPageInfoItem struct {
	VideoID              string             `json:"video_id"`
	OriStatus            int                `json:"ori_status"`
	SourceBizuin         string             `json:"source_bizuin"`
	SourceNickname       string             `json:"source_nickname"`
	SourceLink           string             `json:"source_link"`
	IsMpVideo            int                `json:"is_mp_video"`
	IsMpVideoDelete      int                `json:"is_mp_video_delete"`
	MpVideoOverseasLimit int                `json:"mp_video_overseas_limit"`
	IsMpVideoForbid      int                `json:"is_mp_video_forbid"`
	MpVideoForbidReason  int                `json:"mp_video_forbid_reason"`
	HitUsername          int                `json:"hit_username"`
	HitBizuin            int                `json:"hit_bizuin"`
	HitVid               string             `json:"hit_vid"`
	ContentNoencode      string             `json:"content_noencode"`
	IsMpVideoUrgentState int                `json:"is_mp_video_urgent_state"`
	IsAppmsgUnauthorized int                `json:"is_appmsg_unauthorized"`
	CoverUrl             string             `json:"cover_url"`
	CoverUrl11           string             `json:"cover_url_1_1"`
	CoverUrl169          string             `json:"cover_url_16_9"`
	VideoidUploadtime    int                `json:"videoid_uploadtime"`
	VideoidBizuin        int                `json:"videoid_bizuin"`
	MpVideoTransInfo     []MpVideoTransInfo `json:"mp_video_trans_info"`
}

type MpVideoTransInfo struct {
	DurationMs          int    `json:"duration_ms"`
	Filesize            any    `json:"filesize"`
	FormatID            int    `json:"format_id"`
	Height              int    `json:"height"`
	Url                 string `json:"url"`
	VideoQualityLevel   int    `json:"video_quality_level"`
	VideoQualityWording string `json:"video_quality_wording"`
	Width               int    `json:"width"`
}

type PicturePageInfo struct {
	CdnUrl     string `json:"cdn_url"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	ThemeColor string `json:"theme_color"`
	IsQrCode   int    `json:"is_qr_code"`
	PoiInfo    []any  `json:"poi_info"`
	WxaInfo    []any  `json:"wxa_info"`
	LivePhoto  struct {
		FormatInfo []any `json:"format_info"`
	} `json:"live_photo"`
	DisableThemeColor bool  `json:"disable_theme_color"`
	BindAdInfo        []any `json:"bind_ad_info"`
	CpsAdInfo         []any `json:"cps_ad_info"`
	PicWindowProduct  struct {
		ProductEncryptKey string `json:"product_encrypt_key"`
		ProductType       int    `json:"product_type"`
		Title             string `json:"title"`
		DataType          int    `json:"data_type"`
		ProductID         string `json:"product_id"`
	} `json:"pic_window_product"`
	ShowWatermark         bool    `json:"show_watermark"`
	BottomRightBrightness float64 `json:"bottom_right_brightness"`
	WatermarkInfo         struct {
		CdnUrl     string `json:"cdn_url"`
		IsUploader bool   `json:"is_uploader"`
	} `json:"watermark_info"`
	SpotProductInfo []any `json:"spot_product_info"`
	ShareCover      struct {
		FileID   int    `json:"file_id"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		CdnUrl   string `json:"cdn_url"`
		CropInfo string `json:"crop_info"`
	} `json:"share_cover"`
}

type WechatOfficialArticle struct {
	Type           int                 `json:"type"`
	Title          string              `json:"title"`
	Content        string              `json:"content"`
	ContentLength  int                 `json:"content_length"`
	Images         []string            `json:"images"`
	Creator        string              `json:"creator"`
	AuthorNickname string              `json:"author_nickname"`
	AuthorAvatar   string              `json:"author_avatar"`
	AuthorID       string              `json:"author_id"`
	PublishTimeStr string              `json:"publish_time_str"`
	Videos         []VideoPageInfoItem `json:"videos"`
}
