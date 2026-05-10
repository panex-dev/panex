package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathsWithPanexToml(t *testing.T) {
	tempDir := resolveSymlinks(t, t.TempDir())
	writePanexConfig(t, filepath.Join(tempDir, "panex.toml"), `
[extension]
source_dir = "./src"
out_dir = "./dist"

[server]
port = 4317
auth_token = "test-token"
`)

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return runPaths(&out)
	})
	if err != nil {
		t.Fatalf("runPaths() returned error: %v", err)
	}

	output := out.String()
	absSource, _ := filepath.Abs(filepath.Join(tempDir, "src"))
	absOut, _ := filepath.Abs(filepath.Join(tempDir, "dist"))

	if !strings.Contains(output, "source_dir="+absSource) {
		t.Fatalf("missing source_dir: %q", output)
	}
	if !strings.Contains(output, "out_dir="+absOut) {
		t.Fatalf("missing out_dir: %q", output)
	}
	if strings.Contains(output, "[") {
		t.Fatalf("single extension should not use bracket labels: %q", output)
	}
}

func TestPathsWithManifestJSON(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), []byte(`{"manifest_version": 3}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return runPaths(&out)
	})
	if err != nil {
		t.Fatalf("runPaths() returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "source_dir=") {
		t.Fatalf("missing source_dir: %q", output)
	}
	if !strings.Contains(output, "out_dir=") {
		t.Fatalf("missing out_dir: %q", output)
	}
	if !strings.Contains(output, filepath.Join(".panex", "dist")) {
		t.Fatalf("expected default .panex/dist out_dir: %q", output)
	}
}

func resolveSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", path, err)
	}
	return resolved
}

func TestPathsMultiExtension(t *testing.T) {
	tempDir := t.TempDir()
	writePanexConfig(t, filepath.Join(tempDir, "panex.toml"), `
[[extensions]]
id = "popup"
source_dir = "./extensions/popup"
out_dir = "./dist/popup"

[[extensions]]
id = "admin"
source_dir = "./extensions/admin"
out_dir = "./dist/admin"

[server]
port = 4317
auth_token = "test-token"
`)

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return runPaths(&out)
	})
	if err != nil {
		t.Fatalf("runPaths() returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "source_dir[popup]=") {
		t.Fatalf("missing popup source_dir: %q", output)
	}
	if !strings.Contains(output, "out_dir[popup]=") {
		t.Fatalf("missing popup out_dir: %q", output)
	}
	if !strings.Contains(output, "source_dir[admin]=") {
		t.Fatalf("missing admin source_dir: %q", output)
	}
	if !strings.Contains(output, "out_dir[admin]=") {
		t.Fatalf("missing admin out_dir: %q", output)
	}
}

func TestPathsNoConfig(t *testing.T) {
	tempDir := t.TempDir()

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return runPaths(&out)
	})

	cliErr := requireCLIError(t, err)
	if cliErr.code != 2 {
		t.Fatalf("unexpected error code: got %d, want 2", cliErr.code)
	}
	if !strings.Contains(cliErr.msg, "panex init") {
		t.Fatalf("expected init guidance: %q", cliErr.msg)
	}
}

func TestPathsViaRunCommand(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), []byte(`{"manifest_version": 3}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return run([]string{"paths"}, &out)
	})
	if err != nil {
		t.Fatalf("run(paths) returned error: %v", err)
	}

	if !strings.Contains(out.String(), "source_dir=") {
		t.Fatalf("expected paths output: %q", out.String())
	}
}

func TestPathsJSONViaRunCommand(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), []byte(`{"manifest_version": 3}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return run([]string{"--json", "paths"}, &out)
	})
	if err != nil {
		t.Fatalf("run(--json paths) returned error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal json output: %v", err)
	}
	if parsed["command"] != "paths" {
		t.Fatalf("command: got %v", parsed["command"])
	}
	data := parsed["data"].(map[string]any)
	extensions := data["extensions"].([]any)
	if len(extensions) != 1 {
		t.Fatalf("extensions length: got %d", len(extensions))
	}
}
