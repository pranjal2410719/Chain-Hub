package adapter

import (
	"fmt"

	"github.com/khurafati/chainhub/internal/eventbus"
)

type AntigravityAdapter struct {
	*BaseAdapter
}

func NewAntigravityAdapter() *AntigravityAdapter {
	info := ToolInfo{
		Name:        "antigravity",
		DisplayName: "Antigravity CLI",
		Specialties: []ToolCapability{CapCoding, CapPlanning, CapResearch, CapDebugging},
		Command:     "agy",
		Args:        []string{"--print"},
		Priority:    "high",
	}
	return &AntigravityAdapter{
		BaseAdapter: NewBaseAdapter(info),
	}
}

func (a *AntigravityAdapter) OnEvent(event eventbus.Event) {
	switch event.Type {
	case eventbus.EventTaskAssigned:
		assignedTool, _ := event.Payload["tool"].(string)
		a.mu.RLock()
		name := a.info.Name
		a.mu.RUnlock()
		if assignedTool != name {
			return
		}

		task, _ := event.Payload["task"].(string)
		phase, _ := event.Payload["phase"].(string)

		prompt := fmt.Sprintf("[%s phase] %s", phase, task)

		if err := a.Start(a.ctx); err != nil {
			return
		}

		if err := a.SendInput(prompt); err != nil {
			if a.bus != nil {
				a.bus.Publish(eventbus.NewEvent(
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
