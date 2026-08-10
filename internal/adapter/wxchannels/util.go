package wxchannelsadapter

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/wxchannels"
)

// PickSpec returns the first h264 spec's FileFormat from the object, or "original" if none.
func PickSpec(obj *wxchannels.ChannelsObject) string {
	specs := obj.Spec
	if len(obj.ObjectDesc.Media) > 0 && len(obj.ObjectDesc.Media[0].Spec) > 0 {
		specs = obj.ObjectDesc.Media[0].Spec
	}
	if len(specs) > 0 {
		return specs[0].FileFormat
	}
	return ""
}

// BuildDownloadURLWithSpec returns the download URL for the given spec.
//
//   - If spec is a codec name (e.g. "xWT111"), appends &X-snsvideoflag= to the base URL.
//   - If spec is "" or "original", strips all query params except encfilekey and token,
//     mirroring the JS __wx_channels_download4 original-video logic.
//   - zip:// URLs are returned as-is.
func BuildDownloadURLWithSpec(obj *wxchannels.ChannelsObject, spec string) string {
	base_url := ObjectURL(obj)

	// When spec is non-empty: append X-snsvideoflag parameter
	if spec != "" {
		return base_url + "&X-snsvideoflag=" + spec
	}

	// When spec is empty, download the original video, keeping only encfilekey and token
	if u, err := url.Parse(base_url); err == nil {
		filekey := u.Query().Get("encfilekey")
		token := u.Query().Get("token")
		if filekey != "" && token != "" {
			clean := &url.URL{
				Scheme: u.Scheme,
				Host:   u.Host,
				Path:   u.Path,
			}
			q := clean.Query()
			q.Set("encfilekey", filekey)
			q.Set("token", token)
			clean.RawQuery = q.Encode()
			return clean.String()
		}
	}

	return base_url
}

// DecryptKeyInt returns the video decrypt key as int, or 0 on failure.
func DecryptKeyInt(obj *wxchannels.ChannelsObject) int {
	if len(obj.ObjectDesc.Media) == 0 {
		return 0
	}
	key, err := strconv.Atoi(obj.ObjectDesc.Media[0].DecodeKey)
	if err != nil {
		return 0
	}
	return key
}

// ObjectTitle returns the object title with fallback logic (description → ID → timestamp).
func ObjectTitle(obj *wxchannels.ChannelsObject) string {
	if obj.LiveInfo != nil {
		return "直播"
	}
	title := strings.TrimSpace(obj.ObjectDesc.Description)
	if title != "" {
		return title
	}
	if strings.TrimSpace(obj.ID) != "" {
		return obj.ID
	}
	return strconv.FormatInt(time.Now().Unix(), 10)
}

// DecryptKey exposes the legacy channels conversion capability through the
// registered handler, so callers do not need to import this package.
func (a *ChannelsAdapter) DecryptKey(content_json json.RawMessage) (int, error) {
	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(content_json, &obj); err != nil {
		return 0, err
	}
	return DecryptKeyInt(&obj), nil
}

// ConvertContent converts a raw channels object into the shared content model.
func (a *ChannelsAdapter) ConvertContent(content_json json.RawMessage) (*model.Content, error) {
	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(content_json, &obj); err != nil {
		return nil, err
	}
	content, _, err := ToContent(&obj)
	return content, err
}
