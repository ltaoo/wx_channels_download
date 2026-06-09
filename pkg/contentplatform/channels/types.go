package channels

// ProbeOutput is reserved for WeChat Channels-specific probe output. Channels
// currently stores probe metadata in the shared content summary and internal data.
type ProbeOutput struct{}

func (ProbeOutput) Map() map[string]any {
	return nil
}
