package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	panexconfig "github.com/panex-dev/panex/internal/config"
)

func TestRunDevOpenFlagOpensBrowser(t *testing.T) {
	tempDir := t.TempDir()
	writePanexConfig(t, filepath.Join(tempDir, "panex.toml"), `
[extension]
source_dir = "./src"
out_dir = "./dist"

[server]
port = 4317
auth_token = "test-token"
`)

	var captured panexconfig.Config
	withStubbedStartDev(t, func(cfg panexconfig.Config, stdout io.Writer) error {
		captured = cfg
		_, err := io.WriteString(stdout, "dev started\n")
		return err
	})

	var openedURL string
	withStubbedOpenBrowserCmd(t, func(url string) (string, []string) {
		openedURL = url
		return "echo", []string{url}
	})

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return run([]string{"dev", "--open"}, &out)
	})
	if err != nil {
		t.Fatalf("run(dev --open) returned error: %v", err)
	}

	if openedURL != "chrome://extensions" {
		t.Fatalf("expected browser opened with chrome://extensions, got %q", openedURL)
	}
	if captured.Server.Port != 4317 {
		t.Fatalf("unexpected port: got %d", captured.Server.Port)
	}
}

func TestRunDevWithoutOpenFlagDoesNotOpenBrowser(t *testing.T) {
	tempDir := t.TempDir()
	writePanexConfig(t, filepath.Join(tempDir, "panex.toml"), `
[extension]
source_dir = "./src"
out_dir = "./dist"

[server]
port = 4317
auth_token = "test-token"
`)

	withStubbedStartDev(t, func(cfg panexconfig.Config, stdout io.Writer) error {
		_, err := io.WriteString(stdout, "dev started\n")
		return err
	})

	var browserCalled bool
	withStubbedOpenBrowserCmd(t, func(url string) (string, []string) {
		browserCalled = true
		return "echo", []string{url}
	})

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return run([]string{"dev"}, &out)
	})
	if err != nil {
		t.Fatalf("run(dev) returned error: %v", err)
	}

	if browserCalled {
		t.Fatal("expected browser not to be opened without --open flag")
	}
}

func TestRunDevOpenFlagBrowserFailureIsNonFatal(t *testing.T) {
	tempDir := t.TempDir()
	writePanexConfig(t, filepath.Join(tempDir, "panex.toml"), `
[extension]
source_dir = "./src"
out_dir = "./dist"

[server]
port = 4317
auth_token = "test-token"
`)

	withStubbedStartDev(t, func(cfg panexconfig.Config, stdout io.Writer) error {
		_, err := io.WriteString(stdout, "dev started\n")
		return err
	})

	// Point to a nonexistent command to simulate browser failure.
	withStubbedOpenBrowserCmd(t, func(url string) (string, []string) {
		return "/nonexistent-browser-command", []string{url}
	})

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return run([]string{"dev", "--open"}, &out)
	})
	if err != nil {
		t.Fatalf("run(dev --open) should not fail when browser fails, got: %v", err)
	}

	if !strings.Contains(out.String(), "could not open browser") {
		t.Fatalf("expected browser failure note in output: %q", out.String())
	}
}

func TestRunDevOpenFlagWithInferredConfig(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), []byte(`{"manifest_version": 3}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	withStubbedStartDev(t, func(cfg panexconfig.Config, stdout io.Writer) error {
		_, err := io.WriteString(stdout, "dev started\n")
		return err
	})

	var openedURL string
	withStubbedOpenBrowserCmd(t, func(url string) (string, []string) {
		openedURL = url
		return "echo", []string{url}
	})

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return run([]string{"dev", "--open"}, &out)
	})
	if err != nil {
		t.Fatalf("run(dev --open) returned error: %v", err)
	}

	if openedURL != "chrome://extensions" {
		t.Fatalf("expected browser opened with chrome://extensions, got %q", openedURL)
	}
	if !strings.Contains(out.String(), "manifest.json") {
		t.Fatalf("expected inference notice: %q", out.String())
	}
}

func TestDefaultOpenBrowserCmd(t *testing.T) {
	name, args := defaultOpenBrowserCmd("https://example.com")
	if name == "" {
		t.Fatal("expected non-empty command name")
	}
	if len(args) == 0 {
		t.Fatal("expected at least one argument")
	}
}

func withStubbedOpenBrowserCmd(t *testing.T, stub func(url string) (string, []string)) {
	t.Helper()

	original := openBrowserCmd
	openBrowserCmd = stub
	t.Cleanup(func() {
		openBrowserCmd = original
	})
}
