// Package source supplies request and metadata helpers used by source scrapers.
package source

import (
	"errors"
	"net/url"
	"strings"
)

const UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"

// Policy is supplied by the platform implementation. No global platform lookup
// or initialization side effect is required to validate a source address.
type Policy struct {
	AnonymousHosts []string
	UpgradeHTTPS   bool
	RequireHTTPS   bool
	Metadata       []string
	Media          []string
}

func (p Policy) check_address(address string, metadata bool) error {
	link, err := url.Parse(address)
	if err != nil || link.User != nil || link.Fragment != "" || strings.ContainsAny(address, "\r\n\t") ||
		(link.Port() != "" && link.Port() != "443" && link.Port() != "80") ||
		(link.Scheme != "https" && (metadata || link.Scheme != "http")) {
		return errors.New("视频服务地址或协议无效")
	}
	if p.RequireHTTPS && link.Scheme != "https" {
		return errors.New("视频服务要求 HTTPS")
	}
	domains := p.Media
	if metadata {
		domains = p.Metadata
	}
	host := strings.ToLower(link.Hostname())
	for _, domain := range domains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return nil
		}
	}
	return errors.New("视频返回不受支持的服务域名")
}

func (p Policy) CheckMediaAddress(address string) error { return p.check_address(address, false) }
