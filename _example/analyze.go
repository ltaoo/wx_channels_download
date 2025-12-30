package main

import (
	"bufio"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type rec struct {
	ts    int64
	kw    string
	http  int
	api   int
	latUS int64
	ok    bool
	err   string
}

func parseLine(line string) (rec, bool) {
	fs := strings.SplitN(line, "\t", 7)
	if len(fs) < 6 {
		return rec{}, false
	}
	var r rec
	ts, e1 := strconv.ParseInt(fs[0], 10, 64)
	http, e2 := strconv.Atoi(fs[2])
	api, e3 := strconv.Atoi(fs[3])
	latUS, e4 := strconv.ParseInt(fs[4], 10, 64)
	ok := fs[5] == "true"
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		return rec{}, false
	}
	r.ts = ts
	r.kw = fs[1]
	r.http = http
	r.api = api
	r.latUS = latUS
	r.ok = ok
	if len(fs) >= 7 {
		r.err = fs[6]
	}
	return r, true
}

func quantiles(sorted []int64, q float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * q)
	return sorted[idx]
}

func main() {
	file := flag.String("file", "result.txt", "结果文件路径")
	out := flag.String("out", "", "输出HTML路径，默认与结果文件同名.html")
	flag.Parse()
	f, err := os.Open(*file)
	if err != nil {
		fmt.Println("打开文件失败", err.Error())
		return
	}
	defer f.Close()
	var total int64
	var success int64
	var failure int64
	var latencies []int64
	httpApi := map[string]int64{}
	kwDist := map[string][2]int64{}
	sc := bufio.NewScanner(f)
	buf := make([]byte, 1024*1024)
	sc.Buffer(buf, 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		r, ok := parseLine(line)
		if !ok {
			continue
		}
		total++
		latencies = append(latencies, r.latUS)
		key := strconv.Itoa(r.http) + ":" + strconv.Itoa(r.api)
		httpApi[key]++
		d := kwDist[r.kw]
		d[0]++
		if r.ok {
			success++
			d[1]++
		} else {
			failure++
		}
		kwDist[r.kw] = d
	}
	if err := sc.Err(); err != nil {
		fmt.Println("读取失败", err.Error())
		return
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	var sum int64
	for _, v := range latencies {
		sum += v
	}
	var mean float64
	if len(latencies) > 0 {
		mean = float64(sum) / float64(len(latencies))
	}
	fmt.Printf("TOTAL=%d\n", total)
	fmt.Printf("SUCCESS=%d\n", success)
	fmt.Printf("FAILURE=%d\n", failure)
	if len(latencies) > 0 {
		fmt.Printf("MEAN_US=%.2f\n", mean)
		fmt.Printf("MIN_US=%d\n", latencies[0])
		fmt.Printf("P50_US=%d\n", quantiles(latencies, 0.50))
		fmt.Printf("P90_US=%d\n", quantiles(latencies, 0.90))
		fmt.Printf("P99_US=%d\n", quantiles(latencies, 0.99))
		fmt.Printf("MAX_US=%d\n", latencies[len(latencies)-1])
	}
	type kv struct {
		k string
		v int64
	}
	var dist []kv
	for k, v := range httpApi {
		dist = append(dist, kv{k: k, v: v})
	}
	sort.Slice(dist, func(i, j int) bool { return dist[i].v > dist[j].v })
	fmt.Println("HTTP_API_DIST_TOP10:")
	for i := 0; i < len(dist) && i < 10; i++ {
		fmt.Printf("%s\t%d\n", dist[i].k, dist[i].v)
	}
	type kwItem struct {
		k   string
		c   int64
		suc int64
	}
	var kws []kwItem
	for k, v := range kwDist {
		kws = append(kws, kwItem{k: k, c: v[0], suc: v[1]})
	}
	sort.Slice(kws, func(i, j int) bool { return kws[i].c > kws[j].c })
	fmt.Println("KEYWORD_TOP20:")
	for i := 0; i < len(kws) && i < 20; i++ {
		fmt.Printf("%d\t%s\t%d\n", kws[i].c, kws[i].k, kws[i].suc)
	}
	type httpItem struct {
		Key   string
		Count int64
	}
	type kwView struct {
		Keyword string
		Count   int64
		Success int64
		Rate    float64
	}
	type report struct {
		File        string
		GeneratedAt string
		Total       int64
		Success     int64
		Failure     int64
		HasLatency  bool
		MeanUS      float64
		MinUS       int64
		P50US       int64
		P90US       int64
		P99US       int64
		MaxUS       int64
		HttpTop     []httpItem
		KwTop       []kwView
	}
	var ht []httpItem
	for i := 0; i < len(dist) && i < 10; i++ {
		ht = append(ht, httpItem{Key: dist[i].k, Count: dist[i].v})
	}
	var kt []kwView
	for i := 0; i < len(kws) && i < 20; i++ {
		r := 0.0
		if kws[i].c > 0 {
			r = float64(kws[i].suc) / float64(kws[i].c)
		}
		kt = append(kt, kwView{Keyword: kws[i].k, Count: kws[i].c, Success: kws[i].suc, Rate: r})
	}
	var p50, p90, p99, min, max int64
	var hasLat bool
	if len(latencies) > 0 {
		hasLat = true
		min = latencies[0]
		max = latencies[len(latencies)-1]
		p50 = quantiles(latencies, 0.50)
		p90 = quantiles(latencies, 0.90)
		p99 = quantiles(latencies, 0.99)
	}
	data := report{
		File:        *file,
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		Total:       total,
		Success:     success,
		Failure:     failure,
		HasLatency:  hasLat,
		MeanUS:      mean,
		MinUS:       min,
		P50US:       p50,
		P90US:       p90,
		P99US:       p99,
		MaxUS:       max,
		HttpTop:     ht,
		KwTop:       kt,
	}
	tpl := `
<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>分析报告</title>
  <style>
    :root { color-scheme: light dark; }
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", "Liberation Sans", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif; margin: 24px; line-height: 1.6; }
    h1 { font-size: 24px; margin: 0 0 8px; }
    .meta { color: #666; font-size: 14px; margin-bottom: 16px; }
    .card { border: 1px solid #ddd; border-radius: 8px; padding: 16px; margin-bottom: 16px; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit,minmax(220px,1fr)); gap: 12px; }
    .stat { background: rgba(0,0,0,0.03); border-radius: 8px; padding: 12px; }
    .stat .label { font-size: 12px; color: #666; }
    .stat .value { font-weight: 600; font-size: 18px; }
    table { width: 100%; border-collapse: collapse; margin-top: 8px; }
    th, td { border: 1px solid #ddd; padding: 8px; text-align: left; font-size: 14px; }
    th { background: rgba(0,0,0,0.05); }
    .section-title { font-size: 18px; font-weight: 600; margin: 8px 0; }
    .two-col { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
    @media (max-width: 800px) { .two-col { grid-template-columns: 1fr; } }
  </style>
</head>
<body>
  <h1>分析报告</h1>
  <div class="meta">源文件：{{.File}} | 生成时间：{{.GeneratedAt}}</div>
  <div class="card">
    <div class="grid">
      <div class="stat"><div class="label">总请求</div><div class="value">{{.Total}}</div></div>
      <div class="stat"><div class="label">成功</div><div class="value">{{.Success}}</div></div>
      <div class="stat"><div class="label">失败</div><div class="value">{{.Failure}}</div></div>
      {{if .HasLatency}}<div class="stat"><div class="label">平均耗时(μs)</div><div class="value">{{printf "%.2f" .MeanUS}}</div></div>{{end}}
    </div>
    {{if .HasLatency}}
    <div class="section-title">耗时分布</div>
    <table>
      <tr><th>指标</th><th>耗时(μs)</th></tr>
      <tr><td>最小</td><td>{{.MinUS}}</td></tr>
      <tr><td>P50</td><td>{{.P50US}}</td></tr>
      <tr><td>P90</td><td>{{.P90US}}</td></tr>
      <tr><td>P99</td><td>{{.P99US}}</td></tr>
      <tr><td>最大</td><td>{{.MaxUS}}</td></tr>
    </table>
    {{end}}
  </div>
  <div class="two-col">
    <div class="card">
      <div class="section-title">HTTP:API 热门 Top 10</div>
      <table>
        <tr><th>HTTP:API</th><th>次数</th></tr>
        {{range .HttpTop}}
        <tr><td>{{.Key}}</td><td>{{.Count}}</td></tr>
        {{end}}
      </table>
    </div>
    <div class="card">
      <div class="section-title">关键词 Top 20</div>
      <table>
        <tr><th>关键词</th><th>次数</th><th>成功</th><th>成功率</th></tr>
        {{range .KwTop}}
        <tr><td>{{.Keyword}}</td><td>{{.Count}}</td><td>{{.Success}}</td><td>{{printf "%.2f%%" (mul100 .Rate)}}</td></tr>
        {{end}}
      </table>
    </div>
  </div>
</body>
</html>
`
	funcMap := template.FuncMap{
		"mul100": func(v float64) float64 { return v * 100 },
	}
	t, err := template.New("report").Funcs(funcMap).Parse(tpl)
	if err != nil {
		fmt.Println("模板解析失败", err.Error())
		return
	}
	outPath := *out
	if outPath == "" {
		base := filepath.Base(*file)
		ext := filepath.Ext(base)
		if ext != "" {
			outPath = strings.TrimSuffix(base, ext) + ".html"
		} else {
			outPath = base + ".html"
		}
	}
	of, err := os.Create(outPath)
	if err != nil {
		fmt.Println("创建HTML失败", err.Error())
		return
	}
	defer of.Close()
	if err := t.Execute(of, data); err != nil {
		fmt.Println("写入HTML失败", err.Error())
		return
	}
	fmt.Println("HTML报告已生成:", outPath)
}
