package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCmdInspect_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	// Capture output by redirecting stdout
	code := captureExitCode(func() int { return CmdInspect(dir) })
	if code != ExitSuccess {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestCmdInspect_WithProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"test","dependencies":{"react":"^18"}}`)
	writeFile(t, filepath.Join(dir, "tsconfig.json"), `{}`)
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "lockfileVersion: 9")

	code := captureExitCode(func() int { return CmdInspect(dir) })
	if code != ExitSuccess {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestCmdInit_NewProject(t *testing.T) {
	dir := t.TempDir()

	code := captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test-ext", Targets: []string{"chrome"}})
	})
	if code != ExitSuccess {
		t.Errorf("expected exit 0, got %d", code)
	}

	// Verify .panex/ was created
	if _, err := os.Stat(filepath.Join(dir, ".panex", "state.json")); err != nil {
		t.Error("expected state.json")
	}
	if _, err := os.Stat(filepath.Join(dir, ".panex", "project.graph.json")); err != nil {
		t.Error("expected project.graph.json")
	}
}

func TestCmdInit_AlreadyInitialized(t *testing.T) {
	dir := t.TempDir()

	// First init
	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test"})
	})

	// Second init should be no-op
	code := captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test"})
	})
	if code != ExitSuccess {
		t.Errorf("expected exit 0 for already initialized, got %d", code)
	}
}

func TestCmdDoctor_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	code := captureExitCode(func() int { return CmdDoctor(dir, false) })
	// Should succeed even with issues (reports them, doesn't fail)
	if code != ExitOperationalFail && code != ExitSuccess {
		t.Errorf("unexpected exit code: %d", code)
	}
}

func TestCmdDoctor_Fix(t *testing.T) {
	dir := t.TempDir()

	code := captureExitCode(func() int { return CmdDoctor(dir, true) })
	_ = code

	// Verify .panex was created by fix
	if _, err := os.Stat(filepath.Join(dir, ".panex")); err != nil {
		t.Error("expected .panex/ after doctor --fix")
	}
}

func TestCmdVerify_InitializedProject(t *testing.T) {
	dir := t.TempDir()

	// Init first
	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test", Targets: []string{"chrome"}})
	})

	code := captureExitCode(func() int { return CmdVerify(dir) })
	// May pass or fail depending on entrypoints — just verify no crash
	if code == ExitInternalFault {
		t.Error("internal fault during verify")
	}
}

func TestCmdPackage(t *testing.T) {
	dir := t.TempDir()

	// Create a buildable project
	writeFile(t, filepath.Join(dir, "manifest.json"), `{"manifest_version":3,"name":"Test","version":"1.0.0"}`)
	writeFile(t, filepath.Join(dir, "background.js"), "// background")
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"test"}`)

	// Init
	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test", Targets: []string{"chrome"}})
	})

	// Package
	code := captureExitCode(func() int {
		return CmdPackage(dir, PackageOptions{SourceDir: dir, Version: "1.0.0"})
	})
	if code != ExitSuccess {
		t.Errorf("expected exit 0, got %d", code)
	}

	// Verify artifact was created
	artifactDir := filepath.Join(dir, ".panex", "artifacts", "chrome")
	entries, _ := os.ReadDir(artifactDir)
	if len(entries) == 0 {
		t.Error("expected artifact in .panex/artifacts/chrome/")
	}

	// Verify run was recorded
	runsDir := filepath.Join(dir, ".panex", "runs")
	runEntries, _ := os.ReadDir(runsDir)
	if len(runEntries) == 0 {
		t.Error("expected run record in .panex/runs/")
	}
}

func TestOutput_JSON(t *testing.T) {
	out := Output{
		Status:  "ok",
		Command: "inspect",
		Summary: "test summary",
		Data:    map[string]string{"key": "value"},
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]any
	if json.Unmarshal(data, &parsed) != nil {
		t.Fatal("output should be valid JSON")
	}
	if parsed["status"] != "ok" {
		t.Errorf("status: got %v", parsed["status"])
	}
}

// --- helpers ---

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(content), 0o644)
}

// captureExitCode runs a function that returns an exit code,
// redirecting stdout to discard during test.
func captureExitCode(fn func() int) int {
	// Save and redirect stdout
	old := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = old }()
	return fn()
}
