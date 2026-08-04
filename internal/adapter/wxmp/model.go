package wxmp

import (
	"errors"
	"strconv"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/internal/database/model"
	wxmp "wx_channel/pkg/scraper/wxmp"
	"wx_channel/pkg/util"
)

const platformIDWxMP = "wxmp"

// PlatformID is the platform identifier for WeChat official accounts.
const PlatformID = platformIDWxMP

// BuildContentID builds a content identifier from an external ID.
func BuildContentID(externalID string) string {
	return PlatformID + ":" + externalID
}

// BuildAccountID builds an account identifier from an external ID.
func BuildAccountID(externalID string) string {
	return PlatformID + ":" + externalID
}

// ArticleExternalID builds a unique external identifier for an official account article.
func ArticleExternalID(data *wxmp.ArticleCgiDataNew) string {
	if data == nil || strings.TrimSpace(data.Bizuin) == "" {
		return ""
	}
	return strings.TrimSpace(data.Bizuin)
}

// articleCoverURL picks the best cover image URL from the article data.
func articleCoverURL(data *wxmp.ArticleCgiDataNew) string {
	return strings.TrimSpace(data.CdnURL)
}

// articleAvatarURL picks the best avatar URL for the publisher account.
func articleAvatarURL(data *wxmp.ArticleCgiDataNew) string {
	return firstNonEmptyStr(
		strings.TrimSpace(data.RoundHeadImg),
		strings.TrimSpace(data.OriHeadImgURL),
		strings.TrimSpace(data.HdHeadImg),
	)
}

// articlePublishTime returns the publish timestamp from the article data.
func articlePublishTime(data *wxmp.ArticleCgiDataNew) *int64 {
	if data.OriCreateTime > 0 {
		t := int64(data.OriCreateTime)
		return &t
	}
	if data.CreateTimestamp > 0 {
		t := int64(data.CreateTimestamp)
		return &t
	}
	return nil
}

// ToContent converts an ArticleCgiData into a slim model.Content and its type-specific extension.
func ToContent(data *wxmp.ArticleCgiDataNew) (*model.Content, any, error) {
	if data == nil {
		return nil, nil, errors.New("article data is nil")
	}
	externalID := ArticleExternalID(data)
	if externalID == "" {
		return nil, nil, errors.New("missing bizuin/mid/idx in article data")
	}

	now := util.NowMillis()
	c := &model.Content{
		Id:          BuildContentID(externalID),
		PlatformId:  PlatformID,
		Type: "article",
		ExternalId:  externalID,
		ExternalId2: strconv.Itoa(data.Mid),
		Title:       strings.TrimSpace(data.Title),
		Description: strings.TrimSpace(data.Desc),
		URL:         strings.TrimSpace(data.Link),
		SourceURL:   strings.TrimSpace(data.SourceURL),
		CoverURL:    articleCoverURL(data),
		PublishTime: articlePublishTime(data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if data.PageType == 2 {
		album, images := buildContentAlbum(data, c.Id)
		return c, &ContentAlbumExt{Album: album, Images: images}, nil
	}

	return c, buildContentArticle(data, c.Id), nil
}

// ToAccount converts an ArticleCgiData publisher into a model.Account.
func ToAccount(data *wxmp.ArticleCgiDataNew) (*model.Account, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	externalID := data.UserName
	if externalID == "" {
		return nil, errors.New("missing bizuin in article data")
	}

	now := util.NowMillis()
	return &model.Account{
		Id:         BuildAccountID(externalID),
		PlatformId: PlatformID,
		ExternalId: data.UserName,
		Alias:      strings.TrimSpace(data.Alias),
		Nickname:   strings.TrimSpace(data.NickName),
		Signature:  strings.TrimSpace(data.Signature),
		AvatarURL:  articleAvatarURL(data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ArticleToHistory converts an ArticleCgiData into a model.BrowseHistory.
func ArticleToHistory(data *wxmp.ArticleCgiDataNew) (*model.BrowseHistory, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	externalID := ArticleExternalID(data)
	if externalID == "" {
		return nil, errors.New("missing bizuin/mid/idx in article data")
	}

	accountID := BuildAccountID(strings.TrimSpace(data.Bizuin))
	now := util.NowMillis()

	return &model.BrowseHistory{
		PlatformId:        PlatformID,
		VisitedTimes:      1,
		AccountId:         &accountID,
		AccountExternalId: strings.TrimSpace(data.Bizuin),
		AccountNickname:   strings.TrimSpace(data.NickName),
		AccountAvatarURL:  articleAvatarURL(data),
		Type:              "article",
		ExternalId:        externalID,
		Title:             strings.TrimSpace(data.Title),
		URL:               strings.TrimSpace(data.Link),
		SourceURL:         strings.TrimSpace(data.SourceURL),
		CoverURL:          articleCoverURL(data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ArticleToContentArticle converts an ArticleCgiData into a model.ContentArticle with the HTML body.
func ArticleToContentArticle(data *wxmp.ArticleCgiDataNew) (*model.ContentArticle, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	externalID := ArticleExternalID(data)
	if externalID == "" {
		return nil, errors.New("missing bizuin/mid/idx in article data")
	}

	return buildContentArticle(data, BuildContentID(externalID)), nil
}

func buildContentArticle(data *wxmp.ArticleCgiDataNew, id string) *model.ContentArticle {
	article := &model.ContentArticle{
		Id:        id,
		Type:      model.ContentArticleTypeHTML,
		WordCount: articleWordCount(data.ContentNoencode),
		HTML:      data.ContentNoencode,
	}
	if data.CopyrightInfo.CopyrightStat > 0 {
		article.IsOriginal = 1
	}
	return article
}

// ContentAlbumExt wraps ContentAlbum with its child images for passing through
// the ToContent any return value.
type ContentAlbumExt struct {
	Album  *model.ContentAlbum
	Images []*model.ContentImage
}

func buildContentAlbum(data *wxmp.ArticleCgiDataNew, contentID string) (*model.ContentAlbum, []*model.ContentImage) {
	albumImages := make([]*model.ContentImage, 0, len(data.PicturePageInfoList))
	for i, picture := range data.PicturePageInfoList {
		imageURL := normalizeImageURL(picture.CdnUrl)
		if imageURL == "" {
			continue
		}
		albumImages = append(albumImages, &model.ContentImage{
			AlbumId:   contentID,
			SortOrder: i,
			URL:       imageURL,
			Width:     picture.Width,
			Height:    picture.Height,
		})
	}

	album := &model.ContentAlbum{
		Id:          contentID,
		ImageCount:  len(albumImages),
		Format:      strings.TrimSpace(data.ImgFormat),
		Description: strings.TrimSpace(data.Desc),
	}
	if len(albumImages) > 0 {
		album.CoverWidth = albumImages[0].Width
		album.CoverHeight = albumImages[0].Height
	}
	return album, albumImages
}

func articleWordCount(contentHTML string) int {
	text := contentHTML
	if doc, err := goquery.NewDocumentFromReader(strings.NewReader(contentHTML)); err == nil {
		text = doc.Text()
	}

	count := 0
	for _, r := range text {
		if !unicode.IsSpace(r) {
			count++
		}
	}
	return count
}

// ArticleToContentAccount creates a model.ContentAccount linking content to its publisher account.
func ArticleToContentAccount(data *wxmp.ArticleCgiDataNew) (*model.ContentAccount, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	externalID := ArticleExternalID(data)
	if externalID == "" {
		return nil, errors.New("missing bizuin/mid/idx in article data")
	}

	return &model.ContentAccount{
		ContentId: BuildContentID(externalID),
		AccountId: BuildAccountID(strings.TrimSpace(data.Bizuin)),
		Role:      "publisher",
		CreatedAt: util.NowMillis(),
	}, nil
}

// firstNonEmptyStr returns the first non-empty string from the given values.
func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
