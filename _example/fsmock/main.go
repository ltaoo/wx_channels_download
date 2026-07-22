package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DownloadConfig holds the parameters for a mock download.
type DownloadConfig struct {
	FileSize  int64
	Speed     int64         // bytes per second, 0 = unlimited
	Delay     time.Duration // initial delay
	FailRate  float64       // 0.0–1.0 random failure probability
	Fail      bool          // randomly fail mid-stream
	ChunkSize int
	Filename  string
}

// MockDownloadServer is the main server struct.
type MockDownloadServer struct {
	port        int
	activeTasks sync.Map
	mu          sync.Mutex
	stats       ServerStats
}

// ServerStats tracks aggregate metrics.
type ServerStats struct {
	TotalRequests      int64
	TotalBytes         int64
	ActiveDownloads    int64
	CompletedDownloads int64
}

// DownloadTask represents an active download.
type DownloadTask struct {
	ID         string
	Config     DownloadConfig
	StartTime  time.Time
	BytesSent  int64
	IsActive   bool
	CancelChan chan struct{}
	mu         sync.Mutex
}

var defaultConfig = DownloadConfig{
	FileSize:  10 * 1024 * 1024, // 10MB
	Speed:     1024 * 1024,      // 1MB/s
	ChunkSize: 32 * 1024,        // 32KB
	FailRate:  0,
}

// ---------------------------------------------------------------------------
// Shared dark-mode CSS
// ---------------------------------------------------------------------------

const pageStyle = `<style>
  :root {
    --bg: #ffffff;
    --bg-card: #f5f5f5;
    --bg-code: #333;
    --text: #222;
    --text-muted: #555;
    --border: #ddd;
    --accent: #4CAF50;
    --accent-hover: #45a049;
    --link: #0066cc;
    --tag-bg: #e8f4f8;
    --tag-text: #1a5276;
    --warn-bg: #fff3cd;
    --warn-text: #856404;
    --table-even: #f2f2f2;
  }
  @media (prefers-color-scheme: dark) {
    :root {
      --bg: #1a1a2e;
      --bg-card: #16213e;
      --bg-code: #0f3460;
      --text: #e0e0e0;
      --text-muted: #a0a0a0;
      --border: #333;
      --accent: #4CAF50;
      --accent-hover: #66bb6a;
      --link: #64b5f6;
      --tag-bg: #1a3a4a;
      --tag-text: #81d4fa;
      --warn-bg: #4a3800;
      --warn-text: #ffd54f;
      --table-even: #1e1e3a;
    }
  }
  * { box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 40px; background: var(--bg); color: var(--text); transition: background 0.3s, color 0.3s; }
  h1, h2, h3 { color: var(--text); }
  a { color: var(--link); text-decoration: none; }
  a:hover { text-decoration: underline; }
  .card { background: var(--bg-card); padding: 20px; border-radius: 8px; margin: 20px 0; border: 1px solid var(--border); }
  .code { background: var(--bg-code); color: #e0e0e0; padding: 12px; border-radius: 4px; font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace; line-height: 1.6; }
  .code span.label { color: #8be9fd; }
  .code span.val { color: #f1fa8c; }
  table { border-collapse: collapse; width: 100%; margin: 20px 0; }
  th, td { border: 1px solid var(--border); padding: 10px 14px; text-align: left; }
  th { background: var(--accent); color: white; }
  tr:nth-child(even) td { background: var(--table-even); }
  .form-group { margin: 10px 0; display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
  .form-group label { min-width: 90px; font-weight: 600; color: var(--text-muted); }
  input, select { padding: 8px 12px; width: 200px; border: 1px solid var(--border); border-radius: 4px; background: var(--bg); color: var(--text); }
  button, .btn { padding: 10px 20px; background: var(--accent); color: white; border: none; cursor: pointer; border-radius: 4px; font-weight: 600; }
  button:hover, .btn:hover { background: var(--accent-hover); }
  .tag { display: inline-block; background: var(--tag-bg); color: var(--tag-text); padding: 3px 8px; border-radius: 3px; font-size: 0.85em; margin: 2px; font-weight: 600; }
  .resources-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 16px; }
  .resource-item { background: var(--bg-card); border: 1px solid var(--border); border-radius: 6px; padding: 16px; }
  .resource-item h4 { margin: 0 0 8px 0; }
  .resource-item .meta { font-size: 0.85em; color: var(--text-muted); margin-bottom: 8px; }
  .resource-item .actions a { margin-right: 8px; font-size: 0.9em; }
  .group { margin-bottom: 20px; border-bottom: 1px solid var(--border); padding-bottom: 10px; }
  textarea { width: 100%; height: 300px; margin-top: 10px; font-family: monospace; background: var(--bg-code); color: #e0e0e0; border: 1px solid var(--border); border-radius: 4px; padding: 10px; }
  .nav { margin-bottom: 24px; }
  .nav a { margin-right: 16px; padding: 6px 12px; border-radius: 4px; }
  .nav a.active { background: var(--accent); color: white; }
  .protocol-tag { display: inline-block; padding: 2px 6px; border-radius: 3px; font-size: 0.8em; font-weight: 700; }
  .protocol-http { background: #e3f2fd; color: #1565c0; }
  .protocol-ftp { background: #fce4ec; color: #c62828; }
  .protocol-s3 { background: #fff3e0; color: #e65100; }
  .protocol-hls { background: #e8f5e9; color: #2e7d32; }
  .protocol-magnet { background: #f3e5f5; color: #6a1b9a; }
  .protocol-torrent { background: #e0f7fa; color: #006064; }
  @media (prefers-color-scheme: dark) {
    .protocol-http { background: #0d3b66; color: #90caf9; }
    .protocol-ftp { background: #4a1525; color: #ef9a9a; }
    .protocol-s3 { background: #4a2c00; color: #ffcc80; }
    .protocol-hls { background: #1b3a1b; color: #a5d6a7; }
    .protocol-magnet { background: #2d1b3a; color: #ce93d8; }
    .protocol-torrent { background: #003840; color: #80deea; }
  }
</style>`

func pageHeader(title string) string {
	return fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s – FSMock</title>%s</head><body>`, title, pageStyle)
}

func pageFooter() string {
	return `<div class="nav"><a href="/">Home</a> <a href="/resources">Resources</a> <a href="/stats">Stats</a> <a href="/tasks">Tasks</a> <a href="/testcases">Test Cases</a></div></body></html>`
}

func main() {
	server := NewMockDownloadServer(7001)

	fmt.Println("Mock Download Server started!")
	fmt.Println("Usage examples:")
	fmt.Println("  1. Simple download:     http://localhost:7001/download")
	fmt.Println("  2. Custom size:         http://localhost:7001/download?size=50M")
	fmt.Println("  3. Custom speed:        http://localhost:7001/download?size=100M&speed=500K")
	fmt.Println("  4. Batch download:      http://localhost:7001/batch?count=5&size=20M")
	fmt.Println("  5. Stats:               http://localhost:7001/stats")
	fmt.Println("  6. List tasks:          http://localhost:7001/tasks")
	fmt.Println("  7. Test Cases:          http://localhost:7001/testcases")
	fmt.Println("  8. Stream:              http://localhost:7001/stream?size=1G&speed=5M")
	fmt.Println("  9. Resources:           http://localhost:7001/resources")
	fmt.Println(" 10. Magnet link:         http://localhost:7001/magnet?name=ubuntu&size=4G")
	fmt.Println(" 11. Torrent:             http://localhost:7001/torrent?name=test&size=100M")
	fmt.Println("\nSize units:  B, K, M, G (case-insensitive)")
	fmt.Println("Speed units: B/s, K/s, M/s, G/s")

	log.Fatal(server.Start())
}

// NewMockDownloadServer creates a new server instance.
func NewMockDownloadServer(port int) *MockDownloadServer {
	return &MockDownloadServer{
		port:  port,
		stats: ServerStats{},
	}
}

// Start begins listening.
func (s *MockDownloadServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/download", s.handleDownload)
	mux.HandleFunc("/batch", s.handleBatchDownload)
	mux.HandleFunc("/stream", s.handleStreamDownload)
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/tasks", s.handleTasks)
	mux.HandleFunc("/task/", s.handleTaskControl)
	mux.HandleFunc("/cancel/", s.handleCancelDownload)
	mux.HandleFunc("/testcases", s.handleTestCases)
	mux.HandleFunc("/resources", s.handleResources)

	// Protocol-specific endpoints
	mux.HandleFunc("/magnet", s.handleMagnet)
	mux.HandleFunc("/torrent", s.handleTorrent)
	mux.HandleFunc("/ftp/", s.handleFTPDir)
	mux.HandleFunc("/s3/", s.handleS3Object)
	mux.HandleFunc("/hls/", s.handleHLSPlaylist)

	// JSON API
	mux.HandleFunc("/api/resources", s.handleAPIResources)
	mux.HandleFunc("/api/stats", s.handleAPIStats)

	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("Server listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (s *MockDownloadServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, pageHeader("Home"))
	fmt.Fprint(w, rootHTML)
	fmt.Fprint(w, pageFooter())
}

// ---------------------------------------------------------------------------
// Protocol resource endpoints
// ---------------------------------------------------------------------------

func (s *MockDownloadServer) handleResources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, pageHeader("Resources"))

	// Generate resource cards for different types and protocols.
	type resourceCard struct {
		Name     string
		Protocol string
		ProtocolClass string
		Type     string
		Size     string
		Endpoint string
	}

	cards := []resourceCard{
		// HTTP resources
		{Name: "video_sample.mp4", Protocol: "HTTP", ProtocolClass: "protocol-http", Type: "Video", Size: "50MB", Endpoint: "/download?filename=video_sample.mp4&size=50M&speed=10M/s"},
		{Name: "audio_track.mp3", Protocol: "HTTP", ProtocolClass: "protocol-http", Type: "Audio", Size: "10MB", Endpoint: "/download?filename=audio_track.mp3&size=10M&speed=2M/s"},
		{Name: "cover_art.png", Protocol: "HTTP", ProtocolClass: "protocol-http", Type: "Image", Size: "5MB", Endpoint: "/download?filename=cover_art.png&size=5M"},
		{Name: "doc_report.pdf", Protocol: "HTTP", ProtocolClass: "protocol-http", Type: "Document", Size: "20MB", Endpoint: "/download?filename=doc_report.pdf&size=20M&speed=5M/s"},
		{Name: "large_archive.zip", Protocol: "HTTP", ProtocolClass: "protocol-http", Type: "Archive", Size: "500MB", Endpoint: "/download?filename=large_archive.zip&size=500M&speed=20M/s"},
		{Name: "small_text.txt", Protocol: "HTTP", ProtocolClass: "protocol-http", Type: "Text", Size: "10KB", Endpoint: "/download?filename=small_text.txt&size=10K"},
		// FTP resources
		{Name: "ubuntu-24.04.iso", Protocol: "FTP", ProtocolClass: "protocol-ftp", Type: "ISO", Size: "5.7GB", Endpoint: "/ftp/ubuntu-24.04.iso"},
		{Name: "release_notes.txt", Protocol: "FTP", ProtocolClass: "protocol-ftp", Type: "Text", Size: "5KB", Endpoint: "/ftp/release_notes.txt"},
		// S3 resources
		{Name: "backup_2024.tar.gz", Protocol: "S3", ProtocolClass: "protocol-s3", Type: "Archive", Size: "2GB", Endpoint: "/s3/backup_2024.tar.gz"},
		{Name: "thumbnail_1080.png", Protocol: "S3", ProtocolClass: "protocol-s3", Type: "Image", Size: "2MB", Endpoint: "/s3/thumbnail_1080.png"},
		// HLS stream resources
		{Name: "live_stream.m3u8", Protocol: "HLS", ProtocolClass: "protocol-hls", Type: "Live Stream", Size: "∞", Endpoint: "/hls/live_stream.m3u8"},
		{Name: "vod_playlist.m3u8", Protocol: "HLS", ProtocolClass: "protocol-hls", Type: "VOD", Size: "1.2GB", Endpoint: "/hls/vod_playlist.m3u8"},
		// Magnet / BitTorrent
		{Name: "ubuntu-24.04-desktop", Protocol: "Magnet", ProtocolClass: "protocol-magnet", Type: "ISO", Size: "5.7GB", Endpoint: "/magnet?name=ubuntu-24.04-desktop-amd64.iso&size=5700M"},
		{Name: "debian-12.5.0", Protocol: "Magnet", ProtocolClass: "protocol-magnet", Type: "ISO", Size: "3.7GB", Endpoint: "/magnet?name=debian-12.5.0-amd64-netinst.iso&size=3700M"},
		{Name: "fedora-40-x86_64", Protocol: "Magnet", ProtocolClass: "protocol-magnet", Type: "ISO", Size: "2.1GB", Endpoint: "/magnet?name=fedora-40-x86_64.iso&size=2100M"},
		{Name: "movie_collection", Protocol: "Torrent", ProtocolClass: "protocol-torrent", Type: "Collection", Size: "12GB", Endpoint: "/torrent?name=movie_collection&size=12G&pieces=1024"},
		{Name: "music_album_flac", Protocol: "Torrent", ProtocolClass: "protocol-torrent", Type: "Audio", Size: "350MB", Endpoint: "/torrent?name=music_album_flac&size=350M&pieces=256"},
		{Name: "ebooks_bundle", Protocol: "Torrent", ProtocolClass: "protocol-torrent", Type: "Document", Size: "150MB", Endpoint: "/torrent?name=ebooks_bundle&size=150M&pieces=128"},
	}

	// Filtering
	qProtocol := r.URL.Query().Get("protocol")
	qType := r.URL.Query().Get("type")

	fmt.Fprint(w, `<h1>Mock Resources</h1>`)
	fmt.Fprint(w, `<p>Resources available across different protocols and content types. Click to download.</p>`)
	fmt.Fprint(w, `<div style="margin-bottom:20px"><span class="tag protocol-http">HTTP</span> <span class="tag protocol-ftp">FTP</span> <span class="tag protocol-s3">S3</span> <span class="tag protocol-hls">HLS</span> <span class="tag protocol-magnet">Magnet</span> <span class="tag protocol-torrent">Torrent</span></div>`)
	fmt.Fprint(w, `<div class="resources-grid">`)

	count := 0
	for _, c := range cards {
		if qProtocol != "" && !strings.EqualFold(c.Protocol, qProtocol) {
			continue
		}
		if qType != "" && !strings.EqualFold(c.Type, qType) {
			continue
		}
		count++
		fmt.Fprintf(w, `<div class="resource-item">
			<h4>%s</h4>
			<div class="meta">
				<span class="tag %s">%s</span>
				<span class="tag">%s</span>
				<span>Size: %s</span>
			</div>
			<div class="actions">
				<a href="%s" class="btn" target="_blank">Download</a>
			</div>
		</div>`, c.Name, c.ProtocolClass, c.Protocol, c.Type, c.Size, c.Endpoint)
	}
	fmt.Fprint(w, `</div>`)
	if count == 0 {
		fmt.Fprint(w, `<p>No resources match the filter.</p>`)
	}
	fmt.Fprint(w, pageFooter())
}

// /magnet — generates a magnet: URI
func (s *MockDownloadServer) handleMagnet(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "testfile.bin"
	}
	sizeStr := r.URL.Query().Get("size")
	size := int64(100 * 1024 * 1024)
	if sizeStr != "" {
		if parsed, err := parseSize(sizeStr); err == nil && parsed > 0 {
			size = parsed
		}
	}

	infoHash := sha1Hash(fmt.Sprintf("fsmock:%s:%d:%d", name, size, time.Now().UnixNano()))
	magnet := fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s&xl=%d&tr=udp://tracker.example.com:6969/announce&tr=http://tracker.example.com:8080/announce",
		infoHash, name, size)

	// Check Accept header for HTML vs plain text.
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, pageHeader("Magnet Link"))
		fmt.Fprintf(w, `<h1>Magnet Link</h1>
<div class="card">
	<h3>%s</h3>
	<p><strong>Info Hash:</strong> <code>%s</code></p>
	<p><strong>Size:</strong> %s</p>
	<p><strong>Tracker:</strong> udp://tracker.example.com:6969/announce</p>
</div>
<div class="card">
	<h3>Magnet URI</h3>
	<div class="code">%s</div>
</div>
<div class="card">
	<h3>Simulated .torrent Metadata</h3>
	<table>
		<tr><th>Field</th><th>Value</th></tr>
		<tr><td>Announce</td><td>udp://tracker.example.com:6969/announce</td></tr>
		<tr><td>Info Hash</td><td>%s</td></tr>
		<tr><td>Piece Length</td><td>%s</td></tr>
		<tr><td>Total Size</td><td>%s</td></tr>
		<tr><td>Private</td><td>No</td></tr>
	</table>
</div>`,
			name, infoHash, formatSize(size),
			magnet, infoHash, formatSize(1024*1024), formatSize(size))
		fmt.Fprint(w, pageFooter())
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, magnet)
}

// /torrent — generates a minimal mock .torrent file
func (s *MockDownloadServer) handleTorrent(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "testfile.bin"
	}
	sizeStr := r.URL.Query().Get("size")
	size := int64(100 * 1024 * 1024)
	if sizeStr != "" {
		if parsed, err := parseSize(sizeStr); err == nil && parsed > 0 {
			size = parsed
		}
	}
	piecesStr := r.URL.Query().Get("pieces")
	pieces := 256
	if piecesStr != "" {
		if p, err := strconv.Atoi(piecesStr); err == nil && p > 0 {
			pieces = p
		}
	}

	infoHash := sha1Hash(fmt.Sprintf("fsmock:torrent:%s:%d:%d:%d", name, size, pieces, time.Now().UnixNano()))
	magnet := fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s&xl=%d", infoHash, name, size)
	pieceLen := int64(1024 * 1024) // 1 MiB pieces
	nPieces := int((size + pieceLen - 1) / pieceLen)

	// Build a minimal .torrent bencode-like payload
	payload := fmt.Sprintf("d8:announce39:udp://tracker.example.com:6969/announce13:announce-listl39:udp://tracker.example.com:6969/announce34:http://tracker.example.com:8080/announcee4:infod5:filesld6:lengthi%de4:pathl%d:%se4:name%d:%s12:piece lengthi%de6:pieces%d:%see",
		size, len(name), name, len(name), name, pieceLen, nPieces*20, strings.Repeat("x", nPieces*20))

	w.Header().Set("Content-Type", "application/x-bittorrent")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.torrent"`, name))
	w.Header().Set("X-Info-Hash", infoHash)
	w.Header().Set("X-Magnet-URI", magnet)
	w.Write([]byte(payload))
}

// /ftp/ — simulated FTP directory listing and file access
func (s *MockDownloadServer) handleFTPDir(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/ftp")
	path = strings.TrimPrefix(path, "/")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if path == "" || path == "/" {
		// Directory listing
		fmt.Fprint(w, pageHeader("FTP Directory"))
		fmt.Fprint(w, `<h1>FTP Root <span class="tag protocol-ftp">FTP</span></h1>
<div class="card">
	<h3>Directory: /</h3>
	<table>
		<tr><th>Name</th><th>Size</th><th>Type</th><th>Action</th></tr>
		<tr><td><a href="/ftp/ubuntu-24.04.iso">ubuntu-24.04.iso</a></td><td>5.7 GB</td><td>ISO</td><td><a href="/ftp/ubuntu-24.04.iso" class="btn">Download</a></td></tr>
		<tr><td><a href="/ftp/release_notes.txt">release_notes.txt</a></td><td>5 KB</td><td>Text</td><td><a href="/ftp/release_notes.txt" class="btn">Download</a></td></tr>
		<tr><td><a href="/ftp/debian/">debian/</a></td><td>-</td><td>Directory</td><td></td></tr>
		<tr><td><a href="/ftp/source/">source/</a></td><td>-</td><td>Directory</td><td></td></tr>
	</table>
</div>`)
		fmt.Fprint(w, pageFooter())
		return
	}

	// File download via FTP simulation (reuse download handler internally).
	config := defaultConfig
	switch path {
	case "ubuntu-24.04.iso":
		config.FileSize = 5700 * 1024 * 1024
		config.Filename = path
	case "release_notes.txt":
		config.FileSize = 5 * 1024
		config.Filename = path
	default:
		config.FileSize = 50 * 1024 * 1024
		config.Filename = path
	}

	if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
		if parsed, err := parseSize(sizeStr); err == nil {
			config.FileSize = parsed
		}
	}

	atomic.AddInt64(&s.stats.TotalRequests, 1)
	taskID := fmt.Sprintf("ftp-%d", time.Now().UnixNano())
	task := &DownloadTask{
		ID:         taskID,
		Config:     config,
		StartTime:  time.Now(),
		IsActive:   true,
		CancelChan: make(chan struct{}, 1),
	}
	s.activeTasks.Store(taskID, task)
	defer s.activeTasks.Delete(taskID)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", config.FileSize))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, config.Filename))

	s.streamData(w, config, task)
	atomic.AddInt64(&s.stats.CompletedDownloads, 1)
	atomic.AddInt64(&s.stats.TotalBytes, config.FileSize)
}

// /s3/ — simulated S3 object access
func (s *MockDownloadServer) handleS3Object(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/s3")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, pageHeader("S3 Bucket"))
		fmt.Fprint(w, `<h1>S3 Bucket: mock-bucket <span class="tag protocol-s3">S3</span></h1>
<div class="card">
	<table>
		<tr><th>Key</th><th>Size</th><th>Storage Class</th><th>ETag</th></tr>
		<tr><td><a href="/s3/backup_2024.tar.gz">backup_2024.tar.gz</a></td><td>2.0 GB</td><td>STANDARD</td><td>"d41d8cd98f00b204e9800998ecf8427e"</td></tr>
		<tr><td><a href="/s3/thumbnail_1080.png">thumbnail_1080.png</a></td><td>2 MB</td><td>STANDARD_IA</td><td>"abc123def456"</td></tr>
		<tr><td><a href="/s3/videos/">videos/</a></td><td>-</td><td>-</td><td>-</td></tr>
	</table>
</div>`)
		fmt.Fprint(w, pageFooter())
		return
	}

	config := defaultConfig
	switch path {
	case "backup_2024.tar.gz":
		config.FileSize = 2 * 1024 * 1024 * 1024
		config.Filename = path
	case "thumbnail_1080.png":
		config.FileSize = 2 * 1024 * 1024
		config.Filename = path
	default:
		config.FileSize = 50 * 1024 * 1024
		config.Filename = path
	}

	atomic.AddInt64(&s.stats.TotalRequests, 1)
	taskID := fmt.Sprintf("s3-%d", time.Now().UnixNano())
	task := &DownloadTask{
		ID:         taskID,
		Config:     config,
		StartTime:  time.Now(),
		IsActive:   true,
		CancelChan: make(chan struct{}, 1),
	}
	s.activeTasks.Store(taskID, task)
	defer s.activeTasks.Delete(taskID)

	etag := sha1Hash(path + fmt.Sprintf("%d", config.FileSize))[:32]
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", config.FileSize))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, config.Filename))
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, etag))
	w.Header().Set("X-Amz-Storage-Class", "STANDARD")
	w.Header().Set("X-Amz-Request-Id", fmt.Sprintf("mock-%d", time.Now().UnixNano()%100000))

	s.streamData(w, config, task)
	atomic.AddInt64(&s.stats.CompletedDownloads, 1)
	atomic.AddInt64(&s.stats.TotalBytes, config.FileSize)
}

// /hls/ — simulated HLS playlist
func (s *MockDownloadServer) handleHLSPlaylist(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/hls")
	path = strings.TrimPrefix(path, "/")

	if strings.HasSuffix(path, ".ts") {
		// Segment file.
		idxStr := path
		idxStr = strings.TrimSuffix(idxStr, ".ts")
		idxStr = strings.ReplaceAll(idxStr, "segment_", "")
		idx, _ := strconv.Atoi(idxStr)
		config := defaultConfig
		config.FileSize = 2 * 1024 * 1024 // 2MB per segment
		config.Filename = path

		atomic.AddInt64(&s.stats.TotalRequests, 1)
		taskID := fmt.Sprintf("hls-ts-%d", time.Now().UnixNano())
		task := &DownloadTask{
			ID:         taskID,
			Config:     config,
			StartTime:  time.Now(),
			IsActive:   true,
			CancelChan: make(chan struct{}, 1),
		}
		s.activeTasks.Store(taskID, task)
		defer s.activeTasks.Delete(taskID)

		w.Header().Set("Content-Type", "video/mp2t")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", config.FileSize))

		// Generate data offset by segment index.
		startOffset := int64(idx) * config.FileSize
		remaining := config.FileSize
		chunkSize := 32 * 1024
		for remaining > 0 {
			chunk := chunkSize
			if int64(chunk) > remaining {
				chunk = int(remaining)
			}
			data := make([]byte, chunk)
			for i := 0; i < chunk; i++ {
				data[i] = byte((startOffset + config.FileSize - remaining + int64(i)) % 256)
			}
			w.Write(data)
			remaining -= int64(chunk)
		}
		atomic.AddInt64(&s.stats.TotalBytes, config.FileSize)
		atomic.AddInt64(&s.stats.CompletedDownloads, 1)
		return
	}

	// Playlist response.
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	baseURL := fmt.Sprintf("http://localhost:%d", s.port)
	totalDuration := 20

	fmt.Fprintf(w, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n", totalDuration/10)
	for i := 0; i < 10; i++ {
		fmt.Fprintf(w, "#EXTINF:%d.0,\n", totalDuration/10)
		fmt.Fprintf(w, "%s/hls/%s_segment_%d.ts\n", baseURL, strings.TrimSuffix(path, ".m3u8"), i)
	}
	fmt.Fprint(w, "#EXT-X-ENDLIST\n")
}

// ---------------------------------------------------------------------------
// Download handler
// ---------------------------------------------------------------------------

func (s *MockDownloadServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.stats.TotalRequests, 1)
	atomic.AddInt64(&s.stats.ActiveDownloads, 1)
	defer atomic.AddInt64(&s.stats.ActiveDownloads, -1)

	config := parseQueryConfig(r)

	if config.FailRate > 0 && rand.Float64() < config.FailRate {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Random failure injected"))
		return
	}

	if config.Delay > 0 {
		time.Sleep(config.Delay)
	}

	taskID := fmt.Sprintf("download-%d-%d", time.Now().UnixNano(), rand.Int63())
	task := &DownloadTask{
		ID:         taskID,
		Config:     config,
		StartTime:  time.Now(),
		IsActive:   true,
		CancelChan: make(chan struct{}, 1),
	}
	s.activeTasks.Store(taskID, task)
	defer s.activeTasks.Delete(taskID)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", config.FileSize))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("X-Resource-Protocol", "http")

	filename := config.Filename
	if filename == "" {
		filename = fmt.Sprintf("testfile_%s.bin", formatSize(config.FileSize))
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		s.handleRangeRequest(w, r, config, task)
		return
	}

	s.streamData(w, config, task)
	atomic.AddInt64(&s.stats.CompletedDownloads, 1)
	atomic.AddInt64(&s.stats.TotalBytes, config.FileSize)
}

func (s *MockDownloadServer) handleRangeRequest(w http.ResponseWriter, r *http.Request, config DownloadConfig, task *DownloadTask) {
	rangeHeader := r.Header.Get("Range")
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	rangeStr := rangeHeader[6:]
	var start, end int64
	n, _ := fmt.Sscanf(rangeStr, "%d-%d", &start, &end)

	if n != 2 || start < 0 || end >= config.FileSize || start > end {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	contentLength := end - start + 1
	w.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, config.FileSize))
	w.WriteHeader(http.StatusPartialContent)
	s.streamRange(w, start, end, config, task)
	atomic.AddInt64(&s.stats.TotalBytes, contentLength)
}

func (s *MockDownloadServer) handleBatchDownload(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	count := 1
	if countStr := query.Get("count"); countStr != "" {
		if n, err := strconv.Atoi(countStr); err == nil && n > 0 && n <= 10 {
			count = n
		}
	}
	config := parseQueryConfig(r)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, pageHeader("Batch"))
	fmt.Fprintf(w, `<h1>Batch Download Links (%d files)</h1>
<div class="card"><p><strong>File Size:</strong> %s | <strong>Speed:</strong> %s/s | <strong>Total:</strong> %s</p></div>`,
		count, formatSize(config.FileSize), formatSpeed(config.Speed), formatSize(config.FileSize*int64(count)))

	for i := 0; i < count; i++ {
		url := fmt.Sprintf("/download?size=%s&speed=%s&delay=%s",
			formatSizeForURL(config.FileSize), formatSpeedForURL(config.Speed),
			time.Duration(i)*time.Second)
		fmt.Fprintf(w, "<div class=\"resource-item\"><strong>File %d:</strong> %s @ %s/s <a href=\"%s\" class=\"btn\">Download</a></div>\n",
			i+1, formatSize(config.FileSize), formatSpeed(config.Speed), url)
	}
	fmt.Fprint(w, pageFooter())
}

func (s *MockDownloadServer) handleStreamDownload(w http.ResponseWriter, r *http.Request) {
	config := parseQueryConfig(r)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"stream_%s.bin\"", formatSize(config.FileSize)))

	chunkSize := 64 * 1024
	if config.ChunkSize > 0 {
		chunkSize = config.ChunkSize
	}

	totalSent := int64(0)
	startTime := time.Now()
	failAt := int64(-1)
	if config.Fail {
		min := int64(float64(config.FileSize) * 0.1)
		max := int64(float64(config.FileSize) * 0.9)
		if max > min {
			failAt = min + rand.Int63n(max-min)
		} else {
			failAt = rand.Int63n(config.FileSize)
		}
	}

	for totalSent < config.FileSize {
		if failAt >= 0 && totalSent >= failAt {
			if hijacker, ok := w.(http.Hijacker); ok {
				conn, _, err := hijacker.Hijack()
				if err == nil {
					conn.Close()
				}
			}
			return
		}

		remaining := config.FileSize - totalSent
		currentChunk := chunkSize
		if int64(currentChunk) > remaining {
			currentChunk = int(remaining)
		}

		chunk := generateChunk(currentChunk, totalSent)
		fmt.Fprintf(w, "%x\r\n", len(chunk))
		w.Write(chunk)
		fmt.Fprint(w, "\r\n")

		totalSent += int64(currentChunk)

		if config.Speed > 0 {
			expectedTime := time.Duration(float64(totalSent) / float64(config.Speed) * float64(time.Second))
			elapsedTime := time.Since(startTime)
			if expectedTime > elapsedTime {
				time.Sleep(expectedTime - elapsedTime)
			}
		}

		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	fmt.Fprint(w, "0\r\n\r\n")
}

func (s *MockDownloadServer) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.getStats()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, pageHeader("Stats"))
	fmt.Fprintf(w, `<h1>Server Statistics</h1>
<div class="card">
<table>
<tr><th>Metric</th><th>Value</th></tr>
<tr><td>Total Requests</td><td>%d</td></tr>
<tr><td>Total Bytes Sent</td><td>%s</td></tr>
<tr><td>Active Downloads</td><td>%d</td></tr>
<tr><td>Completed Downloads</td><td>%d</td></tr>
</table></div>`,
		stats.TotalRequests, formatSize(stats.TotalBytes), stats.ActiveDownloads, stats.CompletedDownloads)
	fmt.Fprint(w, pageFooter())
}

func (s *MockDownloadServer) handleTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, pageHeader("Tasks"))
	fmt.Fprint(w, `<h1>Active Tasks</h1>`)

	hasTasks := false
	s.activeTasks.Range(func(key, value interface{}) bool {
		if _, ok := value.(*DownloadTask); ok {
			hasTasks = true
		}
		return false
	})

	if !hasTasks {
		fmt.Fprint(w, `<p>No active tasks.</p>`)
	} else {
		fmt.Fprint(w, `<div class="card"><table><tr><th>ID</th><th>Active</th><th>Start</th><th>Sent</th><th>Total</th></tr>`)
		s.activeTasks.Range(func(key, value interface{}) bool {
			if task, ok := value.(*DownloadTask); ok {
				fmt.Fprintf(w, `<tr><td>%s</td><td>%v</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
					task.ID, task.IsActive, task.StartTime.Format(time.RFC3339),
					formatSize(atomic.LoadInt64(&task.BytesSent)), formatSize(task.Config.FileSize))
			}
			return true
		})
		fmt.Fprint(w, `</table></div>`)
	}
	fmt.Fprint(w, pageFooter())
}

func (s *MockDownloadServer) handleTaskControl(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/task/")
	if taskID == "" {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}
	if val, ok := s.activeTasks.Load(taskID); ok {
		task := val.(*DownloadTask)
		action := r.URL.Query().Get("action")
		switch action {
		case "cancel":
			select {
			case task.CancelChan <- struct{}{}:
				task.IsActive = false
				w.Write([]byte("Task cancelled"))
			default:
				w.Write([]byte("Task already cancelled"))
			}
		default:
			http.Error(w, "Invalid action", http.StatusBadRequest)
		}
	} else {
		http.Error(w, "Task not found", http.StatusNotFound)
	}
}

func (s *MockDownloadServer) handleCancelDownload(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/cancel/")
	if taskID == "" {
		http.Redirect(w, r, "/tasks", http.StatusFound)
		return
	}
	if val, ok := s.activeTasks.Load(taskID); ok {
		task := val.(*DownloadTask)
		select {
		case task.CancelChan <- struct{}{}:
			task.IsActive = false
		default:
		}
	}
	http.Redirect(w, r, "/tasks", http.StatusFound)
}

func (s *MockDownloadServer) handleTestCases(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("text") == "true" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		s.writeTestCasesText(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, pageHeader("Test Cases"))
	fmt.Fprint(w, `<h1>50 Download Test Links</h1>`)
	fmt.Fprint(w, `<h3>Copy All Links:</h3><textarea readonly id="all-links">`)
	s.writeTestCasesText(w)
	fmt.Fprint(w, `</textarea>
<button onclick="document.getElementById('all-links').select();document.execCommand('copy');">Copy All</button><hr>
<h3>Detailed List</h3>`)

	sizes := []string{"1B", "100B", "1K", "10K", "1M", "10M", "100M", "1G", "2G", "100M"}

	fmt.Fprint(w, "<div class='group'><h3>1. Pagination / Standard Files (1-20) <span class='tag protocol-http'>HTTP</span></h3>")
	for i := 1; i <= 20; i++ {
		url := fmt.Sprintf("/download?filename=file_page_%02d.bin&size=2M", i)
		fmt.Fprintf(w, "<div>%02d. <a href='%s'>http://localhost:%d%s</a> (2MB)</div>\n", i, url, s.port, url)
	}
	fmt.Fprint(w, "</div>")

	fmt.Fprint(w, "<div class='group'><h3>2. Failure Scenarios (21-30) <span class='tag' style='background:#fff3cd;color:#856404'>FAIL</span></h3>")
	for i := 1; i <= 10; i++ {
		failRate := float64(i) * 0.1
		url := fmt.Sprintf("/download?filename=fail_%02d.bin&size=10M&fail=%.1f", i, failRate)
		desc := fmt.Sprintf("Fail Rate %.0f%%", failRate*100)
		if i == 10 {
			url = fmt.Sprintf("/download?filename=fail_%02d.bin&size=10M&error=true", i)
			desc = "Immediate Error"
		}
		fmt.Fprintf(w, "<div>%d. <a href='%s'>http://localhost:%d%s</a> (%s)</div>\n", 20+i, url, s.port, url, desc)
	}
	fmt.Fprint(w, "</div>")

	fmt.Fprint(w, "<div class='group'><h3>3. Slow & Delayed (31-40)</h3>")
	for i := 1; i <= 5; i++ {
		url := fmt.Sprintf("/download?filename=slow_%02d.bin&size=5M&speed=%dK", i, 100*i)
		fmt.Fprintf(w, "<div>%d. <a href='%s'>http://localhost:%d%s</a> (Speed %dKB/s)</div>\n", 30+i, url, s.port, url, 100*i)
	}
	for i := 1; i <= 5; i++ {
		url := fmt.Sprintf("/download?filename=delay_%02d.bin&size=1M&delay=%ds", i, i)
		fmt.Fprintf(w, "<div>%d. <a href='%s'>http://localhost:%d%s</a> (Delay %ds)</div>\n", 35+i, url, s.port, url, i)
	}
	fmt.Fprint(w, "</div>")

	fmt.Fprint(w, "<div class='group'><h3>4. Edge Cases (41-50)</h3>")
	for i, size := range sizes {
		url := fmt.Sprintf("/download?filename=edge_%02d.bin&size=%s", i+1, size)
		fmt.Fprintf(w, "<div>%d. <a href='%s'>http://localhost:%d%s</a> (%s)</div>\n", 40+i+1, url, s.port, url, size)
	}
	fmt.Fprint(w, "</div>")

	// New: Protocol-specific test cases
	fmt.Fprint(w, "<div class='group'><h3>5. Multi-Protocol Test Cases (51-65)</h3>")
	protocolCases := []struct {
		name     string
		endpoint string
		desc     string
	}{
		{"magnet_linux.iso", "/magnet?name=linux-distro.iso&size=5700M", "Magnet Link (5.7GB)"},
		{"torrent_linux.torrent", "/torrent?name=linux-distro.iso&size=3700M&pieces=512", "Torrent File (3.7GB)"},
		{"ftp_archive.zip", "/ftp/archive.zip?size=500M", "FTP Download (500MB)"},
		{"s3_backup.tar", "/s3/backup/mydata.tar?size=2G", "S3 Object (2GB)"},
		{"hls_stream.m3u8", "/hls/live_stream.m3u8", "HLS Playlist (20s)"},
		{"http_video_4k.mp4", "/download?filename=4k_video.mp4&size=5G&speed=50M/s", "HTTP 4K Video (5GB)"},
		{"http_ebook.pdf", "/download?filename=ebook.pdf&size=25M&speed=10M/s", "HTTP Document (25MB)"},
		{"magnet_movie.mkv", "/magnet?name=1080p_movie.mkv&size=8G", "Magnet Large File (8GB)"},
		{"torrent_games.torrent", "/torrent?name=game_collection&size=50G&pieces=2048", "Torrent Multi-File (50GB)"},
		{"s3_large_video.mp4", "/s3/large_video.mp4?size=10G", "S3 Large Object (10GB)"},
		{"ftp_document.iso", "/ftp/document.iso?size=1G", "FTP Large File (1GB)"},
		{"http_ebooks_collection.zip", "/download?filename=ebooks.zip&size=500M&speed=25M/s", "HTTP Archive (500MB)"},
		{"hls_vod.m3u8", "/hls/vod_playlist.m3u8", "HLS VOD (1.2GB)"},
		{"magnet_ebook.torrent", "/torrent?name=ebook_bundle&size=150M&pieces=128", "Torrent Small (150MB)"},
		{"http_small.bin", "/download?filename=small.bin&size=1K", "HTTP 1KB File"},
	}
	for i, pc := range protocolCases {
		fmt.Fprintf(w, "<div>%d. <a href='%s'>http://localhost:%d%s</a> (%s)</div>\n", 50+i+1, pc.endpoint, s.port, pc.endpoint, pc.desc)
	}
	fmt.Fprint(w, "</div>")

	fmt.Fprint(w, pageFooter())
}

func (s *MockDownloadServer) writeTestCasesText(w http.ResponseWriter) {
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(w, "http://localhost:%d/download?filename=file_page_%02d.bin&size=2M\n", s.port, i)
	}
	for i := 1; i <= 10; i++ {
		failRate := float64(i) * 0.1
		url := fmt.Sprintf("/download?filename=fail_%02d.bin&size=10M&fail=%.1f", i, failRate)
		if i == 10 {
			url = fmt.Sprintf("/download?filename=fail_%02d.bin&size=10M&error=true", i)
		}
		fmt.Fprintf(w, "http://localhost:%d%s\n", s.port, url)
	}
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(w, "http://localhost:%d/download?filename=slow_%02d.bin&size=5M&speed=%dK\n", s.port, i, 100*i)
	}
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(w, "http://localhost:%d/download?filename=delay_%02d.bin&size=1M&delay=%ds\n", s.port, i, i)
	}
	sizes := []string{"1B", "100B", "1K", "10K", "1M", "10M", "100M", "1G", "2G", "100M"}
	for i, size := range sizes {
		fmt.Fprintf(w, "http://localhost:%d/download?filename=edge_%02d.bin&size=%s\n", s.port, i+1, size)
	}
}

// ---------------------------------------------------------------------------
// JSON API endpoints
// ---------------------------------------------------------------------------

func (s *MockDownloadServer) handleAPIResources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resources := []map[string]interface{}{
		{"name": "video_sample.mp4", "protocol": "http", "type": "video", "size": 52428800, "endpoint": "/download?filename=video_sample.mp4&size=50M"},
		{"name": "ubuntu-24.04.iso", "protocol": "magnet", "type": "iso", "size": 6123686400, "endpoint": "/magnet?name=ubuntu-24.04.iso&size=5700M"},
		{"name": "movie_collection", "protocol": "torrent", "type": "collection", "size": 12884901888, "endpoint": "/torrent?name=movie_collection&size=12G&pieces=1024"},
		{"name": "backup_2024.tar.gz", "protocol": "s3", "type": "archive", "size": 2147483648, "endpoint": "/s3/backup_2024.tar.gz"},
		{"name": "live_stream.m3u8", "protocol": "hls", "type": "live_stream", "size": -1, "endpoint": "/hls/live_stream.m3u8"},
		{"name": "ubuntu-24.04.iso", "protocol": "ftp", "type": "iso", "size": 6123686400, "endpoint": "/ftp/ubuntu-24.04.iso"},
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"resources": resources})
}

func (s *MockDownloadServer) handleAPIStats(w http.ResponseWriter, r *http.Request) {
	stats := s.getStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_requests":       stats.TotalRequests,
		"total_bytes":          stats.TotalBytes,
		"active_downloads":     stats.ActiveDownloads,
		"completed_downloads":  stats.CompletedDownloads,
	})
}

// ---------------------------------------------------------------------------
// Data streaming
// ---------------------------------------------------------------------------

func (s *MockDownloadServer) streamData(w http.ResponseWriter, config DownloadConfig, task *DownloadTask) {
	chunkSize := config.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 32 * 1024
	}

	startTime := time.Now()
	totalSent := int64(0)

	failAt := int64(-1)
	if config.Fail {
		min := int64(float64(config.FileSize) * 0.1)
		max := int64(float64(config.FileSize) * 0.9)
		if max > min {
			failAt = min + rand.Int63n(max-min)
		} else {
			failAt = rand.Int63n(config.FileSize)
		}
	}

	for totalSent < config.FileSize {
		select {
		case <-task.CancelChan:
			return
		default:
		}

		if failAt >= 0 && totalSent >= failAt {
			if hijacker, ok := w.(http.Hijacker); ok {
				conn, _, err := hijacker.Hijack()
				if err == nil {
					conn.Close()
				}
			}
			return
		}

		remaining := config.FileSize - totalSent
		currentChunk := chunkSize
		if int64(currentChunk) > remaining {
			currentChunk = int(remaining)
		}

		chunk := generateChunk(currentChunk, totalSent)

		n, err := w.Write(chunk)
		if err != nil {
			return
		}

		totalSent += int64(n)
		atomic.AddInt64(&task.BytesSent, int64(n))

		if config.Speed > 0 {
			expectedTime := time.Duration(float64(totalSent) / float64(config.Speed) * float64(time.Second))
			elapsedTime := time.Since(startTime)
			if expectedTime > elapsedTime {
				time.Sleep(expectedTime - elapsedTime)
			}
		}

		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	task.IsActive = false
}

func (s *MockDownloadServer) streamRange(w http.ResponseWriter, start, end int64, config DownloadConfig, task *DownloadTask) {
	chunkSize := config.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 32 * 1024
	}

	current := start
	startTime := time.Now()

	failAt := int64(-1)
	if config.Fail {
		rangeLen := end - start + 1
		min := int64(float64(rangeLen) * 0.1)
		max := int64(float64(rangeLen) * 0.9)
		var relativeFailAt int64
		if max > min {
			relativeFailAt = min + rand.Int63n(max-min)
		} else {
			relativeFailAt = rand.Int63n(rangeLen)
		}
		failAt = start + relativeFailAt
	}

	for current <= end {
		select {
		case <-task.CancelChan:
			return
		default:
		}

		if failAt >= 0 && current >= failAt {
			if hijacker, ok := w.(http.Hijacker); ok {
				conn, _, err := hijacker.Hijack()
				if err == nil {
					conn.Close()
				}
			}
			return
		}

		remaining := end - current + 1
		currentChunk := chunkSize
		if int64(currentChunk) > remaining {
			currentChunk = int(remaining)
		}

		chunk := generateChunk(currentChunk, current)

		n, err := w.Write(chunk)
		if err != nil {
			return
		}

		current += int64(n)
		atomic.AddInt64(&task.BytesSent, int64(n))

		if config.Speed > 0 {
			sent := current - start
			expectedTime := time.Duration(float64(sent) / float64(config.Speed) * float64(time.Second))
			elapsedTime := time.Since(startTime)
			if expectedTime > elapsedTime {
				time.Sleep(expectedTime - elapsedTime)
			}
		}

		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (s *MockDownloadServer) getStats() ServerStats {
	return ServerStats{
		TotalRequests:      atomic.LoadInt64(&s.stats.TotalRequests),
		TotalBytes:         atomic.LoadInt64(&s.stats.TotalBytes),
		ActiveDownloads:    atomic.LoadInt64(&s.stats.ActiveDownloads),
		CompletedDownloads: atomic.LoadInt64(&s.stats.CompletedDownloads),
	}
}

func parseQueryConfig(r *http.Request) DownloadConfig {
	config := defaultConfig
	if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
		if size, err := parseSize(sizeStr); err == nil {
			config.FileSize = size
		}
	}
	if speedStr := r.URL.Query().Get("speed"); speedStr != "" {
		if speed, err := parseSpeed(speedStr); err == nil {
			config.Speed = speed
		}
	}
	if chunkStr := r.URL.Query().Get("chunk"); chunkStr != "" {
		if chunk, err := parseSize(chunkStr); err == nil {
			config.ChunkSize = int(chunk)
		}
	}
	if delayStr := r.URL.Query().Get("delay"); delayStr != "" {
		if delay, err := time.ParseDuration(delayStr); err == nil {
			config.Delay = delay
		}
	}
	if failStr := r.URL.Query().Get("fail"); failStr != "" {
		if failRate, err := strconv.ParseFloat(failStr, 64); err == nil && failRate >= 0 && failRate <= 1 {
			config.FailRate = failRate
		}
	}
	if r.URL.Query().Has("error") {
		config.Fail = true
	}
	if filename := r.URL.Query().Get("filename"); filename != "" {
		config.Filename = filename
	}
	return config
}

func parseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return defaultConfig.FileSize, nil
	}
	sizeStr = strings.ToUpper(sizeStr)
	var multiplier int64 = 1
	if strings.HasSuffix(sizeStr, "G") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "G")
	} else if strings.HasSuffix(sizeStr, "M") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "M")
	} else if strings.HasSuffix(sizeStr, "K") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "K")
	} else if strings.HasSuffix(sizeStr, "B") {
		sizeStr = strings.TrimSuffix(sizeStr, "B")
	}
	value, err := strconv.ParseFloat(sizeStr, 64)
	if err != nil {
		return 0, err
	}
	return int64(value * float64(multiplier)), nil
}

func parseSpeed(speedStr string) (int64, error) {
	if speedStr == "" {
		return defaultConfig.Speed, nil
	}
	speedStr = strings.ToUpper(speedStr)
	speedStr = strings.TrimSuffix(speedStr, "/S")
	return parseSize(speedStr)
}

func generateChunk(size int, offset int64) []byte {
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte((offset + int64(i)) % 256)
	}
	return data
}

func sha1Hash(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatSpeed(bytesPerSec int64) string {
	return formatSize(bytesPerSec)
}

func formatSizeForURL(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%d%c", bytes/div, "KMGTPE"[exp])
}

func formatSpeedForURL(bytesPerSec int64) string {
	return formatSizeForURL(bytesPerSec)
}

// ---------------------------------------------------------------------------
// Root HTML page
// ---------------------------------------------------------------------------

const rootHTML = `<h1>Mock Download Server</h1>

<div class="card">
	<h2>Quick Download <span class="tag protocol-http">HTTP</span></h2>
	<p>
		<a href="/download?size=10M&speed=1M/s" target="_blank" class="btn">10MB @ 1MB/s</a>
		<a href="/download?size=50M&speed=5M/s" target="_blank" class="btn">50MB @ 5MB/s</a>
		<a href="/download?size=100M&speed=10M/s" target="_blank" class="btn">100MB @ 10MB/s</a>
		<a href="/download?size=1G&speed=50M/s" target="_blank" class="btn">1GB @ 50MB/s</a>
	</p>
</div>

<div class="card">
	<h2>Multi-Protocol Resources</h2>
	<div class="resources-grid">
		<div class="resource-item">
			<h4>Magnet Links <span class="tag protocol-magnet">Magnet</span></h4>
			<div class="meta">BitTorrent magnet URIs with simulated trackers</div>
			<div class="actions">
				<a href="/magnet?name=ubuntu-24.04-desktop-amd64.iso&size=5700M" class="btn">Ubuntu 24.04 (5.7GB)</a>
				<a href="/magnet?name=debian-12.5.0-amd64.iso&size=3700M" class="btn">Debian 12.5 (3.7GB)</a>
				<a href="/magnet?name=1080p_movie.mkv&size=8G" class="btn">Movie (8GB)</a>
			</div>
		</div>
		<div class="resource-item">
			<h4>Torrent Files <span class="tag protocol-torrent">Torrent</span></h4>
			<div class="meta">Mock .torrent files with info hash and tracker URLs</div>
			<div class="actions">
				<a href="/torrent?name=movie_collection&size=12G&pieces=1024" class="btn">Movies (12GB)</a>
				<a href="/torrent?name=music_album_flac&size=350M&pieces=256" class="btn">Music (350MB)</a>
				<a href="/torrent?name=game_collection&size=50G&pieces=2048" class="btn">Games (50GB)</a>
			</div>
		</div>
		<div class="resource-item">
			<h4>FTP <span class="tag protocol-ftp">FTP</span></h4>
			<div class="meta">Simulated FTP directory listing and file downloads</div>
			<div class="actions">
				<a href="/ftp/" class="btn">Browse FTP Root</a>
				<a href="/ftp/ubuntu-24.04.iso" class="btn">Ubuntu ISO (5.7GB)</a>
			</div>
		</div>
		<div class="resource-item">
			<h4>S3 <span class="tag protocol-s3">S3</span></h4>
			<div class="meta">Simulated S3 bucket with object metadata</div>
			<div class="actions">
				<a href="/s3/" class="btn">Browse Bucket</a>
				<a href="/s3/backup_2024.tar.gz" class="btn">Backup (2GB)</a>
			</div>
		</div>
		<div class="resource-item">
			<h4>HLS Streams <span class="tag protocol-hls">HLS</span></h4>
			<div class="meta">M3U8 playlists with mock .ts segments</div>
			<div class="actions">
				<a href="/hls/live_stream.m3u8" class="btn">Live Stream</a>
				<a href="/hls/vod_playlist.m3u8" class="btn">VOD (1.2GB)</a>
			</div>
		</div>
	</div>
</div>

<div class="card">
	<h2>Custom Download <span class="tag protocol-http">HTTP</span></h2>
	<form action="/download" method="get" target="_blank">
		<div class="form-group">
			<label>File Size</label>
			<input type="text" name="size" value="100M" placeholder="e.g., 100M, 1G">
		</div>
		<div class="form-group">
			<label>Speed</label>
			<input type="text" name="speed" value="1M/s" placeholder="e.g., 1M/s, 500K/s">
		</div>
		<div class="form-group">
			<label>Chunk Size</label>
			<select name="chunk">
				<option value="">Default (32KB)</option>
				<option value="64K">64KB</option>
				<option value="128K">128KB</option>
				<option value="1M">1MB</option>
			</select>
		</div>
		<div class="form-group">
			<label>Filename</label>
			<input type="text" name="filename" placeholder="custom.bin">
		</div>
		<button type="submit">Start Download</button>
	</form>
</div>

<div class="card">
	<h2>Batch Download <span class="tag protocol-http">HTTP</span></h2>
	<form action="/batch" method="get" target="_blank">
		<div class="form-group">
			<label>Count</label>
			<input type="number" name="count" value="3" min="1" max="10">
		</div>
		<div class="form-group">
			<label>File Size</label>
			<input type="text" name="size" value="10M">
		</div>
		<div class="form-group">
			<label>Speed</label>
			<input type="text" name="speed" value="1M/s">
		</div>
		<button type="submit">Start Batch</button>
	</form>
</div>

<div class="card">
	<h2>API Examples</h2>
	<div class="code">
		<span class="label"># Single download</span><br>
		GET <span class="val">/download?size=100M&speed=1M/s</span><br>
		<br>
		<span class="label"># Magnet link</span><br>
		GET <span class="val">/magnet?name=ubuntu.iso&size=4G</span><br>
		<br>
		<span class="label"># Torrent file</span><br>
		GET <span class="val">/torrent?name=test&size=100M&pieces=256</span><br>
		<br>
		<span class="label"># HLS playlist</span><br>
		GET <span class="val">/hls/live_stream.m3u8</span><br>
		<br>
		<span class="label"># With Range header</span><br>
		curl -H <span class="val">"Range: bytes=0-1048575"</span> http://localhost:7001/download?size=10M<br>
		<br>
		<span class="label"># JSON Resources API</span><br>
		GET <span class="val">/api/resources</span><br>
		<br>
		<span class="label"># JSON Stats API</span><br>
		GET <span class="val">/api/stats</span>
	</div>
</div>

<div class="card">
	<h2>Quick Links</h2>
	<p>
		<a href="/resources" class="btn">Browse All Resources</a>
		<a href="/testcases" class="btn">View 65 Test Cases</a>
		<a href="/stats" class="btn">View Stats</a>
		<a href="/tasks" class="btn">Active Tasks</a>
		<a href="/api/resources" class="btn" target="_blank">API (JSON)</a>
	</p>
</div>`
