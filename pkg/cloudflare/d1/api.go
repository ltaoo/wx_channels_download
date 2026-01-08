package d1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// D1Result represents the response from D1 query API
type D1Response struct {
	Success  bool       `json:"success"`
	Errors   []ApiError `json:"errors"`
	Messages []ApiError `json:"messages"`
	Result   []D1Result `json:"result"`
}

type ApiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type D1Result struct {
	Success bool             `json:"success"`
	Meta    Meta             `json:"meta"`
	Results []map[string]any `json:"results"`
}

type Meta struct {
	ChangedDB   bool    `json:"changed_db"`
	Changes     int     `json:"changes"`
	Duration    float64 `json:"duration"`
	LastRowID   int     `json:"last_row_id"`
	RowsRead    int     `json:"rows_read"`
	RowsWritten int     `json:"rows_written"`
	SizeAfter   int     `json:"size_after"`
}

// QueryRequest represents the request body for D1 query
type QueryRequest struct {
	SQL    string `json:"sql"`
	Params []any  `json:"params,omitempty"`
}

// Client handles D1 API interactions
type Client struct {
	AccountID string
	AuthToken string
	Client    *http.Client
}

func NewClient(accountID, authToken string) *Client {
	return &Client{
		AccountID: accountID,
		AuthToken: authToken,
		Client:    &http.Client{},
	}
}

// Query executes a SQL query on D1
func (c *Client) Query(databaseID string, sql string, params []any) (*D1Response, error) {
	reqBody := QueryRequest{
		SQL:    sql,
		Params: params,
	}
	if reqBody.Params == nil {
		reqBody.Params = []any{}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query", c.AccountID, databaseID)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse the error body even if status is not 200
		var errorResp D1Response
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil && len(errorResp.Errors) > 0 {
			var errMsgs string
			for _, e := range errorResp.Errors {
				errMsgs += fmt.Sprintf("[%d] %s; ", e.Code, e.Message)
				if strings.Contains(e.Message, "SQLITE_AUTH") {
					errMsgs += fmt.Sprintf(" (Hint: Check if Token has 'D1:Edit' permission, and AccountID '%s' matches DatabaseID '%s'. Also ensure you are using a Token, not Global Key)", c.AccountID, databaseID)
				}
			}
			return nil, fmt.Errorf("D1 API error (Status %d): %s", resp.StatusCode, errMsgs)
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var d1Resp D1Response
	if err := json.Unmarshal(body, &d1Resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v, body: %s", err, string(body))
	}

	if !d1Resp.Success {
		var errMsgs string
		for _, e := range d1Resp.Errors {
			errMsgs += fmt.Sprintf("[%d] %s; ", e.Code, e.Message)
			if strings.Contains(e.Message, "SQLITE_AUTH") {
				errMsgs += fmt.Sprintf(" (Hint: Check if Token has 'D1:Edit' permission, and AccountID '%s' matches DatabaseID '%s')", c.AccountID, databaseID)
			}
		}
		return nil, fmt.Errorf("D1 API error (Account: %s, DB: %s): %s", c.AccountID, databaseID, errMsgs)
	}

	return &d1Resp, nil
}
