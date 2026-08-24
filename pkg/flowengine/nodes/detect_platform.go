package nodes

import (
	"errors"
	"net/url"
	"strings"

	"wx_channel/pkg/flowengine/engine"
)

type DetectPlatformNode struct {
	Id     string
	Config map[string]interface{}
}

func NewDetectPlatformNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &DetectPlatformNode{Id: id, Config: config}
}

func (n *DetectPlatformNode) ID() string   { return n.Id }
func (n *DetectPlatformNode) Type() string { return "DetectPlatformNode" }

func (n *DetectPlatformNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	raw, _ := ctx.Data["url"].(string)
	if raw == "" {
		return false, nil, errors.New("missing url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false, nil, err
	}
	host := strings.ToLower(u.Host)
	platform := ""
	if host == "" {
		if strings.Contains(strings.ToLower(raw), "qq.com") {
			platform = "wechat"
		} else if strings.Contains(strings.ToLower(raw), "bilibili") || strings.Contains(strings.ToLower(raw), "b23.tv") || strings.Contains(strings.ToLower(raw), "hdslb.com") {
			platform = "bilibili"
		} else if strings.Contains(strings.ToLower(raw), "xiaohongshu.com") || strings.Contains(strings.ToLower(raw), "xhslink.cn") || strings.Contains(strings.ToLower(raw), "xhslink.com") {
			platform = "xhs"
		}
	} else {
		if strings.Contains(host, "qq.com") || strings.Contains(host, "video.qq.com") || strings.Contains(host, "weixin.qq.com") {
			platform = "wechat"
		} else if strings.Contains(host, "bilibili.com") || strings.Contains(host, "hdslb.com") || strings.Contains(host, "b23.tv") {
			platform = "bilibili"
		} else if strings.Contains(host, "xiaohongshu.com") || strings.Contains(host, "xhslink.cn") || strings.Contains(host, "xhslink.com") {
			platform = "xhs"
		}
	}
	if platform == "" {
		return false, nil, errors.New("unsupported platform")
	}
	ctx.Data["platform"] = platform
	next := ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id)
	return true, next, nil
}
