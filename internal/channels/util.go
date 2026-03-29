package channels

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/internal/interceptor"
	"wx_channel/pkg/util"
)

type Client struct {
	Conn *websocket.Conn
	Send chan []byte
}

func (c *Client) writePump() {
	ticker := time.NewTicker(5 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

type APIClientWSMessage struct {
	Type  string      `json:"type"`
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

type ClientWSMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}
type ClientWebsocketRequestBody struct {
	ID   string      `json:"id"`
	Key  string      `json:"key"`
	Body interface{} `json:"data"`
}
type ClientWebsocketResponse struct {
	Id string `json:"id"`
	// 调用 wx api 原始响应
	Data json.RawMessage `json:"data"`
}

type BrowseHistoryCreator interface {
	CreateBrowseHistory(browse *model.BrowseHistory) error
}

func CreateBrowseHistoryFromProfile(db *gorm.DB, logger *zerolog.Logger, creator BrowseHistoryCreator, profile *interceptor.ChannelMediaProfile) error {
	if db == nil {
		return errors.New("db is nil")
	}
	if creator == nil {
		return errors.New("creator is nil")
	}
	if profile == nil || profile.Id == "" {
		return errors.New("profile is nil")
	}

	now := util.NowMillis()
	extraDataBytes, _ := json.Marshal(map[string]any{
		"nonce_id":   profile.NonceId,
		"decode_key": profile.Key,
	})
	contentType := "video"
	if profile.Type == "picture" {
		contentType = "image"
	} else if profile.Type == "live" {
		contentType = "live"
	}

	normalizeUint64String := func(value string) string {
		s := strings.TrimSpace(value)
		if s == "" {
			return ""
		}
		head := s
		if idx := strings.IndexByte(s, '_'); idx >= 0 {
			head = s[:idx]
		}
		if head == "" {
			return ""
		}
		for i := 0; i < len(head); i++ {
			ch := head[i]
			if ch < '0' || ch > '9' {
				return ""
			}
		}
		return head
	}

	buildSourceURL := func() string {
		u := url.URL{
			Scheme: "https",
			Host:   "channels.weixin.qq.com",
		}
		if contentType == "live" {
			u.Path = "/web/pages/live"
		} else {
			u.Path = "/web/pages/feed"
		}
		q := u.Query()
		if profile.Contact.Id != "" {
			q.Set("username", profile.Contact.Id)
		}
		rawOid := normalizeUint64String(profile.Id)
		if rawOid != "" {
			if oid := util.EncodeUint64ToBase64(rawOid); oid != "" {
				q.Set("oid", oid)
			}
		}
		if contentType != "live" {
			rawNid := normalizeUint64String(profile.NonceId)
			if rawNid != "" {
				if nid := util.EncodeUint64ToBase64(rawNid); nid != "" {
					q.Set("nid", nid)
				}
			}
		}
		u.RawQuery = q.Encode()
		return u.String()
	}
	sourceURL := buildSourceURL()

	var accountId *int
	var influencerId *int
	if profile.Contact.Id != "" {
		var acc model.Account
		err := db.Where("platform_id = ? AND external_id = ?", "wx_channels", profile.Contact.Id).First(&acc).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			if logger != nil {
				logger.Error().Err(err).Str("account_external_id", profile.Contact.Id).Msg("fetch account failed")
			}
		} else {
			if acc.Id == 0 {
				acc = model.Account{
					PlatformId:   "wx_channels",
					ExternalId:   profile.Contact.Id,
					Username:     profile.Contact.Id,
					Nickname:     profile.Contact.Nickname,
					AvatarURL:    profile.Contact.AvatarURL,
					InfluencerId: nil,
					Timestamps: model.Timestamps{
						CreatedAt: now,
						UpdatedAt: now,
					},
				}
				if err := db.Create(&acc).Error; err != nil {
					var existing model.Account
					if err2 := db.Where("platform_id = ? AND external_id = ?", "wx_channels", profile.Contact.Id).First(&existing).Error; err2 == nil {
						acc = existing
					} else {
						if logger != nil {
							logger.Error().Err(err).Str("account_external_id", profile.Contact.Id).Msg("create account failed")
						}
					}
				}
			} else {
				updates := map[string]any{
					"updated_at": now,
				}
				if profile.Contact.Nickname != "" && profile.Contact.Nickname != acc.Nickname {
					updates["nickname"] = profile.Contact.Nickname
				}
				if profile.Contact.AvatarURL != "" && profile.Contact.AvatarURL != acc.AvatarURL {
					updates["avatar_url"] = profile.Contact.AvatarURL
				}
				if len(updates) > 1 {
					_ = db.Model(&model.Account{}).Where("id = ?", acc.Id).Updates(updates).Error
					_ = db.First(&acc, acc.Id).Error
				}
			}
			if acc.Id > 0 {
				accountId = &acc.Id
				if acc.InfluencerId != nil {
					influencerId = acc.InfluencerId
				}
			}
		}
	}

	browse := model.BrowseHistory{
		PlatformId:        "wx_channels",
		VisitedTimes:      1,
		AccountId:         accountId,
		InfluencerId:      influencerId,
		ContentType:       contentType,
		ContentExternalId: profile.Id,
		ContentTitle:      profile.Title,
		ContentURL:        profile.URL,
		ContentSourceURL:  sourceURL,
		ContentCoverURL:   profile.CoverURL,
		ExtraData:         string(extraDataBytes),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	return creator.CreateBrowseHistory(&browse)
}
