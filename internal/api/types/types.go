package types

import channelspkg "wx_channel/pkg/scraper/wxchannels"

type ChannelsRequestResp[T any] struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	Data    T      `json:"data"`
}

type BaseResponse = channelspkg.BaseResponse
type BaseErrMsg = channelspkg.BaseErrMsg
type ChannelsCodecInfo = channelspkg.ChannelsCodecInfo
type ChannelsMediaSpec = channelspkg.ChannelsMediaSpec
type ChannelsScalingInfo = channelspkg.ChannelsScalingInfo
type ChannelsMediaCdnInfo = channelspkg.ChannelsMediaCdnInfo
type ChannelsMediaItem = channelspkg.ChannelsMediaItem
type ChannelsMusicInfo = channelspkg.ChannelsMusicInfo
type ChannelsFollowPostInfo = channelspkg.ChannelsFollowPostInfo
type ChannelsLocationLang = channelspkg.ChannelsLocationLang
type ChannelsLocation = channelspkg.ChannelsLocation
type ChannelsLiveMicSetting = channelspkg.ChannelsLiveMicSetting
type ChannelsLiveInfo = channelspkg.ChannelsLiveInfo
type ChannelsContactExtInfo = channelspkg.ChannelsContactExtInfo
type ChannelsContact = channelspkg.ChannelsContact
type ShortTitle = channelspkg.ShortTitle
type ChannelsObjectDesc = channelspkg.ChannelsObjectDesc
type InfoListItem = channelspkg.InfoListItem
type ObjectExtend = channelspkg.ObjectExtend
type IPRegionInfo = channelspkg.IPRegionInfo
type ChannelsObject = channelspkg.ChannelsObject
type ChannelsContactSearchResp = channelspkg.ChannelsContactSearchResp
type ChannelsFeedListOfAccountResp = channelspkg.ChannelsFeedListOfAccountResp
type ChannelsSharedFeedProfileResp = channelspkg.ChannelsSharedFeedProfileResp
type SharedFeedProfileData = channelspkg.SharedFeedProfileData
type SharedFeedSceneinfo = channelspkg.SharedFeedSceneinfo
type Errmsg = channelspkg.Errmsg
type SharedFeedinfo = channelspkg.SharedFeedinfo
type SharedFeedAuthorinfo = channelspkg.SharedFeedAuthorinfo
type ChannelsFeedProfileResp = channelspkg.ChannelsFeedProfileResp
type ChannelsAccountSearchBody = channelspkg.ChannelsAccountSearchBody
type ChannelsFeedListBody = channelspkg.ChannelsFeedListBody
type ChannelsLiveReplayListBody = channelspkg.ChannelsLiveReplayListBody
type ChannelsInteractionedFeedListBody = channelspkg.ChannelsInteractionedFeedListBody
type ChannelsFeedProfileBody = channelspkg.ChannelsFeedProfileBody
type ChannelsSharedFeedProfileBody = channelspkg.ChannelsSharedFeedProfileBody
type RequestResponse = channelspkg.RequestResponse
type ChannelsFeedCommentListBody = channelspkg.ChannelsFeedCommentListBody
type ChannelsFeedCommentListResp = channelspkg.ChannelsFeedCommentListResp
type FeedCommentNewlifeinfo = channelspkg.FeedCommentNewlifeinfo
type Monotonicdata = channelspkg.Monotonicdata
type FeedCommentCount = channelspkg.FeedCommentCount
type FeedCommentCountInfo = channelspkg.FeedCommentCountInfo
type Versiondata = channelspkg.Versiondata
type FeedCommentInfo = channelspkg.FeedCommentInfo
type Ipregioninfo = channelspkg.Ipregioninfo
type Authorcontact = channelspkg.Authorcontact
type Baseresponse = channelspkg.Baseresponse
type ChannelsFeedShareUrlBody = channelspkg.ChannelsFeedShareUrlBody
type ChannelsFeedShareUrlResp = channelspkg.ChannelsFeedShareUrlResp
