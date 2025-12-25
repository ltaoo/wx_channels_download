package api

import (
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

func handleDownload(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	targetURL := q.Get("url")
	if targetURL == "" {
		http.Error(w, "missing targetURL", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(targetURL, "http") {
		targetURL = "https://" + targetURL
	}
	if _, err := url.Parse(targetURL); err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	filename := q.Get("filename")
	if filename == "" {
		if u, err := url.Parse(targetURL); err == nil {
			if base := path.Base(u.Path); base != "" && base != "/" {
				filename = base
			}
		}
		if filename == "" {
			filename = "download.mp4"
		}
	}
	decryptKeyStr := q.Get("key")
	toMP3 := q.Get("mp3")
	mp := NewChannelsVideoDecryptor()
	if decryptKeyStr != "" {
		decryptKey, err := strconv.ParseUint(decryptKeyStr, 0, 64)
		if err != nil {
			http.Error(w, "invalid decryptKey", http.StatusBadRequest)
			return
		}
		if toMP3 == "1" {
			mp.convertWithDecrypt(w, targetURL, decryptKey, 131072, filename)
			return
		}
		mp.decryptOnly(w, r, targetURL, decryptKey, 131072, filename)
		return
	}
	mp.convertOnly(targetURL, w, filename, "mp3")
}

func handlePlay(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	targetURL := q.Get("url")
	if targetURL == "" {
		http.Error(w, "missing targetURL", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(targetURL, "http") {
		targetURL = "https://" + targetURL
	}
	if _, err := url.Parse(targetURL); err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	decryptKeyStr := q.Get("key")
	mp := NewChannelsVideoDecryptor()
	if decryptKeyStr != "" {
		decryptKey, err := strconv.ParseUint(decryptKeyStr, 0, 64)
		if err != nil {
			http.Error(w, "invalid decryptKey", http.StatusBadRequest)
			return
		}
		mp.decryptOnlyInline(w, r, targetURL, decryptKey, 131072)
		return
	}
	mp.simpleProxy(targetURL, w, r)
}
