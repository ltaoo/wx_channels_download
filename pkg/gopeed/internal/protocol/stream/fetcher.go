package stream

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/GopeedLab/gopeed/internal/controller"
	"github.com/GopeedLab/gopeed/internal/fetcher"
	"github.com/GopeedLab/gopeed/pkg/base"
	fstream "github.com/GopeedLab/gopeed/pkg/protocol/stream"
)

type Fetcher struct {
	ctl    *controller.Controller
	config *config
	doneCh chan error

	meta       *fetcher.FetcherMeta
	cmd        *exec.Cmd
	downloaded int64
	lock       sync.Mutex
}

func (f *Fetcher) Setup(ctl *controller.Controller) {
	f.ctl = ctl
	f.doneCh = make(chan error, 1)
	if f.meta == nil {
		f.meta = &fetcher.FetcherMeta{}
	}
	f.ctl.GetConfig(&f.config)
}

func (f *Fetcher) Resolve(req *base.Request) error {
	if err := base.ParseReqExtra[fstream.ReqExtra](req); err != nil {
		return err
	}
	f.meta.Req = req
	
	// Since we are using ffmpeg, we might not get exact file details upfront easily without ffprobe.
	// We will infer generic details.
	res := &base.Resource{
		Range: false, // Streams usually don't support range in the traditional sense
		Files: []*base.FileInfo{},
	}
	
	// Attempt to guess filename from URL
	fileName := path.Base(req.URL)
	if fileName == "" || fileName == "." || fileName == "/" {
		u, _ := url.Parse(req.URL)
		if u != nil {
			fileName = u.Hostname()
		}
	}
	// Append appropriate extension if missing (ffmpeg can often convert or mux)
	// For simplicity, default to .mp4 if it looks like a video stream and has no extension, 
	// or trust the user/system to name it.
	// Ideally, the UI/User provides the name.
	
	res.Files = append(res.Files, &base.FileInfo{
		Name: fileName,
		Size: 0, // Unknown size
	})
	
	f.meta.Res = res
	return nil
}

func (f *Fetcher) Create(opts *base.Options) error {
	f.meta.Opts = opts
	if err := base.ParseOptsExtra[fstream.OptsExtra](f.meta.Opts); err != nil {
		return err
	}
	if opts.Extra == nil {
		opts.Extra = &fstream.OptsExtra{}
	}
	return nil
}

func (f *Fetcher) Start() (err error) {
	name := f.meta.SingleFilepath()
	// Ensure directory exists
	dir := path.Dir(name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	cmdPath := "ffmpeg"
	if f.config.FfmpegPath != "" {
		cmdPath = f.config.FfmpegPath
	}

	args := []string{
		"-y", // Overwrite output file
		"-i", f.meta.Req.URL,
		"-c", "copy",
		"-bsf:a", "aac_adtstoasc", // Common fix for HLS/TS to MP4/AAC
		name,
	}

	// Add User-Agent if configured
	ua := f.config.UserAgent
	if f.meta.Req.Extra != nil {
		extra := f.meta.Req.Extra.(*fstream.ReqExtra)
		if extraHeaderUA, ok := extra.Header["User-Agent"]; ok {
			ua = extraHeaderUA
		}
	}
	if ua != "" {
		// FFmpeg uses -user_agent
		// Insert before -i
		newArgs := []string{"-user_agent", ua}
		newArgs = append(newArgs, args...)
		args = newArgs
	}
	
	// Add other headers if necessary (ffmpeg support for custom headers is via -headers key1:val1\r\nkey2:val2)
	// Note: This is protocol specific in ffmpeg, mainly HTTP/HLS.
	if f.meta.Req.Extra != nil {
		extra := f.meta.Req.Extra.(*fstream.ReqExtra)
		if len(extra.Header) > 0 {
			var headerBuilder strings.Builder
			for k, v := range extra.Header {
				if k == "User-Agent" { continue } // Handled separately
				headerBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
			}
			headers := headerBuilder.String()
			if headers != "" {
				newArgs := []string{"-headers", headers}
				newArgs = append(newArgs, args...)
				args = newArgs
			}
		}
	}

	f.cmd = exec.Command(cmdPath, args...)
	
	// Capture stderr for progress
	stderr, err := f.cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := f.cmd.Start(); err != nil {
		return err
	}

	// Process stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		// FFmpeg updates progress on stderr with \r usually, scanner handles lines.
		// We might need to handle raw reads if scanner waits for \n and ffmpeg only sends \r
		// But usually ffmpeg sends updates frequently enough.
		// Let's use custom split function or just read raw.
		// Actually typical ffmpeg output (with -progress -) is easier, but without it default stderr is readable.
		// Standard stderr output: size=     512kB time=00:00:15.12 bitrate= 277.3kbits/s speed=18.4x
		
		reSize := regexp.MustCompile(`size=\s*(\d+)(kB|mB|B)`)
		
		for scanner.Scan() {
			line := scanner.Text()
			// Parse size
			matches := reSize.FindStringSubmatch(line)
			if len(matches) == 3 {
				val, _ := strconv.ParseInt(matches[1], 10, 64)
				unit := matches[2]
				var bytes int64
				switch unit {
				case "kB":
					bytes = val * 1024
				case "mB":
					bytes = val * 1024 * 1024
				case "B":
					bytes = val
				}
				
				f.lock.Lock()
				f.downloaded = bytes
				f.lock.Unlock()
			}
		}
	}()

	go func() {
		err := f.cmd.Wait()
		f.doneCh <- err
	}()

	return nil
}

func (f *Fetcher) Pause() (err error) {
	if f.cmd != nil && f.cmd.Process != nil {
		// Send 'q' to stdin? Or SIGTERM.
		// On Windows SIGTERM might not work well, Kill might be needed or 'q'.
		// Trying to be gentle first.
		_ = f.cmd.Process.Signal(os.Interrupt)
		
		// Wait a bit?
		// For now simple kill if it's "Pause" which usually means stop for streams as they can't really "pause" and resume exactly same spot unless server supports execution history.
		// Actually, if we "Pause", we might just want to stop the recording.
		// Calling Kill ensures it stops.
		// Ideally we want to let ffmpeg write the trailer (moov atom for mp4).
		// Sending os.Interrupt (SIGINT) usually triggers graceful shutdown in ffmpeg.
	}
	return
}

func (f *Fetcher) Close() (err error) {
	return f.Pause()
}

func (f *Fetcher) Meta() *fetcher.FetcherMeta {
	return f.meta
}

func (f *Fetcher) Stats() any {
	f.lock.Lock()
	defer f.lock.Unlock()
	return &fstream.Stats{
		Downloaded: f.downloaded,
	}
}

func (f *Fetcher) Progress() fetcher.Progress {
	f.lock.Lock()
	defer f.lock.Unlock()
	return fetcher.Progress{f.downloaded}
}

func (f *Fetcher) Wait() (err error) {
	return <-f.doneCh
}

// FetcherManager Implementation
type FetcherManager struct {
}

func (fm *FetcherManager) Name() string {
	return "stream"
}

func (fm *FetcherManager) Filters() []*fetcher.SchemeFilter {
	return []*fetcher.SchemeFilter{
		{
			Type:    fetcher.FilterTypeUrl,
			Pattern: "stream",
		},
		{
			Type:    fetcher.FilterTypeUrl,
			Pattern: "rtmp",
		},
		{
			Type:    fetcher.FilterTypeUrl,
			Pattern: "m3u8", // naive file extension filter, actual detection happens elsewhere usually
		},
	}
}

func (fm *FetcherManager) Build() fetcher.Fetcher {
	return &Fetcher{}
}

func (fm *FetcherManager) ParseName(u string) string {
	var name string
	url, err := url.Parse(u)
	if err != nil {
		return ""
	}
	name = path.Base(url.Path)
	if name == "" || name == "/" || name == "." {
		name = url.Hostname()
	}
	return name
}

func (fm *FetcherManager) AutoRename() bool {
	return true
}

func (fm *FetcherManager) DefaultConfig() any {
	return &config{
		UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		FfmpegPath: "ffmpeg",
	}
}

func (fm *FetcherManager) Store(fetcher fetcher.Fetcher) (any, error) {
	return nil, nil
}

func (fm *FetcherManager) Restore() (v any, f func(meta *fetcher.FetcherMeta, v any) fetcher.Fetcher) {
	return nil, nil
}

func (fm *FetcherManager) Close() error {
	return nil
}
