package weibo

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/interceptor"
	webcontent "wx_channel/internal/webcontent"
)

const (
	PlatformId   = "weibo"
	PlatformName = "微博"
	Match        = "weibo.com"
	ContentType  = "article"
)

func Config() interceptor.PlatformBrowserConfig {
	return interceptor.PlatformBrowserConfig{
		PlatformId:   PlatformId,
		PlatformName: PlatformName,
		Match:        Match,
		ContentType:  ContentType,
	}
}

func FormatProfile(profile *Profile) *Profile {
	return profile
}

func HandleLoaded(db *gorm.DB, recorder webcontent.BrowseRecorder, logger zerolog.Logger, profile *Profile) {
	webcontent.RecordLoadedProfile(db, recorder, logger, FormatProfile(profile))
}
