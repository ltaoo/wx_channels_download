package hub

import "encoding/json"

type server_message struct {
	Type              string `json:"type"`
	Task              *Task  `json:"task,omitempty"`
	TaskID            string `json:"task_id,omitempty"`
	LeaseToken        string `json:"lease_token,omitempty"`
	LeaseMilliseconds int64  `json:"lease_milliseconds,omitempty"`
	Error             string `json:"error,omitempty"`
}

type client_message struct {
	Type       string          `json:"type"`
	TaskID     string          `json:"task_id,omitempty"`
	LeaseToken string          `json:"lease_token,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
	Error      string          `json:"error,omitempty"`
	Retryable  bool            `json:"retryable,omitempty"`
}

type task_response struct {
	Task Task `json:"task"`
}

type task_list_response struct {
	Tasks []Task `json:"tasks"`
}

type error_response struct {
	Error string `json:"error"`
}
