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
	runCmd.Flags().BoolVar(&noTUI, "no-tui", false, "Run without TUI (headless mode)")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(toolsCmd)
	rootCmd.AddCommand(pipelineCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(assignCmd)
	rootCmd.AddCommand(modeCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(sessionsCmd)
}

// findConfigFile looks for a config file in multiple locations:
// 1. The specified path (if absolute or exists relative to cwd)
// 2. Relative to the binary's location
// 3. Relative to the binary's parent directory (for development)
func findConfigFile(name string) string {
	// Check if the specified path exists
	if _, err := os.Stat(name); err == nil {
		return name
	}

	// Get the directory where the binary is located
	if execPath, err := os.Executable(); err == nil {
		binDir := filepath.Dir(execPath)
		candidate := filepath.Join(binDir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		// Check parent directory (for development layout)
		candidate = filepath.Join(binDir, "..", name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return name // Return original as fallback
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
		if err := os.WriteFile(toolsCfg, []byte("tools: []\n"), 0o644); err != nil {
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
		cfg, err := loadToolsConfig(toolsCfg)
		if err != nil {
			cfg = &ToolsConfig{Tools: []ToolConfigEntry{}}
		}

		exists := false
		for _, t := range cfg.Tools {
			if t.Name == name {
				exists = true
				break
			}
		}

		if exists {
			fmt.Println(cliInfo.Render("\nℹ ") + name + " is already connected")
			return nil
		}

		cfg.Tools = append(cfg.Tools, ToolConfigEntry{
			Name:    name,
			Command: binary,
			Enabled: true,
		})

		if err := saveToolsConfig(cfg, toolsCfg); err != nil {
			return fmt.Errorf("saving tools config: %w", err)
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

var noTUI bool

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
		os.Setenv("CHAINHUB_WORKSPACE", cfg.ChainHub.Workspace)

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
		toolsCfgPath := findConfigFile(filepath.Join("configs", "tools.yaml"))
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

		// Save session for recovery.
		if err := pipeline.Save(cfg.ChainHub.Workspace); err != nil && verbose {
			fmt.Println(cliDim.Render("  session save: " + err.Error()))
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

		// Launch TUI or run headless.
		if noTUI {
			fmt.Println(cliInfo.Render("\n  Running in headless mode (no TUI)"))
			fmt.Println(cliDim.Render("  Pipeline will run in background..."))

			// Wait for context cancellation or pipeline completion.
			<-ctx.Done()
			fmt.Println(cliDim.Render("  Pipeline stopped."))
		} else {
			app := tui.NewApp(engine, registry, mon)
			p := tea.NewProgram(app, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("TUI error: %w", err)
			}
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
		os.Setenv("CHAINHUB_WORKSPACE", cfg.ChainHub.Workspace)

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

		// Connected tools.
		toolsCfgPath := filepath.Join("configs", "tools.yaml")
		if toolsData, err := os.ReadFile(toolsCfgPath); err == nil {
			var toolsList struct {
				Tools []struct {
					Name    string `yaml:"name"`
					Command string `yaml:"command"`
					Enabled bool   `yaml:"enabled"`
				} `yaml:"tools"`
			}
			if err := yaml.Unmarshal(toolsData, &toolsList); err == nil && len(toolsList.Tools) > 0 {
				fmt.Println(cliInfo.Render("\n  Connected Tools:"))
				for _, t := range toolsList.Tools {
					if !t.Enabled {
						continue
					}
					_, err := exec.LookPath(t.Command)
					status := cliSuccess.Render("active")
					if err != nil {
						status = cliDim.Render("binary missing")
					}
					fmt.Printf("    %-18s %s\n", t.Name, status)
				}
			}
		}

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
		os.Setenv("CHAINHUB_WORKSPACE", cfg.ChainHub.Workspace)

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
		os.Setenv("CHAINHUB_WORKSPACE", cfg.ChainHub.Workspace)

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
		os.Setenv("CHAINHUB_WORKSPACE", cfg.ChainHub.Workspace)

		cfg.ChainHub.Pipeline.Mode = mode
		if err := core.SaveConfig(cfg, cfgPath); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		fmt.Printf("\n✅ Orchestration mode switched to %s!\n\n", cliTitle.Render(mode))
		return nil
	},
}

// ─── resume Command ─────────────────────────────────────────────────────────

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the last interrupted pipeline session",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(cliTitle.Render("\n🔗 ChainHub — Resuming Session"))

		// Load config.
		cfg, err := core.LoadConfig(cfgPath)
		if err != nil {
			cfg = core.DefaultConfig()
		}
		os.Setenv("CHAINHUB_WORKSPACE", cfg.ChainHub.Workspace)

		// Load latest session.
		pipeline, err := core.LoadLatestPipeline(cfg.ChainHub.Workspace)
		if err != nil {
			fmt.Println(cliError.Render("  ✗ No session found to resume"))
			fmt.Println(cliDim.Render("  Run `chainhub run` to start a new pipeline"))
			return nil
		}

		fmt.Printf("%s Resuming pipeline: %s\n",
			cliInfo.Render("  →"),
			cliTitle.Render(pipeline.Problem),
		)
		fmt.Printf("%s Status: %s | Progress: %.0f%%\n",
			cliInfo.Render("  →"),
			pipeline.Status,
			pipeline.Progress()*100,
		)

		// Find the current active phase
		currentPhase := pipeline.CurrentPhaseConfig()
		if currentPhase != nil {
			fmt.Printf("%s Current phase: %s\n",
				cliInfo.Render("  →"),
				cliTitle.Render(string(currentPhase.Phase)),
			)
		}

		fmt.Println()

		// Create event bus and engine.
		bus := eventbus.NewEventBus()
		bus.Start()
		defer bus.Stop()

		engine := core.NewEngine(cfg, bus)
		engine.SetPipeline(pipeline)

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

		// Load tools from configs/tools.yaml.
		toolsCfgPath := findConfigFile(filepath.Join("configs", "tools.yaml"))
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

		// Connect all adapters to the event bus.
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

		// Launch TUI or run headless.
		if noTUI {
			fmt.Println(cliInfo.Render("\n  Running in headless mode (no TUI)"))
			fmt.Println(cliDim.Render("  Pipeline will run in background..."))

			// Wait for context cancellation or pipeline completion.
			<-ctx.Done()
			fmt.Println(cliDim.Render("  Pipeline stopped."))
		} else {
			app := tui.NewApp(engine, registry, mon)
			p := tea.NewProgram(app, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("TUI error: %w", err)
			}
		}

		fmt.Println(cliSuccess.Render("\n✅ Pipeline finished. Goodbye!"))
		return nil
	},
}

// ─── sessions Command ───────────────────────────────────────────────────────

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List saved pipeline sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(cliTitle.Render("\n📋 Saved Sessions"))

		// Load config.
		cfg, err := core.LoadConfig(cfgPath)
		if err != nil {
			cfg = core.DefaultConfig()
		}

		sessions, err := core.ListSessions(cfg.ChainHub.Workspace)
		if err != nil {
			return fmt.Errorf("listing sessions: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Println(cliDim.Render("  No sessions found"))
			fmt.Println(cliDim.Render("  Run `chainhub run` to create a new session"))
			return nil
		}

		for i, s := range sessions {
			status := cliSuccess.Render(string(s.Status))
			if s.Status == core.PipelineStatusFailed {
				status = cliError.Render(string(s.Status))
			} else if s.Status == core.PipelineStatusRunning {
				status = cliInfo.Render(string(s.Status))
			}

			prob := s.Problem
			if len(prob) > 50 {
				prob = prob[:47] + "..."
			}

			fmt.Printf("  %d. %s %s (%.0f%%)\n",
				i+1,
				status,
				cliTitle.Render(prob),
				s.Progress()*100,
			)
			fmt.Printf("     ID: %s | Phases: %d/%d completed\n",
				cliDim.Render(s.ID[:8]),
				countCompleted(s),
				len(s.Phases),
			)
		}

		fmt.Println()
		fmt.Println(cliInfo.Render("  Use `chainhub resume` to continue the last session"))
		return nil
	},
}

func countCompleted(p core.Pipeline) int {
	count := 0
	for _, phase := range p.Phases {
		if phase.Status == core.PhaseStatusCompleted {
			count++
		}
	}
	return count
}

type ToolConfigEntry struct {
	Name        string   `yaml:"name"`
	DisplayName string   `yaml:"display_name,omitempty"`
	Command     string   `yaml:"command"`
	Args        []string `yaml:"args,omitempty"`
	Specialties []string `yaml:"specialties,omitempty"`
	Priority    string   `yaml:"priority,omitempty"`
	Enabled     bool     `yaml:"enabled"`
}

type ToolsConfig struct {
	Tools []ToolConfigEntry `yaml:"tools"`
}

func loadToolsConfig(path string) (*ToolsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolsConfig{Tools: []ToolConfigEntry{}}, nil
		}
		return nil, err
	}
	var cfg ToolsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return &ToolsConfig{Tools: []ToolConfigEntry{}}, nil
	}
	return &cfg, nil
}

func saveToolsConfig(cfg *ToolsConfig, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
