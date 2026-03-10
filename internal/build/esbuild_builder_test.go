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
	nestedSourceDir := filepath.Join(tmpDir, "src")
	if err := os.WriteFile(plainFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	if err := os.MkdirAll(nestedSourceDir, 0o755); err != nil {
		t.Fatalf("create nested source directory: %v", err)
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
			wantError: "source and output directories must not overlap",
		},
		{
			name:      "output nested within source directory",
			sourceDir: tmpDir,
			outDir:    filepath.Join(tmpDir, "dist"),
			wantError: "source and output directories must not overlap",
		},
		{
			name:      "source nested within output directory",
			sourceDir: nestedSourceDir,
			outDir:    tmpDir,
			wantError: "source and output directories must not overlap",
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

func TestBuildCopiesHTMLAndInjectsChromeSim(t *testing.T) {
	rootDir := t.TempDir()
	sourceDir := filepath.Join(rootDir, "src")
	outDir := filepath.Join(rootDir, "dist")
	simDir := filepath.Join(rootDir, "chrome-sim")
	if err := os.MkdirAll(filepath.Join(sourceDir, "pages"), 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	if err := os.MkdirAll(simDir, 0o755); err != nil {
		t.Fatalf("create chrome-sim fixture directory: %v", err)
	}

	writeFixture(t, filepath.Join(sourceDir, "popup.ts"), `console.log("popup")`)
	writeFixture(t, filepath.Join(sourceDir, "pages", "options.ts"), `console.log("options")`)
	writeFixture(t, filepath.Join(sourceDir, "popup.html"), `<!doctype html><html><head><title>Popup</title></head><body><script type="module" src="./popup.ts"></script></body></html>`)
	writeFixture(t, filepath.Join(sourceDir, "pages", "options.html"), `<!doctype html><html><head><title>Options</title></head><body><script type="module" src="./options.ts"></script></body></html>`)
	writeFixture(t, filepath.Join(simDir, "entry.ts"), `console.log("chrome-sim")`)

	builder, err := NewEsbuildBuilder(
		sourceDir,
		outDir,
		WithChromeSimInjection(ChromeSimInjectionOptions{
			AuthToken:        "dev-token",
			DaemonURL:        "ws://127.0.0.1:4317/ws",
			ModuleOutputName: "chrome-sim.js",
			ModuleSourcePath: filepath.Join(simDir, "entry.ts"),
		}),
	)
	if err != nil {
		t.Fatalf("NewEsbuildBuilder() returned error: %v", err)
	}

	result, err := builder.Build(context.Background(), []string{"popup.html", "pages/options.html"})
	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected successful build, got errors: %v", result.Errors)
	}

	popupHTML := readFixture(t, filepath.Join(outDir, "popup.html"))
	if !strings.Contains(popupHTML, `src="./popup.js"`) {
		t.Fatalf("expected popup html to reference bundled popup.js, got %q", popupHTML)
	}
	if !strings.Contains(popupHTML, `src="./chrome-sim.js"`) {
		t.Fatalf("expected popup html to inject chrome-sim.js, got %q", popupHTML)
	}
	if !strings.Contains(popupHTML, `data-panex-token="dev-token"`) {
		t.Fatalf("expected popup html to include panex token bootstrap, got %q", popupHTML)
	}

	optionsHTML := readFixture(t, filepath.Join(outDir, "pages", "options.html"))
	if !strings.Contains(optionsHTML, `src="./options.js"`) {
		t.Fatalf("expected options html to reference bundled options.js, got %q", optionsHTML)
	}
	if !strings.Contains(optionsHTML, `src="../chrome-sim.js"`) {
		t.Fatalf("expected nested html to use relative chrome-sim path, got %q", optionsHTML)
	}

	if _, err := os.Stat(filepath.Join(outDir, "chrome-sim.js")); err != nil {
		t.Fatalf("expected chrome-sim.js output file: %v", err)
	}
}

func TestBuildCopiesStaticExtensionAssets(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "src")
	outDir := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(filepath.Join(sourceDir, "icons"), 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "styles"), 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}

	writeFixture(t, filepath.Join(sourceDir, "background.ts"), `console.log("background");`)
	writeFixture(t, filepath.Join(sourceDir, "popup.ts"), `console.log("popup");`)
	writeFixture(t, filepath.Join(sourceDir, "popup.html"), `<!doctype html><html><head><link rel="stylesheet" href="./styles/popup.css"></head><body><script type="module" src="./popup.ts"></script></body></html>`)
	writeFixture(t, filepath.Join(sourceDir, "styles", "popup.css"), `body { color: #102033; }`)
	writeFixture(t, filepath.Join(sourceDir, "icons", "icon-16.png"), "png-bits")
	writeFixture(t, filepath.Join(sourceDir, "manifest.json"), `{
  "manifest_version": 3,
  "name": "Panex Test Extension",
  "version": "0.0.1",
  "background": {
    "service_worker": "background.js"
  },
  "action": {
    "default_popup": "popup.html"
  },
  "icons": {
    "16": "icons/icon-16.png"
  }
}`)

	builder, err := NewEsbuildBuilder(sourceDir, outDir)
	if err != nil {
		t.Fatalf("NewEsbuildBuilder() returned error: %v", err)
	}

	result, err := builder.Build(context.Background(), []string{"manifest.json", "icons/icon-16.png"})
	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected successful build, got errors: %v", result.Errors)
	}

	if got := readFixture(t, filepath.Join(outDir, "manifest.json")); !strings.Contains(got, `"service_worker": "background.js"`) {
		t.Fatalf("expected copied manifest.json in output, got %q", got)
	}
	if got := readFixture(t, filepath.Join(outDir, "styles", "popup.css")); got != `body { color: #102033; }` {
		t.Fatalf("expected copied popup.css, got %q", got)
	}
	if got := readFixture(t, filepath.Join(outDir, "icons", "icon-16.png")); got != "png-bits" {
		t.Fatalf("expected copied icon asset, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(outDir, "background.js")); err != nil {
		t.Fatalf("expected bundled background.js output file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "popup.js")); err != nil {
		t.Fatalf("expected bundled popup.js output file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "background.ts")); !os.IsNotExist(err) {
		t.Fatalf("expected source background.ts not to be copied, got err=%v", err)
	}
}

func TestBuildDoesNotDuplicateExistingChromeSimInjection(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "src")
	outDir := filepath.Join(t.TempDir(), "dist")
	simDir := filepath.Join(t.TempDir(), "chrome-sim")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	if err := os.MkdirAll(simDir, 0o755); err != nil {
		t.Fatalf("create chrome-sim fixture directory: %v", err)
	}

	writeFixture(t, filepath.Join(sourceDir, "popup.ts"), `console.log("popup")`)
	writeFixture(t, filepath.Join(sourceDir, "popup.html"), `<!doctype html><html><head><script data-panex-chrome-sim="1" src="./chrome-sim.js"></script></head><body><script type="module" src="./popup.ts"></script></body></html>`)
	writeFixture(t, filepath.Join(simDir, "entry.ts"), `console.log("chrome-sim")`)

	builder, err := NewEsbuildBuilder(
		sourceDir,
		outDir,
		WithChromeSimInjection(ChromeSimInjectionOptions{
			DaemonURL:        "ws://127.0.0.1:4317/ws",
			ModuleOutputName: "chrome-sim.js",
			ModuleSourcePath: filepath.Join(simDir, "entry.ts"),
		}),
	)
	if err != nil {
		t.Fatalf("NewEsbuildBuilder() returned error: %v", err)
	}

	result, err := builder.Build(context.Background(), []string{"popup.html"})
	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected successful build, got errors: %v", result.Errors)
	}

	popupHTML := readFixture(t, filepath.Join(outDir, "popup.html"))
	if strings.Count(popupHTML, "data-panex-chrome-sim") != 1 {
		t.Fatalf("expected single chrome-sim injection marker, got %q", popupHTML)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	value, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return string(value)
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

func TestAutoDetectChromeSimInjection(t *testing.T) {
	rootDir := t.TempDir()
	sourceDir := filepath.Join(rootDir, "agent", "src")
	chromeSimSource := filepath.Join(rootDir, "shared", "chrome-sim", "src")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.MkdirAll(chromeSimSource, 0o755); err != nil {
		t.Fatalf("create chrome-sim source dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(rootDir, "agent", "node_modules"), 0o755); err != nil {
		t.Fatalf("create agent node_modules: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(rootDir, "shared", "chrome-sim", "node_modules"), 0o755); err != nil {
		t.Fatalf("create chrome-sim node_modules: %v", err)
	}
	writeFixture(t, filepath.Join(chromeSimSource, "index.ts"), `console.log("chrome-sim")`)

	options, ok := AutoDetectChromeSimInjection(sourceDir, "ws://127.0.0.1:4317/ws", "dev-token", "ext-1")
	if !ok {
		t.Fatal("expected chrome-sim injection auto-detection to succeed")
	}
	if options.ModuleSourcePath != filepath.Join(chromeSimSource, "index.ts") {
		t.Fatalf("unexpected module source path: got %q", options.ModuleSourcePath)
	}
	if options.ModuleOutputName != "chrome-sim.js" {
		t.Fatalf("unexpected module output name: got %q", options.ModuleOutputName)
	}
	if len(options.NodePaths) != 2 {
		t.Fatalf("unexpected node path count: got %d, want 2 (%v)", len(options.NodePaths), options.NodePaths)
	}
}
