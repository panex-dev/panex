package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	r := Run(Options{ProjectDir: dir})

	if r.Status != "issues_found" {
		t.Errorf("expected issues_found for empty dir, got %s", r.Status)
	}

	codes := diagnosisCodes(r)
	if !codes["PANEX_NOT_INITIALIZED"] {
		t.Error("expected PANEX_NOT_INITIALIZED")
	}
	if !codes["MANIFEST_NOT_FOUND"] {
		t.Error("expected MANIFEST_NOT_FOUND")
	}
}

func TestRun_HealthyProject(t *testing.T) {
	dir := setupHealthyProject(t)
	r := Run(Options{ProjectDir: dir})

	if r.Status != "healthy" {
		t.Errorf("expected healthy, got %s", r.Status)
		for _, d := range r.Diagnoses {
			t.Logf("  %s: %s", d.Code, d.Message)
		}
	}
}

func TestRun_InvalidManifest(t *testing.T) {
	dir := t.TempDir()
	setupPanexDir(t, dir)
	os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{not json}"), 0o644)

	r := Run(Options{ProjectDir: dir})

	codes := diagnosisCodes(r)
	if !codes["MANIFEST_INVALID_JSON"] {
		t.Error("expected MANIFEST_INVALID_JSON")
	}
}

func TestRun_MissingDependencies(t *testing.T) {
	dir := t.TempDir()
	setupPanexDir(t, dir)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644)
	// No node_modules/

	r := Run(Options{ProjectDir: dir})

	codes := diagnosisCodes(r)
	if !codes["DEPENDENCIES_NOT_INSTALLED"] {
		t.Error("expected DEPENDENCIES_NOT_INSTALLED")
	}
}

func TestRun_CorruptState(t *testing.T) {
	dir := t.TempDir()
	panexDir := filepath.Join(dir, ".panex")
	os.MkdirAll(panexDir, 0o755)
	os.WriteFile(filepath.Join(panexDir, "state.json"), []byte("not json"), 0o644)

	r := Run(Options{ProjectDir: dir})

	codes := diagnosisCodes(r)
	if !codes["STATE_CORRUPT"] {
		t.Error("expected STATE_CORRUPT")
	}
}

func TestRun_StaleLock(t *testing.T) {
	dir := t.TempDir()
	setupPanexDir(t, dir)
	locksDir := filepath.Join(dir, ".panex", "locks")
	os.MkdirAll(locksDir, 0o755)
	os.WriteFile(filepath.Join(locksDir, "project.lock"), []byte("pid:1234"), 0o644)

	r := Run(Options{ProjectDir: dir})

	codes := diagnosisCodes(r)
	if !codes["STALE_LOCK"] {
		t.Error("expected STALE_LOCK")
	}
}

func TestRun_Fix_InitStateDir(t *testing.T) {
	dir := t.TempDir()
	r := Run(Options{ProjectDir: dir, Fix: true})

	repaired := make(map[string]bool)
	for _, rep := range r.Repaired {
		repaired[rep] = true
	}
	if !repaired["init_state_dir"] {
		t.Error("expected init_state_dir repair")
	}

	// Verify .panex was created
	if _, err := os.Stat(filepath.Join(dir, ".panex", "state.json")); err != nil {
		t.Error("expected state.json after repair")
	}
}

func TestRun_Fix_RemoveStaleLock(t *testing.T) {
	dir := t.TempDir()
	setupPanexDir(t, dir)
	locksDir := filepath.Join(dir, ".panex", "locks")
	os.MkdirAll(locksDir, 0o755)
	lockPath := filepath.Join(locksDir, "project.lock")
	os.WriteFile(lockPath, []byte("pid:1234"), 0o644)

	r := Run(Options{ProjectDir: dir, Fix: true})

	repaired := make(map[string]bool)
	for _, rep := range r.Repaired {
		repaired[rep] = true
	}
	if !repaired["remove_stale_lock"] {
		t.Error("expected remove_stale_lock repair")
	}

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file should have been removed")
	}
}

func TestRun_Fix_ResetState(t *testing.T) {
	dir := t.TempDir()
	panexDir := filepath.Join(dir, ".panex")
	os.MkdirAll(panexDir, 0o755)
	os.WriteFile(filepath.Join(panexDir, "state.json"), []byte("corrupt"), 0o644)

	r := Run(Options{ProjectDir: dir, Fix: true})

	repaired := make(map[string]bool)
	for _, rep := range r.Repaired {
		repaired[rep] = true
	}
	if !repaired["reset_state"] {
		t.Error("expected reset_state repair")
	}

	// Verify state.json is now valid
	data, _ := os.ReadFile(filepath.Join(panexDir, "state.json"))
	var state map[string]any
	if json.Unmarshal(data, &state) != nil {
		t.Error("state.json should be valid JSON after repair")
	}
}

// --- helpers ---

func setupPanexDir(t *testing.T, dir string) {
	t.Helper()
	panexDir := filepath.Join(dir, ".panex")
	for _, d := range []string{panexDir, filepath.Join(panexDir, "runs"), filepath.Join(panexDir, "sessions"), filepath.Join(panexDir, "reports"), filepath.Join(panexDir, "cache"), filepath.Join(panexDir, "artifacts"), filepath.Join(panexDir, "locks")} {
		os.MkdirAll(d, 0o755)
	}
	state := map[string]any{"schema_version": 1}
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(filepath.Join(panexDir, "state.json"), data, 0o644)
}

func setupHealthyProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	setupPanexDir(t, dir)

	manifest := map[string]any{"manifest_version": 3, "name": "Test", "version": "1.0.0"}
	data, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644)

	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644)
	os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755)

	return dir
}

func diagnosisCodes(r *Report) map[string]bool {
	codes := make(map[string]bool)
	for _, d := range r.Diagnoses {
		codes[d.Code] = true
	}
	return codes
}
