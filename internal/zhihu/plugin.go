package zhihu

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/internal/interceptor/proxy"
	"wx_channel/pkg/util"
)

const PlatformID = "zhihu"

func CreateZhihuInterceptorPlugin(cfg *ZhihuConfig, db *gorm.DB, logger *zerolog.Logger) *proxy.Plugin {
	return &proxy.Plugin{
		Match: "zhihu.com",
		OnResponse: func(ctx proxy.Context) {
			if cfg != nil && cfg.Disabled {
				return
			}
			if db == nil {
				return
			}
			setCookieLines := ctx.Res().Header.Values("Set-Cookie")
			if len(setCookieLines) == 0 {
				return
			}
			cookieHeader, payload, ok := buildMergedCookiePayload(db, setCookieLines, ctx.Req().URL.Hostname(), ctx.Req().URL.Path)
			if !ok {
				return
			}
			now := util.NowMillis()
			if err := upsertDefaultCookieCredential(db, cookieHeader, payload, now); err != nil {
				if logger != nil {
					logger.Error().Err(err).Msg("zhihu cookie upsert failed")
				}
			}
		},
	}
}

func buildMergedCookiePayload(db *gorm.DB, setCookieLines []string, host string, path string) (string, string, bool) {
	var existing model.AuthCredential
	err := db.
		Where("platform_id = ? AND kind = ? AND deleted_at IS NULL", PlatformID, "cookie").
		Order("is_default DESC, updated_at DESC, id DESC").
		First(&existing).
		Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", false
	}

	cookies := parseCookieHeader(existing.Secret)
	changed := false
	for _, line := range setCookieLines {
		name, value, maxAge, ok := parseSetCookieLine(line)
		if !ok {
			continue
		}
		if value == "" || (maxAge != nil && *maxAge <= 0) {
			if _, ok := cookies[name]; ok {
				delete(cookies, name)
				changed = true
			}
			continue
		}
		if cookies[name] != value {
			cookies[name] = value
			changed = true
		}
	}
	if !changed {
		return "", "", false
	}

	mergedHeader := formatCookieHeader(cookies)
	payloadBytes, _ := json.Marshal(map[string]any{
		"cookies":     cookies,
		"set_cookie":  setCookieLines,
		"last_origin": host,
		"last_path":   path,
	})
	return mergedHeader, string(payloadBytes), true
}

func upsertDefaultCookieCredential(db *gorm.DB, cookieHeader string, payload string, now int64) error {
	var existing model.AuthCredential
	err := db.
		Where("platform_id = ? AND kind = ? AND deleted_at IS NULL", PlatformID, "cookie").
		Order("is_default DESC, updated_at DESC, id DESC").
		First(&existing).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		cred := model.AuthCredential{
			PlatformId: PlatformID,
			Name:       "zhihu.default",
			Kind:       "cookie",
			Secret:     cookieHeader,
			Payload:    payload,
			Status:     1,
			IsDefault:  1,
			Timestamps: model.Timestamps{
				CreatedAt: now,
				UpdatedAt: now,
			},
		}
		return db.Create(&cred).Error
	}
	if err != nil {
		return err
	}
	existing.Secret = cookieHeader
	existing.Payload = payload
	existing.UpdatedAt = now
	return db.Save(&existing).Error
}

func parseCookieHeader(cookieHeader string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(cookieHeader, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" {
			continue
		}
		out[k] = v
	}
	return out
}

func formatCookieHeader(cookies map[string]string) string {
	if len(cookies) == 0 {
		return ""
	}
	keys := make([]string, 0, len(cookies))
	for k := range cookies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := cookies[k]
		if k == "" || v == "" {
			continue
		}
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; ")
}

func parseSetCookieLine(line string) (string, string, *int, bool) {
	parts := strings.Split(line, ";")
	if len(parts) == 0 {
		return "", "", nil, false
	}
	nameValue := strings.TrimSpace(parts[0])
	name, value, ok := strings.Cut(nameValue, "=")
	if !ok {
		return "", "", nil, false
	}
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	if name == "" {
		return "", "", nil, false
	}
	var maxAge *int
	for _, attr := range parts[1:] {
		attr = strings.TrimSpace(attr)
		if attr == "" {
			continue
		}
		k, v, ok := strings.Cut(attr, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(k), "max-age") {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				maxAge = &n
			}
		}
	}
	return name, value, maxAge, true
}
