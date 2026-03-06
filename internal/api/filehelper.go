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

	"wx_channel/pkg/filehelper"
)

// FinderAutoDownloadCallback 视频号自动下载回调
type FinderAutoDownloadCallback func(objectID, objectNonceID string) error

// FileHelperHandler 文件传输助手处理器
type FileHelperHandler struct {
	client               *filehelper.Client
	mu                   sync.RWMutex
	onFinderAutoDownload FinderAutoDownloadCallback
}

// NewFileHelperHandler 创建处理器
func NewFileHelperHandler() *FileHelperHandler {
	return &FileHelperHandler{}
}

// SetFinderAutoDownloadCallback 设置视频号自动下载回调
func (h *FileHelperHandler) SetFinderAutoDownloadCallback(cb FinderAutoDownloadCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onFinderAutoDownload = cb
}

// GetClient 获取或创建客户端
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

// HandlePage 返回前端页面
// GET /filehelper
func (h *FileHelperHandler) HandlePage(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, string(files.HTMLFilehelper))
}

// HandleGetQRCode 获取登录二维码
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

// HandleWaitLogin 等待登录（阻塞接口）
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
		// 登录成功，启动同步检查
		go client.StartSyncCheck()
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "登录成功",
			"data": gin.H{
				"status": "logged_in",
			},
		})

	case 201:
		// 已扫码，等待确认
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "已扫码，等待确认",
			"data": gin.H{
				"status":      "scanned",
				"user_avatar": data,
			},
		})

	case 400:
		// 二维码过期
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "二维码已过期",
			"data": gin.H{
				"status": "expired",
			},
		})

	case 408:
		// 等待扫码，继续轮询
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

// HandleGetStatus 获取登录状态
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

// HandleGetMessages 获取消息列表
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

// HandleSyncMessages 同步消息（返回完整响应）
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

	// 检查是否启用自动下载
	if viper.GetBool("filehelper.enabled") && resp != nil && len(resp.AddMsgList) > 0 {
		h.processFinderMessages(resp.AddMsgList)
	}

	c.JSON(http.StatusOK, resp)
}

// processFinderMessages 处理视频号消息，自动创建下载任务
func (h *FileHelperHandler) processFinderMessages(messages []map[string]interface{}) {
	h.mu.RLock()
	callback := h.onFinderAutoDownload
	h.mu.RUnlock()

	if callback == nil {
		return
	}

	for _, msg := range messages {
		// 检查 MsgType 是否为 49（应用消息）
		msgType, ok := msg["MsgType"].(float64)
		if !ok || int(msgType) != 49 {
			continue
		}

		// 获取消息内容
		content, ok := msg["Content"].(string)
		if !ok || content == "" {
			continue
		}

		// 解析视频号消息
		finderData, err := parseFinderFeed(content)
		if err != nil || finderData == nil {
			continue
		}

		// 检查是否包含必要的字段
		if finderData.ObjectID == "" || finderData.ObjectNonceID == "" {
			continue
		}

		// 调用回调创建下载任务
		go callback(finderData.ObjectID, finderData.ObjectNonceID)
	}
}

// HandleSyncCheck 阻塞等待同步检查
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

// HandleSendMessage 发送消息
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

// HandleLogout 登出
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

// FinderFeedData 视频号消息解析结果
type FinderFeedData struct {
	Username      string `json:"username"`
	Nickname      string `json:"nickname"`
	Desc          string `json:"desc"`
	Avatar        string `json:"avatar"`
	ThumbURL      string `json:"thumb_url"`
	ObjectID      string `json:"object_id"`
	ObjectNonceID string `json:"object_nonce_id"`
}

// finderFeedXML 视频号 XML 结构
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

// HandleParseFinderFeed 解析视频号消息
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

// parseFinderFeed 解析视频号 XML 内容
func parseFinderFeed(content string) (*FinderFeedData, error) {
	// 解码 HTML 实体
	decoded := html.UnescapeString(content)
	// 移除 <br/> 标签
	decoded = regexp.MustCompile(`<br\s*/?>`).ReplaceAllString(decoded, "")

	// 提取 finderFeed 节点
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
