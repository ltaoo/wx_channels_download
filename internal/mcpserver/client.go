package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"wx_channel/pkg/dm"
)

const default_poll_interval = dm.DefaultPollInterval

// Type aliases keep this package's existing names while the wire types live in
// the shared pkg/dm SDK. DownloadTaskCreateRequest aliases the REST create body
// on purpose: the MCP tool contract and the REST contract must not drift, so a
// JSON tag change in one place is a compile-time change in the other.
type (
	ScraperJob                = dm.ScraperJob
	scraper_output            = dm.ScraperOutput
	download_task             = dm.DownloadTask
	download_create_response  = dm.DownloadCreateResponse
	download_create_item      = dm.DownloadCreateItem
	DownloadTaskListQuery     = dm.DownloadTaskListQuery
	AccountListQuery          = dm.AccountListQuery
	BrowseHistoryListQuery    = dm.BrowseHistoryListQuery
	LogListQuery              = dm.LogListQuery
	DownloadTaskCreateRequest = dm.CreateDownloadTaskBody
)

// api_client is the MCP-facing shim over the pkg/dm SDK. It deliberately does
// not embed *dm.Client: callers read poll_interval directly, and embedding
// would leak unmapped dm methods that new call sites could reach without the
// tool-error mapping below.
type api_client struct {
	client        *dm.Client
	poll_interval time.Duration
}

func new_api_client(raw_base_url string, http_client *http.Client, poll_interval time.Duration) (*api_client, error) {
	client, err := dm.NewClient(dm.ClientOptions{
		BaseURL:      raw_base_url,
		HTTPClient:   http_client,
		PollInterval: poll_interval,
	})
	if err != nil {
		return nil, err
	}
	return &api_client{client: client, poll_interval: client.PollInterval()}, nil
}

// map_api_error converts SDK errors into MCP tool errors here, inside the shim,
// so callers that wrap the result with %w still surface structured details.
func map_api_error(err error) error {
	if err == nil {
		return nil
	}
	var api_error *dm.APIError
	if errors.As(err, &api_error) {
		return new_tool_execution_error(api_error.Msg, raw_json_value(api_error.Data))
	}
	var task_error *dm.TaskFailedError
	if errors.As(err, &task_error) {
		return new_tool_execution_error(task_error.Message, task_error.Task)
	}
	return err
}

func (c *api_client) get_platform_status(ctx context.Context) (json.RawMessage, error) {
	raw_status, err := c.client.PlatformStatus(ctx)
	return raw_status, map_api_error(err)
}

func (c *api_client) get_config(ctx context.Context) (json.RawMessage, error) {
	raw_config, err := c.client.Config(ctx)
	return raw_config, map_api_error(err)
}

func (c *api_client) update_config(ctx context.Context, values map[string]any) (json.RawMessage, error) {
	raw_result, err := c.client.UpdateConfig(ctx, values)
	return raw_result, map_api_error(err)
}

func (c *api_client) get_restart_status(ctx context.Context, restart_token string) (json.RawMessage, error) {
	raw_result, err := c.client.RestartStatus(ctx, restart_token)
	return raw_result, map_api_error(err)
}

func (c *api_client) create_scraper_job(ctx context.Context, raw_url string, force_refresh bool) (*ScraperJob, error) {
	job, err := c.client.CreateScraperJob(ctx, raw_url, force_refresh)
	return job, map_api_error(err)
}

func (c *api_client) get_scraper_job(ctx context.Context, job_id string) (*ScraperJob, error) {
	job, err := c.client.GetScraperJob(ctx, job_id)
	return job, map_api_error(err)
}

func (c *api_client) create_download_task(ctx context.Context, request DownloadTaskCreateRequest) (*download_create_response, error) {
	response, err := c.client.CreateDownloadTask(ctx, dm.CreateDownloadTaskRequest{
		Objects: []dm.CreateDownloadTaskBody{request},
	})
	return response, map_api_error(err)
}

func (c *api_client) wait_download_task(ctx context.Context, task_id int) (*download_task, error) {
	task, err := c.client.WaitDownloadTask(ctx, task_id)
	return task, map_api_error(err)
}

func (c *api_client) download_tasks(ctx context.Context, query DownloadTaskListQuery) (json.RawMessage, error) {
	raw_result, err := c.client.DownloadTasks(ctx, query)
	return raw_result, map_api_error(err)
}

func (c *api_client) download_task_detail(ctx context.Context, task_id int) (json.RawMessage, error) {
	raw_result, err := c.client.DownloadTaskDetail(ctx, task_id)
	return raw_result, map_api_error(err)
}

func (c *api_client) accounts(ctx context.Context, query AccountListQuery) (json.RawMessage, error) {
	raw_result, err := c.client.Accounts(ctx, query)
	return raw_result, map_api_error(err)
}

func (c *api_client) browse_history(ctx context.Context, query BrowseHistoryListQuery) (json.RawMessage, error) {
	raw_result, err := c.client.BrowseHistory(ctx, query)
	return raw_result, map_api_error(err)
}

func (c *api_client) logs(ctx context.Context, query LogListQuery) (json.RawMessage, error) {
	raw_result, err := c.client.Logs(ctx, query)
	return raw_result, map_api_error(err)
}

func (c *api_client) certificate_status(ctx context.Context) (json.RawMessage, error) {
	raw_result, err := c.client.CertificateStatus(ctx)
	return raw_result, map_api_error(err)
}

func (c *api_client) search_contact(ctx context.Context, keyword string, next_marker string) (json.RawMessage, error) {
	raw_result, err := c.client.SearchContact(ctx, keyword, next_marker)
	return raw_result, map_api_error(err)
}

func (c *api_client) contact_feed_list(ctx context.Context, username string, next_marker string) (json.RawMessage, error) {
	raw_result, err := c.client.ContactFeedList(ctx, username, next_marker)
	return raw_result, map_api_error(err)
}

func (c *api_client) live_replay_list(ctx context.Context, username string, next_marker string) (json.RawMessage, error) {
	raw_result, err := c.client.LiveReplayList(ctx, username, next_marker)
	return raw_result, map_api_error(err)
}

func (c *api_client) feed_profile(ctx context.Context, oid string, nid string, request_url string, eid string) (json.RawMessage, error) {
	raw_result, err := c.client.FeedProfile(ctx, oid, nid, request_url, eid)
	return raw_result, map_api_error(err)
}

func (c *api_client) feed_comment_list(ctx context.Context, oid string, nid string, comment_id string, next_marker string) (json.RawMessage, error) {
	raw_result, err := c.client.FeedCommentList(ctx, oid, nid, comment_id, next_marker)
	return raw_result, map_api_error(err)
}

func (c *api_client) feed_share_url(ctx context.Context, oid string) (json.RawMessage, error) {
	raw_result, err := c.client.FeedShareURL(ctx, oid)
	return raw_result, map_api_error(err)
}

func (c *api_client) channel_status(ctx context.Context) (json.RawMessage, error) {
	raw_result, err := c.client.ChannelStatus(ctx)
	return raw_result, map_api_error(err)
}

func (c *api_client) live_profile(ctx context.Context, username string, oid string, nid string, live_id string) (json.RawMessage, error) {
	raw_result, err := c.client.LiveProfile(ctx, username, oid, nid, live_id)
	return raw_result, map_api_error(err)
}

func (c *api_client) interacted_feed_list(ctx context.Context, flag int, next_marker string) (json.RawMessage, error) {
	raw_result, err := c.client.InteractedFeedList(ctx, flag, next_marker)
	return raw_result, map_api_error(err)
}

func (c *api_client) followed_accounts(ctx context.Context, next_marker string) (json.RawMessage, error) {
	raw_result, err := c.client.FollowedAccounts(ctx, next_marker)
	return raw_result, map_api_error(err)
}

func (c *api_client) play_history(ctx context.Context, next_marker string) (json.RawMessage, error) {
	raw_result, err := c.client.PlayHistory(ctx, next_marker)
	return raw_result, map_api_error(err)
}

func (c *api_client) decrypt_wxchannels_video(ctx context.Context, file_path string, key uint64) (json.RawMessage, error) {
	raw_result, err := c.client.DecryptVideo(ctx, file_path, key)
	return raw_result, map_api_error(err)
}

func has_json_value(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

func raw_json_value(raw json.RawMessage) any {
	if !has_json_value(raw) {
		return nil
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return string(raw)
	}
	return value
}

func value_or_default(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
