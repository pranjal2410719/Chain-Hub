package adapter

import (
	"fmt"

	"github.com/khurafati/chainhub/internal/eventbus"
)

type FreeBufAdapter struct {
	*BaseAdapter
}

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

func (f *FreeBufAdapter) OnEvent(event eventbus.Event) {
	switch event.Type {
	case eventbus.EventTaskAssigned:
		assignedTool, _ := event.Payload["tool"].(string)
		f.mu.RLock()
		name := f.info.Name
		f.mu.RUnlock()
		if assignedTool != name {
			return
		}

		task, _ := event.Payload["task"].(string)
		phase, _ := event.Payload["phase"].(string)

		prompt := fmt.Sprintf("[%s phase] %s", phase, task)

		if err := f.Start(f.ctx); err != nil {
			return
		}

		if err := f.SendInput(prompt); err != nil {
			if f.bus != nil {
				f.bus.Publish(eventbus.NewEvent(
					eventbus.EventToolError,
					name,
					map[string]interface{}{
						"tool":  name,
						"error": fmt.Sprintf("failed to send task: %v", err),
					},
				))
			}
		}
	}
}
