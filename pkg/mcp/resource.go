package mcp

// Resource is server-owned text exposed through resources/list and
// resources/read. Text never appears in the resource catalog.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
	Text        string `json:"-"`
}

func (s *Server) AddResource(resource Resource) {
	if s == nil || resource.URI == "" {
		return
	}
	s.tools_mu.Lock()
	defer s.tools_mu.Unlock()
	for index := range s.resources {
		if s.resources[index].URI == resource.URI {
			s.resources[index] = resource
			return
		}
	}
	s.resources = append(s.resources, resource)
}

func (s *Server) Resources() []Resource {
	if s == nil {
		return []Resource{}
	}
	s.tools_mu.RLock()
	defer s.tools_mu.RUnlock()
	return append([]Resource{}, s.resources...)
}

func (s *Server) capabilities() map[string]any {
	capabilities := map[string]any{"tools": map[string]any{}}
	if len(s.Resources()) > 0 {
		capabilities["resources"] = map[string]any{}
	}
	return capabilities
}
