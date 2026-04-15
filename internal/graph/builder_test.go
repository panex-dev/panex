package graph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panex-dev/panex/internal/inspector"
)

func TestBuildFromInspection(t *testing.T) {
	report := &inspector.Report{
		PackageManager: &inspector.Finding[string]{Value: "pnpm", Confidence: 0.99},
		Framework:      &inspector.Finding[string]{Value: "react", Confidence: 0.95},
		Bundler:        &inspector.Finding[string]{Value: "vite", Confidence: 0.98},
		Language:       &inspector.Finding[string]{Value: "typescript", Confidence: 0.99},
		WorkspaceType:  &inspector.Finding[string]{Value: "single-package", Confidence: 0.8},
		Entrypoints: map[string]inspector.EntryCandidate{
			"background": {Path: "src/background/index.ts", Type: "service-worker", Source: "manifest.json"},
			"popup":      {Path: "src/popup/main.tsx", Type: "html-app", Source: "manifest.json"},
		},
		Targets: []inspector.Finding[string]{
			{Value: "chrome", Confidence: 0.95},
		},
	}

	b := NewBuilder("/project")
	g, err := b.BuildFromInspection(report)
	if err != nil {
		t.Fatalf("BuildFromInspection: %v", err)
	}

	if g.SchemaVersion != 1 {
		t.Errorf("schema_version: got %d, want 1", g.SchemaVersion)
	}
	if g.PackageManager != "pnpm" {
		t.Errorf("package_manager: got %s, want pnpm", g.PackageManager)
	}
	if g.Framework.Name != "react" {
		t.Errorf("framework: got %s, want react", g.Framework.Name)
	}
	if g.Bundler.Name != "vite" {
		t.Errorf("bundler: got %s, want vite", g.Bundler.Name)
	}
	if g.Language.Name != "typescript" {
		t.Errorf("language: got %s, want typescript", g.Language.Name)
	}

	if len(g.Entries) != 2 {
		t.Errorf("entries: got %d, want 2", len(g.Entries))
	}
	if bg, ok := g.Entries["background"]; !ok || bg.Path != "src/background/index.ts" {
		t.Error("expected background entry")
	}

	if len(g.TargetsResolved) != 1 || g.TargetsResolved[0] != "chrome" {
		t.Errorf("targets: got %v, want [chrome]", g.TargetsResolved)
	}

	if g.GraphHash == "" || !strings.HasPrefix(g.GraphHash, "sha256:") {
		t.Errorf("expected graph hash, got %q", g.GraphHash)
	}
}

func TestBuildFromConfig_OverridesInspection(t *testing.T) {
	report := &inspector.Report{
		PackageManager: &inspector.Finding[string]{Value: "npm", Confidence: 0.5},
		Entrypoints: map[string]inspector.EntryCandidate{
			"background": {Path: "bg.js", Type: "service-worker", Source: "convention"},
		},
		Targets: []inspector.Finding[string]{
			{Value: "chrome", Confidence: 0.5},
		},
	}

	config := &ProjectConfig{
		Project: ProjectConfigBlock{Name: "tab-organizer", ID: "acme.tab-organizer"},
		Entries: map[string]EntryConfig{
			"background": {Path: "src/background/index.ts", Type: "service-worker"},
			"popup":      {Path: "src/popup/index.html", Type: "html-page"},
		},
		Targets:      []string{"chrome", "firefox"},
		Capabilities: map[string]any{"tabs": "read-write"},
		Hash:         "sha256:abc123",
	}

	b := NewBuilder("/project")
	g, err := b.BuildFromConfig(config, report)
	if err != nil {
		t.Fatalf("BuildFromConfig: %v", err)
	}

	// Config overrides
	if g.Project.Name != "tab-organizer" {
		t.Errorf("project name: got %s, want tab-organizer", g.Project.Name)
	}
	if g.Project.ID != "acme.tab-organizer" {
		t.Errorf("project id: got %s, want acme.tab-organizer", g.Project.ID)
	}

	// Config entries override detected
	bg := g.Entries["background"]
	if bg.Path != "src/background/index.ts" {
		t.Errorf("config entry should override: got %s", bg.Path)
	}
	if bg.Source != "config" {
		t.Errorf("source should be config: got %s", bg.Source)
	}

	// Config adds popup that inspection didn't find
	if _, ok := g.Entries["popup"]; !ok {
		t.Error("expected popup from config")
	}

	// Config targets override
	if len(g.TargetsResolved) != 2 {
		t.Errorf("targets: got %v, want [chrome, firefox]", g.TargetsResolved)
	}

	if g.ConfigHash != "sha256:abc123" {
		t.Errorf("config hash: got %s", g.ConfigHash)
	}
}

func TestGraphHash_Deterministic(t *testing.T) {
	report := &inspector.Report{
		PackageManager: &inspector.Finding[string]{Value: "npm", Confidence: 0.99},
		Entrypoints:    map[string]inspector.EntryCandidate{},
		Targets:        []inspector.Finding[string]{{Value: "chrome", Confidence: 0.95}},
	}

	b := NewBuilder("/project")
	g1, _ := b.BuildFromInspection(report)
	g2, _ := b.BuildFromInspection(report)

	if g1.GraphHash != g2.GraphHash {
		t.Errorf("graph hashes should be deterministic: %s != %s", g1.GraphHash, g2.GraphHash)
	}
}

func TestGraphHash_ChangesOnMutation(t *testing.T) {
	report := &inspector.Report{
		PackageManager: &inspector.Finding[string]{Value: "npm", Confidence: 0.99},
		Entrypoints:    map[string]inspector.EntryCandidate{},
		Targets:        []inspector.Finding[string]{{Value: "chrome", Confidence: 0.95}},
	}

	b := NewBuilder("/project")
	g1, _ := b.BuildFromInspection(report)
	hash1 := g1.GraphHash

	// Mutate the graph
	g1.TargetsResolved = append(g1.TargetsResolved, "firefox")
	hash2, _ := g1.ComputeHash()

	if hash1 == hash2 {
		t.Error("graph hash should change when content changes")
	}
}

func TestWriteAndReadGraph(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.json")

	g := &Graph{
		SchemaVersion:   1,
		Project:         ProjectIdentity{ID: "acme.test", Name: "test"},
		SourceRoot:      "/project",
		PackageManager:  "npm",
		Language:        DetectedFact{Name: "typescript", Confidence: 0.99},
		Entries:         map[string]Entry{"background": {Path: "bg.ts", Type: "service-worker"}},
		TargetsResolved: []string{"chrome"},
		Capabilities:    map[string]any{},
		Dependencies:    map[string]string{},
		StateDir:        ".panex",
		GraphHash:       "sha256:abc",
	}

	if err := WriteToFile(g, path); err != nil {
		t.Fatalf("WriteToFile: %v", err)
	}

	got, err := ReadFromFile(path)
	if err != nil {
		t.Fatalf("ReadFromFile: %v", err)
	}

	if got.Project.ID != "acme.test" {
		t.Errorf("project.id: got %s, want acme.test", got.Project.ID)
	}
	if got.PackageManager != "npm" {
		t.Errorf("package_manager: got %s, want npm", got.PackageManager)
	}
	if len(got.Entries) != 1 {
		t.Errorf("entries: got %d, want 1", len(got.Entries))
	}
}

func TestLoadProjectConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "panex.config.json")

	content := `{
  "project": {"name": "my-ext", "id": "dev.my-ext"},
  "entries": {
    "background": {"path": "src/bg.ts", "type": "service-worker"}
  },
  "targets": ["chrome", "firefox"],
  "capabilities": {"tabs": "read"}
}`
	os.WriteFile(path, []byte(content), 0o644)

	cfg, err := LoadProjectConfig(path)
	if err != nil {
		t.Fatalf("LoadProjectConfig: %v", err)
	}

	if cfg.Project.Name != "my-ext" {
		t.Errorf("project.name: got %s", cfg.Project.Name)
	}
	if len(cfg.Targets) != 2 {
		t.Errorf("targets: got %d, want 2", len(cfg.Targets))
	}
	if cfg.Hash == "" {
		t.Error("expected config hash")
	}
}
