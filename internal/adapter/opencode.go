package adapter

import (
	"fmt"

	"github.com/khurafati/chainhub/internal/eventbus"
)

type OpenCodeAdapter struct {
	*BaseAdapter
}

func NewOpenCodeAdapter() *OpenCodeAdapter {
	info := ToolInfo{
		Name:        "opencode",
		DisplayName: "OpenCode",
		Specialties: []ToolCapability{CapCoding, CapTesting, CapDebugging, CapResearch},
		Command:     "opencode",
		Args:        []string{"run", "--format", "json"},
		Priority:    "medium",
	}
	return &OpenCodeAdapter{
		BaseAdapter: NewBaseAdapter(info),
	}
}

func (o *OpenCodeAdapter) OnEvent(event eventbus.Event) {
	switch event.Type {
	case eventbus.EventTaskAssigned:
		assignedTool, _ := event.Payload["tool"].(string)
		o.mu.RLock()
		name := o.info.Name
		o.mu.RUnlock()
		if assignedTool != name {
			return
		}

		task, _ := event.Payload["task"].(string)
		phase, _ := event.Payload["phase"].(string)

		prompt := fmt.Sprintf("[%s phase] %s", phase, task)

		if err := o.Start(o.ctx); err != nil {
			return
		}

		if err := o.SendInput(prompt); err != nil {
			if o.bus != nil {
				o.bus.Publish(eventbus.NewEvent(
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
