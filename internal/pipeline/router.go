package pipeline

type Route struct {
	Name     string
	Match    func(input string) bool
	Pipeline *Pipeline
}

type Router struct {
	routes          []*Route
	defaultPipeline *Pipeline
}

func NewRouter(defaultPipeline *Pipeline) *Router {
	return &Router{defaultPipeline: defaultPipeline}
}

func (r *Router) AddRoute(route *Route) {
	r.routes = append(r.routes, route)
}

func (r *Router) Resolve(input string) *Pipeline {
	for _, route := range r.routes {
		if route.Match != nil && route.Match(input) {
			return route.Pipeline
		}
	}
	return r.defaultPipeline
}
