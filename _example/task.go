package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type FeedDownloadTaskBody struct {
	Id       string `json:"id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Key      int    `json:"key"`
	Spec     string `json:"spec"`
	Suffix   string `json:"suffix"`
}

func main() {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	baseURL := "http://localhost:2022"
	apiPath := "/api/task/create_batch"
	total := 150
	batchSize := 15
	interval := 3 * time.Second

	created := 0
	for created < total {
		remaining := total - created
		currentBatchSize := batchSize
		if remaining < batchSize {
			currentBatchSize = remaining
		}

		feeds := make([]FeedDownloadTaskBody, 0, currentBatchSize)
		for i := 0; i < currentBatchSize; i++ {
			id := created + i + 1
			feeds = append(feeds, FeedDownloadTaskBody{
				Id:       fmt.Sprintf("%d", id),
				URL:      "http://localhost:7001/download?size=1M",
				Title:    "test",
				Filename: "test",
				Key:      0,
				Spec:     "original",
				Suffix:   ".mp4",
			})
		}

		body := map[string]interface{}{
			"feeds": feeds,
		}
		data, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}

		req, err := http.NewRequest(http.MethodPost, baseURL+apiPath, bytes.NewReader(data))
		if err != nil {
			panic(err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}

		var respBody struct {
			Code int      `json:"code"`
			Msg  string   `json:"msg"`
			Ids  []string `json:"ids"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
			resp.Body.Close()
			panic(err)
		}
		resp.Body.Close()

		fmt.Printf("batch created %d tasks, server code=%d msg=%s ids=%v\n", len(respBody.Ids), respBody.Code, respBody.Msg, respBody.Ids)

		created += currentBatchSize
		if created >= total {
			break
		}
		time.Sleep(interval)
	}
}
