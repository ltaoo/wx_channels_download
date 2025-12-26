package api

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
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

type OpenFolderAndHighlightFileBody struct {
	FilePath string `json:"filepath"`
}

func (c *APIClient) handleHighlightFileInFolder(ctx *gin.Context) {
	var body OpenFolderAndHighlightFileBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fmt.Println(body)
	if body.FilePath == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Missing the `filepath`"})
		return
	}
	full_filepath := filepath.Join(body.FilePath)
	_, err := os.Stat(full_filepath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", "/select,", full_filepath)
	case "darwin":
		cmd = exec.Command("open", "-R", full_filepath)
	case "linux":
		cmd = exec.Command("xdg-open", full_filepath)
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Unsupported operating system"})
		return
	}
	err = cmd.Start()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Success"})
	return
}
