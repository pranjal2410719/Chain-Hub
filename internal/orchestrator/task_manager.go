package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Task represents a unit of work to be executed by an external tool.
type Task struct {
	ID          string            `json:"id"`
	Phase       string            `json:"phase"`
	Tool        string            `json:"tool,omitempty"`
	Problem     string            `json:"problem"`
	Description string            `json:"description"`
	Status      string            `json:"status"` // pending, assigned, running, completed, failed
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Result      string            `json:"result,omitempty"`
	Error       string            `json:"error,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// TaskManager handles task lifecycle for external tool coordination.
type TaskManager struct {
	workspaceDir string
}

// NewTaskManager creates a TaskManager for the given workspace.
func NewTaskManager(workspaceDir string) *TaskManager {
	return &TaskManager{workspaceDir: workspaceDir}
}

// CreateTask creates a new task file that an external tool can pick up.
func (tm *TaskManager) CreateTask(task *Task) error {
	taskDir := filepath.Join(tm.workspaceDir, ".chainhub", "tasks")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("creating tasks dir: %w", err)
	}

	if task.ID == "" {
		task.ID = fmt.Sprintf("task-%d", time.Now().UnixMilli())
	}
	if task.Status == "" {
		task.Status = "pending"
	}
	task.CreatedAt = time.Now()

	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling task: %w", err)
	}

	path := filepath.Join(taskDir, task.ID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing task file: %w", err)
	}

	return nil
}

// UpdateTask updates an existing task file.
func (tm *TaskManager) UpdateTask(task *Task) error {
	taskDir := filepath.Join(tm.workspaceDir, ".chainhub", "tasks")
	path := filepath.Join(taskDir, task.ID+".json")

	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling task: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing task file: %w", err)
	}

	return nil
}

// GetTask reads a task by ID.
func (tm *TaskManager) GetTask(id string) (*Task, error) {
	taskDir := filepath.Join(tm.workspaceDir, ".chainhub", "tasks")
	path := filepath.Join(taskDir, id+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading task file: %w", err)
	}

	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("unmarshalling task: %w", err)
	}

	return &task, nil
}

// ListPendingTasks returns all tasks with status "pending".
func (tm *TaskManager) ListPendingTasks() ([]*Task, error) {
	return tm.listTasksByStatus("pending")
}

// ListActiveTasks returns all tasks that are assigned or running.
func (tm *TaskManager) ListActiveTasks() ([]*Task, error) {
	taskDir := filepath.Join(tm.workspaceDir, ".chainhub", "tasks")

	entries, err := os.ReadDir(taskDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading tasks dir: %w", err)
	}

	var tasks []*Task
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(taskDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var task Task
		if err := json.Unmarshal(data, &task); err != nil {
			continue
		}

		if task.Status == "assigned" || task.Status == "running" {
			tasks = append(tasks, &task)
		}
	}

	return tasks, nil
}

// ListCompletedTasks returns all completed tasks.
func (tm *TaskManager) ListCompletedTasks() ([]*Task, error) {
	return tm.listTasksByStatus("completed")
}

func (tm *TaskManager) listTasksByStatus(status string) ([]*Task, error) {
	taskDir := filepath.Join(tm.workspaceDir, ".chainhub", "tasks")

	entries, err := os.ReadDir(taskDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading tasks dir: %w", err)
	}

	var tasks []*Task
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(taskDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var task Task
		if err := json.Unmarshal(data, &task); err != nil {
			continue
		}

		if task.Status == status {
			tasks = append(tasks, &task)
		}
	}

	return tasks, nil
}

// CompleteTask marks a task as completed with a result.
func (tm *TaskManager) CompleteTask(id, result string) error {
	task, err := tm.GetTask(id)
	if err != nil {
		return err
	}

	now := time.Now()
	task.Status = "completed"
	task.Result = result
	task.CompletedAt = &now

	return tm.UpdateTask(task)
}

// FailTask marks a task as failed with an error message.
func (tm *TaskManager) FailTask(id, errMsg string) error {
	task, err := tm.GetTask(id)
	if err != nil {
		return err
	}

	now := time.Now()
	task.Status = "failed"
	task.Error = errMsg
	task.CompletedAt = &now

	return tm.UpdateTask(task)
}

// ClaimTask marks a task as running by a specific tool.
func (tm *TaskManager) ClaimTask(id, toolName string) error {
	task, err := tm.GetTask(id)
	if err != nil {
		return err
	}

	now := time.Now()
	task.Status = "running"
	task.Tool = toolName
	task.StartedAt = &now

	return tm.UpdateTask(task)
}

// WatchForCompletion polls for task completion.
func (tm *TaskManager) WatchForCompletion(id string, timeout time.Duration) (*Task, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		task, err := tm.GetTask(id)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if task.Status == "completed" || task.Status == "failed" {
			return task, nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("timeout waiting for task %s", id)
}
