package api

import (
	"testing"

	"github.com/rs/zerolog"

	"wx_channel/internal/services"
	"wx_channel/pkg/hermes"
)

func newTestAPIClientForTaskBroadcaster() *APIClient {
	logger := zerolog.Nop()
	return &APIClient{
		logger:                &logger,
		download_task_service: services.NewDownloadTaskService(nil, &logger, nil, nil, "", ""),
	}
}

func TestTaskBroadcasterQueuesTerminalWhileActive(t *testing.T) {
	c := newTestAPIClientForTaskBroadcaster()
	b := newTaskBroadcaster()
	const taskID = 7

	b.mu.Lock()
	b.active[taskID] = true
	b.mu.Unlock()

	b.notify(c, taskID, hermes.EventFinished, nil)

	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.active[taskID] {
		t.Fatal("expected task broadcast to remain active")
	}
	pending, ok := b.pending[taskID]
	if !ok {
		t.Fatal("expected finished event to be queued while task broadcast is active")
	}
	if pending.event != hermes.EventFinished {
		t.Fatalf("expected pending event %q, got %q", hermes.EventFinished, pending.event)
	}
}

func TestTaskBroadcasterDoesNotQueueProgressWhileActive(t *testing.T) {
	c := newTestAPIClientForTaskBroadcaster()
	b := newTaskBroadcaster()
	const taskID = 8

	b.mu.Lock()
	b.active[taskID] = true
	b.mu.Unlock()

	b.notify(c, taskID, hermes.EventProgress, &hermes.TaskProgress{Downloaded: 1})

	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.pending[taskID]; ok {
		t.Fatal("did not expect progress event to be queued while task broadcast is active")
	}
}

func TestTaskBroadcasterDrainsPendingTerminalAfterProgress(t *testing.T) {
	c := newTestAPIClientForTaskBroadcaster()
	b := newTaskBroadcaster()
	const taskID = 9

	b.mu.Lock()
	b.active[taskID] = true
	b.pending[taskID] = taskBroadcastRequest{event: hermes.EventFinished}
	b.mu.Unlock()

	b.runTaskBroadcast(c, taskID, taskBroadcastRequest{
		event: hermes.EventProgress,
		progress: &hermes.TaskProgress{
			TotalSize:  1,
			Downloaded: 1,
		},
	})

	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.active[taskID]; ok {
		t.Fatal("expected active marker to be cleared after pending terminal event drains")
	}
	if _, ok := b.pending[taskID]; ok {
		t.Fatal("expected pending terminal event to be cleared after drain")
	}
}
