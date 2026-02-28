package main

import (
	"bytes"
	"errors"
	"io"
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
	var out bytes.Buffer

	err := run([]string{"dev"}, &out)
	if err != nil {
		t.Fatalf("run(dev) returned error: %v", err)
	}

	const want = "panex dev (skeleton)\nconfig=panex.toml\n"
	if out.String() != want {
		t.Fatalf("unexpected dev output: got %q, want %q", out.String(), want)
	}
}

func TestRunDevCustomConfig(t *testing.T) {
	var out bytes.Buffer

	err := run([]string{"dev", "--config", "custom.toml"}, &out)
	if err != nil {
		t.Fatalf("run(dev --config) returned error: %v", err)
	}

	const want = "panex dev (skeleton)\nconfig=custom.toml\n"
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
