package adapter

// MimoCodeAdapter wraps the Mimo Code CLI (`mimo`) and integrates it with the
// ChainHub orchestrator.  Mimo specialises in planning, coding, and code review.
type MimoCodeAdapter struct {
	*BaseAdapter
}

// NewMimoCodeAdapter creates a MimoCodeAdapter configured with sensible
// defaults for the Mimo Code CLI.
func NewMimoCodeAdapter() *MimoCodeAdapter {
	info := ToolInfo{
		Name:        "mimo-code",
		DisplayName: "Mimo Code",
		Specialties: []ToolCapability{CapPlanning, CapCoding, CapReview},
		Command:     "mimo",
		Args:        []string{},
		Priority:    "medium",
	}
	return &MimoCodeAdapter{
		BaseAdapter: NewBaseAdapter(info),
	}
}
