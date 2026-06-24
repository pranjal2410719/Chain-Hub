package adapter

import (
	"fmt"

	"github.com/khurafati/chainhub/internal/eventbus"
)

// ClaudeCodeAdapter wraps Anthropic's Claude Code CLI (`claude`) and integrates
// it with the ChainHub orchestrator.  It adds a system prompt on start and
// handles task-assignment events by formatting them as input prompts.
type ClaudeCodeAdapter struct {
	*BaseAdapter
}

// NewClaudeCodeAdapter creates a ClaudeCodeAdapter configured with sensible
// defaults for Claude Code.
func NewClaudeCodeAdapter() *ClaudeCodeAdapter {
	systemPrompt := "You are part of ChainHub, a multi-AI CLI orchestrator. " +
		"Collaborate with other tools, follow task assignments precisely, " +
		"and report your results clearly. Focus on the task at hand and " +
		"produce high-quality, production-ready code."
	info := ToolInfo{
		Name:        "claude-code",
		DisplayName: "Claude Code",
		Specialties: []ToolCapability{CapCoding, CapDebugging, CapRefactoring, CapReview},
		Command:     "claude",
		Args:        []string{"-p", systemPrompt, "--dangerously-skip-permissions"},
		Priority:    "high",
	}
	return &ClaudeCodeAdapter{
		BaseAdapter: NewBaseAdapter(info),
	}
}

// OnEvent handles incoming events.  Task-assignment events are translated into
// input prompts for the Claude CLI.
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
		details, _ := event.Payload["details"].(string)

		prompt := fmt.Sprintf("[ChainHub Task] %s", task)
		if details != "" {
			prompt += fmt.Sprintf("\n\nDetails:\n%s", details)
		}

		if err := c.SendInput(prompt); err != nil {
			c.mu.Lock()
			if c.bus != nil {
				c.bus.Publish(eventbus.NewEvent(
					eventbus.EventToolError,
					c.info.Name,
					map[string]interface{}{
						"tool":  c.info.Name,
						"error": fmt.Sprintf("failed to send task: %v", err),
					},
				))
			}
			c.mu.Unlock()
		}
	}
}
