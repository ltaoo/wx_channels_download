package officialaccount

import officialaccountdownload "github.com/GopeedLab/gopeed/pkg/officialaccount"

type OfficialAccountDownload = officialaccountdownload.OfficialAccountDownload
type WechatOfficialArticle = officialaccountdownload.WechatOfficialArticle
type CgiDataNew = officialaccountdownload.CgiDataNew
type FlexibleInt = officialaccountdownload.FlexibleInt
type VideoPageInfoItem = officialaccountdownload.VideoPageInfoItem
type MpVideoTransInfo = officialaccountdownload.MpVideoTransInfo
type PicturePageInfo = officialaccountdownload.PicturePageInfo

func ExtractArticleID(rawURL string) string {
	return officialaccountdownload.ExtractArticleID(rawURL)
}
