package fsmodel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewRoot(t *testing.T) {
	dir := t.TempDir()
	root, err := NewRoot(dir)
	if err != nil {
		t.Fatalf("NewRoot: %v", err)
	}
	if root.ProjectDir != dir {
		t.Fatalf("expected project dir %s, got %s", dir, root.ProjectDir)
	}
}

func TestNewRoot_NotADirectory(t *testing.T) {
	f, err := os.CreateTemp("", "panex-test-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	_, err = NewRoot(f.Name())
	if err == nil {
		t.Fatal("expected error for non-directory path")
	}
}

func TestNewRoot_DoesNotExist(t *testing.T) {
	_, err := NewRoot("/nonexistent/path/that/should/not/exist")
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestInit_CreatesDirectoryStructure(t *testing.T) {
	dir := t.TempDir()
	root, _ := NewRoot(dir)

	if root.IsInitialized() {
		t.Fatal("should not be initialized before Init()")
	}

	if err := root.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !root.IsInitialized() {
		t.Fatal("should be initialized after Init()")
	}

	// Check all expected directories exist
	expectedDirs := []string{
		root.StateRoot(),
		root.RunsRoot(),
		root.SessionsRoot(),
		root.ReportsRoot(),
		root.CacheRoot(),
		filepath.Join(root.CacheRoot(), "downloads"),
		filepath.Join(root.CacheRoot(), "package-manager"),
		filepath.Join(root.CacheRoot(), "browser-profiles"),
		filepath.Join(root.CacheRoot(), "launch-artifacts"),
		root.ArtifactsRoot(),
		root.LocksRoot(),
	}
	for _, d := range expectedDirs {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", d, err)
		} else if !info.IsDir() {
			t.Errorf("expected %s to be a directory", d)
		}
	}

	// Check state.json was created
	if _, err := os.Stat(root.StatePath()); err != nil {
		t.Errorf("expected state.json: %v", err)
	}
}

func TestInit_Idempotent(t *testing.T) {
	dir := t.TempDir()
	root, _ := NewRoot(dir)

	if err := root.Init(); err != nil {
		t.Fatalf("first Init: %v", err)
	}

	// Read state after first init
	s1, err := root.ReadState()
	if err != nil {
		t.Fatalf("ReadState after first init: %v", err)
	}

	// Second init should not overwrite state.json
	if err := root.Init(); err != nil {
		t.Fatalf("second Init: %v", err)
	}

	s2, err := root.ReadState()
	if err != nil {
		t.Fatalf("ReadState after second init: %v", err)
	}

	if s1.InitializedAt != s2.InitializedAt {
		t.Errorf("state.json was overwritten: %s != %s", s1.InitializedAt, s2.InitializedAt)
	}
}

func TestReadWriteState(t *testing.T) {
	dir := t.TempDir()
	root, _ := NewRoot(dir)
	_ = root.Init()

	s := State{
		SchemaVersion:  1,
		InitializedAt:  "2026-03-28T12:00:00Z",
		LatestRunID:    "run_001",
		LatestReportAt: "2026-03-28T12:05:00Z",
		ActiveSession:  "ses_001",
	}
	if err := root.WriteState(s); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	got, err := root.ReadState()
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}

	if got.LatestRunID != "run_001" {
		t.Errorf("expected latest_run_id=run_001, got %s", got.LatestRunID)
	}
	if got.ActiveSession != "ses_001" {
		t.Errorf("expected active_session=ses_001, got %s", got.ActiveSession)
	}
}

func TestPathAccessors(t *testing.T) {
	root := &Root{ProjectDir: "/project"}

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"StateRoot", root.StateRoot(), "/project/.panex"},
		{"ConfigFilePath", root.ConfigFilePath(), "/project/panex.config.ts"},
		{"PolicyFilePath", root.PolicyFilePath(), "/project/panex.policy.yaml"},
		{"StatePath", root.StatePath(), "/project/.panex/state.json"},
		{"ConfigLockPath", root.ConfigLockPath(), "/project/.panex/config.lock.json"},
		{"ProjectGraphPath", root.ProjectGraphPath(), "/project/.panex/project.graph.json"},
		{"EnvironmentPath", root.EnvironmentPath(), "/project/.panex/environment.json"},
		{"RunDir", root.RunDir("run_001"), "/project/.panex/runs/run_001"},
		{"SessionDir", root.SessionDir("ses_001"), "/project/.panex/sessions/ses_001"},
		{"ArtifactDir", root.ArtifactDir("chrome"), "/project/.panex/artifacts/chrome"},
		{"LockPath", root.LockPath("project.lock"), "/project/.panex/locks/project.lock"},
		{"RunManifestDir", root.RunManifestDir("run_001", "chrome"), "/project/.panex/runs/run_001/generated/manifests/chrome"},
		{"RunTracePath", root.RunTracePath("run_001"), "/project/.panex/runs/run_001/trace/events.jsonl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("got %s, want %s", tt.got, tt.expected)
			}
		})
	}
}
