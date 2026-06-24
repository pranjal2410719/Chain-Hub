package adapter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/khurafati/chainhub/internal/eventbus"
)

// BaseAdapter provides a default, production-quality implementation of the
// ToolAdapter interface.  Concrete adapters (ClaudeCodeAdapter, etc.) embed
// *BaseAdapter and optionally override individual methods.
type BaseAdapter struct {
	info       ToolInfo
	status     ToolStatus
	statusText string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	outputCh chan string
	errorCh  chan string

	bus       *eventbus.EventBus
	sub       *eventbus.Subscriber
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
	alive     bool
	exitCh    chan struct{}
	autopilot bool
}

// NewBaseAdapter creates a BaseAdapter pre-populated with the given ToolInfo.
// The adapter starts in ToolStatusIdle and must be explicitly started.
func NewBaseAdapter(info ToolInfo) *BaseAdapter {
	return &BaseAdapter{
		info:       info,
		status:     ToolStatusIdle,
		statusText: "idle",
		outputCh:   make(chan string, 256),
		errorCh:    make(chan string, 256),
	}
}

// ---------------------------------------------------------------------------
// Getter methods
// ---------------------------------------------------------------------------

// Name returns the tool's unique identifier.
func (b *BaseAdapter) Name() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.info.Name
}

// DisplayName returns the human-friendly label for the tool.
func (b *BaseAdapter) DisplayName() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.info.DisplayName
}

// Specialties returns the list of capabilities this tool supports.
func (b *BaseAdapter) Specialties() []ToolCapability {
	b.mu.RLock()
	defer b.mu.RUnlock()
	caps := make([]ToolCapability, len(b.info.Specialties))
	copy(caps, b.info.Specialties)
	return caps
}

// Status returns the current lifecycle state of the adapter.
func (b *BaseAdapter) Status() ToolStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.status
}

// StatusText returns a short human-readable description of the current state.
func (b *BaseAdapter) StatusText() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.statusText
}

// Info returns a snapshot of all metadata and runtime state.
func (b *BaseAdapter) Info() ToolInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()
	info := b.info
	info.Status = b.status
	info.StatusText = b.statusText
	return info
}

// ---------------------------------------------------------------------------
// Process lifecycle
// ---------------------------------------------------------------------------

// Start launches the underlying CLI process, sets up stdin/stdout/stderr pipes,
// and starts goroutines that stream output and error lines into their respective
// channels. An EventToolStatusChanged event is published when the status changes.
func (b *BaseAdapter) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.status == ToolStatusActive {
		return fmt.Errorf("adapter %s is already running", b.info.Name)
	}

	b.ctx, b.cancel = context.WithCancel(ctx)

	b.cmd = exec.CommandContext(b.ctx, b.info.Command, b.info.Args...)
	if len(b.info.Env) > 0 {
		b.cmd.Env = os.Environ()
		for k, v := range b.info.Env {
			b.cmd.Env = append(b.cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	b.exitCh = make(chan struct{})

	var err error
	b.stdin, err = b.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe for %s: %w", b.info.Name, err)
	}

	b.stdout, err = b.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe for %s: %w", b.info.Name, err)
	}

	b.stderr, err = b.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe for %s: %w", b.info.Name, err)
	}

	if err := b.cmd.Start(); err != nil {
		b.setStatusLocked(ToolStatusError, fmt.Sprintf("failed to start: %v", err))
		return fmt.Errorf("failed to start %s: %w", b.info.Name, err)
	}

	b.alive = true
	b.setStatusLocked(ToolStatusActive, "running")

	// Stream stdout into outputCh.
	go b.readStream(b.stdout, b.outputCh, eventbus.EventToolOutput)
	// Stream stderr into errorCh.
	go b.readStream(b.stderr, b.errorCh, eventbus.EventToolError)
	// Wait for process exit in the background.
	go b.waitForExit()

	// Subscribe to event bus if set.
	if b.bus != nil {
		b.sub = b.bus.SubscribeAll()
		go b.listenToBusEvents()
	}

	return nil
}

// Stop gracefully terminates the managed process.  It first sends SIGTERM and
// waits up to 5 seconds before force-killing.
func (b *BaseAdapter) Stop() error {
	b.mu.Lock()

	if b.cmd == nil || b.cmd.Process == nil || !b.alive {
		b.setStatusLocked(ToolStatusStopped, "stopped (no process)")
		b.cleanupLocked()
		b.mu.Unlock()
		return nil
	}

	// Close stdin so the child sees EOF.
	if b.stdin != nil {
		_ = b.stdin.Close()
	}

	// Send SIGTERM.
	_ = b.cmd.Process.Signal(syscall.SIGTERM)

	exitCh := b.exitCh
	b.mu.Unlock()

	select {
	case <-exitCh:
		// Exited gracefully.
	case <-time.After(5 * time.Second):
		b.mu.Lock()
		if b.cmd != nil && b.cmd.Process != nil && b.alive {
			_ = b.cmd.Process.Kill()
		}
		b.mu.Unlock()
		<-exitCh
	}

	b.mu.Lock()
	b.setStatusLocked(ToolStatusStopped, "stopped")
	b.cleanupLocked()
	b.mu.Unlock()

	return nil
}

// SendInput writes a line to the tool's standard input. A trailing newline is
// appended automatically.
func (b *BaseAdapter) SendInput(input string) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.stdin == nil {
		return fmt.Errorf("stdin not available for %s", b.info.Name)
	}
	if !b.alive {
		return fmt.Errorf("adapter %s is not alive", b.info.Name)
	}

	_, err := fmt.Fprintln(b.stdin, input)
	if err != nil {
		return fmt.Errorf("failed to write to stdin for %s: %w", b.info.Name, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Channels
// ---------------------------------------------------------------------------

// OutputChan returns a read-only channel that emits lines from stdout.
func (b *BaseAdapter) OutputChan() <-chan string {
	return b.outputCh
}

// ErrorChan returns a read-only channel that emits lines from stderr.
func (b *BaseAdapter) ErrorChan() <-chan string {
	return b.errorCh
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

// HealthCheck verifies that the underlying process is still running.
func (b *BaseAdapter) HealthCheck() error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.alive {
		return fmt.Errorf("adapter %s process is not alive", b.info.Name)
	}
	if b.cmd == nil || b.cmd.Process == nil {
		return fmt.Errorf("adapter %s has no running process", b.info.Name)
	}

	// On Unix a zero-signal checks whether the process exists.
	if err := b.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("adapter %s process check failed: %w", b.info.Name, err)
	}
	return nil
}

// IsAlive returns true when the underlying process is running.
func (b *BaseAdapter) IsAlive() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.alive
}

// ---------------------------------------------------------------------------
// Event bus
// ---------------------------------------------------------------------------

// SetEventBus attaches the shared event bus to this adapter.
func (b *BaseAdapter) SetEventBus(bus *eventbus.EventBus) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.bus = bus
}

// SetAutopilot enables or disables autopilot mode for this adapter.
// In autopilot mode, the adapter auto-responds to common prompts.
func (b *BaseAdapter) SetAutopilot(enabled bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.autopilot = enabled
}

// IsAutopilot returns whether autopilot mode is enabled.
func (b *BaseAdapter) IsAutopilot() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.autopilot
}

// RespondToPrompt sends a response to the tool's stdin for a pending prompt.
func (b *BaseAdapter) RespondToPrompt(response string) error {
	return b.SendInput(response)
}

// listenToBusEvents reads events from the subscription channel in a loop and calls OnEvent.
func (b *BaseAdapter) listenToBusEvents() {
	b.mu.RLock()
	sub := b.sub
	b.mu.RUnlock()
	if sub == nil {
		return
	}

	for {
		b.mu.RLock()
		ctx := b.ctx
		b.mu.RUnlock()
		if ctx == nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case ev, ok := <-sub.Channel:
			if !ok {
				return
			}
			b.OnEvent(ev)
		}
	}
}

// OnEvent is the default event handler. Concrete adapters should override this
// to implement tool-specific behaviour.
func (b *BaseAdapter) OnEvent(event eventbus.Event) {
	switch event.Type {
	case eventbus.EventTaskAssigned:
		assignedTool, _ := event.Payload["tool"].(string)
		b.mu.RLock()
		name := b.info.Name
		b.mu.RUnlock()
		if assignedTool != name {
			return
		}

		task, _ := event.Payload["task"].(string)
		details, _ := event.Payload["details"].(string)

		prompt := fmt.Sprintf("[ChainHub Task] %s", task)
		if details != "" {
			prompt += fmt.Sprintf("\n\nDetails:\n%s", details)
		}

		if err := b.SendInput(prompt); err != nil {
			b.mu.Lock()
			bus := b.bus
			b.mu.Unlock()

			if bus != nil {
				bus.Publish(eventbus.NewEvent(
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

// ---------------------------------------------------------------------------
// Capabilities
// ---------------------------------------------------------------------------

// HasCapability reports whether this adapter supports the given capability.
func (b *BaseAdapter) HasCapability(cap ToolCapability) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, c := range b.info.Specialties {
		if c == cap {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// setStatusLocked updates status fields and publishes an event. MUST be called
// with b.mu already held.
func (b *BaseAdapter) setStatusLocked(status ToolStatus, text string) {
	b.status = status
	b.statusText = text
	b.info.Status = status
	b.info.StatusText = text

	if b.bus != nil {
		b.bus.Publish(eventbus.NewEvent(
			eventbus.EventToolStatusChanged,
			b.info.Name,
			map[string]interface{}{
				"tool":        b.info.Name,
				"status":      string(status),
				"status_text": text,
			},
		))
	}
}

// readStream reads lines from the given reader and sends them to the channel.
// When the event bus is set it also publishes events of the given type.
func (b *BaseAdapter) readStream(r io.ReadCloser, ch chan<- string, evtType eventbus.EventType) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		select {
		case ch <- line:
		default:
		}

		b.mu.RLock()
		bus := b.bus
		name := b.info.Name
		autopilot := b.autopilot
		b.mu.RUnlock()

		if bus != nil {
			bus.Publish(eventbus.NewEvent(
				evtType,
				name,
				map[string]interface{}{
					"tool": name,
					"line": line,
				},
			))

			// Detect tool prompts asking for user input
			if evtType == eventbus.EventToolOutput && isToolPrompt(line) {
				bus.Publish(eventbus.NewEvent(
					eventbus.EventInputNeeded,
					name,
					map[string]interface{}{
						"tool":     name,
						"prompt":   line,
						"autopilot": autopilot,
					},
				))
			}
		}
	}
}

// isToolPrompt detects common tool prompts asking for user input.
func isToolPrompt(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	prompts := []string{
		"yes, i trust",
		"no, exit",
		"do you want",
		"press enter",
		"continue?",
		"proceed?",
		"allow",
		"confirm",
		"accept",
		"reject",
		"[y/n]",
		"[yes/no]",
		"y/n",
		"permission",
		"authorize",
	}
	for _, p := range prompts {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// cleanupLocked performs event bus and context cleanup. MUST be called with b.mu held.
func (b *BaseAdapter) cleanupLocked() {
	if b.bus != nil && b.sub != nil {
		b.bus.Unsubscribe(b.sub.ID)
		b.sub = nil
	}
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
}

// waitForExit waits for the process to exit and updates the adapter state.
func (b *BaseAdapter) waitForExit() {
	if b.cmd == nil {
		return
	}
	err := b.cmd.Wait()

	b.mu.Lock()
	defer b.mu.Unlock()

	b.alive = false
	if err != nil {
		b.setStatusLocked(ToolStatusError, fmt.Sprintf("exited with error: %v", err))
	} else {
		b.setStatusLocked(ToolStatusStopped, "exited normally")
	}
	b.cleanupLocked()
	if b.exitCh != nil {
		select {
		case <-b.exitCh:
		default:
			close(b.exitCh)
		}
	}
}
