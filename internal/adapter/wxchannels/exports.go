package wxchannels

import scraper "wx_channel/pkg/scraper/wxchannels"

// ChannelsObject is a type alias re-exporting the scraper's ChannelsObject.
type ChannelsObject = scraper.ChannelsObject

// ChannelsContact is a type alias re-exporting the scraper's ChannelsContact.
type ChannelsContact = scraper.ChannelsContact

// ChannelsObjectDesc is a type alias re-exporting the scraper's ChannelsObjectDesc.
type ChannelsObjectDesc = scraper.ChannelsObjectDesc

// ChannelsMediaItem is a type alias re-exporting the scraper's ChannelsMediaItem.
type ChannelsMediaItem = scraper.ChannelsMediaItem

// ChannelsMediaSpec is a type alias re-exporting the scraper's ChannelsMediaSpec.
type ChannelsMediaSpec = scraper.ChannelsMediaSpec

// ChannelsLiveInfo is a type alias re-exporting the scraper's ChannelsLiveInfo.
type ChannelsLiveInfo = scraper.ChannelsLiveInfo

// FeedURLParts is a type alias re-exporting the scraper's FeedURLParts.
type FeedURLParts = scraper.FeedURLParts

// SphURLParts is a type alias re-exporting the scraper's SphURLParts.
type SphURLParts = scraper.SphURLParts

// MediaType constants re-exported from the scraper package.
const (
	MediaTypePicture = scraper.MediaTypePicture
	MediaTypeVideo   = scraper.MediaTypeVideo
	MediaTypeLive    = scraper.MediaTypeLive
)

// ParseFeedURL is a re-export of the scraper's ParseFeedURL.
var ParseFeedURL = scraper.ParseFeedURL

// ParseSphShareURL is a re-export of the scraper's ParseSphShareURL.
var ParseSphShareURL = scraper.ParseSphShareURL

// NewVideoDecryptor returns a new video decryptor from the scraper package.
func NewVideoDecryptor() *scraper.ChannelsVideoDecryptor {
	return scraper.NewChannelsVideoDecryptor()
}
