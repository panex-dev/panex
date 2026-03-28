package inspector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Inspector scans a project directory and produces a Report.
type Inspector struct {
	root string
}

// New creates an Inspector rooted at the given directory.
func New(root string) *Inspector {
	return &Inspector{root: root}
}

// Inspect runs all detection passes and returns a Report.
// It never mutates the project.
func (ins *Inspector) Inspect() (*Report, error) {
	r := &Report{
		Entrypoints: make(map[string]EntryCandidate),
	}

	pkg, err := ins.readPackageJSON()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	ins.detectPackageManager(r)
	ins.detectLanguage(r)
	ins.detectFramework(r, pkg)
	ins.detectBundler(r, pkg)
	ins.detectWorkspaceType(r, pkg)
	ins.detectEntrypoints(r, pkg)
	ins.detectTargets(r, pkg)
	ins.checkMissing(r)
	ins.computeRecommendations(r)

	return r, nil
}

// --- package.json reading ---

type packageJSON struct {
	Name            string            `json:"name"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Workspaces      json.RawMessage   `json:"workspaces"`
}

func (ins *Inspector) readPackageJSON() (*packageJSON, error) {
	data, err := os.ReadFile(filepath.Join(ins.root, "package.json"))
	if err != nil {
		return nil, err
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

// --- detection passes ---

func (ins *Inspector) detectPackageManager(r *Report) {
	checks := []struct {
		lockfile string
		manager  string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"bun.lockb", "bun"},
		{"yarn.lock", "yarn"},
		{"package-lock.json", "npm"},
	}
	for _, c := range checks {
		if fileExists(filepath.Join(ins.root, c.lockfile)) {
			r.PackageManager = &Finding[string]{
				Value:          c.manager,
				Source:         "lockfile:" + c.lockfile,
				Confidence:     0.99,
				Classification: Detected,
			}
			return
		}
	}
	if fileExists(filepath.Join(ins.root, "package.json")) {
		r.PackageManager = &Finding[string]{
			Value:          "npm",
			Source:         "package.json_exists",
			Confidence:     0.5,
			Classification: Inferred,
		}
	}
}

func (ins *Inspector) detectLanguage(r *Report) {
	if fileExists(filepath.Join(ins.root, "tsconfig.json")) {
		r.Language = &Finding[string]{
			Value:          "typescript",
			Source:         "tsconfig.json",
			Confidence:     0.99,
			Classification: Detected,
		}
		return
	}
	if fileExists(filepath.Join(ins.root, "jsconfig.json")) {
		r.Language = &Finding[string]{
			Value:          "javascript",
			Source:         "jsconfig.json",
			Confidence:     0.95,
			Classification: Detected,
		}
		return
	}
	// Walk top-level src/ for ts/js files
	srcDir := filepath.Join(ins.root, "src")
	ts, js := 0, 0
	_ = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".ts", ".tsx":
			ts++
		case ".js", ".jsx":
			js++
		}
		return nil
	})
	if ts > 0 || js > 0 {
		if ts >= js {
			r.Language = &Finding[string]{
				Value:          "typescript",
				Source:         "file_extension_count",
				Confidence:     0.7,
				Classification: Inferred,
			}
		} else {
			r.Language = &Finding[string]{
				Value:          "javascript",
				Source:         "file_extension_count",
				Confidence:     0.7,
				Classification: Inferred,
			}
		}
	}
}

func (ins *Inspector) detectFramework(r *Report, pkg *packageJSON) {
	if pkg == nil {
		return
	}
	allDeps := mergeMaps(pkg.Dependencies, pkg.DevDependencies)

	frameworks := []struct {
		dep        string
		name       string
		confidence float64
	}{
		{"react", "react", 0.95},
		{"react-dom", "react", 0.95},
		{"vue", "vue", 0.95},
		{"svelte", "svelte", 0.95},
		{"solid-js", "solid", 0.95},
		{"preact", "preact", 0.95},
		{"lit", "lit", 0.90},
		{"@anthropic-ai/sdk", "none", 0.0}, // not a UI framework
	}

	for _, f := range frameworks {
		if _, ok := allDeps[f.dep]; ok && f.confidence > 0 {
			r.Framework = &Finding[string]{
				Value:          f.name,
				Source:         "package.json:" + f.dep,
				Confidence:     f.confidence,
				Classification: Detected,
			}
			return
		}
	}
}

func (ins *Inspector) detectBundler(r *Report, pkg *packageJSON) {
	// Check config files first (highest confidence)
	bundlerConfigs := []struct {
		pattern string
		name    string
	}{
		{"vite.config.*", "vite"},
		{"webpack.config.*", "webpack"},
		{"rollup.config.*", "rollup"},
		{"esbuild.config.*", "esbuild"},
		{"rspack.config.*", "rspack"},
		{"tsup.config.*", "tsup"},
	}
	for _, bc := range bundlerConfigs {
		matches, _ := filepath.Glob(filepath.Join(ins.root, bc.pattern))
		if len(matches) > 0 {
			r.Bundler = &Finding[string]{
				Value:          bc.name,
				Source:         "config_file:" + filepath.Base(matches[0]),
				Confidence:     0.98,
				Classification: Detected,
			}
			return
		}
	}

	// Fall back to package.json deps
	if pkg == nil {
		return
	}
	allDeps := mergeMaps(pkg.Dependencies, pkg.DevDependencies)
	bundlerDeps := []struct {
		dep  string
		name string
	}{
		{"vite", "vite"},
		{"webpack", "webpack"},
		{"rollup", "rollup"},
		{"esbuild", "esbuild"},
		{"rspack", "rspack"},
		{"tsup", "tsup"},
		{"parcel", "parcel"},
	}
	for _, bd := range bundlerDeps {
		if _, ok := allDeps[bd.dep]; ok {
			r.Bundler = &Finding[string]{
				Value:          bd.name,
				Source:         "package.json:" + bd.dep,
				Confidence:     0.85,
				Classification: Detected,
			}
			return
		}
	}
}

func (ins *Inspector) detectWorkspaceType(r *Report, pkg *packageJSON) {
	if pkg != nil && len(pkg.Workspaces) > 0 {
		r.WorkspaceType = &Finding[string]{
			Value:          "monorepo",
			Source:         "package.json:workspaces",
			Confidence:     0.95,
			Classification: Detected,
		}
		return
	}
	if fileExists(filepath.Join(ins.root, "pnpm-workspace.yaml")) {
		r.WorkspaceType = &Finding[string]{
			Value:          "monorepo",
			Source:         "pnpm-workspace.yaml",
			Confidence:     0.95,
			Classification: Detected,
		}
		return
	}
	r.WorkspaceType = &Finding[string]{
		Value:          "single-package",
		Source:         "no_workspace_markers",
		Confidence:     0.8,
		Classification: Inferred,
	}
}

func (ins *Inspector) detectEntrypoints(r *Report, pkg *packageJSON) {
	// 1. Check for existing manifest.json
	manifestPath := ins.findManifestJSON()
	if manifestPath != "" {
		ins.detectEntrypointsFromManifest(r, manifestPath)
		return
	}

	// 2. Scan common directory patterns
	bgCandidates := []string{
		"src/background/index.ts",
		"src/background/index.js",
		"src/background.ts",
		"src/background.js",
		"src/sw.ts",
		"src/service-worker.ts",
		"background.ts",
		"background.js",
	}
	for _, c := range bgCandidates {
		if fileExists(filepath.Join(ins.root, c)) {
			r.Entrypoints["background"] = EntryCandidate{
				Path:           c,
				Type:           "service-worker",
				Source:         "directory_convention",
				Confidence:     0.75,
				Classification: Inferred,
			}
			break
		}
	}

	popupCandidates := []string{
		"src/popup/index.html",
		"src/popup/main.tsx",
		"src/popup/main.ts",
		"src/popup/index.tsx",
		"src/popup/index.ts",
		"src/popup.html",
		"popup.html",
		"popup/index.html",
	}
	for _, c := range popupCandidates {
		if fileExists(filepath.Join(ins.root, c)) {
			typ := "html-app"
			if strings.HasSuffix(c, ".html") {
				typ = "html-page"
			}
			r.Entrypoints["popup"] = EntryCandidate{
				Path:           c,
				Type:           typ,
				Source:         "directory_convention",
				Confidence:     0.70,
				Classification: Inferred,
			}
			break
		}
	}

	optionsCandidates := []string{
		"src/options/index.html",
		"src/options/main.tsx",
		"src/options/main.ts",
		"src/options.html",
		"options.html",
	}
	for _, c := range optionsCandidates {
		if fileExists(filepath.Join(ins.root, c)) {
			r.Entrypoints["options"] = EntryCandidate{
				Path:           c,
				Type:           "html-app",
				Source:         "directory_convention",
				Confidence:     0.65,
				Classification: Inferred,
			}
			break
		}
	}

	contentCandidates := []string{
		"src/content/index.ts",
		"src/content/index.js",
		"src/content.ts",
		"src/content.js",
		"src/content-script.ts",
		"src/content-script.js",
	}
	for _, c := range contentCandidates {
		if fileExists(filepath.Join(ins.root, c)) {
			r.Entrypoints["content_script"] = EntryCandidate{
				Path:           c,
				Type:           "content-script",
				Source:         "directory_convention",
				Confidence:     0.65,
				Classification: Inferred,
			}
			break
		}
	}
}

func (ins *Inspector) detectEntrypointsFromManifest(r *Report, manifestPath string) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}

	// Background
	if raw, ok := m["background"]; ok {
		var bg struct {
			ServiceWorker string `json:"service_worker"`
			Scripts       []string `json:"scripts"`
			Page          string `json:"page"`
		}
		if json.Unmarshal(raw, &bg) == nil {
			if bg.ServiceWorker != "" {
				r.Entrypoints["background"] = EntryCandidate{
					Path: bg.ServiceWorker, Type: "service-worker",
					Source: "manifest.json", Confidence: 0.99, Classification: Declared,
				}
			} else if bg.Page != "" {
				r.Entrypoints["background"] = EntryCandidate{
					Path: bg.Page, Type: "background-page",
					Source: "manifest.json", Confidence: 0.99, Classification: Declared,
				}
			} else if len(bg.Scripts) > 0 {
				r.Entrypoints["background"] = EntryCandidate{
					Path: bg.Scripts[0], Type: "background-page",
					Source: "manifest.json", Confidence: 0.99, Classification: Declared,
				}
			}
		}
	}

	// Popup
	if raw, ok := m["action"]; ok {
		var action struct {
			DefaultPopup string `json:"default_popup"`
		}
		if json.Unmarshal(raw, &action) == nil && action.DefaultPopup != "" {
			r.Entrypoints["popup"] = EntryCandidate{
				Path: action.DefaultPopup, Type: "html-page",
				Source: "manifest.json", Confidence: 0.99, Classification: Declared,
			}
		}
	} else if raw, ok := m["browser_action"]; ok {
		var ba struct {
			DefaultPopup string `json:"default_popup"`
		}
		if json.Unmarshal(raw, &ba) == nil && ba.DefaultPopup != "" {
			r.Entrypoints["popup"] = EntryCandidate{
				Path: ba.DefaultPopup, Type: "html-page",
				Source: "manifest.json", Confidence: 0.99, Classification: Declared,
			}
		}
	}

	// Options
	if raw, ok := m["options_ui"]; ok {
		var opts struct {
			Page string `json:"page"`
		}
		if json.Unmarshal(raw, &opts) == nil && opts.Page != "" {
			r.Entrypoints["options"] = EntryCandidate{
				Path: opts.Page, Type: "html-page",
				Source: "manifest.json", Confidence: 0.99, Classification: Declared,
			}
		}
	} else if raw, ok := m["options_page"]; ok {
		var page string
		if json.Unmarshal(raw, &page) == nil && page != "" {
			r.Entrypoints["options"] = EntryCandidate{
				Path: page, Type: "html-page",
				Source: "manifest.json", Confidence: 0.99, Classification: Declared,
			}
		}
	}

	// Side panel
	if raw, ok := m["side_panel"]; ok {
		var sp struct {
			DefaultPath string `json:"default_path"`
		}
		if json.Unmarshal(raw, &sp) == nil && sp.DefaultPath != "" {
			r.Entrypoints["side_panel"] = EntryCandidate{
				Path: sp.DefaultPath, Type: "html-page",
				Source: "manifest.json", Confidence: 0.99, Classification: Declared,
			}
		}
	}

	// Content scripts
	if raw, ok := m["content_scripts"]; ok {
		var scripts []struct {
			JS      []string `json:"js"`
			Matches []string `json:"matches"`
		}
		if json.Unmarshal(raw, &scripts) == nil && len(scripts) > 0 && len(scripts[0].JS) > 0 {
			r.Entrypoints["content_script"] = EntryCandidate{
				Path: scripts[0].JS[0], Type: "content-script",
				Source: "manifest.json", Confidence: 0.99, Classification: Declared,
			}
		}
	}
}

func (ins *Inspector) detectTargets(r *Report, pkg *packageJSON) {
	// Check for existing manifest.json manifest_version field
	manifestPath := ins.findManifestJSON()
	if manifestPath != "" {
		data, _ := os.ReadFile(manifestPath)
		var m map[string]json.RawMessage
		if json.Unmarshal(data, &m) == nil {
			if _, ok := m["manifest_version"]; ok {
				// MV3 with service_worker → Chrome-first
				if raw, bgOk := m["background"]; bgOk {
					var bg struct {
						ServiceWorker string `json:"service_worker"`
					}
					if json.Unmarshal(raw, &bg) == nil && bg.ServiceWorker != "" {
						r.Targets = append(r.Targets, Finding[string]{
							Value: "chrome", Source: "manifest.json:service_worker",
							Confidence: 0.95, Classification: Detected,
						})
					}
				}
				// browser_specific_settings → Firefox
				if _, ok := m["browser_specific_settings"]; ok {
					r.Targets = append(r.Targets, Finding[string]{
						Value: "firefox", Source: "manifest.json:browser_specific_settings",
						Confidence: 0.95, Classification: Detected,
					})
				}
			}
		}
	}

	// If no targets detected, default to Chrome
	if len(r.Targets) == 0 {
		r.Targets = append(r.Targets, Finding[string]{
			Value: "chrome", Source: "default",
			Confidence: 0.5, Classification: Inferred,
		})
	}
}

func (ins *Inspector) checkMissing(r *Report) {
	if !fileExists(filepath.Join(ins.root, "package.json")) {
		r.Missing = append(r.Missing, "package.json")
	}
	if r.PackageManager == nil {
		r.Missing = append(r.Missing, "package_manager_lockfile")
	}
	if len(r.Entrypoints) == 0 {
		r.Missing = append(r.Missing, "extension_entrypoints")
	}
	if _, ok := r.Entrypoints["background"]; !ok {
		r.Missing = append(r.Missing, "background_entrypoint")
	}
}

func (ins *Inspector) computeRecommendations(r *Report) {
	if !fileExists(filepath.Join(ins.root, "panex.config.ts")) {
		r.Recommended = append(r.Recommended, "generate_panex_config")
	}
	if !fileExists(filepath.Join(ins.root, "panex.policy.yaml")) {
		r.Recommended = append(r.Recommended, "generate_panex_policy")
	}

	hasFirefox := false
	for _, t := range r.Targets {
		if t.Value == "firefox" {
			hasFirefox = true
		}
	}
	if !hasFirefox {
		r.Recommended = append(r.Recommended, "add_firefox_target")
	}

	if _, ok := r.Entrypoints["background"]; !ok {
		r.Recommended = append(r.Recommended, "add_background_entrypoint")
	}
}

// --- manifest.json discovery ---

func (ins *Inspector) findManifestJSON() string {
	candidates := []string{
		filepath.Join(ins.root, "manifest.json"),
		filepath.Join(ins.root, "src", "manifest.json"),
		filepath.Join(ins.root, "public", "manifest.json"),
		filepath.Join(ins.root, "static", "manifest.json"),
	}
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	return ""
}

// --- helpers ---

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func mergeMaps(a, b map[string]string) map[string]string {
	out := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
