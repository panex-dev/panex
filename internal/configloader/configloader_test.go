package configloader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	path := filepath.Join(dir, "panex.config.json")
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
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "panex.config.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
