package interceptor

import (
	"net/url"
	"strings"

	"wx_channel/internal/interceptor/proxy"
)

type cspDirective struct {
	name   string
	values []string
}

type cspPolicy struct {
	directives []cspDirective
	index      map[string]int
}

func RewriteResponseCSPForLocalAssets(ctx proxy.Context, assetBaseURL string) {
	for _, header := range []string{"Content-Security-Policy", "Content-Security-Policy-Report-Only"} {
		policy := ctx.GetResponseHeader(header)
		rewritten := RewriteCSPForLocalAssets(policy, assetBaseURL)
		if rewritten != "" && rewritten != policy {
			ctx.SetResponseHeader(header, rewritten)
		}
	}
}

func RewriteCSPForLocalAssets(policy string, assetBaseURL string) string {
	assetOrigin := cspOrigin(assetBaseURL)
	if strings.TrimSpace(policy) == "" || assetOrigin == "" {
		return policy
	}

	parsed := parseCSP(policy)
	parsed.addSourcesWithFallback("style-src", []string{assetOrigin}, "default-src")
	parsed.addSourcesIfPresent("style-src-elem", []string{assetOrigin})
	parsed.addSourcesWithFallback("script-src", []string{assetOrigin}, "default-src")
	parsed.addSourcesIfPresent("script-src-elem", []string{assetOrigin})
	parsed.addSourcesWithFallback("connect-src", []string{assetOrigin, cspWebSocketOrigin(assetOrigin)}, "default-src")
	return parsed.String()
}

func cspOrigin(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func cspWebSocketOrigin(origin string) string {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	switch u.Scheme {
	case "https":
		return "wss://" + u.Host
	case "http":
		return "ws://" + u.Host
	default:
		return ""
	}
}

func parseCSP(policy string) *cspPolicy {
	parsed := &cspPolicy{index: map[string]int{}}
	for _, rawDirective := range strings.Split(policy, ";") {
		fields := strings.Fields(strings.TrimSpace(rawDirective))
		if len(fields) == 0 {
			continue
		}
		name := strings.ToLower(fields[0])
		if _, exists := parsed.index[name]; exists {
			continue
		}
		parsed.index[name] = len(parsed.directives)
		parsed.directives = append(parsed.directives, cspDirective{
			name:   name,
			values: append([]string(nil), fields[1:]...),
		})
	}
	return parsed
}

func (p *cspPolicy) addSourcesIfPresent(name string, sources []string) {
	idx, ok := p.index[name]
	if !ok {
		return
	}
	p.directives[idx].values = appendCSPSources(p.directives[idx].values, sources)
}

func (p *cspPolicy) addSourcesWithFallback(name string, sources []string, fallback string) {
	if idx, ok := p.index[name]; ok {
		p.directives[idx].values = appendCSPSources(p.directives[idx].values, sources)
		return
	}
	if fallback == "" {
		return
	}
	fallbackIdx, ok := p.index[fallback]
	if !ok {
		return
	}
	p.index[name] = len(p.directives)
	p.directives = append(p.directives, cspDirective{
		name:   name,
		values: appendCSPSources(append([]string(nil), p.directives[fallbackIdx].values...), sources),
	})
}

func appendCSPSources(values []string, sources []string) []string {
	if len(sources) == 0 {
		return values
	}
	next := make([]string, 0, len(values)+len(sources))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "'none'" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		next = append(next, value)
	}
	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		if _, ok := seen[source]; ok {
			continue
		}
		seen[source] = struct{}{}
		next = append(next, source)
	}
	return next
}

func (p *cspPolicy) String() string {
	if len(p.directives) == 0 {
		return ""
	}
	parts := make([]string, 0, len(p.directives))
	for _, directive := range p.directives {
		if len(directive.values) == 0 {
			parts = append(parts, directive.name)
			continue
		}
		parts = append(parts, directive.name+" "+strings.Join(directive.values, " "))
	}
	return strings.Join(parts, "; ")
}
