package wxchannelsadapter

import "wx_channel/pkg/scraper/wxchannels"

// ChannelsObject is a type alias re-exporting the scraper's ChannelsObject.
type ChannelsObject = wxchannels.ChannelsObject

// ChannelsContact is a type alias re-exporting the scraper's ChannelsContact.
type ChannelsContact = wxchannels.ChannelsContact

// ChannelsObjectDesc is a type alias re-exporting the scraper's ChannelsObjectDesc.
type ChannelsObjectDesc = wxchannels.ChannelsObjectDesc

// ChannelsMediaItem is a type alias re-exporting the scraper's ChannelsMediaItem.
type ChannelsMediaItem = wxchannels.ChannelsMediaItem

// ChannelsMediaSpec is a type alias re-exporting the scraper's ChannelsMediaSpec.
type ChannelsMediaSpec = wxchannels.ChannelsMediaSpec

// ChannelsLiveInfo is a type alias re-exporting the scraper's ChannelsLiveInfo.
type ChannelsLiveInfo = wxchannels.ChannelsLiveInfo

// FeedURLParts is a type alias re-exporting the scraper's FeedURLParts.
type FeedURLParts = wxchannels.FeedURLParts

// SphURLParts is a type alias re-exporting the scraper's SphURLParts.
type SphURLParts = wxchannels.SphURLParts

// MediaType constants re-exported from the scraper package.
const (
	MediaTypePicture = wxchannels.MediaTypePicture
	MediaTypeVideo   = wxchannels.MediaTypeVideo
	MediaTypeLive    = wxchannels.MediaTypeLive
)

// ParseFeedURL is a re-export of the scraper's ParseFeedURL.
var ParseFeedURL = wxchannels.ParseFeedURL

// ParseSphShareURL is a re-export of the scraper's ParseSphShareURL.
var ParseSphShareURL = wxchannels.ParseSphShareURL

// NewVideoDecryptor returns a new video decryptor from the scraper package.
func NewVideoDecryptor() *wxchannels.ChannelsVideoDecryptor {
	return wxchannels.NewChannelsVideoDecryptor()
}
