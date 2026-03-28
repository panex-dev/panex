package inspector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInspect_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	ins := New(dir)

	r, err := ins.Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	if r.PackageManager != nil {
		t.Error("expected nil package manager for empty dir")
	}
	if len(r.Entrypoints) != 0 {
		t.Errorf("expected no entrypoints, got %d", len(r.Entrypoints))
	}
	if len(r.Missing) == 0 {
		t.Error("expected missing requirements for empty dir")
	}

	// Should recommend generating config
	found := false
	for _, rec := range r.Recommended {
		if rec == "generate_panex_config" {
			found = true
		}
	}
	if !found {
		t.Error("expected recommendation to generate panex config")
	}
}

func TestInspect_DetectsPackageManager(t *testing.T) {
	tests := []struct {
		lockfile string
		expected string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"package-lock.json", "npm"},
		{"bun.lockb", "bun"},
	}
	for _, tt := range tests {
		t.Run(tt.lockfile, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "package.json"), `{"name":"test"}`)
			writeFile(t, filepath.Join(dir, tt.lockfile), "")

			r, err := New(dir).Inspect()
			if err != nil {
				t.Fatalf("Inspect: %v", err)
			}
			if r.PackageManager == nil {
				t.Fatal("expected package manager detection")
			}
			if r.PackageManager.Value != tt.expected {
				t.Errorf("got %s, want %s", r.PackageManager.Value, tt.expected)
			}
			if r.PackageManager.Confidence < 0.9 {
				t.Errorf("lockfile detection should have high confidence, got %.2f", r.PackageManager.Confidence)
			}
		})
	}
}

func TestInspect_DetectsLanguage_TypeScript(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tsconfig.json"), `{}`)

	r, err := New(dir).Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if r.Language == nil || r.Language.Value != "typescript" {
		t.Error("expected typescript detection from tsconfig.json")
	}
	if r.Language.Confidence < 0.95 {
		t.Errorf("tsconfig detection should have very high confidence, got %.2f", r.Language.Confidence)
	}
}

func TestInspect_DetectsFramework(t *testing.T) {
	tests := []struct {
		dep      string
		expected string
	}{
		{"react", "react"},
		{"vue", "vue"},
		{"svelte", "svelte"},
		{"solid-js", "solid"},
		{"preact", "preact"},
	}
	for _, tt := range tests {
		t.Run(tt.dep, func(t *testing.T) {
			dir := t.TempDir()
			pkg := map[string]any{
				"name":         "test",
				"dependencies": map[string]string{tt.dep: "^1.0.0"},
			}
			data, _ := json.Marshal(pkg)
			writeFile(t, filepath.Join(dir, "package.json"), string(data))

			r, err := New(dir).Inspect()
			if err != nil {
				t.Fatalf("Inspect: %v", err)
			}
			if r.Framework == nil {
				t.Fatal("expected framework detection")
			}
			if r.Framework.Value != tt.expected {
				t.Errorf("got %s, want %s", r.Framework.Value, tt.expected)
			}
		})
	}
}

func TestInspect_DetectsBundler_ConfigFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "vite.config.ts"), "export default {}")

	r, err := New(dir).Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if r.Bundler == nil {
		t.Fatal("expected bundler detection")
	}
	if r.Bundler.Value != "vite" {
		t.Errorf("got %s, want vite", r.Bundler.Value)
	}
	if r.Bundler.Confidence < 0.95 {
		t.Errorf("config file detection should have very high confidence, got %.2f", r.Bundler.Confidence)
	}
}

func TestInspect_DetectsBundler_PackageJSON(t *testing.T) {
	dir := t.TempDir()
	pkg := map[string]any{
		"name":            "test",
		"devDependencies": map[string]string{"webpack": "^5.0.0"},
	}
	data, _ := json.Marshal(pkg)
	writeFile(t, filepath.Join(dir, "package.json"), string(data))

	r, err := New(dir).Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if r.Bundler == nil {
		t.Fatal("expected bundler detection from package.json")
	}
	if r.Bundler.Value != "webpack" {
		t.Errorf("got %s, want webpack", r.Bundler.Value)
	}
}

func TestInspect_DetectsEntrypoints_FromConvention(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src", "background"), 0o755)
	os.MkdirAll(filepath.Join(dir, "src", "popup"), 0o755)
	writeFile(t, filepath.Join(dir, "src", "background", "index.ts"), "// bg")
	writeFile(t, filepath.Join(dir, "src", "popup", "main.tsx"), "// popup")

	r, err := New(dir).Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	bg, ok := r.Entrypoints["background"]
	if !ok {
		t.Fatal("expected background entrypoint")
	}
	if bg.Path != "src/background/index.ts" {
		t.Errorf("background path: got %s, want src/background/index.ts", bg.Path)
	}
	if bg.Type != "service-worker" {
		t.Errorf("background type: got %s, want service-worker", bg.Type)
	}
	if bg.Classification != Inferred {
		t.Errorf("convention-based should be inferred, got %s", bg.Classification)
	}

	popup, ok := r.Entrypoints["popup"]
	if !ok {
		t.Fatal("expected popup entrypoint")
	}
	if popup.Path != "src/popup/main.tsx" {
		t.Errorf("popup path: got %s, want src/popup/main.tsx", popup.Path)
	}
}

func TestInspect_DetectsEntrypoints_FromManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := map[string]any{
		"manifest_version": 3,
		"name":             "Test Extension",
		"version":          "1.0.0",
		"background": map[string]any{
			"service_worker": "background.js",
		},
		"action": map[string]any{
			"default_popup": "popup.html",
		},
		"side_panel": map[string]any{
			"default_path": "sidepanel.html",
		},
	}
	data, _ := json.Marshal(manifest)
	writeFile(t, filepath.Join(dir, "manifest.json"), string(data))

	r, err := New(dir).Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	bg := r.Entrypoints["background"]
	if bg.Path != "background.js" {
		t.Errorf("background: got %s, want background.js", bg.Path)
	}
	if bg.Classification != Declared {
		t.Errorf("manifest-based should be declared, got %s", bg.Classification)
	}
	if bg.Confidence < 0.99 {
		t.Errorf("manifest-based should have highest confidence, got %.2f", bg.Confidence)
	}

	popup := r.Entrypoints["popup"]
	if popup.Path != "popup.html" {
		t.Errorf("popup: got %s, want popup.html", popup.Path)
	}

	sp := r.Entrypoints["side_panel"]
	if sp.Path != "sidepanel.html" {
		t.Errorf("side_panel: got %s, want sidepanel.html", sp.Path)
	}
}

func TestInspect_DetectsTargets_FromManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := map[string]any{
		"manifest_version": 3,
		"background": map[string]any{
			"service_worker": "bg.js",
		},
		"browser_specific_settings": map[string]any{
			"gecko": map[string]any{"id": "ext@example.com"},
		},
	}
	data, _ := json.Marshal(manifest)
	writeFile(t, filepath.Join(dir, "manifest.json"), string(data))

	r, err := New(dir).Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	targets := make(map[string]bool)
	for _, t := range r.Targets {
		targets[t.Value] = true
	}
	if !targets["chrome"] {
		t.Error("expected chrome target from service_worker")
	}
	if !targets["firefox"] {
		t.Error("expected firefox target from browser_specific_settings")
	}
}

func TestInspect_DefaultTarget_Chrome(t *testing.T) {
	dir := t.TempDir()

	r, err := New(dir).Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	if len(r.Targets) != 1 || r.Targets[0].Value != "chrome" {
		t.Errorf("expected default chrome target, got %+v", r.Targets)
	}
	if r.Targets[0].Classification != Inferred {
		t.Errorf("default target should be inferred, got %s", r.Targets[0].Classification)
	}
}

func TestInspect_FullProject(t *testing.T) {
	dir := t.TempDir()

	// Set up a realistic project
	pkg := map[string]any{
		"name":            "tab-organizer",
		"dependencies":    map[string]string{"react": "^18.0.0", "react-dom": "^18.0.0"},
		"devDependencies": map[string]string{"typescript": "^5.0.0"},
	}
	data, _ := json.Marshal(pkg)
	writeFile(t, filepath.Join(dir, "package.json"), string(data))
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "lockfileVersion: 9")
	writeFile(t, filepath.Join(dir, "tsconfig.json"), `{"compilerOptions":{}}`)
	writeFile(t, filepath.Join(dir, "vite.config.ts"), "export default {}")

	manifest := map[string]any{
		"manifest_version": 3,
		"name":             "Tab Organizer",
		"version":          "1.0.0",
		"background":       map[string]any{"service_worker": "src/background/index.ts"},
		"action":           map[string]any{"default_popup": "src/popup/index.html"},
	}
	mdata, _ := json.Marshal(manifest)
	writeFile(t, filepath.Join(dir, "manifest.json"), string(mdata))

	r, err := New(dir).Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	// Verify all detections
	if r.PackageManager == nil || r.PackageManager.Value != "pnpm" {
		t.Error("expected pnpm")
	}
	if r.Language == nil || r.Language.Value != "typescript" {
		t.Error("expected typescript")
	}
	if r.Framework == nil || r.Framework.Value != "react" {
		t.Error("expected react")
	}
	if r.Bundler == nil || r.Bundler.Value != "vite" {
		t.Error("expected vite")
	}

	if _, ok := r.Entrypoints["background"]; !ok {
		t.Error("expected background entrypoint")
	}
	if _, ok := r.Entrypoints["popup"]; !ok {
		t.Error("expected popup entrypoint")
	}

	// No missing requirements for a well-formed project
	for _, m := range r.Missing {
		if m == "extension_entrypoints" || m == "background_entrypoint" {
			t.Errorf("unexpected missing requirement: %s", m)
		}
	}
}

func TestInspect_WorkspaceDetection(t *testing.T) {
	t.Run("pnpm workspace", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "pnpm-workspace.yaml"), "packages:\n  - packages/*")

		r, _ := New(dir).Inspect()
		if r.WorkspaceType == nil || r.WorkspaceType.Value != "monorepo" {
			t.Error("expected monorepo detection from pnpm-workspace.yaml")
		}
	})

	t.Run("npm workspaces", func(t *testing.T) {
		dir := t.TempDir()
		pkg := map[string]any{
			"name":       "root",
			"workspaces": []string{"packages/*"},
		}
		data, _ := json.Marshal(pkg)
		writeFile(t, filepath.Join(dir, "package.json"), string(data))

		r, _ := New(dir).Inspect()
		if r.WorkspaceType == nil || r.WorkspaceType.Value != "monorepo" {
			t.Error("expected monorepo detection from package.json workspaces")
		}
	})

	t.Run("single package", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "package.json"), `{"name":"single"}`)

		r, _ := New(dir).Inspect()
		if r.WorkspaceType == nil || r.WorkspaceType.Value != "single-package" {
			t.Error("expected single-package for project without workspaces")
		}
	})
}

// --- helpers ---

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
