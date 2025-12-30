package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type apiResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func buildClient(insecure bool) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        1024,
		MaxIdleConnsPerHost: 1024,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecure,
		},
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
}

func buildURL(base string, p string, keyword string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if p == "" {
		p = "/api/channels/contact/search"
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + p
	q := u.Query()
	q.Set("keyword", keyword)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func percentile(sorted []time.Duration, q float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * q)
	return sorted[idx]
}

func main() {
	addr := flag.String("addr", "http://127.0.0.1:2022", "API服务地址")
	path := flag.String("path", "/api/channels/contact/search", "接口路径")
	concurrency := flag.Int("concurrency", 20, "并发数")
	requests := flag.Int("requests", 0, "总请求数(0表示按时长运行)")
	duration := flag.Duration("duration", 10*time.Second, "运行时长")
	insecure := flag.Bool("insecure", false, "忽略TLS证书校验")
	keywordsStr := flag.String("keywords", "龙虾,山海观雾,陶桃,纸鱼,小埋姐姐", "关键词列表，逗号分隔")
	outfile := flag.String("outfile", "result.txt", "输出文件路径")
	flag.Parse()

	keywords := strings.Split(*keywordsStr, ",")
	for i := range keywords {
		keywords[i] = strings.TrimSpace(keywords[i])
	}
	if len(keywords) == 0 {
		keywords = []string{"测试"}
	}
	rand.Seed(time.Now().UnixNano())

	client := buildClient(*insecure)

	var ctx context.Context
	var cancel context.CancelFunc
	if *requests == 0 {
		ctx, cancel = context.WithTimeout(context.Background(), *duration)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	var total int64
	var success int64
	var failure int64
	var latencies []time.Duration
	var latMu sync.Mutex
	var httpErrs sync.Map

	jobs := make(chan int, *concurrency*2)
	lines := make(chan string, *concurrency*4)

	var wwg sync.WaitGroup
	wwg.Add(1)
	go func() {
		defer wwg.Done()
		f, err := os.Create(*outfile)
		if err != nil {
			fmt.Println("文件创建失败", err.Error())
			return
		}
		defer f.Close()
		w := bufio.NewWriterSize(f, 1<<20)
		for line := range lines {
			_, _ = w.WriteString(line)
			_, _ = w.WriteString("\n")
		}
		_ = w.Flush()
	}()

	var wg sync.WaitGroup
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				kw := keywords[rand.Intn(len(keywords))]
				fullURL, err := buildURL(*addr, *path, kw)
				start := time.Now()
				var ok bool
				var httpCode int
				var apiCode int
				var e string
				if err != nil {
					e = err.Error()
				} else {
					req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, fullURL, nil)
					if reqErr != nil {
						e = reqErr.Error()
					} else {
						resp, doErr := client.Do(req)
						if doErr != nil {
							e = doErr.Error()
						} else {
							httpCode = resp.StatusCode
							var ar apiResponse
							decodeErr := func() error {
								defer resp.Body.Close()
								return json.NewDecoder(resp.Body).Decode(&ar)
							}()
							if decodeErr != nil {
								e = decodeErr.Error()
							} else {
								apiCode = ar.Code
								if httpCode == 200 && ar.Code == 0 {
									ok = true
								}
							}
						}
					}
				}
				lat := time.Since(start)
				atomic.AddInt64(&total, 1)
				latMu.Lock()
				latencies = append(latencies, lat)
				latMu.Unlock()
				ts := time.Now().UnixMilli()
				line := fmt.Sprintf("%d\t%s\t%d\t%d\t%d\t%t\t%s", ts, kw, httpCode, apiCode, lat.Microseconds(), ok, e)
				select {
				case lines <- line:
				default:
					lines <- line
				}
				if ok {
					atomic.AddInt64(&success, 1)
				} else {
					atomic.AddInt64(&failure, 1)
					key := strconv.Itoa(httpCode) + ":" + strconv.Itoa(apiCode)
					v, _ := httpErrs.LoadOrStore(key, int64(0))
					httpErrs.Store(key, v.(int64)+1)
				}
			}
		}()
	}

	startAll := time.Now()
	if *requests > 0 {
		for i := 0; i < *requests; i++ {
			select {
			case <-ctx.Done():
				break
			default:
			}
			jobs <- i
		}
		close(jobs)
		wg.Wait()
	} else {
	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			default:
				jobs <- 1
			}
		}
		close(jobs)
		wg.Wait()
	}
	elapsed := time.Since(startAll)
	close(lines)
	wwg.Wait()

	if len(latencies) == 0 {
		fmt.Println("无结果")
		return
	}
	latMu.Lock()
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	latMu.Unlock()
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var sum time.Duration
	for _, v := range sorted {
		sum += v
	}
	mean := time.Duration(int64(sum) / int64(len(sorted)))
	rps := float64(total) / elapsed.Seconds()

	fmt.Printf("地址: %s%s\n", *addr, *path)
	fmt.Printf("并发: %d\n", *concurrency)
	if *requests > 0 {
		fmt.Printf("请求: %d\n", *requests)
	} else {
		fmt.Printf("时长: %s\n", duration.String())
	}
	fmt.Printf("总数: %d 成功: %d 失败: %d RPS: %.2f\n", total, success, failure, rps)
	fmt.Printf("延迟 平均: %s P50: %s P90: %s P99: %s 最大: %s\n", mean.String(), percentile(sorted, 0.50).String(), percentile(sorted, 0.90).String(), percentile(sorted, 0.99).String(), sorted[len(sorted)-1].String())

	type kv struct {
		k string
		v int64
	}
	var errs []kv
	httpErrs.Range(func(key, value any) bool {
		errs = append(errs, kv{k: key.(string), v: value.(int64)})
		return true
	})
	if len(errs) > 0 {
		fmt.Println("错误分布:")
		for _, e := range errs {
			fmt.Printf("%s -> %d\n", e.k, e.v)
		}
	}
}
