// Package main provides the CLI entry point for ChainHub, a multi-AI CLI
// orchestrator. It uses Cobra for command routing and the internal TUI
// package for the interactive dashboard.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/khurafati/chainhub/internal/adapter"
	"github.com/khurafati/chainhub/internal/core"
	"github.com/khurafati/chainhub/internal/eventbus"
	"github.com/khurafati/chainhub/internal/monitor"
	"github.com/khurafati/chainhub/internal/plugin"
	"github.com/khurafati/chainhub/internal/tui"
)

// Build-time variables — injected via ldflags.
var (
	Version   = "0.1.0"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// ─── Styles (for CLI output, not TUI) ───────────────────────────────────────

var (
	cliTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	cliSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	cliError   = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	cliInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("#06B6D4"))
	cliDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
)

// ─── Root Command ───────────────────────────────────────────────────────────

var cfgPath string
var verbose bool

var rootCmd = &cobra.Command{
	Use:   "chainhub",
	Short: "🔗 Multi-AI CLI Orchestrator",
	Long: `ChainHub orchestrates multiple AI-powered CLI tools into a unified
pipeline — planning, coding, scanning, testing, and review — so your
AI assistants collaborate instead of competing.`,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "./configs/default.yaml", "Path to configuration file")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(toolsCmd)
	rootCmd.AddCommand(pipelineCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(assignCmd)
	rootCmd.AddCommand(modeCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, cliError.Render("Error: "+err.Error()))
		os.Exit(1)
	}
}

// ─── init Command ───────────────────────────────────────────────────────────

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new ChainHub workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(cliTitle.Render("\n🔗 Initializing ChainHub workspace…"))

		dirs := []string{
			"configs",
			"workspace",
			"workspace/plans",
			"workspace/code",
			"workspace/reports",
			"plugins",
		}

		for _, d := range dirs {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", d, err)
			}
			fmt.Println(cliSuccess.Render("  ✓ ") + cliDim.Render(d+"/"))
		}

		// Write default config.
		cfg := core.DefaultConfig()
		cfgFile := filepath.Join("configs", "default.yaml")
		if err := core.SaveConfig(cfg, cfgFile); err != nil {
			return fmt.Errorf("writing default config: %w", err)
		}
		fmt.Println(cliSuccess.Render("  ✓ ") + cliDim.Render(cfgFile))

		// Write empty tools config.
		toolsCfg := filepath.Join("configs", "tools.yaml")
		if err := os.WriteFile(toolsCfg, []byte("# ChainHub connected tools\ntools: []\n"), 0o644); err != nil {
			return fmt.Errorf("writing tools config: %w", err)
		}
		fmt.Println(cliSuccess.Render("  ✓ ") + cliDim.Render(toolsCfg))

		fmt.Println(cliSuccess.Render("\n✅ Workspace initialized successfully!"))
		fmt.Println(cliInfo.Render("   Next: chainhub connect <tool-name>"))
		return nil
	},
}

// ─── connect Command ────────────────────────────────────────────────────────

// knownTools maps friendly names to the binary expected in PATH.
var knownTools = map[string]string{
	"claude-code":  "claude",
	"antigravity":  "agy",
	"mimo-code":    "mimo",
	"opencode":     "opencode",
	"freebuff":     "freebuff",
}

var connectCmd = &cobra.Command{
	Use:   "connect <tool-name>",
	Short: "Connect/register an AI CLI tool",
	Long: `Register an AI CLI tool for use in pipelines.

Supported tools: claude-code, antigravity, mimo-code, opencode, freebuff`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(args[0])

		binary, ok := knownTools[name]
		if !ok {
			return fmt.Errorf("unknown tool %q — supported: %s",
				name, strings.Join(knownToolNames(), ", "))
		}

		fmt.Printf("%s Connecting %s…\n",
			cliInfo.Render("🔗"),
			cliTitle.Render(name),
		)

		// Verify binary exists.
		path, err := exec.LookPath(binary)
		if err != nil {
			fmt.Printf("%s Binary %q not found in PATH\n",
				cliError.Render("  ✗"),
				binary,
			)
			fmt.Println(cliDim.Render("  Install it first, then retry."))
			return fmt.Errorf("%s not found in PATH", binary)
		}
		fmt.Printf("%s Found: %s\n", cliSuccess.Render("  ✓"), cliDim.Render(path))

		// Update tools config.
		toolsCfg := filepath.Join("configs", "tools.yaml")
		entry := fmt.Sprintf("\n  - name: %s\n    command: %s\n    enabled: true\n", name, binary)

		f, err := os.OpenFile(toolsCfg, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("updating tools config: %w", err)
		}
		defer f.Close()

		if _, err := f.WriteString(entry); err != nil {
			return fmt.Errorf("writing tool entry: %w", err)
		}

		fmt.Println(cliSuccess.Render("\n✅ " + name + " connected successfully!"))
		return nil
	},
}

func knownToolNames() []string {
	names := make([]string, 0, len(knownTools))
	for n := range knownTools {
		names = append(names, n)
	}
	return names
}

// ─── run Command ────────────────────────────────────────────────────────────

var runCmd = &cobra.Command{
	Use:   `run "<problem statement>"`,
	Short: "Submit a problem and launch the pipeline + TUI",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		problem := args[0]

		fmt.Println(cliTitle.Render("\n🔗 ChainHub — Starting Pipeline"))
		fmt.Printf("%s %s\n\n", cliInfo.Render("Problem:"), problem)

		// Load config.
		cfg, err := core.LoadConfig(cfgPath)
		if err != nil {
			fmt.Println(cliDim.Render("  (config not found, using defaults)"))
			cfg = core.DefaultConfig()
		}

		if err := core.EnsureWorkspace(cfg); err != nil {
			return fmt.Errorf("ensuring workspace: %w", err)
		}

		// Event bus.
		bus := eventbus.NewEventBus()
		bus.Start()
		defer bus.Stop()

		// Engine.
		engine := core.NewEngine(cfg, bus)

		// Context for cancellation.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Signal handling.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		// Registry — load plugins and register adapters.
		registry := adapter.NewRegistry()
		engine.SetRegistry(registry)

		// Load tools from configs/tools.yaml if exists.
		toolsCfgPath := filepath.Join("configs", "tools.yaml")
		if toolsData, err := os.ReadFile(toolsCfgPath); err == nil {
			var toolsList struct {
				Tools []struct {
					Name        string   `yaml:"name"`
					DisplayName string   `yaml:"display_name"`
					Command     string   `yaml:"command"`
					Args        []string `yaml:"args"`
					Specialties []string `yaml:"specialties"`
					Priority    string   `yaml:"priority"`
					Enabled     bool     `yaml:"enabled"`
				} `yaml:"tools"`
			}
			if err := yaml.Unmarshal(toolsData, &toolsList); err == nil {
				specialtyMap := map[string]adapter.ToolCapability{
					"planning":       adapter.CapPlanning,
					"coding":         adapter.CapCoding,
					"implementation": adapter.CapCoding,
					"scanning":       adapter.CapScanning,
					"security":       adapter.CapScanning,
					"testing":        adapter.CapTesting,
					"research":       adapter.CapResearch,
					"debugging":      adapter.CapDebugging,
					"review":         adapter.CapReview,
					"refactoring":    adapter.CapRefactoring,
					"browsing":       adapter.CapBrowsing,
				}

				for _, t := range toolsList.Tools {
					if !t.Enabled {
						continue
					}
					var a adapter.ToolAdapter
					switch t.Name {
					case "claude-code":
						a = adapter.NewClaudeCodeAdapter()
					case "antigravity":
						a = adapter.NewAntigravityAdapter()
					case "mimo-code":
						a = adapter.NewMimoCodeAdapter()
					case "opencode":
						a = adapter.NewOpenCodeAdapter()
					case "freebuff":
						a = adapter.NewFreeBufAdapter()
					default:
						info := adapter.ToolInfo{
							Name:        t.Name,
							DisplayName: t.DisplayName,
							Command:     t.Command,
							Args:        t.Args,
							Priority:    t.Priority,
						}
						for _, spec := range t.Specialties {
							if cap, ok := specialtyMap[spec]; ok {
								info.Specialties = append(info.Specialties, cap)
							}
						}
						a = adapter.NewGenericAdapter(info)
					}
					_ = registry.Register(a)
				}
			}
		}

		loader := plugin.NewLoader("./plugins")
		if err := loader.LoadAll(); err != nil && verbose {
			fmt.Println(cliDim.Render("  plugin load: " + err.Error()))
		}
		for _, manifest := range loader.ListManifests() {
			a, err := loader.CreateAdapter(manifest.Name)
			if err != nil {
				if verbose {
					fmt.Printf("  skip plugin %s: %v\n", manifest.Name, err)
				}
				continue
			}
			_ = registry.Register(a)
		}

		// Connect all adapters in the registry to the event bus.
		for _, a := range registry.List() {
			a.SetEventBus(bus)
		}

		// Start engine and registry.
		if err := engine.Start(ctx); err != nil {
			return fmt.Errorf("starting engine: %w", err)
		}
		defer func() { _ = engine.Stop() }()

		if err := registry.StartAll(ctx); err != nil && verbose {
			fmt.Println(cliDim.Render("  registry start: " + err.Error()))
		}
		defer func() { _ = registry.StopAll() }()

		// Submit problem.
		pipeline, err := engine.SubmitProblem(problem)
		if err != nil {
			return fmt.Errorf("submitting problem: %w", err)
		}
		if verbose {
			fmt.Printf("  Pipeline %s created with %d phases\n",
				pipeline.ID, len(pipeline.Phases))
		}

		// System monitor.
		mon := monitor.NewSystemMonitor(
			2*time.Second,
			bus,
			monitor.AlertThresholds{CPUPercent: 90, MemoryPercent: 90, DiskPercent: 95},
		)
		if err := mon.Start(ctx); err != nil && verbose {
			fmt.Println(cliDim.Render("  monitor start: " + err.Error()))
		}
		defer mon.Stop()

		// Launch TUI.
		app := tui.NewApp(engine, registry, mon)
		p := tea.NewProgram(app, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}

		fmt.Println(cliSuccess.Render("\n✅ Pipeline finished. Goodbye!"))
		return nil
	},
}

// ─── status Command ─────────────────────────────────────────────────────────

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current ChainHub status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(cliTitle.Render("\n🔗 ChainHub Status"))

		cfg, err := core.LoadConfig(cfgPath)
		if err != nil {
			cfg = core.DefaultConfig()
		}

		bus := eventbus.NewEventBus()
		engine := core.NewEngine(cfg, bus)

		// Pipeline.
		pipeline := engine.GetPipeline()
		if pipeline != nil {
			fmt.Println(cliInfo.Render("\n  Pipeline:"))
			fmt.Printf("    ID:       %s\n", pipeline.ID)
			fmt.Printf("    Problem:  %s\n", pipeline.Problem)
			fmt.Printf("    Status:   %s\n", pipeline.Status)
			fmt.Printf("    Progress: %.0f%%\n", pipeline.Progress()*100)
		} else {
			fmt.Println(cliDim.Render("\n  No active pipeline"))
		}

		// System metrics.
		mon := monitor.NewSystemMonitor(
			time.Second,
			bus,
			monitor.AlertThresholds{CPUPercent: 90, MemoryPercent: 90, DiskPercent: 95},
		)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = mon.Start(ctx)
		time.Sleep(time.Second) // let metrics collect
		metrics := mon.GetMetrics()
		mon.Stop()

		fmt.Println(cliInfo.Render("\n  System:"))
		fmt.Printf("    CPU:    %.1f%%\n", metrics.CPUPercent)
		fmt.Printf("    Memory: %.1f%% (%d/%d MB)\n",
			metrics.MemoryPercent, metrics.MemoryUsedMB, metrics.MemoryTotalMB)
		fmt.Printf("    Disk:   %.1f%% (%.1f/%.1f GB)\n",
			metrics.DiskPercent, metrics.DiskUsedGB, metrics.DiskTotalGB)

		fmt.Println()
		return nil
	},
}

// ─── tools Command ──────────────────────────────────────────────────────────

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Manage connected AI tools",
}

var toolsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available tools and their status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(cliTitle.Render("\n🛠  Available Tools"))
		fmt.Println()

		header := fmt.Sprintf("  %-18s %-12s %-10s %s", "TOOL", "BINARY", "STATUS", "SPECIALTIES")
		fmt.Println(cliInfo.Render(header))
		fmt.Println(cliDim.Render("  " + strings.Repeat("─", 70)))

		type toolRow struct {
			name, binary, status, specs string
		}

		// Build known tool table.
		rows := []toolRow{
			{"claude-code", "claude", "", "planning, coding, review"},
			{"antigravity", "agy", "", "coding, planning, scanning"},
			{"mimo-code", "mimo", "", "coding, implementation"},
			{"opencode", "opencode", "", "coding, research"},
			{"freebuff", "freebuff", "", "scanning, security"},
		}

		for _, row := range rows {
			_, err := exec.LookPath(row.binary)
			if err == nil {
				row.status = cliSuccess.Render("installed")
			} else {
				row.status = cliDim.Render("not found")
			}
			fmt.Printf("  %-18s %-12s %-20s %s\n",
				cliTitle.Render(row.name),
				cliDim.Render(row.binary),
				row.status,
				cliDim.Render(row.specs),
			)
		}

		fmt.Println()
		fmt.Println(cliInfo.Render("  Use `chainhub connect <tool>` to register a tool."))
		fmt.Println()
		return nil
	},
}

func init() {
	toolsCmd.AddCommand(toolsListCmd)
}

// ─── pipeline Command ───────────────────────────────────────────────────────

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Pipeline management commands",
}

var pipelineShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current pipeline details",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := core.LoadConfig(cfgPath)
		if err != nil {
			cfg = core.DefaultConfig()
		}

		bus := eventbus.NewEventBus()
		engine := core.NewEngine(cfg, bus)
		pipeline := engine.GetPipeline()

		if pipeline == nil {
			fmt.Println(cliDim.Render("\n  No active pipeline.\n"))
			return nil
		}

		fmt.Println(cliTitle.Render("\n🔗 Pipeline Details"))
		fmt.Println()
		fmt.Printf("  %s %s\n", cliInfo.Render("ID:"), pipeline.ID)
		fmt.Printf("  %s %s\n", cliInfo.Render("Problem:"), pipeline.Problem)
		fmt.Printf("  %s %s\n", cliInfo.Render("Status:"), string(pipeline.Status))
		fmt.Printf("  %s %s\n", cliInfo.Render("Created:"), pipeline.CreatedAt.Format(time.RFC3339))
		fmt.Printf("  %s %.0f%%\n", cliInfo.Render("Progress:"), pipeline.Progress()*100)
		fmt.Println()

		fmt.Println(cliInfo.Render("  Phases:"))
		fmt.Println(cliDim.Render("  " + strings.Repeat("─", 50)))

		for i, phase := range pipeline.Phases {
			icon := "⏳"
			switch phase.Status {
			case core.PhaseStatusCompleted:
				icon = "✅"
			case core.PhaseStatusActive:
				icon = "🔄"
			case core.PhaseStatusFailed:
				icon = "❌"
			}

			marker := " "
			if i == pipeline.CurrentPhase {
				marker = "►"
			}

			tools := "(none)"
			if len(phase.AssignedTools) > 0 {
				tools = strings.Join(phase.AssignedTools, ", ")
			}

			fmt.Printf("  %s %s %-20s %-12s  tools: %s\n",
				marker, icon,
				cliTitle.Render(string(phase.Phase)),
				cliDim.Render(string(phase.Status)),
				cliDim.Render(tools),
			)
		}

		fmt.Println()
		return nil
	},
}

func init() {
	pipelineCmd.AddCommand(pipelineShowCmd)
}

// ─── version Command ────────────────────────────────────────────────────────

const banner = `
   _____ _           _       _    _       _     
  / ____| |         (_)     | |  | |     | |    
 | |    | |__   __ _ _ _ __ | |__| |_   _| |__  
 | |    | '_ \ / _` + "`" + ` | | '_ \|  __  | | | | '_ \ 
 | |____| | | | (_| | | | | | |  | | |_| | |_) |
  \_____|_| |_|\__,_|_|_| |_|_|  |_|\__,_|_.__/ 
`

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		bannerStyled := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true).
			Render(banner)

		fmt.Println(bannerStyled)
		fmt.Printf("  %s %s\n", cliInfo.Render("Version:"), Version)
		fmt.Printf("  %s %s\n", cliInfo.Render("Commit:"), GitCommit)
		fmt.Printf("  %s %s\n", cliInfo.Render("Built:"), BuildTime)
		fmt.Printf("  %s %s\n", cliInfo.Render("Module:"), "github.com/khurafati/chainhub")
		fmt.Println()
	},
}

// ─── assign Command ──────────────────────────────────────────────────────────

var assignCmd = &cobra.Command{
	Use:   "assign <phase> <tool-name>",
	Short: "Assign an AI tool to a pipeline phase (enables manual mode)",
	Long: `Explicitly assign an AI tool to a specific pipeline phase.
This switches ChainHub to manual mode, overriding capability auto-matching.

Supported phases: planning, research, implementation, scanning, testing`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		phase := strings.ToLower(args[0])
		tool := strings.ToLower(args[1])

		// Validate phase.
		validPhases := map[string]bool{
			"planning":       true,
			"research":       true,
			"implementation": true,
			"scanning":       true,
			"testing":        true,
		}
		if !validPhases[phase] {
			return fmt.Errorf("invalid phase %q — supported: planning, research, implementation, scanning, testing", phase)
		}

		fmt.Printf("%s Assigning %s to %s phase…\n",
			cliInfo.Render("🔗"),
			cliTitle.Render(tool),
			cliTitle.Render(phase),
		)

		// Load config.
		cfg, err := core.LoadConfig(cfgPath)
		if err != nil {
			fmt.Println(cliDim.Render("  (config not found, initializing with defaults)"))
			cfg = core.DefaultConfig()
		}

		// Update config.
		cfg.ChainHub.Pipeline.Mode = "manual"
		if cfg.ChainHub.Pipeline.PhaseAssignments == nil {
			cfg.ChainHub.Pipeline.PhaseAssignments = make(map[string]string)
		}
		cfg.ChainHub.Pipeline.PhaseAssignments[phase] = tool

		// Save config.
		if err := core.SaveConfig(cfg, cfgPath); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		fmt.Println(cliSuccess.Render("\n✅ Assignment saved successfully!"))
		fmt.Printf("   Manual Mode enabled. %s will run during the %s phase.\n\n",
			cliTitle.Render(tool),
			cliTitle.Render(phase),
		)
		return nil
	},
}

// ─── mode Command ────────────────────────────────────────────────────────────

var modeCmd = &cobra.Command{
	Use:   "mode <auto|manual>",
	Short: "Switch orchestration mode between auto and manual",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mode := strings.ToLower(args[0])
		if mode != "auto" && mode != "manual" {
			return fmt.Errorf("invalid mode %q — supported: auto, manual", mode)
		}

		// Load config.
		cfg, err := core.LoadConfig(cfgPath)
		if err != nil {
			cfg = core.DefaultConfig()
		}

		cfg.ChainHub.Pipeline.Mode = mode
		if err := core.SaveConfig(cfg, cfgPath); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		fmt.Printf("\n✅ Orchestration mode switched to %s!\n\n", cliTitle.Render(mode))
		return nil
	},
}
