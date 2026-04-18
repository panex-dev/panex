package target

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestChrome_Name(t *testing.T) {
	c := NewChrome()
	if c.Name() != "chrome" {
		t.Errorf("got %s, want chrome", c.Name())
	}
}

func TestChrome_Catalog(t *testing.T) {
	c := NewChrome()
	cat := c.Catalog()

	if cat.Target != "chrome" {
		t.Errorf("target: got %s, want chrome", cat.Target)
	}

	// Verify key capabilities exist
	expected := []string{"tabs", "storage", "sideSurface", "backgroundExecution", "content", "cookies"}
	for _, name := range expected {
		if _, ok := cat.Capabilities[name]; !ok {
			t.Errorf("expected capability %q in catalog", name)
		}
	}

	// sidebarSurface should be blocked on Chrome
	if cat.Capabilities["sidebarSurface"].State != "blocked" {
		t.Error("sidebarSurface should be blocked on Chrome")
	}

	// sideSurface should be native on Chrome
	if cat.Capabilities["sideSurface"].State != "native" {
		t.Error("sideSurface should be native on Chrome")
	}
}

func TestChrome_ResolveCapabilities(t *testing.T) {
	c := NewChrome()

	caps := map[string]any{
		"tabs":           "read-write",
		"storage":        map[string]string{"mode": "sync"},
		"sideSurface":    "preferred",
		"sidebarSurface": "preferred",
	}

	resolved, result := c.ResolveCapabilities(caps)
	if result.Outcome != Success {
		t.Fatalf("expected success, got %s: %s", result.Outcome, result.Reason)
	}

	// tabs should be native with permission
	if resolved["tabs"].State != "native" {
		t.Errorf("tabs: got %s, want native", resolved["tabs"].State)
	}
	if len(resolved["tabs"].Permissions) == 0 || resolved["tabs"].Permissions[0] != "tabs" {
		t.Error("tabs should require 'tabs' permission")
	}

	// sideSurface should be native
	if resolved["sideSurface"].State != "native" {
		t.Errorf("sideSurface: got %s, want native", resolved["sideSurface"].State)
	}

	// sidebarSurface should be blocked
	if resolved["sidebarSurface"].State != "blocked" {
		t.Errorf("sidebarSurface: got %s, want blocked", resolved["sidebarSurface"].State)
	}
}

func TestChrome_ResolveCapabilities_Unknown(t *testing.T) {
	c := NewChrome()
	caps := map[string]any{"nonexistent": true}
	resolved, _ := c.ResolveCapabilities(caps)

	if resolved["nonexistent"].State != "blocked" {
		t.Errorf("unknown capability should be blocked, got %s", resolved["nonexistent"].State)
	}
}

func TestChrome_CompileManifest(t *testing.T) {
	c := NewChrome()

	opts := ManifestCompileOptions{
		ProjectName:    "Tab Organizer",
		ProjectVersion: "1.0.0",
		Entries: map[string]EntrySpec{
			"background": {Path: "background.js", Type: "service-worker"},
			"popup":      {Path: "popup.html", Type: "html-page"},
		},
		Permissions:     []string{"tabs", "storage"},
		HostPermissions: []string{"https://*.example.com/*"},
	}

	output, result := c.CompileManifest(opts)
	if result.Outcome != Success {
		t.Fatalf("expected success, got %s", result.Outcome)
	}

	m := output.Manifest
	if m["manifest_version"] != 3 {
		t.Error("expected manifest_version 3")
	}
	if m["name"] != "Tab Organizer" {
		t.Errorf("name: got %v", m["name"])
	}

	bg, ok := m["background"].(map[string]any)
	if !ok {
		t.Fatal("expected background key")
	}
	if bg["service_worker"] != "background.js" {
		t.Errorf("service_worker: got %v", bg["service_worker"])
	}

	action, ok := m["action"].(map[string]any)
	if !ok {
		t.Fatal("expected action key")
	}
	if action["default_popup"] != "popup.html" {
		t.Errorf("default_popup: got %v", action["default_popup"])
	}

	perms, ok := m["permissions"].([]string)
	if !ok || len(perms) != 2 {
		t.Errorf("permissions: got %v", m["permissions"])
	}

	hostPerms, ok := m["host_permissions"].([]string)
	if !ok || len(hostPerms) != 1 {
		t.Errorf("host_permissions: got %v", m["host_permissions"])
	}
}

func TestChrome_CompileManifest_SidePanel(t *testing.T) {
	c := NewChrome()
	opts := ManifestCompileOptions{
		ProjectName:    "Test",
		ProjectVersion: "1.0.0",
		Entries: map[string]EntrySpec{
			"side_panel": {Path: "sidepanel.html", Type: "html-page"},
		},
	}

	output, _ := c.CompileManifest(opts)
	sp, ok := output.Manifest["side_panel"].(map[string]any)
	if !ok {
		t.Fatal("expected side_panel key")
	}
	if sp["default_path"] != "sidepanel.html" {
		t.Errorf("default_path: got %v", sp["default_path"])
	}
}

func TestChrome_PackageArtifact(t *testing.T) {
	// Create a fake extension directory
	srcDir := t.TempDir()
	mustWrite(t, filepath.Join(srcDir, "manifest.json"), []byte(`{"manifest_version":3}`))
	mustWrite(t, filepath.Join(srcDir, "background.js"), []byte("// bg"))
	mustMkdir(t, filepath.Join(srcDir, "popup"))
	mustWrite(t, filepath.Join(srcDir, "popup", "index.html"), []byte("<html></html>"))

	outDir := t.TempDir()

	c := NewChrome()
	record, result := c.PackageArtifact(context.Background(), PackageOptions{
		SourceDir:    srcDir,
		OutputDir:    outDir,
		ArtifactName: "my-ext",
		Version:      "1.0.0",
	})

	if result.Outcome != Success {
		t.Fatalf("expected success, got %s: %s", result.Outcome, result.Reason)
	}

	if record.ArtifactType != "chrome_zip" {
		t.Errorf("artifact_type: got %s", record.ArtifactType)
	}
	if record.FileSize == 0 {
		t.Error("expected non-zero file size")
	}
	if record.SHA256 == "" {
		t.Error("expected SHA256 digest")
	}

	// Verify zip was created
	if _, err := os.Stat(record.FilePath); err != nil {
		t.Errorf("zip file not found: %v", err)
	}
}

func TestChrome_PackageArtifact_MissingSourceDir(t *testing.T) {
	c := NewChrome()
	_, result := c.PackageArtifact(context.Background(), PackageOptions{
		SourceDir: "/nonexistent",
		OutputDir: t.TempDir(),
	})

	if result.Outcome != Blocked {
		t.Errorf("expected blocked, got %s", result.Outcome)
	}
	if result.ReasonCode != "source_dir_missing" {
		t.Errorf("reason_code: got %s", result.ReasonCode)
	}
}

func TestChrome_InspectEnvironment(t *testing.T) {
	c := NewChrome()
	info, result := c.InspectEnvironment(context.Background())

	// We can't guarantee Chrome is installed in CI, so just check structure
	if result.Outcome == Success {
		if !info.Available {
			t.Error("success but not available")
		}
		if info.BinaryPath == "" {
			t.Error("success but no binary path")
		}
	} else if result.Outcome == EnvironmentMissing {
		if info.Available {
			t.Error("environment_missing but available=true")
		}
	} else {
		t.Errorf("unexpected outcome: %s", result.Outcome)
	}
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}
