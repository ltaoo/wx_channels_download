package frontend

import (
	"net/url"
	"strings"
)

type csp_directive struct {
	name   string
	values []string
}

type csp_policy struct {
	directives []csp_directive
	index      map[string]int
}

func RewriteCSPForWebSocket(policy string, websocket_url string) string {
	websocket_origin := csp_origin(websocket_url)
	if strings.TrimSpace(policy) == "" || websocket_origin == "" {
		return policy
	}

	parsed := parse_csp(policy)
	parsed.add_sources_with_fallback("connect-src", []string{websocket_origin}, "default-src")
	return parsed.render()
}

func RewriteCSPForLocalAssets(policy string, asset_base_url string) string {
	asset_origin := csp_origin(asset_base_url)
	if strings.TrimSpace(policy) == "" || asset_origin == "" {
		return policy
	}

	parsed := parse_csp(policy)
	parsed.add_sources_with_fallback("style-src", []string{asset_origin}, "default-src")
	parsed.add_sources_if_present("style-src-elem", []string{asset_origin})
	parsed.add_sources_with_fallback("script-src", []string{asset_origin}, "default-src")
	parsed.add_sources_if_present("script-src-elem", []string{asset_origin})
	parsed.add_sources_with_fallback("connect-src", []string{asset_origin, csp_websocket_origin(asset_origin)}, "default-src")
	return parsed.render()
}

func csp_origin(raw_url string) string {
	parsed_url, err := url.Parse(raw_url)
	if err != nil || parsed_url.Scheme == "" || parsed_url.Host == "" {
		return ""
	}
	return parsed_url.Scheme + "://" + parsed_url.Host
}

func csp_websocket_origin(origin string) string {
	parsed_url, err := url.Parse(origin)
	if err != nil || parsed_url.Scheme == "" || parsed_url.Host == "" {
		return ""
	}
	switch parsed_url.Scheme {
	case "https":
		return "wss://" + parsed_url.Host
	case "http":
		return "ws://" + parsed_url.Host
	default:
		return ""
	}
}

func parse_csp(policy string) *csp_policy {
	parsed := &csp_policy{index: map[string]int{}}
	for _, raw_directive := range strings.Split(policy, ";") {
		fields := strings.Fields(strings.TrimSpace(raw_directive))
		if len(fields) == 0 {
			continue
		}
		name := strings.ToLower(fields[0])
		if _, exists := parsed.index[name]; exists {
			continue
		}
		parsed.index[name] = len(parsed.directives)
		parsed.directives = append(parsed.directives, csp_directive{
			name:   name,
			values: append([]string(nil), fields[1:]...),
		})
	}
	return parsed
}

func (p *csp_policy) add_sources_if_present(name string, sources []string) {
	index, ok := p.index[name]
	if !ok {
		return
	}
	p.directives[index].values = append_csp_sources(p.directives[index].values, sources)
}

func (p *csp_policy) add_sources_with_fallback(name string, sources []string, fallback string) {
	if index, ok := p.index[name]; ok {
		p.directives[index].values = append_csp_sources(p.directives[index].values, sources)
		return
	}
	if fallback == "" {
		return
	}
	fallback_index, ok := p.index[fallback]
	if !ok {
		return
	}
	p.index[name] = len(p.directives)
	p.directives = append(p.directives, csp_directive{
		name:   name,
		values: append_csp_sources(append([]string(nil), p.directives[fallback_index].values...), sources),
	})
}

func append_csp_sources(values []string, sources []string) []string {
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

func (p *csp_policy) render() string {
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
