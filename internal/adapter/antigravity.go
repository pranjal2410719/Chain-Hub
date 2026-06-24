package adapter

// AntigravityAdapter wraps the Antigravity CLI (`agy`) and integrates it with
// the ChainHub orchestrator.  Antigravity excels at planning, coding, research,
// and debugging tasks.
type AntigravityAdapter struct {
	*BaseAdapter
}

// NewAntigravityAdapter creates an AntigravityAdapter configured with sensible
// defaults for the Antigravity CLI.
func NewAntigravityAdapter() *AntigravityAdapter {
	info := ToolInfo{
		Name:        "antigravity",
		DisplayName: "Antigravity CLI",
		Specialties: []ToolCapability{CapCoding, CapPlanning, CapResearch, CapDebugging},
		Command:     "agy",
		Args:        []string{"auto"},
		Priority:    "high",
	}
	return &AntigravityAdapter{
		BaseAdapter: NewBaseAdapter(info),
	}
}
