package configloader

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoad_NoConfig(t *testing.T) {
	dir := t.TempDir()
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil loaded for missing config")
	}
}

func TestLoad_JSONConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Project: ProjectConfig{Name: "test-ext", ID: "com.test.ext"},
		Targets: TargetConfigMap{
			"chrome": {Enabled: true},
		},
		Capabilities: map[string]any{"tabs": true, "storage": true},
		Entries: map[string]EntryConfig{
			"background": {Path: "src/background.ts", ModuleType: "esm"},
			"popup":      {Path: "src/popup.html"},
		},
	}
	writeConfig(t, dir, &cfg)

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil || loaded.Config == nil {
		t.Fatal("expected loaded config")
		return
	}
	loadedCfg := loaded.Config
	if loadedCfg.Project.Name != "test-ext" {
		t.Errorf("name: got %s", loadedCfg.Project.Name)
	}
	if loadedCfg.Project.ID != "com.test.ext" {
		t.Errorf("id: got %s", loadedCfg.Project.ID)
	}
	if !strings.HasPrefix(loaded.ConfigHash, "sha256:") {
		t.Errorf("hash: got %s", loaded.ConfigHash)
	}
}

func TestLoad_TypeScriptConfig(t *testing.T) {
	requireNode(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "panex.config.ts"), []byte(`
type Targets = Record<string, { enabled: boolean }>;

const targets: Targets = {
  chrome: { enabled: true },
};

export default {
  project: { name: "ts-ext", id: "com.test.ts" },
  targets,
  capabilities: { tabs: true },
};
`), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil || loaded.Config == nil {
		t.Fatal("expected loaded config")
	}
	if filepath.Base(loaded.SourcePath) != TypeScriptConfigFileName {
		t.Fatalf("source path: got %q", loaded.SourcePath)
	}
	if loaded.Config.Project.Name != "ts-ext" {
		t.Fatalf("project name: got %q", loaded.Config.Project.Name)
	}
	if !loaded.Config.Targets["chrome"].Enabled {
		t.Fatal("expected chrome target to be enabled")
	}
}

func TestLoad_TypeScriptConfigWithImportedHelper(t *testing.T) {
	requireNode(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helper.ts"), []byte(`
export const config = {
  project: { name: "imported-ext", id: "com.test.imported" },
  targets: {
    chrome: { enabled: true },
    firefox: { enabled: true },
  },
};
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "panex.config.ts"), []byte(`
import { config } from "./helper";

export default config;
`), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Config.Project.ID != "com.test.imported" {
		t.Fatalf("project id: got %q", loaded.Config.Project.ID)
	}
	if !loaded.Config.Targets["firefox"].Enabled {
		t.Fatal("expected firefox target from imported helper")
	}
}

func TestLoad_TypeScriptConfigPreferredOverJSON(t *testing.T) {
	requireNode(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "panex.config.ts"), []byte(`
export default {
  project: { name: "ts-wins", id: "ts-wins" },
  targets: { chrome: { enabled: true } },
};
`), 0o644); err != nil {
		t.Fatal(err)
	}

	writeConfigFile(t, filepath.Join(dir, JSONConfigFileName), &Config{
		Project: ProjectConfig{Name: "json-loses", ID: "json-loses"},
	})

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Config.Project.Name != "ts-wins" {
		t.Fatalf("expected ts config to win, got %q", loaded.Config.Project.Name)
	}
	if filepath.Base(loaded.SourcePath) != TypeScriptConfigFileName {
		t.Fatalf("source path: got %q", loaded.SourcePath)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "panex.config.json"), []byte("{not json}"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoad_InvalidTypeScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, TypeScriptConfigFileName), []byte("export default {"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid TypeScript")
	}
	if !strings.Contains(err.Error(), "transpile config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_TypeScriptConfigWithoutNode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, TypeScriptConfigFileName), []byte(`export default { project: { name: "x", id: "x" } }`), 0o644); err != nil {
		t.Fatal(err)
	}

	restore := stubExecLookPath(func(string) (string, error) {
		return "", exec.ErrNotFound
	})
	defer restore()

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected node lookup error")
	}
	if !strings.Contains(err.Error(), "node binary not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvaluateBundledConfigTimeout(t *testing.T) {
	restoreExec := stubCommandExec(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestEvaluateBundledConfigTimeoutHelperProcess", "--")
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	})
	defer restoreExec()

	restoreTimeout := stubTypeScriptConfigEvalTimeout(10 * time.Millisecond)
	defer restoreTimeout()

	_, err := evaluateBundledConfig("node", "ignored")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out after 10ms") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.json")
	cfg := Config{Project: ProjectConfig{Name: "custom"}}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil || loaded.Config == nil {
		t.Fatal("expected loaded config")
		return
	}
	if loaded.Config.Project.Name != "custom" {
		t.Errorf("name: got %s", loaded.Config.Project.Name)
	}
}

func TestDefault(t *testing.T) {
	cfg := Default("my-ext")
	if cfg.Project.Name != "my-ext" {
		t.Errorf("name: got %s", cfg.Project.Name)
	}
	if cfg.Project.ID != "my-ext" {
		t.Errorf("id: got %s", cfg.Project.ID)
	}
	tc, ok := cfg.Targets["chrome"]
	if !ok || !tc.Enabled {
		t.Error("chrome target should be enabled by default")
	}
}

func TestWriteToFile(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		Project: ProjectConfig{Name: "write-test", ID: "write-test"},
		Entries: map[string]EntryConfig{
			"background": {Path: "bg.ts"},
		},
	}
	path := filepath.Join(dir, JSONConfigFileName)
	if err := WriteToFile(cfg, path); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read back
	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if loaded == nil || loaded.Config == nil {
		t.Fatal("expected loaded config")
		return
	}
	if loaded.Config.Project.Name != "write-test" {
		t.Errorf("name: got %s", loaded.Config.Project.Name)
	}
	if loaded.Config.Entries["background"].Path != "bg.ts" {
		t.Errorf("entry path: got %s", loaded.Config.Entries["background"].Path)
	}
}

func TestConfigHashStable(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Project: ProjectConfig{Name: "hash-test"}}
	writeConfig(t, dir, &cfg)

	l1, _ := Load(dir)
	l2, _ := Load(dir)

	if l1.ConfigHash != l2.ConfigHash {
		t.Error("hash should be stable across loads")
	}
}

func TestConfigEntries(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Project: ProjectConfig{Name: "entries-test"},
		Entries: map[string]EntryConfig{
			"background":    {Path: "src/bg.ts", ModuleType: "esm"},
			"popup":         {Path: "src/popup.html"},
			"contentScript": {Path: "src/content.ts", Targets: []string{"chrome"}},
		},
	}
	writeConfig(t, dir, &cfg)

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Config.Entries) != 3 {
		t.Errorf("entries: got %d", len(loaded.Config.Entries))
	}
	if loaded.Config.Entries["contentScript"].Targets[0] != "chrome" {
		t.Errorf("content target: got %v", loaded.Config.Entries["contentScript"].Targets)
	}
}

func TestRuntimeConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Project: ProjectConfig{Name: "rt-test"},
		Runtime: RuntimeConfig{
			LogVerbosity:   "debug",
			TraceEnabled:   true,
			ReloadStrategy: "smart",
		},
	}
	writeConfig(t, dir, &cfg)

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Config.Runtime.LogVerbosity != "debug" {
		t.Errorf("log_verbosity: got %s", loaded.Config.Runtime.LogVerbosity)
	}
	if !loaded.Config.Runtime.TraceEnabled {
		t.Error("trace should be enabled")
	}
}

// --- helpers ---

func writeConfig(t *testing.T, dir string, cfg *Config) {
	t.Helper()
	writeConfigFile(t, filepath.Join(dir, JSONConfigFileName), cfg)
}

func writeConfigFile(t *testing.T, path string, cfg *Config) {
	t.Helper()
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireNode(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not found in PATH")
	}
}

func TestEvaluateBundledConfigTimeoutHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	time.Sleep(time.Second)
	os.Exit(0)
}

func stubExecLookPath(fn func(string) (string, error)) func() {
	original := execLookPath
	execLookPath = fn
	return func() {
		execLookPath = original
	}
}

func stubCommandExec(fn func(context.Context, string, ...string) *exec.Cmd) func() {
	original := commandExec
	commandExec = fn
	return func() {
		commandExec = original
	}
}

func stubTypeScriptConfigEvalTimeout(timeout time.Duration) func() {
	original := typeScriptConfigEvalTimeout
	typeScriptConfigEvalTimeout = timeout
	return func() {
		typeScriptConfigEvalTimeout = original
	}
}
