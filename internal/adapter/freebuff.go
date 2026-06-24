package adapter

// FreeBufAdapter wraps the FreeBuff CLI (`freebuf`) and integrates it with the
// ChainHub orchestrator.  FreeBuff specialises in security scanning, research,
// and web browsing tasks.
type FreeBufAdapter struct {
	*BaseAdapter
}

// NewFreeBufAdapter creates a FreeBufAdapter configured with sensible defaults
// for the FreeBuff CLI.
func NewFreeBufAdapter() *FreeBufAdapter {
	info := ToolInfo{
		Name:        "freebuff",
		DisplayName: "FreeBuff",
		Specialties: []ToolCapability{CapScanning, CapResearch, CapBrowsing},
		Command:     "freebuff",
		Args:        []string{},
		Priority:    "medium",
	}
	return &FreeBufAdapter{
		BaseAdapter: NewBaseAdapter(info),
	}
}
