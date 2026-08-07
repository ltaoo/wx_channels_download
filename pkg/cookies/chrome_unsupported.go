//go:build !darwin && !windows

package cookies

import "fmt"

func newChromeCookieDecryptor() (*chromeCookieDecryptor, error) {
	return nil, fmt.Errorf("cookies: automatic Chrome import is not supported on this operating system")
}

func findChromeCookiesDB() (string, error) {
	return "", fmt.Errorf("cookies: automatic Chrome import is not supported on this operating system")
}
