package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"wx_channel/pkg/dm"
)

var (
	download_api_base_url     string
	download_existing_action  string
	download_force_refresh    bool
	download_timeout          time.Duration
	download_existing_actions = []string{"error", "skip", "overwrite", "duplicate"}
)

var download_cmd = &cobra.Command{
	Use:   "download <url>",
	Short: "解析内容链接并创建下载任务",
	Long: "\n通过已启动的下载器服务解析内容 URL，解析成功后创建并启动下载任务；解析失败时给出错误原因。" +
		"\n需要主服务已启动（默认读取 api.protocol、api.hostname 和 api.port 配置）。",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		raw_url := strings.TrimSpace(args[0])
		if raw_url == "" {
			return fmt.Errorf("url 不能为空")
		}
		existing_action := strings.TrimSpace(download_existing_action)
		if existing_action == "" {
			existing_action = "error"
		}
		if !download_existing_action_allowed(existing_action) {
			return fmt.Errorf("existing-action 必须是 %s", strings.Join(download_existing_actions, "、"))
		}
		api_base_url := strings.TrimSpace(download_api_base_url)
		if api_base_url == "" {
			api_base_url = configured_api_base_url()
		}
		client, err := dm.NewClient(dm.ClientOptions{BaseURL: api_base_url})
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), download_timeout)
		defer cancel()

		if err := run_download(ctx, cmd.OutOrStdout(), client, raw_url, existing_action, download_force_refresh); err != nil {
			fmt.Fprintf(root_cmd.ErrOrStderr(), "%s %v\n", error_prefix, err)
			os.Exit(1)
		}
		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	download_cmd.Flags().StringVar(
		&download_api_base_url,
		"api-base-url",
		"",
		"下载器 API 地址，默认读取 api.protocol、api.hostname 和 api.port 配置",
	)
	download_cmd.Flags().StringVar(
		&download_existing_action,
		"existing-action",
		"error",
		"解析结果已有下载任务时的处理方式：error、skip、overwrite、duplicate",
	)
	download_cmd.Flags().BoolVar(
		&download_force_refresh,
		"force-refresh",
		false,
		"忽略抓取缓存，强制重新请求平台",
	)
	download_cmd.Flags().DurationVar(
		&download_timeout,
		"timeout",
		10*time.Minute,
		"解析与创建任务的总超时时间",
	)
	Register(download_cmd)
}

func download_existing_action_allowed(action string) bool {
	for _, allowed := range download_existing_actions {
		if allowed == action {
			return true
		}
	}
	return false
}

// run_download resolves the URL, creates the download task, and reports progress
// on out. It returns errors without exiting so tests can drive the whole flow.
func run_download(ctx context.Context, out io.Writer, client *dm.Client, raw_url string, existing_action string, force_refresh bool) error {
	fmt.Fprintf(out, "正在解析: %s\n", raw_url)
	job, err := client.CreateScraperJob(ctx, raw_url, force_refresh)
	if err != nil {
		return fmt.Errorf("解析失败: %w", err)
	}
	output, err := client.WaitScraperJob(ctx, job)
	if err != nil {
		return fmt.Errorf("解析失败: %w", err)
	}

	title := download_output_title(output)
	fmt.Fprintf(out, "解析成功: [%s] %s\n", output.Platform, title)

	task_id, task_name, err := download_create_task(ctx, client, output, raw_url, existing_action)
	if err != nil {
		return fmt.Errorf("创建下载任务失败: %w", err)
	}
	if task_id > 0 {
		fmt.Fprintf(out, "下载任务创建成功: #%d %s\n", task_id, task_name)
	} else {
		fmt.Fprintln(out, "已存在相同下载任务，按 existing-action 处理")
	}
	return nil
}

func download_create_task(ctx context.Context, client *dm.Client, output *dm.ScraperOutput, raw_url string, existing_action string) (int, string, error) {
	auto_start := true
	response, err := client.CreateDownloadTask(ctx, dm.CreateDownloadTaskRequest{
		Objects: []dm.CreateDownloadTaskBody{{
			Platform:       output.Platform,
			Content:        output.Result,
			BuildFromFetch: len(bytes.TrimSpace(output.DownloadInfo)) > 0,
			Config:         download_task_config(output.Platform, existing_action),
			AutoStart:      &auto_start,
		}},
	})
	if err != nil {
		return 0, "", err
	}
	item := response.Tasks[0]
	if item.Code != 0 {
		var duplicate struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(item.Data, &duplicate)
		message := strings.TrimSpace(item.Msg)
		if message == "" {
			message = fmt.Sprintf("下载任务创建失败，错误码 %d", item.Code)
		}
		if duplicate.ID > 0 || duplicate.Name != "" {
			message += fmt.Sprintf("（已有任务 #%d %s）", duplicate.ID, duplicate.Name)
		}
		return 0, "", fmt.Errorf("%s", message)
	}
	var task struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	_ = json.Unmarshal(item.Data, &task)
	task_id := task.ID
	if task_id == 0 && len(response.IDs) > 0 {
		task_id = response.IDs[0]
	}
	if task_id == 0 {
		return 0, "", fmt.Errorf("下载任务响应缺少任务 id")
	}
	name := strings.TrimSpace(task.Name)
	if name == "" {
		name = download_output_title(output)
	}
	if name == "" {
		name = raw_url
	}
	return task_id, name, nil
}

// download_task_config maps the --existing-action flag onto the REST config
// object consumed by the downloader.
func download_task_config(platform string, existing_action string) map[string]any {
	config := map[string]any{
		"platform":        platform,
		"existing_action": existing_action,
	}
	if existing_action == "overwrite" {
		config["overwrite"] = true
	}
	if existing_action == "duplicate" {
		config["duplicate"] = true
	}
	return config
}

func download_output_title(output *dm.ScraperOutput) string {
	var content struct {
		Title string `json:"title"`
		ID    string `json:"id"`
	}
	if json.Unmarshal(output.Content, &content) == nil {
		if title := strings.TrimSpace(content.Title); title != "" {
			return title
		}
		if id := strings.TrimSpace(content.ID); id != "" {
			return id
		}
	}
	return strings.TrimSpace(output.URL)
}
