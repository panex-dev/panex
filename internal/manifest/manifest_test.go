package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panex-dev/panex/internal/capability"
	"github.com/panex-dev/panex/internal/graph"
	"github.com/panex-dev/panex/internal/target"
)

func TestCompile_Basic(t *testing.T) {
	g := &graph.Graph{
		Project:         graph.ProjectIdentity{Name: "test-ext"},
		TargetsResolved: []string{"chrome"},
		Entries: map[string]graph.Entry{
			"background": {Path: "background.js", Type: "service-worker"},
		},
	}
	matrix := &capability.TargetMatrix{
		Resolutions: []capability.Resolution{
			{Capability: "tabs", Target: "chrome", State: "native", Permissions: []string{"tabs"}},
			{Capability: "storage", Target: "chrome", State: "native", Permissions: []string{"storage"}},
		},
		Permissions: []string{"tabs", "storage"},
	}

	result := Compile(CompileInput{
		Graph:    g,
		Matrix:   matrix,
		Adapters: map[string]target.Adapter{"chrome": target.NewChrome()},
		Version:  "1.0.0",
	})

	if len(result.Errors) > 0 {
		t.Fatalf("errors: %v", result.Errors)
	}
	if len(result.Outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(result.Outputs))
	}

	out := result.Outputs[0]
	if out.Target != "chrome" {
		t.Errorf("target: got %s", out.Target)
	}
	if !strings.HasPrefix(out.ManifestHash, "sha256:") {
		t.Errorf("hash: got %s", out.ManifestHash)
	}

	m := out.Manifest
	if fmt.Sprintf("%v", m["manifest_version"]) != "3" {
		t.Errorf("manifest_version: got %v", m["manifest_version"])
	}
	if m["name"] != "test-ext" {
		t.Errorf("name: got %v", m["name"])
	}
}

func TestCompile_WithEntries(t *testing.T) {
	g := &graph.Graph{
		Project:         graph.ProjectIdentity{Name: "full-ext"},
		TargetsResolved: []string{"chrome"},
		Entries: map[string]graph.Entry{
			"background": {Path: "bg.js", Type: "service-worker"},
			"popup":      {Path: "popup.html"},
			"options":    {Path: "options.html"},
		},
	}

	result := Compile(CompileInput{
		Graph:    g,
		Adapters: map[string]target.Adapter{"chrome": target.NewChrome()},
	})

	if len(result.Errors) > 0 {
		t.Fatalf("errors: %v", result.Errors)
	}

	m := result.Outputs[0].Manifest
	if m["background"] == nil {
		t.Error("expected background in manifest")
	}
	if m["action"] == nil {
		t.Error("expected action (popup) in manifest")
	}
	if m["options_ui"] == nil {
		t.Error("expected options_ui in manifest")
	}
}

func TestCompile_NilGraph(t *testing.T) {
	result := Compile(CompileInput{})
	if len(result.Errors) == 0 {
		t.Error("expected error for nil graph")
	}
}

func TestCompile_MissingAdapter(t *testing.T) {
	g := &graph.Graph{
		Project:         graph.ProjectIdentity{Name: "test"},
		TargetsResolved: []string{"firefox"},
		Entries:         map[string]graph.Entry{"bg": {Path: "bg.js"}},
	}

	result := Compile(CompileInput{
		Graph:    g,
		Adapters: map[string]target.Adapter{"chrome": target.NewChrome()},
	})

	if len(result.Errors) == 0 {
		t.Error("expected error for missing firefox adapter")
	}
}

func TestCompile_VersionFallback(t *testing.T) {
	g := &graph.Graph{
		Project:         graph.ProjectIdentity{Name: "test"},
		TargetsResolved: []string{"chrome"},
		Entries:         map[string]graph.Entry{"bg": {Path: "bg.js"}},
	}

	result := Compile(CompileInput{
		Graph:    g,
		Adapters: map[string]target.Adapter{"chrome": target.NewChrome()},
	})

	m := result.Outputs[0].Manifest
	if m["version"] != "0.0.1" {
		t.Errorf("default version: got %v", m["version"])
	}

	// Explicit version override
	result2 := Compile(CompileInput{
		Graph:    g,
		Adapters: map[string]target.Adapter{"chrome": target.NewChrome()},
		Version:  "3.0.0",
	})
	m2 := result2.Outputs[0].Manifest
	if m2["version"] != "3.0.0" {
		t.Errorf("override version: got %v", m2["version"])
	}
}

func TestCompile_PermissionsFromCapabilities(t *testing.T) {
	g := &graph.Graph{
		Project:         graph.ProjectIdentity{Name: "test"},
		TargetsResolved: []string{"chrome"},
		Entries:         map[string]graph.Entry{"bg": {Path: "bg.js"}},
	}
	matrix := &capability.TargetMatrix{
		Resolutions: []capability.Resolution{
			{Capability: "tabs", Target: "chrome", State: "native", Permissions: []string{"tabs"}},
			{Capability: "cookies", Target: "chrome", State: "native", Permissions: []string{"cookies"}},
			{Capability: "sidebar", Target: "chrome", State: "blocked", Reason: "not supported"},
		},
		Permissions: []string{"tabs", "cookies"},
	}

	result := Compile(CompileInput{
		Graph:    g,
		Matrix:   matrix,
		Adapters: map[string]target.Adapter{"chrome": target.NewChrome()},
	})

	if len(result.Errors) > 0 {
		t.Fatalf("errors: %v", result.Errors)
	}

	out := result.Outputs[0]
	// Only native/adapted capabilities should contribute permissions
	if len(out.Permissions) != 2 {
		t.Errorf("expected 2 permissions, got %d: %v", len(out.Permissions), out.Permissions)
	}
}

func TestCompile_ManifestHashStable(t *testing.T) {
	g := &graph.Graph{
		Project:         graph.ProjectIdentity{Name: "test"},
		TargetsResolved: []string{"chrome"},
		Entries:         map[string]graph.Entry{"bg": {Path: "bg.js"}},
	}

	r1 := Compile(CompileInput{Graph: g, Adapters: map[string]target.Adapter{"chrome": target.NewChrome()}})
	r2 := Compile(CompileInput{Graph: g, Adapters: map[string]target.Adapter{"chrome": target.NewChrome()}})

	if r1.Outputs[0].ManifestHash != r2.Outputs[0].ManifestHash {
		t.Error("manifest hash should be stable across compilations")
	}
}

func TestWriteManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	m := map[string]any{
		"manifest_version": 3,
		"name":             "test",
		"version":          "1.0.0",
	}

	if err := WriteManifest(m, path); err != nil {
		t.Fatalf("write: %v", err)
	}

	data, _ := os.ReadFile(path)
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed["name"] != "test" {
		t.Errorf("name: got %v", parsed["name"])
	}
}
