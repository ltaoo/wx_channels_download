package wxmp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

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
func ArticleExternalID(data *wxmp.ArticleCgiData) string {
	if data == nil || strings.TrimSpace(data.Bizuin) == "" || data.Mid <= 0 || data.Idx <= 0 {
		return ""
	}
	return fmt.Sprintf("%s_%d_%d", strings.TrimSpace(data.Bizuin), data.Mid, data.Idx)
}

// articleCoverURL picks the best cover image URL from the article data.
func articleCoverURL(data *wxmp.ArticleCgiData) string {
	return strings.TrimSpace(data.CdnURL)
}

// articleAvatarURL picks the best avatar URL for the publisher account.
func articleAvatarURL(data *wxmp.ArticleCgiData) string {
	return firstNonEmptyStr(
		strings.TrimSpace(data.RoundHeadImg),
		strings.TrimSpace(data.OriHeadImgURL),
		strings.TrimSpace(data.HdHeadImg),
	)
}

// articlePublishTime returns the publish timestamp from the article data.
func articlePublishTime(data *wxmp.ArticleCgiData) *int64 {
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

// ToContent converts an ArticleCgiData into a model.Content for an official account article.
func ToContent(data *wxmp.ArticleCgiData) (*model.Content, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	externalID := ArticleExternalID(data)
	if externalID == "" {
		return nil, errors.New("missing bizuin/mid/idx in article data")
	}

	now := util.NowMillis()
	c := &model.Content{
		Id:          BuildContentID(externalID),
		PlatformId:  PlatformID,
		ContentType: "article",
		ExternalId:  externalID,
		ExternalId2: data.CommentID,
		Title:       strings.TrimSpace(data.Title),
		Description: strings.TrimSpace(data.Desc),
		ContentURL:  strings.TrimSpace(data.Link),
		URL:         strings.TrimSpace(data.Link),
		SourceURL:   strings.TrimSpace(data.SourceURL),
		CoverURL:    articleCoverURL(data),
		PublishTime: articlePublishTime(data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if author := strings.TrimSpace(data.Author); author != "" {
		extra := map[string]string{"author": author}
		if b, err := json.Marshal(extra); err == nil {
			c.ExtraData = string(b)
		}
	}

	if data.CopyrightInfo.CopyrightStat > 0 {
		c.IsOriginal = 1
	}

	if len(data.PicturePageInfoList) > 0 {
		c.CoverWidth = strconv.Itoa(data.PicturePageInfoList[0].Width)
		c.CoverHeight = strconv.Itoa(data.PicturePageInfoList[0].Height)
	}

	return c, nil
}

// ToAccount converts an ArticleCgiData publisher into a model.Account.
func ToAccount(data *wxmp.ArticleCgiData) (*model.Account, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	externalID := strings.TrimSpace(data.Bizuin)
	if externalID == "" {
		return nil, errors.New("missing bizuin in article data")
	}

	now := util.NowMillis()
	return &model.Account{
		Id:         BuildAccountID(externalID),
		PlatformId: PlatformID,
		ExternalId: externalID,
		Username:   strings.TrimSpace(data.UserName),
		Alias:      strings.TrimSpace(data.Alias),
		Nickname:   strings.TrimSpace(data.NickName),
		AvatarURL:  articleAvatarURL(data),
		// Signature:  strings.TrimSpace(data.Signature),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ArticleToHistory converts an ArticleCgiData into a model.BrowseHistory.
func ArticleToHistory(data *wxmp.ArticleCgiData) (*model.BrowseHistory, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	externalID := ArticleExternalID(data)
	if externalID == "" {
		return nil, errors.New("missing bizuin/mid/idx in article data")
	}

	contentID := BuildContentID(externalID)
	accountID := BuildAccountID(strings.TrimSpace(data.Bizuin))
	now := util.NowMillis()

	return &model.BrowseHistory{
		PlatformId:        PlatformID,
		VisitedTimes:      1,
		AccountId:         &accountID,
		AccountExternalId: strings.TrimSpace(data.Bizuin),
		AccountUsername:   strings.TrimSpace(data.UserName),
		AccountNickname:   strings.TrimSpace(data.NickName),
		AccountAvatarURL:  articleAvatarURL(data),
		ContentId:         &contentID,
		ContentType:       "article",
		ContentExternalId: externalID,
		ContentTitle:      strings.TrimSpace(data.Title),
		ContentURL:        strings.TrimSpace(data.Link),
		ContentSourceURL:  strings.TrimSpace(data.SourceURL),
		ContentCoverURL:   articleCoverURL(data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ArticleToContentArticle converts an ArticleCgiData into a model.ContentArticle with the HTML body.
func ArticleToContentArticle(data *wxmp.ArticleCgiData) (*model.ContentArticle, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	externalID := ArticleExternalID(data)
	if externalID == "" {
		return nil, errors.New("missing bizuin/mid/idx in article data")
	}

	return &model.ContentArticle{
		ContentId:   BuildContentID(externalID),
		ContentHTML: data.ContentNoencode,
		AuthorName:  strings.TrimSpace(data.Author),
	}, nil
}

// ArticleToContentAccount creates a model.ContentAccount linking content to its publisher account.
func ArticleToContentAccount(data *wxmp.ArticleCgiData) (*model.ContentAccount, error) {
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
