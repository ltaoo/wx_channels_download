package dm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// OrganizeAccountFilesRequest moves downloaded videos into Kodi movie directories and updates their stored paths.
type OrganizeAccountFilesRequest struct {
	AccountID string `json:"account_id"`
	OutputDir string `json:"output_dir"`
	DryRun    *bool  `json:"dry_run,omitempty"`
}

func (r OrganizeAccountFilesRequest) Validate() error {
	if strings.TrimSpace(r.AccountID) == "" {
		return fmt.Errorf("account_id 不能为空，请使用 get_accounts 返回的账号 ID")
	}
	if strings.TrimSpace(r.OutputDir) == "" {
		return fmt.Errorf("output_dir 不能为空")
	}
	return nil
}

type OrganizeAccountFile struct {
	ResourceID int    `json:"resource_id"`
	Source     string `json:"source"`
	Video      string `json:"video"`
	NFO        string `json:"nfo"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

type OrganizeAccountFilesResult struct {
	AccountID string                `json:"account_id"`
	Directory string                `json:"directory"`
	DryRun    bool                  `json:"dry_run"`
	Files     []OrganizeAccountFile `json:"files"`
}

// OrganizeAccountFiles runs on the downloader host, including for remote clients.
func (c *Client) OrganizeAccountFiles(ctx context.Context, request OrganizeAccountFilesRequest) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/account/organize_files", nil, request)
}
