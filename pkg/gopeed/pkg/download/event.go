package download

import "encoding/json"

type EventKey string

const (
	EventKeyStart    = "start"
	EventKeyPause    = "pause"
	EventKeyProgress = "progress"
	EventKeyError    = "error"
	EventKeyDelete   = "delete"
	EventKeyDone     = "done"
	EventKeyFinally  = "finally"
)

type Event struct {
	Key  EventKey
	Task *Task
	Err  error
}

func (e *Event) MarshalJSON() ([]byte, error) {
	errText := ""
	if e != nil && e.Err != nil {
		errText = e.Err.Error()
	}
	type eventJSON struct {
		Key   EventKey `json:"Key"`
		Task  *Task    `json:"Task"`
		Err   string   `json:"Err,omitempty"`
		Error string   `json:"error,omitempty"`
	}
	return json.Marshal(eventJSON{
		Key:   e.Key,
		Task:  e.Task,
		Err:   errText,
		Error: errText,
	})
}
