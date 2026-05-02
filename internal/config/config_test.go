package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Role != "dev" {
		t.Errorf("expected dev role, got %q", cfg.Role)
	}
	if cfg.MaxTurns != 50 {
		t.Errorf("expected 50 max turns, got %d", cfg.MaxTurns)
	}
	if cfg.Permission != "workspace_write" {
		t.Errorf("expected workspace_write, got %q", cfg.Permission)
	}
	if cfg.Workspace == "" {
		t.Error("expected non-empty workspace")
	}
}

func TestLoadDroneMD(t *testing.T) {
	// No DRONE.md → empty
	result := LoadDroneMD(t.TempDir())
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}

	// Create DRONE.md
	dir := t.TempDir()
	content := "# Project Knowledge\nThis is a test project."
	os.WriteFile(filepath.Join(dir, "DRONE.md"), []byte(content), 0o644)

	result = LoadDroneMD(dir)
	if result != content {
		t.Errorf("expected %q, got %q", content, result)
	}
}

func TestLoadDroneMDFromSubdir(t *testing.T) {
	dir := t.TempDir()
	droneDir := filepath.Join(dir, ".drone")
	os.MkdirAll(droneDir, 0o755)
	content := "# From .drone subdir"
	os.WriteFile(filepath.Join(droneDir, "DRONE.md"), []byte(content), 0o644)

	result := LoadDroneMD(dir)
	if result != content {
		t.Errorf("expected %q, got %q", content, result)
	}
}

func TestLoadDroneSkills(t *testing.T) {
	// No skills → nil
	result := LoadDroneSkills(t.TempDir())
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}

	// Create skills
	dir := t.TempDir()
	skillDir := filepath.Join(dir, ".drone", "skills")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "deploy.md"), []byte("# Deploy skill"), 0o644)
	os.WriteFile(filepath.Join(skillDir, "test.md"), []byte("# Test skill"), 0o644)
	os.WriteFile(filepath.Join(skillDir, "not-a-skill.txt"), []byte("ignored"), 0o644)

	result = LoadDroneSkills(dir)
	if len(result) != 2 {
		t.Errorf("expected 2 skills, got %d", len(result))
	}
}
