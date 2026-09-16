package dm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DownloadTask is one download task record.
type DownloadTask struct {
	ID           int             `json:"id"`
	Name         string          `json:"name"`
	Status       int             `json:"status"`
	Error        string          `json:"error"`
	ErrorMessage string          `json:"error_message"`
	Files        json.RawMessage `json:"files"`
}

// DownloadCreateItem is one per-object result inside a create response.
type DownloadCreateItem struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// DownloadCreateResponse is the batch result of a create call.
type DownloadCreateResponse struct {
	Tasks []DownloadCreateItem `json:"tasks"`
	IDs   []int                `json:"ids"`
}

// CreateDownloadTaskBody is one object in the REST create body. Its JSON tags
// are the wire contract; MCP aliases this type rather than redeclaring it so
// the two surfaces cannot drift.
type CreateDownloadTaskBody struct {
	Platform        string          `json:"platform"`
	Content         json.RawMessage `json:"content"`
	BuildFromFetch  bool            `json:"build_from_fetch"`
	ResourceIndexes []int           `json:"resource_indexes,omitempty"`
	DownloadDir     string          `json:"download_dir"`
	Filename        string          `json:"filename"`
	Config          map[string]any  `json:"config"`
	AutoStart       *bool           `json:"auto_start"`
	ParentTaskID    *int            `json:"parent_task_id,omitempty"`
	RelationType    string          `json:"relation_type,omitempty"`
}

// CreateDownloadTaskRequest is the REST create body.
type CreateDownloadTaskRequest struct {
	Objects []CreateDownloadTaskBody `json:"objects"`
}

// TaskFailedError reports a task that reached a terminal failure state, or a
// wait that was cancelled. Status is the task status, not an envelope code.
type TaskFailedError struct {
	Status  int
	Message string
	Task    *DownloadTask
}

func (e *TaskFailedError) Error() string { return e.Message }

type task_list_response struct {
	List []DownloadTask `json:"list"`
}

// CreateDownloadTask submits one batch of download task objects.
func (c *Client) CreateDownloadTask(ctx context.Context, request CreateDownloadTaskRequest) (*DownloadCreateResponse, error) {
	raw_data, err := c.do(ctx, http.MethodPost, "/api/v1/download_task/create", nil, request)
	if err != nil {
		return nil, err
	}
	var response DownloadCreateResponse
	if err := json.Unmarshal(raw_data, &response); err != nil {
		return nil, fmt.Errorf("解析下载任务响应失败: %w", err)
	}
	if len(response.Tasks) == 0 {
		return nil, fmt.Errorf("下载任务响应缺少 tasks")
	}
	return &response, nil
}

// WaitDownloadTask polls one task until it reaches status 5 (completed). A
// terminal status 6/7 or a cancelled wait yields *TaskFailedError.
func (c *Client) WaitDownloadTask(ctx context.Context, task_id int) (*DownloadTask, error) {
	var poll_timer *time.Timer
	defer func() {
		if poll_timer != nil {
			poll_timer.Stop()
		}
	}()
	for {
		raw_data, err := c.get(ctx, "/api/v1/download_task/list", url.Values{"task_id": []string{strconv.Itoa(task_id)}})
		if err != nil {
			return nil, err
		}
		task, err := decode_download_task(raw_data, task_id)
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, fmt.Errorf("下载任务不存在: %d", task_id)
		}
		switch task.Status {
		case 5:
			return task, nil
		case 6, 7:
			message := value_or_default(task.ErrorMessage, task.Error)
			message = value_or_default(message, fmt.Sprintf("下载任务以状态 %d 结束", task.Status))
			return nil, &TaskFailedError{Status: task.Status, Message: message, Task: task}
		}

		if poll_timer == nil {
			poll_timer = time.NewTimer(c.PollInterval())
		} else {
			poll_timer.Reset(c.PollInterval())
		}
		select {
		case <-ctx.Done():
			return nil, &TaskFailedError{
				Status:  task.Status,
				Message: "等待下载完成超时或已取消: " + ctx.Err().Error(),
				Task:    task,
			}
		case <-poll_timer.C:
		}
	}
}

// DownloadTaskListQuery describes a read-only download task query.
type DownloadTaskListQuery struct {
	Page         int
	PageSize     int
	Statuses     []int
	ParentTaskID int
	RootTaskID   int
	ContentID    string
}

// DownloadTasks returns the raw paginated task list response.
func (c *Client) DownloadTasks(ctx context.Context, query DownloadTaskListQuery) (json.RawMessage, error) {
	values := url.Values{
		"page":      []string{strconv.Itoa(query.Page)},
		"page_size": []string{strconv.Itoa(query.PageSize)},
	}
	if query.ParentTaskID > 0 {
		values.Set("parent_task_id", strconv.Itoa(query.ParentTaskID))
	}
	if query.RootTaskID > 0 {
		values.Set("root_task_id", strconv.Itoa(query.RootTaskID))
	}
	if query.ContentID != "" {
		values.Set("content_id", query.ContentID)
	}
	if len(query.Statuses) > 0 {
		values.Set("status", join_ints(query.Statuses))
	}
	return c.get(ctx, "/api/v1/download_task/list", values)
}

// DownloadTaskDetail returns the raw detail response for one task.
func (c *Client) DownloadTaskDetail(ctx context.Context, task_id int) (json.RawMessage, error) {
	return c.get(ctx, "/api/v1/download_task/detail", url.Values{"id": []string{strconv.Itoa(task_id)}})
}

func decode_download_task(raw_data json.RawMessage, task_id int) (*DownloadTask, error) {
	var task DownloadTask
	if err := json.Unmarshal(raw_data, &task); err != nil {
		return nil, fmt.Errorf("解析下载进度响应失败: %w", err)
	}
	if task.ID > 0 {
		if task_id <= 0 || task.ID == task_id {
			return &task, nil
		}
		return nil, nil
	}

	var response task_list_response
	if err := json.Unmarshal(raw_data, &response); err != nil {
		return nil, fmt.Errorf("解析下载进度响应失败: %w", err)
	}
	for index := range response.List {
		if task_id <= 0 || response.List[index].ID == task_id {
			return &response.List[index], nil
		}
	}
	return nil, nil
}

func join_ints(values []int) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.Itoa(value)
	}
	return strings.Join(parts, ",")
}
