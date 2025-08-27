//go:build windows

package proxy

import "errors"

func enableProxy(ps ProxySettings) error {
	return errors.New("Windows 平台暂不支持")
}

func disableProxy() error {
	return errors.New("Windows 平台暂不支持")
}
