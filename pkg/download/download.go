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

// 显示所有线程的进度
func display_progress(progress_chans []chan FileDownloadProgress, stop chan bool) {
	// 存储每个线程的进度
	progresses := make([]FileDownloadProgress, len(progress_chans))

	// 清屏并移动光标到左上角
	fmt.Print("\033[2J\033[H")

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			// 移动光标到左上角
			fmt.Print("\033[H")

			// 显示每个线程的进度
			total_downloaded := int64(0)
			for i, progress := range progresses {
				if progress.Total > 0 {
					percentage := float64(progress.Current) / float64(progress.Total) * 100
					fmt.Printf("线程 %d: [%-50s] %.1f%% (%.1f KB/s)\n",
						i+1,
						progress_bar(percentage, 50),
						percentage,
						progress.Speed/1024,
					)
					total_downloaded += progress.Current
				} else {
					fmt.Printf("线程 %d: 等待开始...\n", i+1)
				}
			}

			// 显示总进度
			if len(progresses) > 0 && progresses[0].Total > 0 {
				totalSize := progresses[0].Total * int64(len(progresses))
				totalPercentage := float64(total_downloaded) / float64(totalSize) * 100
				fmt.Printf("\n总进度: [%-50s] %.1f%%\n",
					progress_bar(totalPercentage, 50),
					totalPercentage,
				)
			}

			// 更新进度信息
			for i, ch := range progress_chans {
				select {
				case progress := <-ch:
					progresses[i] = progress
				default:
					// 不阻塞，使用上次的进度
				}
			}
		}
	}
}

// 生成进度条字符串
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
		minChunkSize = 1 * 1024 * 1024 // 1MB最小分块
		maxChunks    = 8               // 最大分块数
		minChunks    = 2               // 最小分块数
	)

	// 计算初始分块数
	num_chunks := total_size / preferred_chunk_size
	if num_chunks < minChunks {
		num_chunks = minChunks
	} else if num_chunks > maxChunks {
		num_chunks = maxChunks
	}

	// 重新计算分块大小
	chunk_size := total_size / num_chunks

	// 确保分块不小于最小值
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

		// 最后一个分块包含剩余所有数据
		if i == num_chunks-1 {
			end = total_size - 1
		}

		chunks = append(chunks, struct{ start, end int64 }{start, end})
	}

	return chunks
}

// 带进度显示的文件分块下载
func download_part_with_progress(url string, file *os.File, start, end int64, thread_idx int, progress_chan chan<- FileDownloadProgress) error {
	client := &http.Client{Timeout: 0} // 无超时限制

	// 创建带Range头的请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	range_header := fmt.Sprintf("bytes=%d-%d", start, end)
	req.Header.Add("Range", range_header)

	// 执行请求
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
	}

	// 创建带进度统计的Reader
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
	// 将下载的数据写入文件的指定位置

	// 同步写入文件
	file_mutex.Lock()
	defer file_mutex.Unlock()
	// 定位到文件的指定位置
	_, err = file.Seek(start, io.SeekStart)
	if err != nil {
		return err
	}
	if _, err := file.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

// 带进度统计的Reader
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

	// 计算下载速度
	now := time.Now()
	elapsed := now.Sub(pr.lastTime).Seconds()

	if elapsed >= 0.1 { // 每100ms更新一次进度
		speed := float64(pr.read-pr.lastRead) / elapsed

		// 发送进度信息
		select {
		case pr.Channel <- FileDownloadProgress{
			Current: pr.read,
			Total:   pr.Total,
			Speed:   speed,
		}:
		default:
			// 不阻塞，如果通道满了就跳过
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
				// fmt.Printf("\r\033[K已下载: %d/%d 字节 (%.2f%%)", downloaded, total_size, percent)
			} else {
				on_progress(&PartialFileDownloadProgress{
					DownloadedSize: downloaded,
					TotalSize:      0,
					Percent:        0,
				})
				// fmt.Printf("\r\033[K已下载: %d 字节", downloaded)
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
	// 发送HEAD请求获取文件信息
	resp, err := client.Head(url)
	if err != nil {
		return fmt.Errorf("获取文件信息失败 %v", err.Error())
	}
	defer resp.Body.Close()

	fmt.Print("\033c")

	// 检查是否支持断点续传
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

	// 计算每个分块的大小
	part_size := file_size / int64(threads)
	// remainder := file_size % int64(threads)

	chunks := calculate_chunks(file_size, part_size)
	// 创建进度通道，每个线程一个
	progress_chans := make([]chan FileDownloadProgress, len(chunks))
	for i := range progress_chans {
		progress_chans[i] = make(chan FileDownloadProgress, 10)
	}

	// 启动进度显示器
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
	// 启动并发下载
	for i, chunk := range chunks {
		wg.Add(1)
		// fmt.Println(i, chunk)
		go func(thread_idx int, start, end int64) {
			defer wg.Done()
			// file, err := os.Create(tmp_dest_filepath + "_" + strconv.Itoa(thread_idx))
			// if err != nil {
			// 	errors <- fmt.Errorf("创建文件 %d 失败: %v", thread_idx+1, err)
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
	// 等待所有下载完成
	wg.Wait()
	close(errors)
	close(stop_progress)

	// 检查错误
	if len(errors) > 0 {
		// for err := range errors {
		// 	fmt.Println(err)
		// }
		return fmt.Errorf("下载失败，共%v个错误", len(errors))
	}
	return nil
}
