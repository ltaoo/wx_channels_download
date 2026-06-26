package master

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

const robotType = "2"

type Config struct {
	Enabled           bool
	BaseURL           string
	Token             string
	Robot             string
	Name              string
	HeartbeatInterval time.Duration
}

type Client struct {
	cfg        Config
	httpClient *http.Client
	logger     *zerolog.Logger
}

type response struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

func NewConfigFromViper() Config {
	interval := viper.GetInt("master.heartbeatInterval")
	if interval <= 0 {
		interval = 30
	}
	return Config{
		Enabled:           viper.GetBool("master.enable"),
		BaseURL:           strings.TrimSpace(viper.GetString("master.url")),
		Token:             viper.GetString("master.token"),
		Robot:             viper.GetString("master.robot"),
		Name:              viper.GetString("master.name"),
		HeartbeatInterval: time.Duration(interval) * time.Second,
	}
}

func NewClient(cfg Config, logger *zerolog.Logger) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		logger: logger,
	}
}

func (c *Client) Start(ctx context.Context) {
	if !c.cfg.Enabled {
		return
	}
	if err := c.validate(); err != nil {
		c.logError("master 配置无效", err)
		return
	}
	if err := c.Register(ctx); err != nil {
		c.logError("注册失败", err)
	}
	go c.heartbeatLoop(ctx)
}

func (c *Client) Register(ctx context.Context) error {
	return c.request(ctx, "/api/wework/v1/register")
}

func (c *Client) Heartbeat(ctx context.Context) error {
	return c.request(ctx, "/api/wework/v1/heartbeat")
}

func (c *Client) request(ctx context.Context, apiPath string) error {
	u, err := c.buildURL(apiPath)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("type", robotType)
	q.Set("robot", c.cfg.Robot)
	q.Set("name", c.cfg.Name)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	if c.cfg.Token != "" {
		req.Header.Set("token", c.cfg.Token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("master 返回 HTTP %d", resp.StatusCode)
	}
	var ret response
	if err := json.NewDecoder(resp.Body).Decode(&ret); err != nil {
		return err
	}
	if ret.Status != 200 {
		return fmt.Errorf("%s", ret.Msg)
	}
	return nil
}

func (c *Client) buildURL(apiPath string) (*url.URL, error) {
	base, err := url.Parse(c.cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, errors.New("master.url 必须包含协议和主机")
	}
	rel, err := url.Parse(apiPath)
	if err != nil {
		return nil, err
	}
	return base.ResolveReference(rel), nil
}

func (c *Client) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.Heartbeat(ctx); err != nil {
				c.logError("心跳上报失败", err)
			}
		}
	}
}

func (c *Client) validate() error {
	if c.cfg.BaseURL == "" {
		return errors.New("master.url 不能为空")
	}
	if c.cfg.Robot == "" {
		return errors.New("master.robot 不能为空")
	}
	if c.cfg.Name == "" {
		return errors.New("master.name 不能为空")
	}
	return nil
}

func (c *Client) logError(msg string, err error) {
	if c.logger != nil {
		c.logger.Error().Err(err).Msg(msg)
	}
}
