package douyin

// ProbeOutput is reserved for Douyin-specific probe output. Douyin currently
// stores probe metadata in the shared content summary and internal data.
type ProbeOutput struct{}

func (ProbeOutput) Map() map[string]any {
	return nil
}
