package pipeline

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestPipelineRunsChain(t *testing.T) {
	var order []string
	b := NewBuilder("download")
	b.Add("start", NewFuncNode("start", "start", func(ctx context.Context, pc *Context) error {
		order = append(order, "start")
		return nil
	}))
	b.Add("resolve", NewFuncNode("resolve", "resolve", func(ctx context.Context, pc *Context) error {
		order = append(order, "resolve")
		pc.Values["download_url"] = "https://example.com/video.mp4"
		return nil
	}))
	b.Add("done", NewFuncNode("done", "done", func(ctx context.Context, pc *Context) error {
		order = append(order, "done")
		return nil
	}))
	b.Chain("start", "resolve", "done")

	pc := NewContext()
	result, err := b.Build().Run(context.Background(), pc)
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if result.Duration == 0 {
		t.Fatal("expected duration to be set")
	}
	if !reflect.DeepEqual(order, []string{"start", "resolve", "done"}) {
		t.Fatalf("unexpected order: %#v", order)
	}
	if pc.Values["download_url"] != "https://example.com/video.mp4" {
		t.Fatalf("context value was not propagated")
	}
	if pc.GetNodeState("done") != StateCompleted {
		t.Fatalf("done state = %s", pc.GetNodeState("done"))
	}
}

func TestPipelineExclusiveGateway(t *testing.T) {
	var order []string
	gateway := NewGatewayNode("format", GatewayExclusive)
	gateway.Rules = []GatewayRule{
		{
			Condition: func(ctx context.Context, pc *Context) bool {
				return pc.Values["format"] == "mp3"
			},
			NextNodes: []string{"mp3"},
		},
	}
	gateway.DefaultNext = []string{"mp4"}

	b := NewBuilder("branch")
	b.Add("start", NewFuncNode("start", "start", func(ctx context.Context, pc *Context) error {
		order = append(order, "start")
		return nil
	}))
	b.Add("format", gateway)
	b.Add("mp3", NewFuncNode("mp3", "transcode_mp3", func(ctx context.Context, pc *Context) error {
		order = append(order, "mp3")
		return nil
	}))
	b.Add("mp4", NewFuncNode("mp4", "keep_mp4", func(ctx context.Context, pc *Context) error {
		order = append(order, "mp4")
		return nil
	}))
	b.Chain("start", "format")

	pc := NewContext()
	pc.Values["format"] = "mp3"
	if _, err := b.Build().Run(context.Background(), pc); err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if !reflect.DeepEqual(order, []string{"start", "mp3"}) {
		t.Fatalf("unexpected order: %#v", order)
	}
	if pc.GetNodeState("mp4") != "" {
		t.Fatalf("mp4 branch should not run")
	}
}

func TestRouterAndEvents(t *testing.T) {
	var events []EventKind
	videoPipeline := NewBuilder("video").
		Add("start", NewFuncNode("start", "start", nil)).
		OnEvent(func(evt Event) {
			events = append(events, evt.Kind)
		}).
		Build()
	defaultPipeline := NewBuilder("default").
		Add("start", NewFuncNode("start", "start", nil)).
		Build()

	router := NewRouter(defaultPipeline)
	router.AddRoute(&Route{
		Name: "video",
		Match: func(input string) bool {
			return strings.Contains(input, "video")
		},
		Pipeline: videoPipeline,
	})

	p := router.Resolve("https://example.com/video/1")
	if p.Name != "video" {
		t.Fatalf("resolved pipeline = %s", p.Name)
	}
	if _, err := p.Run(context.Background(), NewContext()); err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	expectedEvents := []EventKind{EventPipelineStart, EventNodeStart, EventNodeDone, EventPipelineDone}
	if !reflect.DeepEqual(events, expectedEvents) {
		t.Fatalf("events = %#v", events)
	}

	p = router.Resolve("https://example.com/article/1")
	if p.Name != "default" {
		t.Fatalf("resolved fallback pipeline = %s", p.Name)
	}
}
