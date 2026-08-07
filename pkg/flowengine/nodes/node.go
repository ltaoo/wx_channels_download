package nodes

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func httpDownload(url string, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("download failed")
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func deriveFilename(url string) string {
	u := url
	if i := strings.LastIndex(u, "/"); i >= 0 {
		name := u[i+1:]
		if name != "" && !strings.HasSuffix(name, "/") {
			return fmt.Sprintf("%s", name)
		}
	}
	return fmt.Sprintf("file-%d", time.Now().UnixNano())
}

func fileCopy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
