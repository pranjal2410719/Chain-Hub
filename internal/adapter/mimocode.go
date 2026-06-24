package adapter

import (
	"fmt"

	"github.com/khurafati/chainhub/internal/eventbus"
)

type MimoCodeAdapter struct {
	*BaseAdapter
}

func NewMimoCodeAdapter() *MimoCodeAdapter {
	info := ToolInfo{
		Name:        "mimo-code",
		DisplayName: "Mimo Code",
		Specialties: []ToolCapability{CapPlanning, CapCoding, CapReview},
		Command:     "mimo",
		Args:        []string{"run", "--format", "json"},
		Priority:    "medium",
	}
	return &MimoCodeAdapter{
		BaseAdapter: NewBaseAdapter(info),
	}
}

func (m *MimoCodeAdapter) OnEvent(event eventbus.Event) {
	switch event.Type {
	case eventbus.EventTaskAssigned:
		assignedTool, _ := event.Payload["tool"].(string)
		m.mu.RLock()
		name := m.info.Name
		m.mu.RUnlock()
		if assignedTool != name {
			return
		}

		task, _ := event.Payload["task"].(string)
		phase, _ := event.Payload["phase"].(string)

		prompt := fmt.Sprintf("[%s phase] %s", phase, task)

		if err := m.Start(m.ctx); err != nil {
			return
		}

		if err := m.SendInput(prompt); err != nil {
			if m.bus != nil {
				m.bus.Publish(eventbus.NewEvent(
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
