package scraper

import (
	"sort"
	"strings"
)

const (
	PlatformID69Shuba         = "69shuba"
	PlatformIDBilibili        = "bilibili"
	PlatformIDCiweimao        = "ciweimao"
	PlatformIDDouban          = "douban"
	PlatformIDDouyin          = "douyin"
	PlatformIDFanqieNovel     = "fanqienovel"
	PlatformIDInstagram       = "instagram"
	PlatformIDIqiyi           = "iqiyi"
	PlatformIDJJWXC           = "jjwxc"
	PlatformIDMGTV            = "mgtv"
	PlatformIDMQidian         = "mqidian"
	PlatformIDOfficialAccount = "officialaccount"
	PlatformIDQidian          = "qidian"
	PlatformIDQQ              = "qq"
	PlatformIDQuanben         = "quanben"
	PlatformIDSFACG           = "sfacg"
	PlatformIDSoundgasm       = "soundgasm"
	PlatformIDTelegram        = "telegram"
	PlatformIDTikTok          = "tiktok"
	PlatformIDTMDB            = "tmdb"
	PlatformIDTTK             = "ttk"
	PlatformIDV2EX            = "v2ex"
	PlatformIDWeibo           = "weibo"
	PlatformIDWxChannels      = "wx_channels"
	PlatformIDX               = "x"
	PlatformIDXiaohongshu     = "xiaohongshu"
	PlatformIDYouTube         = "youtube"
	PlatformIDYouku           = "youku"
	PlatformIDZhihu           = "zhihu"
	PlatformIDZongheng        = "zongheng"
)

type Platform struct {
	ID          string
	Name        string
	HomepageURL string
}

var platforms = map[string]Platform{
	PlatformID69Shuba: {
		ID:          PlatformID69Shuba,
		Name:        "69书吧",
		HomepageURL: "https://www.69shuba.com/",
	},
	PlatformIDBilibili: {
		ID:          PlatformIDBilibili,
		Name:        "Bilibili",
		HomepageURL: "https://www.bilibili.com/",
	},
	PlatformIDCiweimao: {
		ID:          PlatformIDCiweimao,
		Name:        "刺猬猫",
		HomepageURL: "https://www.ciweimao.com/",
	},
	PlatformIDDouban: {
		ID:          PlatformIDDouban,
		Name:        "豆瓣",
		HomepageURL: "https://www.douban.com/",
	},
	PlatformIDDouyin: {
		ID:          PlatformIDDouyin,
		Name:        "抖音",
		HomepageURL: "https://www.douyin.com/",
	},
	PlatformIDFanqieNovel: {
		ID:          PlatformIDFanqieNovel,
		Name:        "番茄小说",
		HomepageURL: "https://fanqienovel.com/",
	},
	PlatformIDInstagram: {
		ID:          PlatformIDInstagram,
		Name:        "Instagram",
		HomepageURL: "https://www.instagram.com/",
	},
	PlatformIDIqiyi: {
		ID:          PlatformIDIqiyi,
		Name:        "爱奇艺",
		HomepageURL: "https://www.iqiyi.com/",
	},
	PlatformIDJJWXC: {
		ID:          PlatformIDJJWXC,
		Name:        "晋江文学城",
		HomepageURL: "https://www.jjwxc.net/",
	},
	PlatformIDMGTV: {
		ID:          PlatformIDMGTV,
		Name:        "芒果TV",
		HomepageURL: "https://www.mgtv.com/",
	},
	PlatformIDMQidian: {
		ID:          PlatformIDMQidian,
		Name:        "起点移动版",
		HomepageURL: "https://m.qidian.com/",
	},
	PlatformIDOfficialAccount: {
		ID:          PlatformIDOfficialAccount,
		Name:        "公众号",
		HomepageURL: "https://mp.weixin.qq.com/",
	},
	PlatformIDQidian: {
		ID:          PlatformIDQidian,
		Name:        "起点中文网",
		HomepageURL: "https://www.qidian.com/",
	},
	PlatformIDQQ: {
		ID:          PlatformIDQQ,
		Name:        "腾讯视频",
		HomepageURL: "https://v.qq.com/",
	},
	PlatformIDQuanben: {
		ID:          PlatformIDQuanben,
		Name:        "全本小说网",
		HomepageURL: "https://www.quanben.io/",
	},
	PlatformIDSFACG: {
		ID:          PlatformIDSFACG,
		Name:        "SF轻小说",
		HomepageURL: "https://book.sfacg.com/",
	},
	PlatformIDSoundgasm: {
		ID:          PlatformIDSoundgasm,
		Name:        "Soundgasm",
		HomepageURL: "https://soundgasm.net/",
	},
	PlatformIDTelegram: {
		ID:          PlatformIDTelegram,
		Name:        "Telegram",
		HomepageURL: "https://telegram.org/",
	},
	PlatformIDTikTok: {
		ID:          PlatformIDTikTok,
		Name:        "TikTok",
		HomepageURL: "https://www.tiktok.com/",
	},
	PlatformIDTMDB: {
		ID:          PlatformIDTMDB,
		Name:        "TMDB",
		HomepageURL: "https://www.themoviedb.org/",
	},
	PlatformIDTTK: {
		ID:          PlatformIDTTK,
		Name:        "天天看小说",
		HomepageURL: "https://ttks.tw/",
	},
	PlatformIDV2EX: {
		ID:          PlatformIDV2EX,
		Name:        "V2EX",
		HomepageURL: "https://www.v2ex.com/",
	},
	PlatformIDWeibo: {
		ID:          PlatformIDWeibo,
		Name:        "微博",
		HomepageURL: "https://weibo.com/",
	},
	PlatformIDWxChannels: {
		ID:          PlatformIDWxChannels,
		Name:        "视频号",
		HomepageURL: "https://channels.weixin.qq.com/",
	},
	PlatformIDX: {
		ID:          PlatformIDX,
		Name:        "X",
		HomepageURL: "https://x.com/",
	},
	PlatformIDXiaohongshu: {
		ID:          PlatformIDXiaohongshu,
		Name:        "小红书",
		HomepageURL: "https://www.xiaohongshu.com/",
	},
	PlatformIDYouTube: {
		ID:          PlatformIDYouTube,
		Name:        "YouTube",
		HomepageURL: "https://www.youtube.com/",
	},
	PlatformIDYouku: {
		ID:          PlatformIDYouku,
		Name:        "优酷",
		HomepageURL: "https://www.youku.com/",
	},
	PlatformIDZhihu: {
		ID:          PlatformIDZhihu,
		Name:        "知乎",
		HomepageURL: "https://www.zhihu.com/",
	},
	PlatformIDZongheng: {
		ID:          PlatformIDZongheng,
		Name:        "纵横中文网",
		HomepageURL: "https://www.zongheng.com/",
	},
}

var platformAliases = map[string]string{
	"channels":            PlatformIDWxChannels,
	"jinjiang":            PlatformIDJJWXC,
	"jj":                  PlatformIDJJWXC,
	"sf":                  PlatformIDSFACG,
	"rednote":             PlatformIDXiaohongshu,
	"tencent_video":       PlatformIDQQ,
	"themoviedb":          PlatformIDTMDB,
	"official_account":    PlatformIDOfficialAccount,
	"wechat_channels":     PlatformIDWxChannels,
	"wechat_mp":           PlatformIDOfficialAccount,
	"weixin_channels":     PlatformIDWxChannels,
	"weixin_mp":           PlatformIDOfficialAccount,
	"wx_official_account": PlatformIDOfficialAccount,
	"twitter":             PlatformIDX,
	"xhs":                 PlatformIDXiaohongshu,
}

func Lookup(platformID string) (Platform, bool) {
	id := NormalizeID(platformID)
	if canonical, ok := platformAliases[id]; ok {
		id = canonical
	}
	platform, ok := platforms[id]
	return platform, ok
}

func NormalizeID(platformID string) string {
	return strings.ToLower(strings.TrimSpace(platformID))
}

func DefaultAuthor(platformID string) string {
	if platform, ok := Lookup(platformID); ok {
		return platform.ID
	}
	return strings.TrimSpace(platformID)
}

func HomepageURL(platformID string) string {
	if platform, ok := Lookup(platformID); ok {
		return platform.HomepageURL
	}
	return ""
}

func DisplayName(platformID string) string {
	if platform, ok := Lookup(platformID); ok && strings.TrimSpace(platform.Name) != "" {
		return platform.Name
	}
	return strings.TrimSpace(platformID)
}

func AllPlatforms() []Platform {
	ids := make([]string, 0, len(platforms))
	for id := range platforms {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]Platform, 0, len(ids))
	for _, id := range ids {
		out = append(out, platforms[id])
	}
	return out
}
