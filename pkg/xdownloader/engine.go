package xdownloader

import "context"

type Engine struct {
	selector Selector
	progress Hook
}

type EngineOption func(*Engine)

func WithProgressHook(hook Hook) EngineOption {
	return func(e *Engine) {
		e.progress = hook
	}
}

func New(selector Selector, opts ...EngineOption) *Engine {
	if selector == nil {
		selector = NewDefaultRegistry(nil)
	}
	e := &Engine{selector: selector}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Engine) Download(ctx context.Context, req Request) (*Result, error) {
	if req.Progress == nil {
		req.Progress = e.progress
	}
	downloader, err := e.selector.Select(req)
	if err != nil {
		return nil, err
	}
	return downloader.Download(ctx, req)
}
