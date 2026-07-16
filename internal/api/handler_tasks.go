package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	"github.com/GopeedLab/gopeed/pkg/base"
	gopeedhttp "github.com/GopeedLab/gopeed/pkg/protocol/http"
	gopeedstream "github.com/GopeedLab/gopeed/pkg/protocol/stream"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	officialaccountdownload "wx_channel/internal/webcontent/officialaccount"
	wxchannels "wx_channel/internal/webcontent/wxchannels"

	apitypes "wx_channel/internal/api/types"
	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
	channels "wx_channel/pkg/scraper/wxchannels"
	"wx_channel/pkg/scraper/douyin"
	"wx_channel/pkg/scraper/zhihu"
	"wx_channel/pkg/system"
	"wx_channel/pkg/util"
)

type ChannelsDownloadRequest struct {
	Object apitypes.ChannelsObject `json:"object"`
	Spec   string                  `json:"spec"`
	Suffix string                  `json:"suffix"`
}

func (c *APIClient) handleCreateFeedDownloadTask(ctx *gin.Context) {
	bodyBytes, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		result.Err(ctx, 400, "读取请求参数失败")
		return
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var dispatchBody struct {
		URL    string                  `json:"url"`
		Object apitypes.ChannelsObject `json:"object"`
	}
	if err := json.Unmarshal(bodyBytes, &dispatchBody); err == nil &&
		dispatchBody.URL != "" &&
		dispatchBody.Object.ID == "" {
		if c.platformDownloadRouter().Match(dispatchBody.URL) != nil {
			var createBody platformCreateTaskBody
			if err := json.Unmarshal(bodyBytes, &createBody); err != nil {
				result.Err(ctx, 400, "不合法的参数")
				return
			}
			id, err := c.startPlatformDownloadTask(ctx.Request.Context(), createBody)
			if err != nil {
				if c.logger != nil {
					c.logger.Error().Str("url", dispatchBody.URL).Err(err).Msg("创建平台下载任务失败")
				}
				result.Err(ctx, 500, "创建任务失败："+err.Error())
				return
			}
			result.Ok(ctx, gin.H{"id": id})
			return
		}
		if !isChannelsDownloadURL(dispatchBody.URL) {
			if articleID := officialaccountdownload.ExtractArticleID(dispatchBody.URL); articleID != "" {
				id, err := c.startDownloadOfficialAccountURL(dispatchBody.URL, articleID)
				if err != nil {
					if c.logger != nil {
						c.logger.Error().Str("url", dispatchBody.URL).Err(err).Msg("创建公众号下载任务失败")
					}
					result.Err(ctx, 500, "创建任务失败："+err.Error())
					return
				}
				result.Ok(ctx, gin.H{"id": id})
				return
			}
			if answerURL, ok := zhihu.ParseAnswerURL(dispatchBody.URL); ok {
				id, err := c.startDownloadZhihuAnswerURL(answerURL)
				if err != nil {
					if c.logger != nil {
						c.logger.Error().Str("url", answerURL.Canonical).Err(err).Msg("创建知乎下载任务失败")
					}
					result.Err(ctx, 500, "创建任务失败："+err.Error())
					return
				}
				result.Ok(ctx, gin.H{"id": id})
				return
			}
			if shareURL := douyin.ExtractShareURL(dispatchBody.URL); shareURL != "" {
				id, err := c.startDownloadDouyinShareURL(ctx, shareURL)
				if err != nil {
					if c.logger != nil {
						c.logger.Error().Str("url", shareURL).Err(err).Msg("创建抖音下载任务失败")
					}
					result.Err(ctx, 500, "创建任务失败："+err.Error())
					return
				}
				result.Ok(ctx, gin.H{"id": id})
				return
			}
			result.Err(ctx, 400, "暂时不支持该下载链接")
			return
		}
		ctx.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		c.handleCreateChannelsTask(ctx)
		return
	}

	var body ChannelsDownloadRequest
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Object.ID == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	if body.Suffix == ".mp3" {
		hasFFmpeg := system.ExistingCommand("ffmpeg")
		if !hasFFmpeg {
			result.Err(ctx, 3001, "下载 mp3 需要支持 ffmpeg 命令")
			return
		}
	}

	id, err := c.startDownloadChannelsObject(&body)
	if err != nil {
		c.logger.Error().Interface("body", body).Err(err).Msg("创建任务失败")
		result.Err(ctx, 500, "创建任务失败："+err.Error())
		return
	}

	result.Ok(ctx, gin.H{"id": id})
}

func isChannelsDownloadURL(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	hostname := parsedURL.Hostname()
	path := parsedURL.EscapedPath()
	if strings.EqualFold(hostname, "finder.video.qq.com") {
		return strings.Contains(path, "/stodownload")
	}
	if strings.EqualFold(hostname, "channels.weixin.qq.com") {
		return path == "/web/pages/feed"
	}
	return false
}

func (c *APIClient) startDownloadOfficialAccountURL(rawURL string, articleID string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || articleID == "" {
		return "", fmt.Errorf("不支持的公众号链接")
	}
	tasks := c.downloader.GetTasks()
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil || t.Meta.Req.Labels == nil {
			continue
		}
		labels := t.Meta.Req.Labels
		if labels["platform"] == "officialaccount" && labels["article_id"] == articleID {
			return "", fmt.Errorf("已存在该下载内容")
		}
	}

	content, err := c.upsertOfficialAccountArticleContent(rawURL, articleID)
	if err != nil {
		return "", fmt.Errorf("解析公众号文章失败: %w", err)
	}

	downloadDir := ""
	if c.cfg != nil {
		downloadDir = c.cfg.DownloadDir
	}
	taskID, err := c.downloader.CreateDirect(
		&base.Request{
			URL: "officialaccount://" + rawURL,
			Labels: map[string]string{
				"platform":   "officialaccount",
				"id":         articleID,
				"article_id": articleID,
				"key":        "0",
				"spec":       "",
				"suffix":     ".html",
				"source_url": rawURL,
			},
		},
		&base.Options{
			Name: fmt.Sprintf("wechat_official_%s.html", articleID),
			Path: downloadDir,
		},
	)
	if err != nil {
		return "", fmt.Errorf("创建任务失败: %w", err)
	}
	task := c.downloader.GetTask(taskID)
	if task != nil && content != nil {
		if _, err := c.CreateContentDownloadTask(content, task, "frontend"); err != nil && c.logger != nil {
			c.logger.Warn().Err(err).Msg("CreateContentDownloadTask failed")
		}
	}
	if task != nil && c.downloader_ws != nil {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: map[string]interface{}{
				"task": task,
			},
		})
	}
	return taskID, nil
}

func (c *APIClient) startDownloadZhihuAnswerURL(answerURL zhihu.AnswerURL) (string, error) {
	if answerURL.QuestionID == "" || answerURL.AnswerID == "" || answerURL.Canonical == "" {
		return "", fmt.Errorf("不支持的知乎回答链接")
	}
	tasks := c.downloader.GetTasks()
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil || t.Meta.Req.Labels == nil {
			continue
		}
		labels := t.Meta.Req.Labels
		if labels["platform"] == "zhihu" && labels["question_id"] == answerURL.QuestionID && labels["answer_id"] == answerURL.AnswerID {
			return "", fmt.Errorf("已存在该下载内容")
		}
	}

	content, err := c.upsertZhihuAnswerContent(answerURL)
	if err != nil {
		return "", fmt.Errorf("解析知乎回答失败: %w", err)
	}

	downloadDir := ""
	if c.cfg != nil {
		downloadDir = c.cfg.DownloadDir
	}
	taskID, err := c.downloader.CreateDirect(
		&base.Request{
			URL: "zhihu://" + answerURL.Canonical,
			Labels: map[string]string{
				"platform":    "zhihu",
				"id":          answerURL.AnswerID,
				"question_id": answerURL.QuestionID,
				"answer_id":   answerURL.AnswerID,
				"key":         "0",
				"spec":        "",
				"suffix":      ".html",
				"source_url":  answerURL.Canonical,
			},
		},
		&base.Options{
			Name: fmt.Sprintf("zhihu_%s_%s.html", answerURL.QuestionID, answerURL.AnswerID),
			Path: downloadDir,
		},
	)
	if err != nil {
		return "", fmt.Errorf("创建任务失败: %w", err)
	}

	task := c.downloader.GetTask(taskID)
	if task != nil && content != nil {
		if _, err := c.CreateContentDownloadTask(content, task, "frontend"); err != nil && c.logger != nil {
			c.logger.Warn().Err(err).Msg("CreateContentDownloadTask failed")
		}
	}
	if task != nil && c.downloader_ws != nil {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: map[string]interface{}{
				"task": task,
			},
		})
	}
	return taskID, nil
}

func (c *APIClient) upsertOfficialAccountArticleContent(rawURL string, articleID string) (*model.Content, error) {
	if c.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	oa := &officialaccountdownload.OfficialAccountDownload{}
	article, err := oa.FetchArticle(rawURL)
	if err != nil {
		return nil, err
	}
	if article == nil {
		return nil, fmt.Errorf("empty article")
	}

	now := util.NowMillis()
	accountExternalID := firstNonEmpty(article.AuthorID, article.AuthorNickname, "officialaccount_"+articleID)
	accountUsername := firstNonEmpty(article.AuthorID, accountExternalID)
	accountNickname := firstNonEmpty(article.AuthorNickname, article.Creator, accountUsername)
	title := firstNonEmpty(article.Title, "公众号文章")
	coverURL := ""
	if len(article.Images) > 0 {
		coverURL = article.Images[0]
	}

	var content model.Content
	err = c.db.Transaction(func(tx *gorm.DB) error {
		account, err := upsertContentAccount(tx, model.Account{
			PlatformId: "wx_official_account",
			ExternalId: accountExternalID,
			Username:   accountUsername,
			Nickname:   accountNickname,
			AvatarURL:  article.AuthorAvatar,
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}, now)
		if err != nil {
			return err
		}

		content = model.Content{
			PlatformId:  "wx_official_account",
			ContentType: "article",
			ExternalId:  articleID,
			ExternalId2: article.AuthorID,
			Title:       title,
			Description: firstNonEmpty(article.Creator, accountNickname),
			ContentURL:  rawURL,
			URL:         rawURL,
			SourceURL:   rawURL,
			CoverURL:    coverURL,
			FileSize:    int64(article.ContentLength),
			Size:        int64(article.ContentLength),
			Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
		if err := upsertContentByPlatformExternalID(tx, &content, now); err != nil {
			return err
		}
		if err := upsertContentArticle(tx, content.Id, model.ContentArticle{
			ContentId:   content.Id,
			ContentHTML: article.Content,
			AuthorName:  accountNickname,
		}); err != nil {
			return err
		}
		return upsertContentOwner(tx, content.Id, account.Id, now)
	})
	if err != nil {
		return nil, err
	}
	return &content, nil
}

func (c *APIClient) upsertZhihuAnswerContent(answerURL zhihu.AnswerURL) (*model.Content, error) {
	if c.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	client := &zhihu.Client{}
	page, err := client.FetchAnswerPage(answerURL.Canonical)
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, fmt.Errorf("empty answer page")
	}

	now := util.NowMillis()
	author := page.Answer.Author
	accountExternalID := firstNonEmpty(author.ID, author.URLToken, author.URLTokenSnake, "zhihu_"+answerURL.AnswerID)
	accountUsername := firstNonEmpty(author.URLToken, author.URLTokenSnake, accountExternalID)
	accountNickname := firstNonEmpty(author.Name, accountUsername)
	avatarURL := firstNonEmpty(author.AvatarURL, author.AvatarURLSnake, author.AvatarURLTemplate)
	profileURL := zhihuUserProfileURL(author)
	title := firstNonEmpty(page.Question.Title, "知乎回答")
	description := firstNonEmpty(page.Answer.Excerpt, strings.TrimSpace(page.Question.Excerpt))
	publishTime := page.Answer.CreatedTime
	contentHTML := zhihu.BuildHTML(page)
	coverURL := zhihu.FirstImageURL(page.Answer.Content, answerURL.Canonical)

	var content model.Content
	err = c.db.Transaction(func(tx *gorm.DB) error {
		account, err := upsertContentAccount(tx, model.Account{
			PlatformId: "zhihu",
			ExternalId: accountExternalID,
			Username:   accountUsername,
			Nickname:   accountNickname,
			AvatarURL:  avatarURL,
			ProfileURL: profileURL,
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}, now)
		if err != nil {
			return err
		}

		content = model.Content{
			PlatformId:  "zhihu",
			ContentType: "article",
			ExternalId:  answerURL.AnswerID,
			ExternalId2: answerURL.QuestionID,
			Title:       title,
			Description: description,
			ContentURL:  answerURL.Canonical,
			URL:         answerURL.Canonical,
			SourceURL:   answerURL.Canonical,
			CoverURL:    coverURL,
			FileSize:    int64(len(contentHTML)),
			Size:        int64(len(contentHTML)),
			PublishTime: &publishTime,
			Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
		if publishTime <= 0 {
			content.PublishTime = nil
		}
		if err := upsertContentByPlatformExternalID(tx, &content, now); err != nil {
			return err
		}
		if err := upsertContentArticle(tx, content.Id, model.ContentArticle{
			ContentId:   content.Id,
			ContentHTML: page.Answer.Content,
			AuthorName:  accountNickname,
		}); err != nil {
			return err
		}
		return upsertContentOwner(tx, content.Id, account.Id, now)
	})
	if err != nil {
		return nil, err
	}
	return &content, nil
}

func (c *APIClient) startDownloadDouyinShareURL(ctx *gin.Context, rawURL string) (string, error) {
	shareURL := douyin.ExtractShareURL(rawURL)
	if shareURL == "" {
		return "", fmt.Errorf("不支持的抖音分享链接")
	}

	info, err := douyin.Parse(ctx.Request.Context(), shareURL)
	if err != nil {
		return "", fmt.Errorf("解析抖音分享链接失败: %w", err)
	}
	if info.VideoID == "" || info.URL == "" {
		return "", fmt.Errorf("抖音视频信息不完整")
	}

	suffix := ".mp4"
	tasks := c.downloader.GetTasks()
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil || t.Meta.Req.Labels == nil {
			continue
		}
		labels := t.Meta.Req.Labels
		if labels["platform"] == "douyin" && labels["id"] == info.VideoID && labels["suffix"] == suffix {
			return "", fmt.Errorf("已存在该下载内容")
		}
	}

	filenameTemplate := ""
	if c.cfg != nil && c.cfg.Original != nil {
		filenameTemplate = c.cfg.Original.GetString("download.filenameTemplate")
	}
	filename := util.BuildFilename(
		struct {
			Title     string
			ObjectId  string
			CreatedAt string
			Contact   struct {
				Nickname string
				Username string
			}
		}{
			Title:     info.Title,
			ObjectId:  info.VideoID,
			CreatedAt: strconv.FormatInt(time.Now().Unix(), 10),
			Contact: struct {
				Nickname string
				Username string
			}{
				Nickname: firstNonEmpty(info.AuthorNickname, douyin.SourceName),
				Username: firstNonEmpty(info.AuthorUsername, info.AuthorID),
			},
		},
		nil,
		struct{ FilenameTemplate string }{FilenameTemplate: filenameTemplate},
	)
	if strings.TrimSpace(filename) == "" {
		filename = info.Title
	}
	dir, name, err := util.ValidateAndSplitFilename(filename)
	if err != nil {
		return "", fmt.Errorf("不合法的文件名: %w", err)
	}

	finalName := name + suffix
	downloadDir := ""
	if c.cfg != nil {
		downloadDir = c.cfg.DownloadDir
	}
	finalPath := filepath.Join(downloadDir, dir)
	if err := os.MkdirAll(finalPath, 0o755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}
	counter := 1
	baseName := name
	for {
		if _, err := os.Stat(filepath.Join(finalPath, finalName)); err == nil {
			finalName = fmt.Sprintf("%s_%d%s", baseName, counter, suffix)
			counter++
		} else {
			break
		}
	}

	labels := map[string]string{
		"platform":   "douyin",
		"id":         info.VideoID,
		"title":      info.Title,
		"key":        "0",
		"spec":       "",
		"suffix":     suffix,
		"source_url": shareURL,
	}
	taskID, err := c.downloader.CreateDirect(
		&base.Request{
			URL: info.URL,
			Extra: &gopeedhttp.ReqExtra{
				Header: map[string]string{
					"User-Agent":      info.UserAgent,
					"Referer":         "https://www.douyin.com/",
					"Accept":          "*/*",
					"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
				},
			},
			Labels: labels,
		},
		&base.Options{
			Name: finalName,
			Path: finalPath,
			Extra: &gopeedhttp.OptsExtra{
				Connections: 4,
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("创建任务失败: %w", err)
	}

	task := c.downloader.GetTask(taskID)
	content, err := c.upsertDouyinContent(info, shareURL)
	if err != nil {
		if c.logger != nil {
			c.logger.Warn().Err(err).Msg("upsert douyin content failed, continuing without DB records")
		}
	} else if task != nil && content != nil {
		if _, err := c.CreateContentDownloadTask(content, task, "frontend"); err != nil {
			if c.logger != nil {
				c.logger.Warn().Err(err).Msg("CreateContentDownloadTask failed")
			}
		}
	}

	if task != nil && c.downloader_ws != nil {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: map[string]interface{}{
				"task": task,
			},
		})
	}

	return taskID, nil
}

func (c *APIClient) upsertDouyinContent(info *douyin.VideoInfo, sourceURL string) (*model.Content, error) {
	if c.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if info == nil {
		return nil, fmt.Errorf("douyin video info is nil")
	}

	now := util.NowMillis()
	accountExternalID := firstNonEmpty(info.AuthorID, info.AuthorSecID, info.AuthorUsername, "douyin_"+info.VideoID)
	accountUsername := firstNonEmpty(info.AuthorUsername, info.AuthorSecID, accountExternalID)
	accountNickname := firstNonEmpty(info.AuthorNickname, accountUsername)

	var content model.Content
	err := c.db.Transaction(func(tx *gorm.DB) error {
		var account model.Account
		err := tx.Where("platform_id = ? AND external_id = ?", "douyin", accountExternalID).First(&account).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			account = model.Account{
				Id:         "douyin:" + accountExternalID,
				PlatformId: "douyin",
				ExternalId: accountExternalID,
				Username:   accountUsername,
				Nickname:   accountNickname,
				AvatarURL:  info.AuthorAvatarURL,
				ProfileURL: douyinProfileURL(accountUsername),
				Timestamps: model.Timestamps{
					CreatedAt: now,
					UpdatedAt: now,
				},
			}
			if err := tx.Create(&account).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(&account).Updates(map[string]any{
				"username":    accountUsername,
				"nickname":    accountNickname,
				"avatar_url":  info.AuthorAvatarURL,
				"profile_url": douyinProfileURL(accountUsername),
				"updated_at":  now,
			}).Error; err != nil {
				return err
			}
		}

		pub := time.Now().Unix()
		err = tx.Where("platform_id = ? AND external_id = ?", "douyin", info.VideoID).First(&content).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			content = model.Content{
				Id:          "douyin:" + info.VideoID,
				PlatformId:  "douyin",
				ContentType: "video",
				ExternalId:  info.VideoID,
				ExternalId2: info.AuthorSecID,
				Title:       info.Title,
				Description: info.Title,
				ContentURL:  info.URL,
				URL:         info.URL,
				SourceURL:   sourceURL,
				CoverURL:    info.CoverURL,
				PublishTime: &pub,
				Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
			}
			if err := tx.Create(&content).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(&content).Updates(map[string]any{
				"content_type": "video",
				"external_id2": info.AuthorSecID,
				"title":        info.Title,
				"description":  info.Title,
				"content_url":  info.URL,
				"url":          info.URL,
				"source_url":   sourceURL,
				"cover_url":    info.CoverURL,
				"publish_time": &pub,
				"updated_at":   now,
			}).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("content_id = ? AND account_id <> ? AND role = ?", content.Id, account.Id, "owner").Delete(&model.ContentAccount{}).Error; err != nil {
			return err
		}
		var link model.ContentAccount
		err = tx.Where("content_id = ? AND account_id = ?", content.Id, account.Id).First(&link).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&model.ContentAccount{
				ContentId: content.Id,
				AccountId: account.Id,
				Role:      "owner",
				CreatedAt: now,
			}).Error
		}
		if err != nil {
			return err
		}
		if link.Role != "owner" {
			return tx.Model(&model.ContentAccount{}).Where("content_id = ? AND account_id = ?", content.Id, account.Id).Update("role", "owner").Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &content, nil
}

func upsertContentAccount(tx *gorm.DB, next model.Account, now int64) (*model.Account, error) {
	next.ExternalId = strings.TrimSpace(next.ExternalId)
	if next.ExternalId == "" {
		return nil, fmt.Errorf("account external_id is empty")
	}
	next.PlatformId = strings.TrimSpace(next.PlatformId)
	if next.PlatformId == "" {
		return nil, fmt.Errorf("account platform_id is empty")
	}
	var account model.Account
	err := tx.Where("platform_id = ? AND external_id = ?", next.PlatformId, next.ExternalId).First(&account).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if next.Id == "" {
			next.Id = next.PlatformId + ":" + next.ExternalId
		}
		if next.CreatedAt == 0 {
			next.CreatedAt = now
		}
		next.UpdatedAt = now
		if err := tx.Create(&next).Error; err != nil {
			return nil, err
		}
		return &next, nil
	}
	updates := map[string]any{
		"username":    next.Username,
		"alias":       next.Alias,
		"nickname":    next.Nickname,
		"avatar_url":  next.AvatarURL,
		"profile_url": next.ProfileURL,
		"updated_at":  now,
	}
	if err := tx.Model(&model.Account{}).Where("id = ?", account.Id).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := tx.First(&account, account.Id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func upsertContentByPlatformExternalID(tx *gorm.DB, content *model.Content, now int64) error {
	content.PlatformId = strings.TrimSpace(content.PlatformId)
	content.ExternalId = strings.TrimSpace(content.ExternalId)
	if content.PlatformId == "" {
		return fmt.Errorf("content platform_id is empty")
	}
	if content.ExternalId == "" {
		return fmt.Errorf("content external_id is empty")
	}

	var existing model.Content
	err := tx.Where("platform_id = ? AND external_id = ?", content.PlatformId, content.ExternalId).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if content.Id == "" {
			content.Id = content.PlatformId + ":" + content.ExternalId
		}
		if content.CreatedAt == 0 {
			content.CreatedAt = now
		}
		content.UpdatedAt = now
		return tx.Create(content).Error
	}

	content.Id = existing.Id
	updates := map[string]any{
		"content_type": content.ContentType,
		"external_id2": content.ExternalId2,
		"external_id3": content.ExternalId3,
		"title":        content.Title,
		"description":  content.Description,
		"content_url":  content.ContentURL,
		"url":          content.URL,
		"source_url":   content.SourceURL,
		"cover_url":    content.CoverURL,
		"cover_width":  content.CoverWidth,
		"cover_height": content.CoverHeight,
		"metadata":     content.Metadata,
		"publish_time": content.PublishTime,
		"update_time":  content.UpdateTime,
		"file_size":    content.FileSize,
		"size":         content.Size,
		"duration":     content.Duration,
		"updated_at":   now,
	}
	if content.DownloadTaskId != nil {
		updates["download_task_id"] = content.DownloadTaskId
	}
	if content.DownloadTaskId != nil || content.DownloadStatus != 0 {
		updates["download_status"] = content.DownloadStatus
	}
	if strings.TrimSpace(content.DownloadPath) != "" {
		updates["download_path"] = content.DownloadPath
	}
	if content.DownloadTime != nil {
		updates["download_time"] = content.DownloadTime
	}
	if strings.TrimSpace(content.ErrorMsg) != "" || content.DownloadStatus == 5 {
		updates["error_msg"] = content.ErrorMsg
	}
	return tx.Model(&model.Content{}).Where("id = ?", existing.Id).Updates(updates).Error
}

func upsertContentArticle(tx *gorm.DB, contentID string, article model.ContentArticle) error {
	if contentID == "" {
		return fmt.Errorf("content id is empty")
	}
	article.ContentId = contentID
	var existing model.ContentArticle
	err := tx.Where("content_id = ?", contentID).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&article).Error
	}
	return tx.Model(&model.ContentArticle{}).Where("content_id = ?", contentID).Updates(map[string]any{
		"word_count":       article.WordCount,
		"reading_time":     article.ReadingTime,
		"content_text":     article.ContentText,
		"content_html":     article.ContentHTML,
		"content_markdown": article.ContentMarkdown,
		"author_name":      article.AuthorName,
		"publish_platform": article.PublishPlatform,
	}).Error
}

func upsertContentOwner(tx *gorm.DB, contentID string, accountID string, now int64) error {
	if contentID == "" || accountID == "" {
		return nil
	}
	if err := tx.Where("content_id = ? AND account_id <> ? AND role = ?", contentID, accountID, "owner").Delete(&model.ContentAccount{}).Error; err != nil {
		return err
	}
	var link model.ContentAccount
	err := tx.Where("content_id = ? AND account_id = ?", contentID, accountID).First(&link).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&model.ContentAccount{
			ContentId: contentID,
			AccountId: accountID,
			Role:      "owner",
			CreatedAt: now,
		}).Error
	}
	if err != nil {
		return err
	}
	if link.Role != "owner" {
		return tx.Model(&model.ContentAccount{}).Where("content_id = ? AND account_id = ?", contentID, accountID).Update("role", "owner").Error
	}
	return nil
}

func douyinProfileURL(username string) string {
	if strings.TrimSpace(username) == "" {
		return ""
	}
	return "https://www.douyin.com/user/" + url.PathEscape(username)
}

func zhihuUserProfileURL(user zhihu.User) string {
	if strings.TrimSpace(user.URL) != "" {
		return user.URL
	}
	token := firstNonEmpty(user.URLToken, user.URLTokenSnake)
	if token == "" {
		return ""
	}
	return "https://www.zhihu.com/people/" + url.PathEscape(token)
}

type DownloadTaskPayload struct {
	URL      string
	Filename string
	Dir      string
	Extra    map[string]string
}


func (c *APIClient) createDownloadTask(body DownloadTaskPayload) (string, int, string) {
	body.URL = strings.TrimSpace(body.URL)
	if body.URL == "" {
		return "", 400, "缺少 url 参数"
	}
	if c.downloader == nil {
		return "", 500, "请先初始化 downloader"
	}

	// Extract article_id for officialaccount URLs
	articleID := officialaccountdownload.ExtractArticleID(body.URL)

	tasks := c.downloader.GetTasks()
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil {
			continue
		}
		// For officialaccount URLs, compare by article_id label
		if articleID != "" && t.Meta.Req.Labels != nil && t.Meta.Req.Labels["article_id"] == articleID {
			return "", 409, "已存在该下载内容"
		}
		// For other URLs, compare by URL directly
		if articleID == "" && t.Meta.Req.URL == body.URL {
			return "", 409, "已存在该下载内容"
		}
	}

	labels := cloneDownloadTaskExtra(body.Extra)
	if labels == nil {
		labels = make(map[string]string)
	}
	if articleID != "" {
		labels["article_id"] = articleID
		// Pass filename template to the officialaccount fetcher
		filenameTemplate := c.cfg.Original.GetString("download.filenameTemplate")
		if filenameTemplate != "" {
			labels["filename_template"] = filenameTemplate
		}
	}

	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL:    body.URL,
			Labels: labels,
		},
		&base.Options{
			Name: body.Filename,
			Path: filepath.Join(c.cfg.DownloadDir, body.Dir),
			Extra: &gopeedhttp.OptsExtra{
				Connections: 1,
			},
		},
	)
	if err != nil {
		return "", 500, "创建任务失败：" + err.Error()
	}
	c.broadcastCreatedDownloadTask(id)
	return id, 0, ""
}

func (c *APIClient) handleCreateDownloadTask(ctx *gin.Context) {
	var body DownloadTaskPayload
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}

	articleID := officialaccountdownload.ExtractArticleID(body.URL)

	tasks := c.downloader.GetTasks()
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil {
			continue
		}
		if articleID != "" && t.Meta.Req.Labels != nil && t.Meta.Req.Labels["article_id"] == articleID {
			result.Err(ctx, 409, "已存在该下载内容")
			return
		}
		if articleID == "" && t.Meta.Req.URL == body.URL {
			result.Err(ctx, 409, "已存在该下载内容")
			return
		}
	}

	labels := body.Extra
	if labels == nil {
		labels = make(map[string]string)
	}
	if articleID != "" {
		labels["article_id"] = articleID
	}

	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL:    body.URL,
			Labels: labels,
		},
		&base.Options{
			Name: body.Filename,
			Path: filepath.Join(c.cfg.DownloadDir, body.Dir),
			Extra: &gopeedhttp.OptsExtra{
				Connections: 1,
			},
		},
	)
	if err != nil {
		result.Err(ctx, 500, "创建任务失败："+err.Error())
		return
	}
	task := c.downloader.GetTask(id)
	if task != nil {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: map[string]interface{}{
				"task": task,
			},
		})
	}
	result.Ok(ctx, gin.H{"id": id})
}

func (c *APIClient) handleFetchTaskList(ctx *gin.Context) {
	status := ctx.Query("status")
	pageStr := ctx.Query("page")
	pageSizeStr := ctx.Query("page_size")

	pageNum := 1
	pageSizeNum := 20
	if pageStr != "" {
		if v, err := strconv.Atoi(pageStr); err == nil && v > 0 {
			pageNum = v
		}
	}
	if pageSizeStr != "" {
		if v, err := strconv.Atoi(pageSizeStr); err == nil && v > 0 {
			pageSizeNum = v
		}
	}

	// Use service
	pageResult := c.downloadService.ListTasks(pageNum, pageSizeNum, status)
	result.Ok(ctx, pageResult)
}

type LiveDownloadTaskBody struct {
	Url       string            `json:"url"`
	Name      string            `json:"name"`
	UserAgent string            `json:"userAgent"`
	Headers   map[string]string `json:"headers"`
}

func (c *APIClient) handleCreateLiveTask(ctx *gin.Context) {
	var body LiveDownloadTaskBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Url == "" {
		result.Err(ctx, 400, "缺少 url 参数")
		return
	}

	name := body.Name
	if name == "" {
		u, _ := url.Parse(body.Url)
		if u != nil {
			name = filepath.Base(u.Path)
		}
		if name == "" || name == "." || name == "/" {
			name = fmt.Sprintf("live_%d.mp4", time.Now().Unix())
		}
	}
	if !strings.HasSuffix(name, ".mp4") && !strings.HasSuffix(name, ".ts") && !strings.HasSuffix(name, ".flv") && !strings.HasSuffix(name, ".mkv") {
		name += ".mp4"
	}

	reqExtra := &gopeedstream.ReqExtra{
		Header: make(map[string]string),
	}
	if body.UserAgent != "" {
		reqExtra.Header["User-Agent"] = body.UserAgent
	}
	for k, v := range body.Headers {
		reqExtra.Header[k] = v
	}

	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL:   body.Url,
			Extra: reqExtra,
			Labels: map[string]string{
				"type": "live",
			},
		},
		&base.Options{
			Name: name,
			Path: c.cfg.DownloadDir,
		},
	)
	if err != nil {
		result.Err(ctx, 500, "创建任务失败: "+err.Error())
		return
	}
	task := c.downloader.GetTask(id)
	if task != nil {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: map[string]interface{}{
				"task": task,
			},
		})
	}
	result.Ok(ctx, gin.H{"id": id})
}

func (c *APIClient) handleBatchCreateTask(ctx *gin.Context) {
	var body struct {
		Feeds []ChannelsDownloadRequest `json:"feeds"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}

	var ids []string
	for _, req := range body.Feeds {
		id, err := c.startDownloadChannelsObject(&req)
		if err != nil {
			c.logger.Warn().Err(err).Interface("req", req).Msg("批量创建任务跳过一项")
			continue
		}
		ids = append(ids, id)
	}

	result.Ok(ctx, gin.H{"ids": ids})
}

type DownloadTaskCreateFailure struct {
	URL     string `json:"url"`
	Code    int    `json:"code,omitempty"`
	Message string `json:"message"`
}


func cloneDownloadTaskExtra(extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return nil
	}
	clone := make(map[string]string, len(extra))
	for k, v := range extra {
		clone[k] = v
	}
	return clone
}

func applyDownloadTaskDefaults(task DownloadTaskPayload, dir string, extra map[string]string) DownloadTaskPayload {
	task.URL = strings.TrimSpace(task.URL)
	if task.Dir == "" {
		task.Dir = dir
	}
	labels := cloneDownloadTaskExtra(extra)
	for k, v := range task.Extra {
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[k] = v
	}
	task.Extra = labels
	return task
}

func parseDownloadTaskText(text string, dir string, extra map[string]string) []DownloadTaskPayload {
	var tasks []DownloadTaskPayload
	normalizedText := strings.ReplaceAll(text, "\r\n", "\n")
	for _, line := range strings.Split(normalizedText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "{") {
			var task DownloadTaskPayload
			if err := json.Unmarshal([]byte(line), &task); err == nil && strings.TrimSpace(task.URL) != "" {
				tasks = append(tasks, applyDownloadTaskDefaults(task, dir, extra))
				continue
			}
		}
		tasks = append(tasks, DownloadTaskPayload{
			URL:   line,
			Dir:   dir,
			Extra: cloneDownloadTaskExtra(extra),
		})
	}
	return tasks
}

func appendDownloadTaskPayload(tasks []DownloadTaskPayload, task DownloadTaskPayload, dir string, extra map[string]string) []DownloadTaskPayload {
	task = applyDownloadTaskDefaults(task, dir, extra)
	if task.URL == "" {
		return tasks
	}
	return append(tasks, task)
}

func (c *APIClient) broadcastCreatedDownloadTask(id string) {
	task := c.downloader.GetTask(id)
	if task == nil {
		return
	}
	c.downloader_ws.Broadcast(APIClientWSMessage{
		Type: "event",
		Data: map[string]interface{}{
			"task":          task,
			"status_counts": c.downloadTaskStatusCounts(),
		},
	})
}


type DownloadTaskBatchPayload struct {
	Text  string                `json:"text"`
	URLs  []string              `json:"urls"`
	Tasks []DownloadTaskPayload `json:"tasks"`
	Dir   string                `json:"dir"`
	Extra map[string]string     `json:"extra"`
}

// 批量创建常规下载任务
func (c *APIClient) handleBatchCreateDownloadTask(ctx *gin.Context) {
	var body DownloadTaskBatchPayload
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}

	var tasks []DownloadTaskPayload
	for _, task := range body.Tasks {
		tasks = appendDownloadTaskPayload(tasks, task, body.Dir, body.Extra)
	}
	for _, rawURL := range body.URLs {
		tasks = appendDownloadTaskPayload(tasks, DownloadTaskPayload{URL: rawURL}, body.Dir, body.Extra)
	}
	tasks = append(tasks, parseDownloadTaskText(body.Text, body.Dir, body.Extra)...)
	if len(tasks) == 0 {
		result.Err(ctx, 400, "请提供下载任务")
		return
	}

	seen := make(map[string]bool)
	ids := make([]string, 0, len(tasks))
	skipped := make([]DownloadTaskCreateFailure, 0)
	failed := make([]DownloadTaskCreateFailure, 0)
	for _, task := range tasks {
		if seen[task.URL] {
			skipped = append(skipped, DownloadTaskCreateFailure{
				URL:     task.URL,
				Code:    409,
				Message: "重复的下载地址",
			})
			continue
		}
		seen[task.URL] = true
		id, code, msg := c.createDownloadTask(task)
		if code == 0 {
			ids = append(ids, id)
			continue
		}
		item := DownloadTaskCreateFailure{
			URL:     task.URL,
			Code:    code,
			Message: msg,
		}
		if code == 409 {
			skipped = append(skipped, item)
			continue
		}
		failed = append(failed, item)
	}
	if len(ids) == 0 && len(skipped) == 0 && len(failed) > 0 {
		result.Err(ctx, failed[0].Code, failed[0].Message)
		return
	}
	result.Ok(ctx, gin.H{
		"ids":     ids,
		"skipped": skipped,
		"failed":  failed,
	})
}


func (c *APIClient) handleDownloadAllOfficialAccountMsgs(ctx *gin.Context) {
	var body struct {
		Biz        string `json:"biz"`
		Uin        string `json:"uin"`
		Key        string `json:"key"`
		PassTicket string `json:"pass_ticket"`
		Token      string `json:"token"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "参数错误")
		return
	}
	if valid := c.official.ValidateToken(body.Token); !valid {
		result.Err(ctx, 401, "token 无效")
		return
	}
	if body.Biz == "" {
		result.Err(ctx, 400, "缺少 biz 参数")
		return
	}
	acct := &officialaccountdownload.OfficialAccount{
		Biz:        body.Biz,
		Uin:        body.Uin,
		Key:        body.Key,
		PassTicket: body.PassTicket,
	}
	urls, err := c.official.FetchAllMsgURLs(acct)
	if err != nil && len(urls) == 0 {
		result.Err(ctx, 500, err.Error())
		return
	}
	count := 0
	for _, u := range urls {
		taskURL := "officialaccount://" + u
		_, code, _ := c.createDownloadTask(DownloadTaskPayload{URL: taskURL})
		if code == 0 {
			count++
		}
	}
	result.Ok(ctx, gin.H{"count": count, "total": len(urls)})
}

func (c *APIClient) startDownloadChannelsObject(body *ChannelsDownloadRequest) (string, error) {
	obj := body.Object

	// 1. Live is not supported here
	if obj.LiveInfo != nil {
		return "", fmt.Errorf("直播类型请使用直播下载")
	}

	// 2. Convert to content + account for DB operations
	content, err := wxchannels.ToContent((*channels.ChannelsObject)(&obj))
	if err != nil {
		// Object validation failed; cannot proceed
		return "", fmt.Errorf("转换失败: %w", err)
	}
	account, err := wxchannels.ToAccount((*channels.ChannelsObject)(&obj))
	if err != nil {
		account = nil // non-fatal for account
	}

	// 3. Upsert Account/Content/ContentAccount in DB (non-fatal)
	if c.channelsUploadService != nil && content != nil && account != nil {
		dbContent, err := c.channelsUploadService.HandleChannelsFeed(content, account)
		if err != nil {
			c.logger.Warn().Err(err).Msg("HandleChannelsFeed failed, continuing without DB records")
		} else {
			content = dbContent
		}
	}

	// 4. Resolve spec: request override > config default
	isPicture := obj.Type == "picture" || obj.ObjectDesc.MediaType == 2

	var objMedia *apitypes.ChannelsMediaItem
	if !isPicture && len(obj.ObjectDesc.Media) > 0 {
		objMedia = &obj.ObjectDesc.Media[0]
	}

	spec := body.Spec
	if spec == "" && !c.cfg.Original.GetBool("download.defaultHighest") {
		if objMedia != nil && len(objMedia.Spec) > 0 {
			spec = objMedia.Spec[0].FileFormat
		}
	}

	title := wxchannels.ObjectTitle((*channels.ChannelsObject)(&obj))

	// 5. Build filename using the template
	filenameTemplate := c.cfg.Original.GetString("download.filenameTemplate")
	filename := util.BuildFilename(
		struct {
			Title     string
			ObjectId  string
			CreatedAt string
			Contact   struct {
				Nickname string
				Username string
			}
		}{
			Title:     title,
			ObjectId:  obj.ID,
			CreatedAt: strconv.Itoa(obj.CreateTime),
			Contact: struct {
				Nickname string
				Username string
			}{
				Nickname: obj.Contact.Nickname,
				Username: obj.Contact.Username,
			},
		},
		func() *struct{ FileFormat string } {
			if spec != "" {
				return &struct{ FileFormat string }{FileFormat: spec}
			}
			return nil
		}(),
		struct{ FilenameTemplate string }{FilenameTemplate: filenameTemplate},
	)

	// 6. Validate and split filename into dir/name
	dir, name, err := util.ValidateAndSplitFilename(filename)
	if err != nil {
		return "", fmt.Errorf("不合法的文件名: %w", err)
	}

	// 7. Determine URL and suffix
	var downloadURL string
	suffix := ".mp4"

	if isPicture {
		suffix = ".zip"
		var files []map[string]string
		for i, f := range obj.Files {
			files = append(files, map[string]string{
				"url":      f.URL + f.URLToken,
				"filename": fmt.Sprintf("%d.jpg", i+1),
			})
		}
		data, _ := json.Marshal(files)
		downloadURL = fmt.Sprintf("zip://weixin.qq.com?files=%s", url.QueryEscape(string(data)))
	} else {
		if objMedia == nil {
			return "", fmt.Errorf("缺少可下载的视频内容")
		}
		downloadURL = objMedia.URL + objMedia.URLToken

		// Apply spec to URL
		if spec != "" {
			downloadURL += "&X-snsvideoflag=" + spec
		} else {
			if u, err := url.Parse(downloadURL); err == nil {
				filekey := u.Query().Get("encfilekey")
				token := u.Query().Get("token")
				if filekey != "" && token != "" {
					newURL := u.Scheme + "://" + u.Host + u.Path
					newURL += "?encfilekey=" + filekey + "&token=" + token
					downloadURL = newURL
				}
			}
		}
	}

	// 8. Apply suffix override from request
	if body.Suffix != "" {
		suffix = body.Suffix
	}

	// 9. Dedup filename on disk
	finalName := name + suffix
	finalPath := filepath.Join(c.cfg.DownloadDir, dir)
	if err := os.MkdirAll(finalPath, 0o755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}
	counter := 1
	baseName := name
	for {
		if _, err := os.Stat(filepath.Join(finalPath, finalName)); err == nil {
			finalName = fmt.Sprintf("%s_%d%s", baseName, counter, suffix)
			counter++
		} else {
			break
		}
	}

	// 10. Dedup by external_id in active downloader tasks
	tasks := c.downloader.GetTasks()
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil || t.Meta.Req.Labels == nil {
			continue
		}
		sameID := t.Meta.Req.Labels["id"] == obj.ID
		sameSpec := t.Meta.Req.Labels["spec"] == spec
		sameSuffix := t.Meta.Req.Labels["suffix"] == suffix
		if sameID && sameSpec && sameSuffix {
			return "", fmt.Errorf("已存在该下载内容")
		}
	}

	// 11. Extract decrypt key
	key := wxchannels.DecryptKeyInt((*channels.ChannelsObject)(&obj))

	// 12. Build labels (preserves listener decrypt+mp3)
	labels := map[string]string{
		"id":       obj.ID,
		"nonce_id": obj.ObjectNonceId,
		"title":    title,
		"key":      strconv.Itoa(key),
		"spec":     spec,
		"suffix":   suffix,
	}

	// 13. Create download task
	taskID, err := c.downloader.CreateDirect(
		&base.Request{
			URL:    downloadURL,
			Labels: labels,
		},
		&base.Options{
			Name: finalName,
			Path: finalPath,
			Extra: &gopeedhttp.OptsExtra{
				Connections: 4,
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("创建任务失败: %w", err)
	}

	// 14. Link DownloadTask to Content in DB
	task := c.downloader.GetTask(taskID)
	if task != nil && content != nil {
		if _, err := c.CreateContentDownloadTask(content, task, "frontend"); err != nil {
			c.logger.Warn().Err(err).Msg("CreateContentDownloadTask failed")
		}
	}

	// 15. WS broadcast
	if task != nil {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: map[string]interface{}{
				"task": task,
			},
		})
	}

	return taskID, nil
}

func (c *APIClient) handleStartTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	// Use service
	c.downloadService.StartTask(body.Id)
	result.Ok(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handlePauseTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	// Use service
	c.downloadService.PauseTask(body.Id)
	result.Ok(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleResumeTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	// Use service
	c.downloadService.ResumeTask(body.Id)
	result.Ok(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleDeleteTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	// Use service
	c.downloadService.DeleteTask(body.Id)
	result.Ok(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleClearTasks(ctx *gin.Context) {
	// Use service
	c.downloadService.Clear()
	result.Ok(ctx, nil)
}

func (c *APIClient) handleFetchTaskProfile(ctx *gin.Context) {
	id := ctx.Query("id")
	if id == "" {
		result.Err(ctx, 400, "missing task id")
		return
	}
	// Use service
	profile, err := c.downloadService.GetTaskProfile(id)
	if err != nil {
		result.Err(ctx, 404, "task not found")
		return
	}
	result.Ok(ctx, profile)
}

func normalizeDownloadTaskStatus(status base.Status) string {
	value := strings.ToLower(strings.TrimSpace(string(status)))
	switch value {
	case "paused":
		return "pause"
	case "failed", "fail", "failure", "errored":
		return "error"
	case "pending", "waiting", "queued":
		return "wait"
	case "completed", "success", "finished":
		return "done"
	default:
		return value
	}
}

func downloadTaskStatusFilter(status string) *downloadpkg.TaskFilter {
	value := normalizeDownloadTaskStatus(base.Status(status))
	if value == "" || value == "all" {
		return nil
	}
	statuses := []base.Status{base.Status(value)}
	if value == "wait" {
		statuses = []base.Status{
			base.DownloadStatusReady,
			base.DownloadStatusWait,
		}
	}
	return &downloadpkg.TaskFilter{Statuses: statuses}
}

func downloadTaskPauseAllFilter(status string) *downloadpkg.TaskFilter {
	filter := downloadTaskStatusFilter(status)
	if filter != nil {
		return filter
	}
	return &downloadpkg.TaskFilter{
		Statuses: []base.Status{
			base.DownloadStatusReady,
			base.DownloadStatusRunning,
			base.DownloadStatusWait,
		},
	}
}

func countDownloadTaskStatuses(tasks []*downloadpkg.Task) map[string]int {
	counts := map[string]int{
		"total":   0,
		"ready":   0,
		"running": 0,
		"wait":    0,
		"pause":   0,
		"error":   0,
		"done":    0,
	}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		counts["total"]++
		status := normalizeDownloadTaskStatus(task.Status)
		if status == "" {
			continue
		}
		counts[status]++
	}
	return counts
}

func (c *APIClient) downloadTaskStatusCounts() map[string]int {
	if c == nil || c.downloader == nil {
		return countDownloadTaskStatuses(nil)
	}
	return countDownloadTaskStatuses(c.downloader.GetTasks())
}

func (c *APIClient) handleStartAllTasks(ctx *gin.Context) {
	var body struct {
		Status string `json:"status"`
	}
	if ctx.Request.Body != nil && ctx.Request.ContentLength != 0 {
		if err := ctx.ShouldBindJSON(&body); err != nil {
			result.Err(ctx, 400, "不合法的参数")
			return
		}
	}
	if err := c.downloader.Continue(downloadTaskStatusFilter(body.Status)); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}

func (c *APIClient) handlePauseAllTasks(ctx *gin.Context) {
	var body struct {
		Status string `json:"status"`
	}
	if ctx.Request.Body != nil && ctx.Request.ContentLength != 0 {
		if err := ctx.ShouldBindJSON(&body); err != nil {
			result.Err(ctx, 400, "不合法的参数")
			return
		}
	}
	if err := c.downloader.Pause(downloadTaskPauseAllFilter(body.Status)); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}
