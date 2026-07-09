package xiaohongshu

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/interceptor"
	platformbrowser "wx_channel/internal/platformbrowser"
)

const (
	PlatformId   = "xiaohongshu"
	PlatformName = "小红书"
	Match        = "xiaohongshu.com"
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

func HandleLoaded(db *gorm.DB, recorder platformbrowser.BrowseRecorder, logger zerolog.Logger, profile *Profile) {
	platformbrowser.RecordLoadedProfile(db, recorder, logger, FormatProfile(profile))
}
