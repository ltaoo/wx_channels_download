package download

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

var file_mutex sync.Mutex

type FileDownloadProgress struct {
	Current int64
	Total   int64
	Speed   float64
}

// Display progress for all threads
func display_progress(progress_chans []chan FileDownloadProgress, stop chan bool) {
	// Store progress for each thread
	progresses := make([]FileDownloadProgress, len(progress_chans))

	// Clear screen and move cursor to top-left
	fmt.Print("\033[2J\033[H")

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			// Move cursor to top-left
			fmt.Print("\033[H")

			// Display progress for each thread
			total_downloaded := int64(0)
			for i, progress := range progresses {
				if progress.Total > 0 {
					percentage := float64(progress.Current) / float64(progress.Total) * 100
					fmt.Printf("Thread %d: [%-50s] %.1f%% (%.1f KB/s)\n",
						i+1,
						progress_bar(percentage, 50),
						percentage,
						progress.Speed/1024,
					)
					total_downloaded += progress.Current
				} else {
					fmt.Printf("Thread %d: waiting to start...\n", i+1)
				}
			}

			// Display total progress
			if len(progresses) > 0 && progresses[0].Total > 0 {
				totalSize := progresses[0].Total * int64(len(progresses))
				totalPercentage := float64(total_downloaded) / float64(totalSize) * 100
				fmt.Printf("\nTotal progress: [%-50s] %.1f%%\n",
					progress_bar(totalPercentage, 50),
					totalPercentage,
				)
			}

			// Update progress information
			for i, ch := range progress_chans {
				select {
				case progress := <-ch:
					progresses[i] = progress
				default:
					// Non-blocking, use previous progress
				}
			}
		}
	}
}

// Generate progress bar string
func progress_bar(percentage float64, width int) string {
	completed := int(percentage / 100 * float64(width))
	if completed > width {
		completed = width
	}

	bar := ""
	for i := 0; i < width; i++ {
		if i < completed {
			bar += "="
		} else if i == completed {
			bar += ">"
		} else {
			bar += " "
		}
	}
	return bar
}

func calculate_chunks(total_size, preferred_chunk_size int64) []struct{ start, end int64 } {
	const (
		minChunkSize = 1 * 1024 * 1024 // 1MB minimum chunk size
		maxChunks    = 8               // Maximum chunk count
		minChunks    = 2               // Minimum chunk count
	)

	// Calculate initial chunk count
	num_chunks := total_size / preferred_chunk_size
	if num_chunks < minChunks {
		num_chunks = minChunks
	} else if num_chunks > maxChunks {
		num_chunks = maxChunks
	}

	// Recalculate chunk size
	chunk_size := total_size / num_chunks

	// Ensure chunk is not smaller than minimum
	if chunk_size < minChunkSize {
		chunk_size = minChunkSize
		num_chunks = total_size / chunk_size
		if num_chunks == 0 {
			num_chunks = 1
		}
	}

	var chunks []struct{ start, end int64 }
	for i := int64(0); i < num_chunks; i++ {
		start := i * chunk_size
		end := start + chunk_size - 1

		// Last chunk contains all remaining data
		if i == num_chunks-1 {
			end = total_size - 1
		}

		chunks = append(chunks, struct{ start, end int64 }{start, end})
	}

	return chunks
}

// Download a file chunk with progress display
func download_part_with_progress(url string, file *os.File, start, end int64, thread_idx int, progress_chan chan<- FileDownloadProgress) error {
	client := &http.Client{Timeout: 0} // No timeout limit

	// Create request with Range header
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	range_header := fmt.Sprintf("bytes=%d-%d", start, end)
	req.Header.Add("Range", range_header)

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
	}

	// Create reader with progress tracking
	total_size := end - start + 1
	progress_reader := &ProgressReader{
		Reader:  resp.Body,
		Total:   total_size,
		Thread:  thread_idx,
		Channel: progress_chan,
	}

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, progress_reader); err != nil {
		return err
	}
	// Write downloaded data to the specified position in the file

	// Synchronized file write
	file_mutex.Lock()
	defer file_mutex.Unlock()
	// Seek to the specified position in the file
	_, err = file.Seek(start, io.SeekStart)
	if err != nil {
		return err
	}
	if _, err := file.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

// Reader with progress tracking
type ProgressReader struct {
	Reader    io.Reader
	Total     int64
	Thread    int
	Channel   chan<- FileDownloadProgress
	read      int64
	lastRead  int64
	lastTime  time.Time
	startTime time.Time
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	if pr.startTime.IsZero() {
		pr.startTime = time.Now()
		pr.lastTime = pr.startTime
	}

	n, err := pr.Reader.Read(p)
	pr.read += int64(n)

	// Calculate download speed
	now := time.Now()
	elapsed := now.Sub(pr.lastTime).Seconds()

	if elapsed >= 0.1 { // Update progress every 100ms
		speed := float64(pr.read-pr.lastRead) / elapsed

		// Send progress information
		select {
		case pr.Channel <- FileDownloadProgress{
			Current: pr.read,
			Total:   pr.Total,
			Speed:   speed,
		}:
		default:
			// Non-blocking, skip if channel is full
		}

		pr.lastRead = pr.read
		pr.lastTime = now
	}

	return n, err
}

type PartialFileDownloadProgress struct {
	DownloadedSize int64
	TotalSize      int64
	Percent        float64
}

func SingleThreadingDownload(url string, dest_filepath string, on_progress func(progress *PartialFileDownloadProgress)) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("下载失败 %v", err.Error())
	}
	defer resp.Body.Close()
	file, err := os.Create(dest_filepath)
	if err != nil {
		return fmt.Errorf("创建文件失败 %v", err.Error())
	}
	defer file.Close()
	content_length := resp.Header.Get("Content-Length")
	total_size := int64(-1)
	if content_length != "" {
		total_size, _ = strconv.ParseInt(content_length, 10, 64)
	}
	buf := make([]byte, 32*1024) // 32KB buffer
	var downloaded int64 = 0
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, werr := file.Write(buf[:n])
			if werr != nil {
				return fmt.Errorf("写入文件失败 %v", werr.Error())
			}
			downloaded += int64(n)
			if total_size > 0 {
				percent := float64(downloaded) / float64(total_size) * 100
				on_progress(&PartialFileDownloadProgress{
					DownloadedSize: downloaded,
					TotalSize:      total_size,
					Percent:        percent,
				})
				// fmt.Printf("\r\033[Kdownloaded: %d/%d bytes (%.2f%%)", downloaded, total_size, percent)
			} else {
				on_progress(&PartialFileDownloadProgress{
					DownloadedSize: downloaded,
					TotalSize:      0,
					Percent:        0,
				})
				// fmt.Printf("\r\033[Kdownloaded: %d bytes", downloaded)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取文件流失败 %v", err.Error())
		}
	}
	return nil
}

func MultiThreadingDownload(url string, threads int, dest_filepath string, tmp_dest_filepath string) error {
	tr := &http.Transport{
		TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}
	client := &http.Client{Transport: tr, Timeout: 30 * time.Second}
	// Send HEAD request to get file info
	resp, err := client.Head(url)
	if err != nil {
		return fmt.Errorf("获取文件信息失败 %v", err.Error())
	}
	defer resp.Body.Close()

	fmt.Print("\033c")

	// Check if range requests are supported
	if resp.Header.Get("Accept-Ranges") != "bytes" {
		return fmt.Errorf("服务器不支持并发下载")
	}
	content_length := resp.Header.Get("Content-Length")
	if content_length == "" {
		return fmt.Errorf("无法获取总文件大小，服务器不支持并发下载")
	}

	file_size, err := strconv.ParseInt(content_length, 10, 64)
	if err != nil {
		return fmt.Errorf("解析文件大小失败: %v", err)
	}

	// start_time := time.Now()
	var wg sync.WaitGroup
	errors := make(chan error, threads)

	// Calculate chunk size for each thread
	part_size := file_size / int64(threads)
	// remainder := file_size % int64(threads)

	chunks := calculate_chunks(file_size, part_size)
	// Create progress channels, one per thread
	progress_chans := make([]chan FileDownloadProgress, len(chunks))
	for i := range progress_chans {
		progress_chans[i] = make(chan FileDownloadProgress, 10)
	}

	// Start progress display
	stop_progress := make(chan bool)
	go display_progress(progress_chans, stop_progress)

	file, err := os.Create(dest_filepath)
	if err != nil {
		return fmt.Errorf("创建文件失败 %v\n", err)
	}
	defer file.Close()
	if err := file.Truncate(file_size); err != nil {
		return fmt.Errorf("设置文件大小失败: %v", err)
	}

	// fmt.Println("the chunk size is", len(chunks))
	// Start concurrent downloads
	for i, chunk := range chunks {
		wg.Add(1)
		// fmt.Println(i, chunk)
		go func(thread_idx int, start, end int64) {
			defer wg.Done()
			// file, err := os.Create(tmp_dest_filepath + "_" + strconv.Itoa(thread_idx))
			// if err != nil {
			// 	errors <- fmt.Errorf("failed to create file %d: %v", thread_idx+1, err)
			// 	return
			// }
			// defer file.Close()
			if err := download_part_with_progress(
				url,
				file,
				start,
				end,
				thread_idx,
				progress_chans[thread_idx],
			); err != nil {
				errors <- fmt.Errorf("线程 %d 下载失败: %v", thread_idx+1, err)
			}
		}(i, chunk.start, chunk.end)
	}
	// Wait for all downloads to complete
	wg.Wait()
	close(errors)
	close(stop_progress)

	// Check for errors
	if len(errors) > 0 {
		// for err := range errors {
		// 	fmt.Println(err)
		// }
		return fmt.Errorf("下载失败，共%v个错误", len(errors))
	}
	return nil
}
