package graph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/panex-dev/panex/internal/inspector"
)

func TestBuildFromInspection(t *testing.T) {
	report := &inspector.Report{
		PackageManager: &inspector.Finding[string]{Value: "pnpm", Confidence: 1.0},
		WorkspaceType:  &inspector.Finding[string]{Value: "standalone", Confidence: 1.0},
		Framework:      &inspector.Finding[string]{Value: "react", Confidence: 0.9},
		Bundler:        &inspector.Finding[string]{Value: "esbuild", Confidence: 0.95},
		Language:       &inspector.Finding[string]{Value: "typescript", Confidence: 1.0},
		Entrypoints: map[string]inspector.EntryCandidate{
			"background": {Path: "background.js", Type: "service-worker", Source: "detection"},
		},
		Targets: []inspector.Finding[string]{{Value: "chrome", Confidence: 1.0}},
	}

	b := NewBuilder("/project")
	g, err := b.BuildFromInspection(report)
	if err != nil {
		t.Fatalf("BuildFromInspection: %v", err)
	}

	if g.PackageManager != "pnpm" {
		t.Errorf("pkg manager: got %s", g.PackageManager)
	}
	if g.Framework.Name != "react" {
		t.Errorf("framework: got %s", g.Framework.Name)
	}
	if len(g.Entries) != 1 {
		t.Errorf("entries: got %d", len(g.Entries))
	}
	if len(g.TargetsRequested) != 1 || g.TargetsRequested[0] != "chrome" {
		t.Errorf("targets: got %v", g.TargetsRequested)
	}
	if g.GraphHash == "" {
		t.Error("expected graph hash")
	}
}

func TestBuildFromConfig_OverridesInspection(t *testing.T) {
	report := &inspector.Report{
		Framework: &inspector.Finding[string]{Value: "react", Confidence: 0.9},
		Entrypoints: map[string]inspector.EntryCandidate{
			"background": {Path: "background.js", Type: "service-worker"},
		},
		Targets: []inspector.Finding[string]{{Value: "chrome", Confidence: 1.0}},
	}

	config := &ProjectConfig{
		Project: ProjectConfigBlock{Name: "tab-organizer", ID: "acme.tab-organizer", Version: "1.2.3"},
		Entries: map[string]EntryConfig{
			"background": {Path: "src/background/index.ts", Type: "service-worker"},
			"popup":      {Path: "src/popup.html"},
		},
		Targets: []string{"chrome"},
		Hash:    "sha256:abc123",
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
	if g.Project.Version != "1.2.3" {
		t.Errorf("version: got %s, want 1.2.3", g.Project.Version)
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
	if len(g.TargetsResolved) != 1 || g.TargetsResolved[0] != "chrome" {
		t.Errorf("targets: got %v, want [chrome]", g.TargetsResolved)
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

	h1, _ := g1.ComputeHash()
	h2, _ := g2.ComputeHash()

	if h1 != h2 {
		t.Errorf("hashes not deterministic: %s != %s", h1, h2)
	}
}

func TestGraphHash_ChangesOnMutation(t *testing.T) {
	report := &inspector.Report{
		Targets: []inspector.Finding[string]{{Value: "chrome", Confidence: 1.0}},
	}
	b := NewBuilder("/project")
	g1, _ := b.BuildFromInspection(report)
	h1, _ := g1.ComputeHash()

	g1.PackageManager = "yarn"
	h2, _ := g1.ComputeHash()

	if h1 == h2 {
		t.Error("graph hash should change when content changes")
	}
}

func TestGraphHash_Portability(t *testing.T) {
	g1 := &Graph{
		SchemaVersion:  1,
		Project:        ProjectIdentity{ID: "test", Name: "test"},
		SourceRoot:     "/absolute/path/one",
		PackageManager: "pnpm",
	}
	g2 := &Graph{
		SchemaVersion:  1,
		Project:        ProjectIdentity{ID: "test", Name: "test"},
		SourceRoot:     "/absolute/path/two",
		PackageManager: "pnpm",
	}

	h1, _ := g1.ComputeHash()
	h2, _ := g2.ComputeHash()

	if h1 != h2 {
		t.Errorf("graph hashes should be portable (SourceRoot agnostic): %s != %s", h1, h2)
	}
}

func TestTargetResolution(t *testing.T) {
	b := NewBuilder("/tmp")
	report := &inspector.Report{
		Targets: []inspector.Finding[string]{
			{Value: "chrome", Confidence: 1.0},
			{Value: "netscape-navigator", Confidence: 0.5},
		},
	}

	g, _ := b.BuildFromInspection(report)

	// chrome should be resolved, netscape should not (no adapter)
	foundChrome := false
	foundNetscape := false
	for _, tgt := range g.TargetsResolved {
		if tgt == "chrome" {
			foundChrome = true
		}
		if tgt == "netscape-navigator" {
			foundNetscape = true
		}
	}

	if !foundChrome {
		t.Error("expected chrome to be resolved")
	}
	if foundNetscape {
		t.Error("expected netscape-navigator NOT to be resolved")
	}
	if len(g.TargetsRequested) != 2 {
		t.Errorf("expected 2 requested targets, got %d", len(g.TargetsRequested))
	}
}

func TestWriteAndReadGraph(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.json")

	g := &Graph{
		SchemaVersion: 1,
		Project:       ProjectIdentity{ID: "test", Name: "test", Version: "1.0.0"},
		Entries: map[string]Entry{
			"bg": {Path: "bg.js", Type: "service-worker"},
		},
	}

	if err := WriteToFile(g, path); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := ReadFromFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if loaded.Project.ID != g.Project.ID {
		t.Errorf("project ID: got %s, want %s", loaded.Project.ID, g.Project.ID)
	}
}

func TestLoadProjectConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "panex.config.json")
	content := `{
		"project": { "name": "my-ext", "id": "my-ext", "version": "1.0.0" },
		"targets": ["chrome", "firefox"]
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadProjectConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
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
