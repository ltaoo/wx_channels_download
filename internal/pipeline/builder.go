package pipeline

type Builder struct {
	name    string
	nodes   map[string]*NodeDef
	order   []string
	onEvent EventHandler
}

func NewBuilder(name string) *Builder {
	return &Builder{
		name:  name,
		nodes: make(map[string]*NodeDef),
	}
}

func (b *Builder) Add(id string, node Node) *Builder {
	b.nodes[id] = &NodeDef{
		ID:   id,
		Type: node.Type(),
		Node: node,
	}
	b.order = append(b.order, id)
	return b
}

func (b *Builder) Config(id string, config map[string]any) *Builder {
	if def, ok := b.nodes[id]; ok {
		def.Config = config
	}
	return b
}

func (b *Builder) Chain(ids ...string) *Builder {
	for i := 0; i < len(ids)-1; i++ {
		b.Edge(ids[i], ids[i+1])
	}
	return b
}

func (b *Builder) Edge(src, dst string) *Builder {
	if def, ok := b.nodes[src]; ok {
		def.NextNodes = append(def.NextNodes, dst)
	}
	return b
}

func (b *Builder) OnEvent(fn EventHandler) *Builder {
	b.onEvent = fn
	return b
}

func (b *Builder) Build() *Pipeline {
	startID := ""
	if len(b.order) > 0 {
		startID = b.order[0]
	}
	return &Pipeline{
		Name:        b.name,
		StartNodeID: startID,
		Nodes:       b.nodes,
		OnEvent:     b.onEvent,
	}
}
