package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var out bytes.Buffer

	err := run([]string{"version"}, &out)
	if err != nil {
		t.Fatalf("run(version) returned error: %v", err)
	}

	const want = "panex dev\n"
	if out.String() != want {
		t.Fatalf("unexpected version output: got %q, want %q", out.String(), want)
	}
}

func TestRunHelpAliases(t *testing.T) {
	testCases := []struct {
		name string
		args []string
	}{
		{name: "help command", args: []string{"help"}},
		{name: "short help flag", args: []string{"-h"}},
		{name: "long help flag", args: []string{"--help"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			err := run(tc.args, &out)
			if err != nil {
				t.Fatalf("run(%v) returned error: %v", tc.args, err)
			}

			if out.String() != usageText {
				t.Fatalf("unexpected help output: got %q, want %q", out.String(), usageText)
			}
		})
	}
}

func TestRunNoArgsReturnsUsageError(t *testing.T) {
	var out bytes.Buffer

	err := run(nil, &out)
	cliErr := requireCLIError(t, err)

	if cliErr.code != 2 {
		t.Fatalf("unexpected error code: got %d, want 2", cliErr.code)
	}
	if cliErr.msg != usageText {
		t.Fatalf("unexpected usage message: got %q, want %q", cliErr.msg, usageText)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
}

func TestRunUnknownCommandReturnsUsageError(t *testing.T) {
	var out bytes.Buffer

	err := run([]string{"nope"}, &out)
	cliErr := requireCLIError(t, err)

	if cliErr.code != 2 {
		t.Fatalf("unexpected error code: got %d, want 2", cliErr.code)
	}
	if !strings.Contains(cliErr.msg, `unknown command "nope"`) {
		t.Fatalf("missing unknown command message: %q", cliErr.msg)
	}
	if !strings.Contains(cliErr.msg, "Usage:") {
		t.Fatalf("missing usage text in error: %q", cliErr.msg)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
}

func TestRunDevDefaultConfig(t *testing.T) {
	tempDir := t.TempDir()
	writePanexConfig(t, filepath.Join(tempDir, "panex.toml"), `
[extension]
source_dir = "./src"
out_dir = "./dist"

[server]
port = 3000
`)

	var out bytes.Buffer

	err := withWorkingDir(tempDir, func() error {
		return run([]string{"dev"}, &out)
	})
	if err != nil {
		t.Fatalf("run(dev) returned error: %v", err)
	}

	const want = "panex dev (skeleton)\nconfig=panex.toml\nsource_dir=./src\nout_dir=./dist\nport=3000\n"
	if out.String() != want {
		t.Fatalf("unexpected dev output: got %q, want %q", out.String(), want)
	}
}

func TestRunDevCustomConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "custom.toml")
	writePanexConfig(t, configPath, `
[extension]
source_dir = "./extension-src"
out_dir = "./build"

[server]
port = 4317
`)

	var out bytes.Buffer

	err := run([]string{"dev", "--config", configPath}, &out)
	if err != nil {
		t.Fatalf("run(dev --config) returned error: %v", err)
	}

	want := "panex dev (skeleton)\nconfig=" + configPath + "\nsource_dir=./extension-src\nout_dir=./build\nport=4317\n"
	if out.String() != want {
		t.Fatalf("unexpected dev output: got %q, want %q", out.String(), want)
	}
}

func TestRunDevUnexpectedPositionalArg(t *testing.T) {
	var out bytes.Buffer

	err := run([]string{"dev", "extra"}, &out)
	cliErr := requireCLIError(t, err)

	if cliErr.code != 2 {
		t.Fatalf("unexpected error code: got %d, want 2", cliErr.code)
	}
	if !strings.Contains(cliErr.msg, "unexpected arguments for dev") {
		t.Fatalf("missing positional-arg validation error: %q", cliErr.msg)
	}
}

func TestRunDevInvalidFlag(t *testing.T) {
	var out bytes.Buffer

	err := run([]string{"dev", "--bad-flag"}, &out)
	cliErr := requireCLIError(t, err)

	if cliErr.code != 2 {
		t.Fatalf("unexpected error code: got %d, want 2", cliErr.code)
	}
	if !strings.Contains(cliErr.msg, "invalid dev flags") {
		t.Fatalf("missing invalid-flag message: %q", cliErr.msg)
	}
}

func TestRunDevMissingConfig(t *testing.T) {
	var out bytes.Buffer

	err := run([]string{"dev", "--config", filepath.Join(t.TempDir(), "missing.toml")}, &out)
	cliErr := requireCLIError(t, err)

	if cliErr.code != 2 {
		t.Fatalf("unexpected error code: got %d, want 2", cliErr.code)
	}
	if !strings.Contains(cliErr.msg, "failed to load config") {
		t.Fatalf("missing load failure message: %q", cliErr.msg)
	}
	if !strings.Contains(cliErr.msg, "config file not found") {
		t.Fatalf("missing file-not-found detail: %q", cliErr.msg)
	}
}

func TestRunWriteFailurePropagates(t *testing.T) {
	err := run([]string{"version"}, failingWriter{})
	if err == nil {
		t.Fatal("expected write failure error, got nil")
	}

	var cliErr *cliError
	if errors.As(err, &cliErr) {
		t.Fatalf("expected raw write error, got cliError: %+v", cliErr)
	}
}

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func requireCLIError(t *testing.T, err error) *cliError {
	t.Helper()

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cliErr *cliError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected cliError, got %T (%v)", err, err)
	}

	return cliErr
}

func writePanexConfig(t *testing.T, path, content string) {
	t.Helper()

	err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600)
	if err != nil {
		t.Fatalf("failed to write config fixture: %v", err)
	}
}

func withWorkingDir(dir string, fn func() error) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := os.Chdir(dir); err != nil {
		return err
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	return fn()
}
