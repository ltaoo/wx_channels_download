package api

import (
	"encoding/xml"
	"html"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"wx_channel/frontend"
	"wx_channel/pkg/filehelper"
)

// FinderAutoDownloadCallback is the auto-download callback for Channels (Finder).
type FinderAutoDownloadCallback func(objectID, objectNonceID string) error

// SphAutoDownloadCallback is the auto-download callback for SPH videos.
type SphAutoDownloadCallback func(sphUrl string) error

// FileHelperHandler is the file transfer helper processor.
type FileHelperHandler struct {
	client               *filehelper.Client
	mu                   sync.RWMutex
	onFinderAutoDownload FinderAutoDownloadCallback
	onSphAutoDownload    SphAutoDownloadCallback
}

// NewFileHelperHandler creates a new FileHelperHandler.
func NewFileHelperHandler() *FileHelperHandler {
	return &FileHelperHandler{}
}

// SetFinderAutoDownloadCallback sets the auto-download callback for Channels.
func (h *FileHelperHandler) SetFinderAutoDownloadCallback(cb FinderAutoDownloadCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onFinderAutoDownload = cb
}

// SetSphAutoDownloadCallback sets the auto-download callback for SPH.
func (h *FileHelperHandler) SetSphAutoDownloadCallback(cb SphAutoDownloadCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onSphAutoDownload = cb
}

// GetClient returns or creates a filehelper client.
func (h *FileHelperHandler) GetClient() *filehelper.Client {
	h.mu.RLock()
	if h.client != nil {
		defer h.mu.RUnlock()
		return h.client
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.client != nil {
		return h.client
	}

	cfg := &filehelper.Config{
		CallbackURL: viper.GetString("filehelper.callbackUrl"),
	}
	logger := h.getLogger()
	h.client = filehelper.NewClient(cfg, logger)
	return h.client
}

func (h *FileHelperHandler) getLogger() *zerolog.Logger {
	nopLogger := zerolog.Nop()
	return &nopLogger
}

// HandlePage serves the frontend page.
// GET /filehelper
func (h *FileHelperHandler) HandlePage(c *gin.Context) {
	data, err := frontend.Assets.ReadRoot("filehelper.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "filehelper page not found")
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, string(data))
}

// HandleGetQRCode returns a login QR code.
// GET /api/filehelper/qrcode
func (h *FileHelperHandler) HandleGetQRCode(c *gin.Context) {
	client := h.GetClient()

	qrcodeURL, err := client.GetQRCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"qrcode_url": qrcodeURL,
			"uuid":       client.GetUUID(),
		},
	})
}

// HandleWaitLogin waits for login (blocking endpoint).
// GET /api/filehelper/login/wait
func (h *FileHelperHandler) HandleWaitLogin(c *gin.Context) {
	client := h.GetClient()

	code, data, err := client.WaitForLogin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	switch code {
	case 200:
		// Login successful, start sync check
		go client.StartSyncCheck()
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "登录成功",
			"data": gin.H{
				"status": "logged_in",
			},
		})

	case 201:
		// Already scanned, waiting for confirmation
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "已扫码，等待确认",
			"data": gin.H{
				"status":      "scanned",
				"user_avatar": data,
			},
		})

	case 400:
		// QR code expired
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "二维码已过期",
			"data": gin.H{
				"status": "expired",
			},
		})

	case 408:
		// Waiting for scan, keep polling
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "等待扫码",
			"data": gin.H{
				"status": "waiting",
			},
		})

	default:
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "未知状态",
			"data": gin.H{
				"status": "unknown",
				"code":   code,
			},
		})
	}
}

// HandleGetStatus returns login status.
// GET /api/filehelper/status
func (h *FileHelperHandler) HandleGetStatus(c *gin.Context) {
	client := h.GetClient()
	detail := client.GetLoginStatusDetail()

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": detail,
	})
}

// HandleGetMessages returns the message list.
// GET /api/filehelper/messages
func (h *FileHelperHandler) HandleGetMessages(c *gin.Context) {
	client := h.GetClient()

	messages := client.GetLatestMessages(50)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"messages": messages,
		},
	})
}

// HandleSyncMessages syncs messages (returns full response).
// GET /api/filehelper/sync
func (h *FileHelperHandler) HandleSyncMessages(c *gin.Context) {
	client := h.GetClient()

	if !client.IsLoggedIn() {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "未登录",
		})
		return
	}

	resp, err := client.SyncMessages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	// Check if auto-download is enabled
	if viper.GetBool("filehelper.enabled") && resp != nil && len(resp.AddMsgList) > 0 {
		h.processFinderMessages(resp.AddMsgList)
	}

	c.JSON(http.StatusOK, resp)
}

// processFinderMessages processes Channels messages and auto-creates download tasks.
func (h *FileHelperHandler) processFinderMessages(messages []map[string]interface{}) {
	h.mu.RLock()
	finderCallback := h.onFinderAutoDownload
	sphCallback := h.onSphAutoDownload
	h.mu.RUnlock()

	for _, msg := range messages {
		msgType, ok := msg["MsgType"].(float64)
		if !ok {
			continue
		}

		// Process app message (Channels / Finder)
		if int(msgType) == 49 {
		// Check if MsgType is 49 (app message)
			content, ok := msg["Content"].(string)
			if !ok || content == "" {
				continue
			}

			// Parse Channels message
			finderData, err := parseFinderFeed(content)
			if err != nil || finderData == nil {
				continue
			}

			// Check required fields
			if finderData.ObjectID == "" || finderData.ObjectNonceID == "" {
				continue
			}

			// Call callback to create download task
			if finderCallback != nil {
				go finderCallback(finderData.ObjectID, finderData.ObjectNonceID)
			}
		}

		// Process text message
		if int(msgType) == 1 {
			content, ok := msg["Content"].(string)
			if !ok || content == "" {
				continue
			}

			// Extract SPH URL
			sphUrl := extractSphUrl(content)
			if sphUrl != "" && sphCallback != nil {
				go sphCallback(sphUrl)
			}
		}
	}
}

// HandleSyncCheck blocks and waits for sync check.
// GET /api/filehelper/synccheck
func (h *FileHelperHandler) HandleSyncCheck(c *gin.Context) {
	client := h.GetClient()

	if !client.IsLoggedIn() {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "未登录",
		})
		return
	}

	status, err := client.WaitSyncCheck()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"status": status,
		},
	})
}

// HandleSendMessage sends a message.
// POST /api/filehelper/send
func (h *FileHelperHandler) HandleSendMessage(c *gin.Context) {
	var body struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	client := h.GetClient()

	if !client.IsLoggedIn() {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "未登录",
		})
		return
	}

	if err := client.SendText(body.Text); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "发送成功",
	})
}

// HandleLogout logs out.
// POST /api/filehelper/logout
func (h *FileHelperHandler) HandleLogout(c *gin.Context) {
	client := h.GetClient()

	if err := client.Logout(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "已登出",
	})
}

// FinderFeedData holds parsed Channels message data.
type FinderFeedData struct {
	Username      string `json:"username"`
	Nickname      string `json:"nickname"`
	Desc          string `json:"desc"`
	Avatar        string `json:"avatar"`
	ThumbURL      string `json:"thumb_url"`
	ObjectID      string `json:"object_id"`
	ObjectNonceID string `json:"object_nonce_id"`
}

// finderFeedXML holds the Channels (Finder) XML structure.
type finderFeedXML struct {
	XMLName       xml.Name `xml:"finderFeed"`
	Username      string   `xml:"username"`
	Nickname      string   `xml:"nickname"`
	Avatar        string   `xml:"avatar"`
	Desc          string   `xml:"desc"`
	ObjectID      string   `xml:"objectId"`
	ObjectNonceID string   `xml:"objectNonceId"`
	MediaList     struct {
		Media struct {
			ThumbURL string `xml:"thumbUrl"`
		} `xml:"media"`
	} `xml:"mediaList"`
}

// HandleParseFinderFeed parses a Channels message.
// POST /api/filehelper/parse_finder_feed
func (h *FileHelperHandler) HandleParseFinderFeed(c *gin.Context) {
	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	if body.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "content 不能为空",
		})
		return
	}

	data, err := parseFinderFeed(body.Content)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  err.Error(),
			"data": nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": data,
	})
}

// parseFinderFeed parses Channels XML content.
func parseFinderFeed(content string) (*FinderFeedData, error) {
	// Decode HTML entities
	decoded := html.UnescapeString(content)
	// Remove <br/> tags
	decoded = regexp.MustCompile(`<br\s*/?>`).ReplaceAllString(decoded, "")

	// Extract finderFeed node
	startIdx := strings.Index(decoded, "<finderFeed>")
	endIdx := strings.Index(decoded, "</finderFeed>")
	if startIdx == -1 || endIdx == -1 {
		return nil, nil
	}
	xmlContent := decoded[startIdx : endIdx+len("</finderFeed>")]

	var feed finderFeedXML
	if err := xml.Unmarshal([]byte(xmlContent), &feed); err != nil {
		return nil, err
	}

	return &FinderFeedData{
		Username:      strings.TrimSpace(feed.Username),
		Nickname:      strings.TrimSpace(feed.Nickname),
		Desc:          strings.TrimSpace(feed.Desc),
		Avatar:        strings.TrimSpace(feed.Avatar),
		ThumbURL:      strings.TrimSpace(feed.MediaList.Media.ThumbURL),
		ObjectID:      strings.TrimSpace(feed.ObjectID),
		ObjectNonceID: strings.TrimSpace(feed.ObjectNonceID),
	}, nil
}

// extractSphUrl extracts an SPH URL from text.
func extractSphUrl(content string) string {
	// Match URLs in the format https://weixin.qq.com/sph/... with possible surrounding whitespace
	pattern := `https://weixin\.qq\.com/sph/\w+`
	re := regexp.MustCompile(pattern)
	matches := re.FindAllString(content, -1)
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}
