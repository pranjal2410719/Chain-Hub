package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExternalTool represents a CLI tool that runs in its own terminal.
type ExternalTool struct {
	Name        string
	DisplayName string
	Binary      string
	Command     string // Full command to launch
	WorkDir     string
	Env         map[string]string
}

// ToolLauncher handles launching external tools in separate terminals.
type ToolLauncher struct {
	workspaceDir string
	tools        map[string]*ExternalTool
}

// NewToolLauncher creates a ToolLauncher for the workspace.
func NewToolLauncher(workspaceDir string) *ToolLauncher {
	return &ToolLauncher{
		workspaceDir: workspaceDir,
		tools:        make(map[string]*ExternalTool),
	}
}

// RegisterTool registers an external tool that can be launched.
func (tl *ToolLauncher) RegisterTool(tool *ExternalTool) {
	tl.tools[tool.Name] = tool
}

// GetTool returns a registered tool by name.
func (tl *ToolLauncher) GetTool(name string) (*ExternalTool, bool) {
	tool, ok := tl.tools[name]
	return tool, ok
}

// ListTools returns all registered tools.
func (tl *ToolLauncher) ListTools() []*ExternalTool {
	tools := make([]*ExternalTool, 0, len(tl.tools))
	for _, tool := range tl.tools {
		tools = append(tools, tool)
	}
	return tools
}

// LaunchCommand returns the shell command to launch a tool in a new terminal.
// This is used for display purposes - the actual launch happens externally.
func (tl *ToolLauncher) LaunchCommand(toolName string) (string, error) {
	tool, ok := tl.tools[toolName]
	if !ok {
		return "", fmt.Errorf("tool %q not registered", toolName)
	}

	// Build the command
	cmd := tool.Binary
	if tool.Command != "" {
		cmd = tool.Command
	}

	// Set working directory
	workDir := tl.workspaceDir
	if tool.WorkDir != "" {
		workDir = tool.WorkDir
	}

	return fmt.Sprintf("cd %s && %s", workDir, cmd), nil
}

// LaunchInTerminal returns a command that can be run in a new terminal window.
// Supports different terminal emulators.
func (tl *ToolLauncher) LaunchInTerminal(toolName string, terminal string) (string, error) {
	cmd, err := tl.LaunchCommand(toolName)
	if err != nil {
		return "", err
	}

	switch terminal {
	case "gnome-terminal":
		return fmt.Sprintf("gnome-terminal -- bash -c '%s; exec bash'", cmd), nil
	case "konsole":
		return fmt.Sprintf("konsole -e bash -c '%s'", cmd), nil
	case "xfce4-terminal":
		return fmt.Sprintf("xfce4-terminal -e 'bash -c \"%s\"'", cmd), nil
	case "alacritty":
		return fmt.Sprintf("alacritty -e bash -c '%s'", cmd), nil
	case "kitty":
		return fmt.Sprintf("kitty %s", cmd), nil
	case "wezterm":
		return fmt.Sprintf("wezterm start -- bash -c '%s'", cmd), nil
	default:
		// Try to detect terminal
		return tl.detectTerminal(cmd)
	}
}

// detectTerminal tries to find and use an available terminal emulator.
func (tl *ToolLauncher) detectTerminal(cmd string) (string, error) {
	terminals := []struct {
		name string
		cmd  func(string) string
	}{
		{"gnome-terminal", func(c string) string { return fmt.Sprintf("gnome-terminal -- bash -c '%s; exec bash'", c) }},
		{"konsole", func(c string) string { return fmt.Sprintf("konsole -e bash -c '%s'", c) }},
		{"xfce4-terminal", func(c string) string { return fmt.Sprintf("xfce4-terminal -e 'bash -c \"%s\"'", c) }},
		{"alacritty", func(c string) string { return fmt.Sprintf("alacritty -e bash -c '%s'", c) }},
		{"kitty", func(c string) string { return fmt.Sprintf("kitty %s", c) }},
		{"xterm", func(c string) string { return fmt.Sprintf("xterm -e bash -c '%s'", c) }},
	}

	for _, t := range terminals {
		if _, err := exec.LookPath(t.name); err == nil {
			return t.cmd(cmd), nil
		}
	}

	return "", fmt.Errorf("no terminal emulator found")
}

// GetLaunchInstructions returns formatted instructions for launching a tool.
func (tl *ToolLauncher) GetLaunchInstructions(toolName string) string {
	tool, ok := tl.tools[toolName]
	if !ok {
		return fmt.Sprintf("Tool %q not found", toolName)
	}

	workDir := tl.workspaceDir
	if tool.WorkDir != "" {
		workDir = tool.WorkDir
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Launch %s in a separate terminal:\n", tool.DisplayName))
	sb.WriteString(fmt.Sprintf("\n  cd %s\n", workDir))
	sb.WriteString(fmt.Sprintf("  %s\n", tool.Binary))
	sb.WriteString("\nOr use a terminal command:\n")
	sb.WriteString(fmt.Sprintf("\n  gnome-terminal -- bash -c 'cd %s && %s; exec bash'\n", workDir, tool.Binary))

	return sb.String()
}

// SetupDefaultTools registers the default ChainHub tools.
func (tl *ToolLauncher) SetupDefaultTools() {
	tl.RegisterTool(&ExternalTool{
		Name:        "antigravity",
		DisplayName: "Antigravity CLI",
		Binary:      "agy",
		Command:     "agy",
	})

	tl.RegisterTool(&ExternalTool{
		Name:        "mimo-code",
		DisplayName: "Mimo Code",
		Binary:      "mimo",
		Command:     "mimo",
	})

	tl.RegisterTool(&ExternalTool{
		Name:        "opencode",
		DisplayName: "OpenCode",
		Binary:      "opencode",
		Command:     "opencode",
	})

	tl.RegisterTool(&ExternalTool{
		Name:        "freebuff",
		DisplayName: "FreeBuff",
		Binary:      "freebuff",
		Command:     "freebuff",
	})
}

// GetToolConfigPath returns the path to a tool's configuration file.
func (tl *ToolLauncher) GetToolConfigPath(toolName string) string {
	return filepath.Join(tl.workspaceDir, ".chainhub", "tools", toolName+".json")
}

// SaveToolConfig saves a tool's configuration to disk.
func (tl *ToolLauncher) SaveToolConfig(toolName string, config map[string]interface{}) error {
	configDir := filepath.Join(tl.workspaceDir, ".chainhub", "tools")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("creating tools config dir: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	path := tl.GetToolConfigPath(toolName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}
