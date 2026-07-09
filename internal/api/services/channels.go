package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"wx_channel/internal/api/types"
	channels "wx_channel/pkg/scraper/wxchannels"
	"wx_channel/pkg/util"
)

type ChannelsService struct {
	client *channels.ChannelsClient
}

func NewChannelsService(client *channels.ChannelsClient) *ChannelsService {
	return &ChannelsService{
		client: client,
	}
}

func (s *ChannelsService) SearchContact(keyword, nextMarker string) (*types.ChannelsContactSearchResp, error) {
	return s.client.SearchChannelsContact(keyword, nextMarker)
}

func (s *ChannelsService) FetchFeedList(username, nextMarker string) (*types.ChannelsFeedListOfAccountResp, error) {
	return s.client.FetchChannelsFeedListOfContact(username, nextMarker)
}

func (s *ChannelsService) FetchLiveReplayList(username, nextMarker string) (*types.ChannelsFeedListOfAccountResp, error) {
	return s.client.FetchChannelsLiveReplayList(username, nextMarker)
}

func (s *ChannelsService) FetchInteractionedFeedList(flag, nextMarker string) (*types.ChannelsFeedListOfAccountResp, error) {
	return s.client.FetchChannelsInteractionedFeedList(flag, nextMarker)
}

func (s *ChannelsService) FetchFeedProfile(oid, nid, reqUrl, eid string) (*types.ChannelsFeedProfileResp, error) {
	if eid == "" && reqUrl != "" {
		if parsedURL, err := url.Parse(reqUrl); err == nil {
			if _eid := parsedURL.Query().Get("eid"); _eid != "" {
				eid = _eid
				reqUrl = ""
			}
		}
	}
	return s.client.FetchChannelsFeedProfile(oid, nid, reqUrl, eid)
}

func (s *ChannelsService) Validate() error {
	return s.client.Validate()
}

type FeedDownloadParams struct {
	Oid      string
	Nid      string
	Eid      string
	URL      string
	MP3      bool
	Cover    bool
	Spec     string
	Filename string
}

func (s *ChannelsService) BuildDownloadTask(params *FeedDownloadParams) (*types.ChannelsFeedProfile, *FeedDownloadTaskBody, error) {
	resp, err := s.client.FetchChannelsFeedProfile(params.Oid, params.Nid, params.URL, params.Eid)
	if err != nil {
		return nil, nil, fmt.Errorf("获取详情失败: %w", err)
	}

	if resp.ErrCode != 0 {
		return nil, nil, fmt.Errorf("获取详情失败: %s", resp.ErrMsg)
	}

	obj := resp.Data.Object
	if len(obj.ObjectDesc.Media) == 0 {
		return nil, nil, fmt.Errorf("缺少可下载的视频内容")
	}

	media := obj.ObjectDesc.Media[0]
	key := 0
	if media.DecodeKey != "" {
		k, err := strconv.Atoi(media.DecodeKey)
		if err != nil {
			return nil, nil, fmt.Errorf("解析 DecodeKey 失败: %w", err)
		}
		key = k
	}

	spec := "original"
	if params.Spec != "" {
		spec = params.Spec
	} else if len(media.Spec) > 0 {
		spec = media.Spec[0].FileFormat
	}

	defaultName := obj.ObjectDesc.Description
	if defaultName == "" {
		if obj.ID != "" {
			defaultName = obj.ID
		} else {
			defaultName = util.NowSecondsStr()
		}
	}

	filename := defaultName
	if params.Filename != "" {
		filename = params.Filename
	}

	downloadURL := media.URL + media.URLToken
	suffix := ".mp4"

	if params.Cover {
		suffix = ".jpg"
		downloadURL = media.CoverUrl
	} else if params.MP3 {
		suffix = ".mp3"
	}

	feed := &types.ChannelsFeedProfile{
		ObjectId:    obj.ID,
		NonceId:     obj.ObjectNonceId,
		SourceURL:   obj.SourceURL,
		URL:         downloadURL,
		Title:       obj.ObjectDesc.Description,
		DecryptKey:  media.DecodeKey,
		CoverURL:    media.CoverUrl,
		CoverWidth:  int(media.Width),
		CoverHeight: int(media.Height),
		Duration:    media.VideoPlayLen,
		FileSize:    media.FileSize,
		CreatedAt:   obj.CreateTime,
		Spec:        media.Spec,
		Contact: types.ChannelsFeedAccount{
			Username:  obj.Contact.Username,
			Nickname:  obj.Contact.Nickname,
			AvatarURL: obj.Contact.HeadUrl,
		},
	}

	body := &FeedDownloadTaskBody{
		Id:       obj.ID,
		NonceId:  obj.ObjectNonceId,
		Title:    obj.ObjectDesc.Description,
		Key:      key,
		Spec:     spec,
		Suffix:   suffix,
		URL:      downloadURL,
		Filename: filename,
	}

	return feed, body, nil
}

func (s *ChannelsService) BuildPictureDownloadTask(obj *types.ChannelsObject) (*FeedDownloadTaskBody, error) {
	if obj == nil {
		return nil, fmt.Errorf("object is nil")
	}

	if len(obj.Files) == 0 {
		return nil, fmt.Errorf("缺少图片文件")
	}

	defaultName := obj.ObjectDesc.Description
	if defaultName == "" {
		defaultName = obj.ID
	}
	if defaultName == "" {
		defaultName = util.NowSecondsStr()
	}

	files := make([]map[string]string, len(obj.Files))
	for i, f := range obj.Files {
		files[i] = map[string]string{
			"url":      f.URL + f.URLToken,
			"filename": fmt.Sprintf("%d.jpg", i+1),
		}
	}
	filesJSON, _ := json.Marshal(files)

	body := &FeedDownloadTaskBody{
		Id:       obj.ID,
		NonceId:  obj.ObjectNonceId,
		Title:    obj.ObjectDesc.Description,
		Filename: defaultName,
		URL:      fmt.Sprintf("zip://weixin.qq.com?files=%s", url.QueryEscape(string(filesJSON))),
		Suffix:   ".zip",
	}

	return body, nil
}

func (s *ChannelsService) BuildLiveDownloadTask(obj *types.ChannelsObject) (*FeedDownloadTaskBody, error) {
	if obj == nil {
		return nil, fmt.Errorf("object is nil")
	}

	defaultName := obj.ObjectDesc.Description
	if defaultName == "" {
		defaultName = "直播"
	}

	body := &FeedDownloadTaskBody{
		Id:       obj.ID,
		NonceId:  obj.ObjectNonceId,
		Title:    defaultName,
		Filename: defaultName,
		URL:      "",
		Suffix:   ".mp4",
	}

	return body, nil
}

func BuildJumpURL(feed *types.ChannelsFeedProfile) string {
	if feed == nil {
		return "https://channels.weixin.qq.com/web/pages/feed"
	}
	if feed.SourceURL != "" {
		return feed.SourceURL
	}
	origin := "https://channels.weixin.qq.com"
	u := origin + "/web/pages/feed"
	if feed.Contact.Username != "" {
		u += "?username=" + feed.Contact.Username
	}
	if feed.ObjectId != "" {
		encodedOid := util.EncodeUint64ToBase64(feed.ObjectId)
		if encodedOid != "" {
			u += "&oid=" + encodedOid
		}
	}
	if feed.NonceId != "" {
		encodedNid := util.EncodeUint64ToBase64(feed.NonceId)
		if encodedNid != "" {
			u += "&nid=" + encodedNid
		}
	}
	return u
}
