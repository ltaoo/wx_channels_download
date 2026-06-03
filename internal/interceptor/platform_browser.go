package interceptor

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"regexp"
	"strings"

	"wx_channel/internal/interceptor/proxy"
)

type PlatformBrowserConfig struct {
	PlatformId   string
	PlatformName string
	Match        string
	ContentType  string
}

type PlatformBrowserProfile struct {
	PlatformId        string          `json:"platform_id"`
	PlatformName      string          `json:"platform_name"`
	ContentType       string          `json:"content_type"`
	ContentExternalId string          `json:"content_external_id"`
	ContentTitle      string          `json:"content_title"`
	ContentURL        string          `json:"content_url"`
	ContentSourceURL  string          `json:"content_source_url"`
	ContentCoverURL   string          `json:"content_cover_url"`
	AccountExternalId string          `json:"account_external_id"`
	AccountUsername   string          `json:"account_username"`
	AccountNickname   string          `json:"account_nickname"`
	AccountAvatarURL  string          `json:"account_avatar_url"`
	Raw               json.RawMessage `json:"raw"`
}

var platformBrowserNonceReg = regexp.MustCompile(`'nonce-([^']+)'`)

func CreatePlatformBrowserPluginWithScript(match string, scriptContent string, onLoaded func(profile *PlatformBrowserProfile)) *proxy.Plugin {
	return &proxy.Plugin{
		Match: match,
		OnRequest: func(ctx proxy.Context) {
			if ctx.Req().URL.Path != "/__wx_channels_api/platform/browser" {
				return
			}
			body, err := io.ReadAll(ctx.Req().Body)
			if err != nil {
				fmt.Println("[ECHO]handler", err.Error())
			}
			profile, err := NewPlatformBrowserProfile(body)
			if err != nil {
				fmt.Println("[ECHO]handler", err.Error())
			}
			if profile != nil && onLoaded != nil {
				go onLoaded(profile)
			}
			if profile != nil {
				fmt.Printf("\n打开了%s内容\n%s\n", profile.PlatformName, profile.ContentTitle)
			}
			ctx.Mock(200, map[string]string{
				"Content-Type": "application/json",
			}, "{}")
		},
		OnResponse: func(ctx proxy.Context) {
			respContentType := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			if !strings.Contains(respContentType, "text/html") {
				return
			}
			body, err := ctx.GetResponseBody()
			if err != nil {
				return
			}
			page := string(body)
			if !strings.Contains(strings.ToLower(page), "</body>") {
				return
			}
			scriptAttr := ""
			csp := firstPlatformValue(
				ctx.GetResponseHeader("Content-Security-Policy-Report-Only"),
				ctx.GetResponseHeader("Content-Security-Policy"),
			)
			if match := platformBrowserNonceReg.FindStringSubmatch(csp); len(match) > 1 {
				scriptAttr = fmt.Sprintf(` nonce="%s" reportloaderror`, match[1])
			}
			script := fmt.Sprintf(`<script%s>%s</script>`, scriptAttr, scriptContent)
			page = strings.Replace(page, "</body>", script+"</body>", 1)
			page = strings.Replace(page, "</BODY>", script+"</BODY>", 1)
			ctx.SetResponseBody(page)
		},
	}
}

func NewPlatformBrowserProfile(raw []byte) (*PlatformBrowserProfile, error) {
	var profile PlatformBrowserProfile
	if err := json.Unmarshal(raw, &profile); err != nil {
		return nil, err
	}
	profile.PlatformId = strings.TrimSpace(profile.PlatformId)
	profile.PlatformName = strings.TrimSpace(profile.PlatformName)
	profile.ContentType = strings.TrimSpace(profile.ContentType)
	if profile.ContentType == "" {
		profile.ContentType = "article"
	}
	profile.ContentTitle = strings.TrimSpace(profile.ContentTitle)
	profile.ContentURL = html.UnescapeString(strings.TrimSpace(profile.ContentURL))
	profile.ContentSourceURL = html.UnescapeString(strings.TrimSpace(profile.ContentSourceURL))
	profile.ContentCoverURL = html.UnescapeString(strings.TrimSpace(profile.ContentCoverURL))
	profile.AccountExternalId = strings.TrimSpace(profile.AccountExternalId)
	profile.AccountUsername = strings.TrimSpace(profile.AccountUsername)
	profile.AccountNickname = strings.TrimSpace(profile.AccountNickname)
	profile.AccountAvatarURL = html.UnescapeString(strings.TrimSpace(profile.AccountAvatarURL))
	if profile.ContentExternalId == "" {
		profile.ContentExternalId = firstPlatformValue(profile.ContentURL, profile.ContentSourceURL, profile.ContentTitle)
	}
	profile.Raw = json.RawMessage(raw)
	if profile.PlatformId == "" || profile.ContentExternalId == "" {
		return nil, fmt.Errorf("missing platform_id or content_external_id")
	}
	return &profile, nil
}

func BuildPlatformBrowserScript(cfg PlatformBrowserConfig) string {
	cfgBytes, _ := json.Marshal(cfg)
	return fmt.Sprintf(`(function(){
  if (window.__wx_platform_browser_reported__) return;
  window.__wx_platform_browser_reported__ = true;
  var cfg = %s;
  function txt(v){ return (v == null ? "" : String(v)).trim(); }
  function meta(sel){ var el = document.querySelector(sel); return txt(el && el.getAttribute("content")); }
  function attr(sel, name){ var el = document.querySelector(sel); return txt(el && el.getAttribute(name)); }
  function canonical(){ return attr('link[rel="canonical"]', "href") || location.href; }
  function stripHash(u){ try { var x = new URL(u, location.href); x.hash = ""; return x.href; } catch(e) { return u || location.href; } }
  function jsonLdAuthor(){
    var scripts = Array.prototype.slice.call(document.querySelectorAll('script[type="application/ld+json"]'));
    for (var i = 0; i < scripts.length; i++) {
      try {
        var data = JSON.parse(scripts[i].textContent || "{}");
        var arr = Array.isArray(data) ? data : [data];
        for (var j = 0; j < arr.length; j++) {
          var a = arr[j] && arr[j].author;
          if (Array.isArray(a)) a = a[0];
          if (a && (a.name || a.url || a.image)) return a;
        }
      } catch(e) {}
    }
    return {};
  }
  function imageURL(v){ return txt(Array.isArray(v) ? v[0] : v); }
  function first(){ for (var i=0; i<arguments.length; i++) { var v = txt(arguments[i]); if (v) return v; } return ""; }
  setTimeout(function(){
    var author = jsonLdAuthor();
    var accountName = first(
      meta('meta[name="author"]'),
      meta('meta[property="article:author"]'),
      author.name,
      attr('[itemprop="author"] [itemprop="name"]', "content"),
      txt(document.querySelector('[itemprop="author"] [itemprop="name"]') && document.querySelector('[itemprop="author"] [itemprop="name"]').textContent)
    );
    var authorURL = first(author.url, attr('a[rel="author"]', "href"));
    var contentURL = stripHash(canonical());
    var payload = {
      platform_id: cfg.PlatformId,
      platform_name: cfg.PlatformName,
      content_type: cfg.ContentType || "article",
      content_external_id: contentURL,
      content_title: first(meta('meta[property="og:title"]'), meta('meta[name="twitter:title"]'), document.title),
      content_url: contentURL,
      content_source_url: location.href,
      content_cover_url: first(meta('meta[property="og:image"]'), meta('meta[name="twitter:image"]')),
      account_external_id: first(authorURL, accountName),
      account_username: authorURL,
      account_nickname: accountName,
      account_avatar_url: imageURL(author.image)
    };
    if (!payload.content_title && !payload.content_url) return;
    fetch("/__wx_channels_api/platform/browser", {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify(payload)
    }).catch(function(){});
  }, 800);
})();`, string(cfgBytes))
}

func firstPlatformValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
