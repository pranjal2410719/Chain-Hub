package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_LoadSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "chainhub-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.ChainHub.LogLevel = "debug"
	cfg.ChainHub.Workspace = filepath.Join(tmpDir, "workspace")

	cfgPath := filepath.Join(tmpDir, "config.yaml")

	// Save.
	if err := SaveConfig(cfg, cfgPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Load.
	loaded, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.ChainHub.LogLevel != "debug" {
		t.Errorf("expected LogLevel 'debug', got %s", loaded.ChainHub.LogLevel)
	}

	if loaded.ChainHub.Workspace != cfg.ChainHub.Workspace {
		t.Errorf("expected workspace %s, got %s", cfg.ChainHub.Workspace, loaded.ChainHub.Workspace)
	}

	// EnsureWorkspace.
	if err := EnsureWorkspace(loaded); err != nil {
		t.Fatalf("failed to ensure workspace: %v", err)
	}

	subdirs := []string{"reports", "logs", "context"}
	for _, d := range subdirs {
		path := filepath.Join(loaded.ChainHub.Workspace, d)
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Errorf("expected workspace directory %s to exist, got error: %v", path, err)
		}
	}
}
