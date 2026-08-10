package hermes

// EventType identifies an event emitted by Hermes.
//
// Event handlers must use the event type to determine the concrete EventData
// value. Keeping the event target out of the handler signature allows Hermes to
// emit task, resource, engine, and other event categories through one API.
type EventType string

const (
	// EventCreated carries TaskCreatedEventData.
	EventCreated EventType = "created"
	// EventPreparing carries TaskPreparingEventData.
	EventPreparing EventType = "preparing"
	// EventStarted carries TaskStartedEventData.
	EventStarted EventType = "started"
	// EventProgress carries TaskProgressEventData.
	EventProgress EventType = "progress"
	// EventPaused carries TaskPausedEventData.
	EventPaused EventType = "paused"
	// EventFinished carries TaskFinishedEventData.
	EventFinished EventType = "finished"
	// EventFailed carries TaskFailedEventData.
	EventFailed EventType = "failed"
	// EventDeleted carries TaskDeletedEventData.
	EventDeleted EventType = "deleted"
)

// EventData is the event-specific payload passed to an EventHandler.
// The concrete type for every built-in event is documented beside its
// EventType constant above.
type EventData any

// EventHandler receives every event emitted by Hermes. Data has the concrete
// struct associated with event; handlers should use a type switch or type
// assertion and ignore event types they do not consume.
type EventHandler func(event EventType, data EventData)

// TaskCreatedEventData is emitted after a task enters the preparing state.
type TaskCreatedEventData struct {
	TaskID int `json:"task_id"`
}

// TaskPreparingEventData is emitted when an existing task is submitted for
// start, resume, or retry and enters the preparing state.
type TaskPreparingEventData struct {
	TaskID int `json:"task_id"`
}

// TaskStartedEventData is emitted after a task enters the downloading state.
type TaskStartedEventData struct {
	TaskID int `json:"task_id"`
}

// TaskProgressEventData carries the latest in-memory progress snapshot.
type TaskProgressEventData struct {
	TaskID   int           `json:"task_id"`
	Progress *TaskProgress `json:"progress"`
}

// TaskPausedEventData is emitted after a task has been paused.
type TaskPausedEventData struct {
	TaskID int `json:"task_id"`
}

// TaskFinishedEventData is emitted after a task and its resource outputs have
// been finalized successfully.
type TaskFinishedEventData struct {
	TaskID    int      `json:"task_id"`
	FilePaths []string `json:"file_paths"`
}

// TaskFailedEventData is emitted after a task has entered the failed state.
type TaskFailedEventData struct {
	TaskID int    `json:"task_id"`
	Error  string `json:"error"`
}

// TaskDeletedEventData is emitted after a running task has been cancelled for
// deletion. Persistent soft deletion is owned by the caller of Hermes.
type TaskDeletedEventData struct {
	TaskID int `json:"task_id"`
}

// TaskProgress carries the current aggregate download progress, computed
// entirely from in-memory state without database queries.
type TaskProgress struct {
	TotalSize     int64              `json:"total_size"`
	Downloaded    int64              `json:"downloaded"`
	Speed         int64              `json:"speed"`
	ResourceCount int                `json:"resource_count"`
	Resources     []ResourceProgress `json:"resources"`
	Keepalive     bool               `json:"-"` // true when emitted as keepalive (no real progress change)
}

// ResourceProgress carries a single resource's download progress.
type ResourceProgress struct {
	ID         int    `json:"id"`
	Name       string `json:"name,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Type       string `json:"resource_type,omitempty"`
	Size       int64  `json:"size"`
	Downloaded int64  `json:"downloaded"`
	Speed      int64  `json:"speed"`
}

// event_record stores replayable events for the high-level in-memory API.
type event_record struct {
	event EventType
	data  EventData
}
