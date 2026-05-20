package target

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFirefox_Name(t *testing.T) {
	f := NewFirefox()
	if f.Name() != "firefox" {
		t.Errorf("got %s, want firefox", f.Name())
	}
}

func TestFirefox_Catalog(t *testing.T) {
	f := NewFirefox()
	cat := f.Catalog()

	if cat.Target != "firefox" {
		t.Errorf("target: got %s, want firefox", cat.Target)
	}

	expected := []string{"tabs", "storage", "sidebarSurface", "backgroundExecution", "content", "cookies"}
	for _, name := range expected {
		if _, ok := cat.Capabilities[name]; !ok {
			t.Errorf("expected capability %q in catalog", name)
		}
	}

	if cat.Capabilities["sideSurface"].State != "blocked" {
		t.Error("sideSurface should be blocked on Firefox")
	}
	if cat.Capabilities["sidebarSurface"].State != "native" {
		t.Error("sidebarSurface should be native on Firefox")
	}
	if cat.Capabilities["backgroundExecution"].State != "adapted" {
		t.Error("backgroundExecution should be adapted on Firefox")
	}
}

func TestFirefox_ResolveCapabilities(t *testing.T) {
	f := NewFirefox()

	caps := map[string]any{
		"tabs":                "read-write",
		"storage":             map[string]string{"mode": "sync"},
		"sideSurface":         "preferred",
		"sidebarSurface":      "preferred",
		"backgroundExecution": true,
	}

	resolved, result := f.ResolveCapabilities(caps)
	if result.Outcome != Success {
		t.Fatalf("expected success, got %s: %s", result.Outcome, result.Reason)
	}

	if resolved["tabs"].State != "native" {
		t.Errorf("tabs: got %s, want native", resolved["tabs"].State)
	}
	if len(resolved["tabs"].Permissions) == 0 || resolved["tabs"].Permissions[0] != "tabs" {
		t.Error("tabs should require 'tabs' permission")
	}
	if resolved["sidebarSurface"].State != "native" {
		t.Errorf("sidebarSurface: got %s, want native", resolved["sidebarSurface"].State)
	}
	if resolved["sideSurface"].State != "blocked" {
		t.Errorf("sideSurface: got %s, want blocked", resolved["sideSurface"].State)
	}
	if resolved["backgroundExecution"].State != "adapted" {
		t.Errorf("backgroundExecution: got %s, want adapted", resolved["backgroundExecution"].State)
	}
}

func TestFirefox_CompileManifest(t *testing.T) {
	f := NewFirefox()

	opts := ManifestCompileOptions{
		ProjectName:    "Tab Organizer",
		ProjectVersion: "1.0.0",
		Entries: map[string]EntrySpec{
			"background": {Path: "background.js", Type: "service-worker"},
			"popup":      {Path: "popup.html", Type: "html-page"},
			"sidebar":    {Path: "sidebar.html", Type: "html-page"},
		},
		Permissions:     []string{"tabs", "storage"},
		HostPermissions: []string{"https://*.example.com/*"},
	}

	output, result := f.CompileManifest(opts)
	if result.Outcome != Success {
		t.Fatalf("expected success, got %s", result.Outcome)
	}

	m := output.Manifest
	if m["manifest_version"] != 2 {
		t.Error("expected manifest_version 2")
	}

	bg, ok := m["background"].(map[string]any)
	if !ok {
		t.Fatal("expected background key")
	}
	scripts, ok := bg["scripts"].([]string)
	if !ok || len(scripts) != 1 || scripts[0] != "background.js" {
		t.Errorf("background scripts: got %v", bg["scripts"])
	}

	action, ok := m["browser_action"].(map[string]any)
	if !ok {
		t.Fatal("expected browser_action key")
	}
	if action["default_popup"] != "popup.html" {
		t.Errorf("default_popup: got %v", action["default_popup"])
	}

	sidebar, ok := m["sidebar_action"].(map[string]any)
	if !ok {
		t.Fatal("expected sidebar_action key")
	}
	if sidebar["default_panel"] != "sidebar.html" {
		t.Errorf("default_panel: got %v", sidebar["default_panel"])
	}

	perms, ok := m["permissions"].([]string)
	if !ok || len(perms) != 3 {
		t.Fatalf("permissions: got %v", m["permissions"])
	}
	if perms[2] != "https://*.example.com/*" {
		t.Errorf("expected host permission in MV2 permissions: got %v", perms)
	}
	if len(output.Permissions) != 2 {
		t.Errorf("output permissions should exclude host permissions: got %v", output.Permissions)
	}
	if len(output.HostPermissions) != 1 {
		t.Errorf("output host permissions: got %v", output.HostPermissions)
	}
}

func TestFirefox_PackageArtifact(t *testing.T) {
	srcDir := t.TempDir()
	mustWrite(t, filepath.Join(srcDir, "manifest.json"), []byte(`{"manifest_version":2}`))
	mustWrite(t, filepath.Join(srcDir, "background.js"), []byte("// bg"))

	outDir := t.TempDir()

	f := NewFirefox()
	record, result := f.PackageArtifact(context.Background(), PackageOptions{
		SourceDir:    srcDir,
		OutputDir:    outDir,
		ArtifactName: "my-ext",
		Version:      "1.0.0",
	})

	if result.Outcome != Success {
		t.Fatalf("expected success, got %s: %s", result.Outcome, result.Reason)
	}
	if record.ArtifactType != "firefox_xpi" {
		t.Errorf("artifact_type: got %s", record.ArtifactType)
	}
	if filepath.Ext(record.FilePath) != ".xpi" {
		t.Errorf("expected .xpi path, got %s", record.FilePath)
	}
	if record.FileSize == 0 {
		t.Error("expected non-zero file size")
	}
	if record.SHA256 == "" {
		t.Error("expected SHA256 digest")
	}
	if _, err := os.Stat(record.FilePath); err != nil {
		t.Errorf("xpi file not found: %v", err)
	}
}

func TestFirefox_PackageArtifact_MissingSourceDir(t *testing.T) {
	f := NewFirefox()
	_, result := f.PackageArtifact(context.Background(), PackageOptions{
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

func TestFirefox_InspectEnvironment(t *testing.T) {
	f := NewFirefox()
	info, result := f.InspectEnvironment(context.Background())

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
