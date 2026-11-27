package mcpserver

import (
	"context"
	"encoding/json"
	"strings"

	"wx_channel/pkg/dm"
)

type FileOrganizer interface {
	OrganizeAccountFiles(ctx context.Context, request dm.OrganizeAccountFilesRequest) (*dm.OrganizeAccountFilesResult, error)
}

func (s *ToolSet) organize_account_files(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var request dm.OrganizeAccountFilesRequest
	if err := decode_tool_arguments(raw, &request); err != nil {
		return nil, err
	}
	request.AccountID = strings.TrimSpace(request.AccountID)
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if s.file_organizer != nil {
		result, err := s.file_organizer.OrganizeAccountFiles(ctx, request)
		if err != nil {
			if result != nil {
				return nil, new_tool_execution_error(err.Error(), result)
			}
			return nil, err
		}
		return successful_tool_result(result)
	}
	result, err := s.api_client.client.OrganizeAccountFiles(ctx, request)
	if err != nil {
		return nil, map_api_error(err)
	}
	return successful_tool_result(raw_json_value(result))
}

var tools_file_organizer = []tool{{
	name: "organize_account_files", title: "按账号整理 Kodi 媒体库",
	description:  "把账号已完成的本地视频移动并重命名到 output_dir 下账号目录，每个视频独立成电影目录并生成同名 movie 格式 NFO（标题、简介、发布日期、年份、账号、封面 URL、唯一 ID），无季集编号。同步更新 download_resource 的 download_dir、name，后续播放与文件管理使用新路径。文件名附资源 ID 区分同名视频；相同目标可重试，冲突不覆盖。数据库提交后移除原路径；提交前失败保留原视频并清理本次新建文件。共享源路径和符号链接文件不移动。默认 dry_run=true 仅预览；用户明确要移动时设为 false。先用 get_accounts 获取完整 account_id。路径属于下载器主机。检查 files[].status/error，不能把部分失败视为全部成功。在 Kodi 中把 output_dir 设为电影来源，选择 Local information only 并启用递归扫描。",
	input_schema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["account_id","output_dir"],"properties":{"account_id":{"type":"string","minLength":1,"description":"get_accounts 返回的完整账号 ID"},"output_dir":{"type":"string","minLength":1,"description":"下载器主机上的媒体库根目录绝对路径"},"dry_run":{"type":"boolean","default":true,"description":"默认只预览，false 执行移动、重命名、NFO 写入及数据库路径更新"}}}`),
	annotations:  json.RawMessage(`{"readOnlyHint":false,"destructiveHint":true,"idempotentHint":true,"openWorldHint":false}`),
	supports:     func(s *ToolSet) bool { return s.file_organizer != nil || s.api_client != nil },
	handle:       (*ToolSet).organize_account_files,
}}
