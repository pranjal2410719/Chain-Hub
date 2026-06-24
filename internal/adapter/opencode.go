package adapter

// OpenCodeAdapter wraps the OpenCode CLI (`opencode`) and integrates it with
// the ChainHub orchestrator.  OpenCode specialises in coding, testing, and
// debugging tasks.
type OpenCodeAdapter struct {
	*BaseAdapter
}

// NewOpenCodeAdapter creates an OpenCodeAdapter configured with sensible
// defaults for the OpenCode CLI.
func NewOpenCodeAdapter() *OpenCodeAdapter {
	info := ToolInfo{
		Name:        "opencode",
		DisplayName: "OpenCode",
		Specialties: []ToolCapability{CapCoding, CapTesting, CapDebugging},
		Command:     "opencode",
		Args:        []string{},
		Priority:    "medium",
	}
	return &OpenCodeAdapter{
		BaseAdapter: NewBaseAdapter(info),
	}
}
