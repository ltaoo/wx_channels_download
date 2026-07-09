package download

import "context"

type Router struct {
	handlers []Handler
}

func NewRouter(handlers ...Handler) *Router {
	return &Router{handlers: handlers}
}

func (r *Router) Register(handler Handler) {
	if handler == nil {
		return
	}
	r.handlers = append(r.handlers, handler)
}

func (r *Router) Match(rawURL string) Handler {
	for _, handler := range r.handlers {
		if handler != nil && handler.Match(rawURL) {
			return handler
		}
	}
	return nil
}

func (r *Router) Probe(ctx context.Context, input ProbeInput) (*Probe, error) {
	handler := r.Match(input.URL)
	if handler == nil {
		return nil, ErrUnsupportedURL
	}
	return handler.Probe(ctx, input)
}

func (r *Router) Resolve(ctx context.Context, input ResolveInput) (*ResolvedRequest, error) {
	handler := r.Match(input.URL)
	if handler == nil {
		return nil, ErrUnsupportedURL
	}
	return handler.Resolve(ctx, input)
}
