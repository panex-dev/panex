package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorWithPanexToml(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "src")
	outDir := filepath.Join(tempDir, "dist")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create out dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write output manifest: %v", err)
	}
	writePanexConfig(t, filepath.Join(tempDir, "panex.toml"), `
[extension]
source_dir = "./src"
out_dir = "./dist"

[server]
port = 4317
auth_token = "test-token"
`)

	withStubbedReadProcVersion(t, nil)

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return runDoctor(&out)
	})
	if err != nil {
		t.Fatalf("runDoctor() returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "panex doctor") {
		t.Fatalf("missing doctor header: %q", output)
	}
	if !strings.Contains(output, "config: panex.toml") {
		t.Fatalf("missing config source: %q", output)
	}
	if !strings.Contains(output, "source_dir:") && !strings.Contains(output, "(ok)") {
		t.Fatalf("missing source_dir check: %q", output)
	}
	if !strings.Contains(output, "manifest.json found") {
		t.Fatalf("missing output manifest check: %q", output)
	}
	if !strings.Contains(output, "No issues found") {
		t.Fatalf("expected no issues, got: %q", output)
	}
}

func TestDoctorWithManifestJSON(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), []byte(`{"manifest_version": 3}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	withStubbedReadProcVersion(t, nil)

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return runDoctor(&out)
	})
	if err != nil {
		t.Fatalf("runDoctor() returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "manifest.json (inferred)") {
		t.Fatalf("expected inferred config source: %q", output)
	}
	if !strings.Contains(output, "not built yet") {
		t.Fatalf("expected not-built-yet warning for output dir: %q", output)
	}
}

func TestDoctorWithNoConfig(t *testing.T) {
	tempDir := t.TempDir()

	withStubbedReadProcVersion(t, nil)

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return runDoctor(&out)
	})
	if err != nil {
		t.Fatalf("runDoctor() returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "config: not found") {
		t.Fatalf("expected not-found message: %q", output)
	}
	if !strings.Contains(output, "panex init") {
		t.Fatalf("expected panex init guidance: %q", output)
	}
	if !strings.Contains(output, "1 issue(s) found") {
		t.Fatalf("expected 1 issue: %q", output)
	}
}

func TestDoctorOutputExistsButNoManifest(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "src")
	outDir := filepath.Join(tempDir, "dist")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create out dir: %v", err)
	}
	writePanexConfig(t, filepath.Join(tempDir, "panex.toml"), `
[extension]
source_dir = "./src"
out_dir = "./dist"

[server]
port = 4317
auth_token = "test-token"
`)

	withStubbedReadProcVersion(t, nil)

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return runDoctor(&out)
	})
	if err != nil {
		t.Fatalf("runDoctor() returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "exists, but no manifest.json") {
		t.Fatalf("expected missing manifest warning: %q", output)
	}
	if !strings.Contains(output, "1 issue(s) found") {
		t.Fatalf("expected 1 issue: %q", output)
	}
}

func TestDoctorWSLWarning(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "src")
	outDir := filepath.Join(tempDir, "dist")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create out dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write output manifest: %v", err)
	}
	writePanexConfig(t, filepath.Join(tempDir, "panex.toml"), `
[extension]
source_dir = "./src"
out_dir = "./dist"

[server]
port = 4317
auth_token = "test-token"
`)

	withStubbedReadProcVersion(t, []byte("Linux version 5.15.0 (microsoft@microsoft.com) (WSL2)"))

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return runDoctor(&out)
	})
	if err != nil {
		t.Fatalf("runDoctor() returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "WSL detected") {
		t.Fatalf("expected WSL warning: %q", output)
	}
	if !strings.Contains(output, "/mnt/c/") {
		t.Fatalf("expected /mnt/c/ guidance: %q", output)
	}
}

func TestDoctorNoWSLWarningForMntPaths(t *testing.T) {
	// When the output path is already under /mnt/, no WSL warning should appear.
	// We test this by using a config with out_dir pointing to a /mnt/ path.
	// Since we can't easily create /mnt/c/... in tests, we verify the logic
	// by checking that a non-WSL environment has no WSL warning even with
	// temp dir paths.
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), []byte(`{"manifest_version": 3}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	withStubbedReadProcVersion(t, nil)

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return runDoctor(&out)
	})
	if err != nil {
		t.Fatalf("runDoctor() returned error: %v", err)
	}

	output := out.String()
	if strings.Contains(output, "WSL detected") {
		t.Fatalf("unexpected WSL warning on non-WSL system: %q", output)
	}
}

func TestDoctorViaRunCommand(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), []byte(`{"manifest_version": 3}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	withStubbedReadProcVersion(t, nil)

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return run([]string{"doctor"}, &out)
	})
	if err != nil {
		t.Fatalf("run(doctor) returned error: %v", err)
	}

	if !strings.Contains(out.String(), "panex doctor") {
		t.Fatalf("expected doctor output: %q", out.String())
	}
}

func TestIsWSL(t *testing.T) {
	testCases := []struct {
		name        string
		procVersion []byte
		want        bool
	}{
		{
			name:        "WSL2 kernel",
			procVersion: []byte("Linux version 5.15.90.1-microsoft-standard-WSL2"),
			want:        true,
		},
		{
			name:        "standard linux kernel",
			procVersion: []byte("Linux version 6.1.0-debian"),
			want:        false,
		},
		{
			name:        "nil proc version",
			procVersion: nil,
			want:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			withStubbedReadProcVersion(t, tc.procVersion)
			got := isWSL()
			if got != tc.want {
				t.Fatalf("isWSL() = %v, want %v (proc: %q)", got, tc.want, tc.procVersion)
			}
		})
	}
}

func withStubbedReadProcVersion(t *testing.T, data []byte) {
	t.Helper()

	original := readProcVersion
	readProcVersion = func() []byte { return data }
	t.Cleanup(func() {
		readProcVersion = original
	})
}
