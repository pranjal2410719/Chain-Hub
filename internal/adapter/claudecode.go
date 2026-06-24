package adapter

import (
	"fmt"

	"github.com/khurafati/chainhub/internal/eventbus"
)

type ClaudeCodeAdapter struct {
	*BaseAdapter
}

func NewClaudeCodeAdapter() *ClaudeCodeAdapter {
	info := ToolInfo{
		Name:        "claude-code",
		DisplayName: "Claude Code",
		Specialties: []ToolCapability{CapCoding, CapDebugging, CapRefactoring, CapReview},
		Command:     "claude",
		Args:        []string{"--dangerously-skip-permissions"},
		Priority:    "high",
	}
	return &ClaudeCodeAdapter{
		BaseAdapter: NewBaseAdapter(info),
	}
}

func (c *ClaudeCodeAdapter) OnEvent(event eventbus.Event) {
	switch event.Type {
	case eventbus.EventTaskAssigned:
		assignedTool, _ := event.Payload["tool"].(string)
		c.mu.RLock()
		name := c.info.Name
		c.mu.RUnlock()
		if assignedTool != name {
			return
		}

		task, _ := event.Payload["task"].(string)
		phase, _ := event.Payload["phase"].(string)

		prompt := fmt.Sprintf("[%s phase] %s", phase, task)

		if err := c.Start(c.ctx); err != nil {
			return
		}

		if err := c.SendInput(prompt); err != nil {
			if c.bus != nil {
				c.bus.Publish(eventbus.NewEvent(
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
