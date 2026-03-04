package build

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewEsbuildBuilderValidation(t *testing.T) {
	tmpDir := t.TempDir()
	plainFile := filepath.Join(tmpDir, "plain.txt")
	if err := os.WriteFile(plainFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	testCases := []struct {
		name      string
		sourceDir string
		outDir    string
		wantError string
	}{
		{
			name:      "missing source directory",
			sourceDir: "",
			outDir:    filepath.Join(tmpDir, "out"),
			wantError: "source directory is required",
		},
		{
			name:      "missing output directory",
			sourceDir: tmpDir,
			outDir:    "",
			wantError: "output directory is required",
		},
		{
			name:      "source is not a directory",
			sourceDir: plainFile,
			outDir:    filepath.Join(tmpDir, "out"),
			wantError: "source directory must be a directory",
		},
		{
			name:      "source equals output directory",
			sourceDir: tmpDir,
			outDir:    tmpDir,
			wantError: "source and output directories must differ",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewEsbuildBuilder(tc.sourceDir, tc.outDir)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("unexpected error: got %v, want contains %q", err, tc.wantError)
			}
		})
	}
}

func TestBuildSuccess(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "src")
	outDir := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(filepath.Join(sourceDir, "nested"), 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}

	writeFixture(t, filepath.Join(sourceDir, "index.ts"), `import { hello } from "./nested/util"; console.log(hello);`)
	writeFixture(t, filepath.Join(sourceDir, "nested", "util.ts"), `export const hello = "world";`)

	builder, err := NewEsbuildBuilder(sourceDir, outDir)
	if err != nil {
		t.Fatalf("NewEsbuildBuilder() returned error: %v", err)
	}

	result, err := builder.Build(context.Background(), []string{"index.ts", "index.ts", "./nested/util.ts"})
	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected successful build, got errors: %v", result.Errors)
	}
	if result.BuildID == "" {
		t.Fatal("expected non-empty build id")
	}
	if result.DurationMS < 0 {
		t.Fatalf("unexpected negative duration: %d", result.DurationMS)
	}
	if len(result.ChangedFiles) != 2 {
		t.Fatalf("unexpected changed file count: got %d, want 2", len(result.ChangedFiles))
	}
	if result.ChangedFiles[0] != "index.ts" || result.ChangedFiles[1] != "nested/util.ts" {
		t.Fatalf("unexpected changed files: %v", result.ChangedFiles)
	}

	if _, err := os.Stat(filepath.Join(outDir, "index.js")); err != nil {
		t.Fatalf("expected index.js output file: %v", err)
	}
}

func TestBuildSyntaxErrorReturnsFailureResult(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "src")
	outDir := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}

	writeFixture(t, filepath.Join(sourceDir, "broken.ts"), `export const x = ;`)

	builder, err := NewEsbuildBuilder(sourceDir, outDir)
	if err != nil {
		t.Fatalf("NewEsbuildBuilder() returned error: %v", err)
	}

	result, err := builder.Build(context.Background(), []string{"broken.ts"})
	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failed build result, got success")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected build diagnostics, got none")
	}
	if result.BuildID == "" {
		t.Fatal("expected non-empty build id")
	}
}

func TestBuildNoEntryPoints(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "src")
	outDir := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}

	writeFixture(t, filepath.Join(sourceDir, "README.md"), `nothing to build`)

	builder, err := NewEsbuildBuilder(sourceDir, outDir)
	if err != nil {
		t.Fatalf("NewEsbuildBuilder() returned error: %v", err)
	}

	_, err = builder.Build(context.Background(), nil)
	if err == nil {
		t.Fatal("expected no-entry-points error, got nil")
	}
	if !strings.Contains(err.Error(), "no entry points found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRespectsCanceledContext(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "src")
	outDir := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	writeFixture(t, filepath.Join(sourceDir, "index.ts"), `export const x = 1`)

	builder, err := NewEsbuildBuilder(sourceDir, outDir)
	if err != nil {
		t.Fatalf("NewEsbuildBuilder() returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = builder.Build(ctx, []string{"index.ts"})
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("unexpected cancellation error: %v", err)
	}
}

func writeFixture(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func TestNextBuildIDMonotonic(t *testing.T) {
	sourceDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "dist")
	writeFixture(t, filepath.Join(sourceDir, "entry.ts"), `export const x = 1`)

	builder, err := NewEsbuildBuilder(sourceDir, outDir)
	if err != nil {
		t.Fatalf("NewEsbuildBuilder() returned error: %v", err)
	}

	first := builder.nextBuildID()
	second := builder.nextBuildID()
	if first == second {
		t.Fatalf("expected unique build ids, got %q and %q", first, second)
	}
}
