package wxmpadapter

import (
	"context"
	"testing"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/hermes"
)

func TestPostprocessSkipsResourceOnlyArticleTask(t *testing.T) {
	task := &hermes.TaskJob{
		ID:       75,
		Metadata: map[string]any{"biz_type": 1},
		Resources: []hermes.ResourceJob{{
			Name:     "video_01",
			Kind:     "video/mp4",
			FilePath: "/tmp/video_01.mp4",
		}},
	}
	deps := adapter.PostprocessDeps{Logger: zerolog.Nop()}

	if err := (&OfficialAccountAdapter{}).Postprocess(context.Background(), task, deps); err != nil {
		t.Fatalf("Postprocess() error = %v, want nil", err)
	}
}
