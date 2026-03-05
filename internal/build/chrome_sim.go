package build

import (
	"os"
	"path/filepath"
	"strings"
)

func AutoDetectChromeSimInjection(
	sourceDir string,
	daemonURL string,
	authToken string,
	extensionID string,
) (ChromeSimInjectionOptions, bool) {
	absSourceDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return ChromeSimInjectionOptions{}, false
	}

	moduleSource, chromeSimDir, ok := locateChromeSimSource(absSourceDir)
	if !ok {
		return ChromeSimInjectionOptions{}, false
	}

	return ChromeSimInjectionOptions{
		AuthToken:        authToken,
		DaemonURL:        daemonURL,
		ExtensionID:      extensionID,
		ModuleOutputName: "chrome-sim.js",
		ModuleSourcePath: moduleSource,
		NodePaths:        collectNodePaths(absSourceDir, chromeSimDir),
	}, true
}

func locateChromeSimSource(sourceDir string) (string, string, bool) {
	for dir := sourceDir; ; dir = filepath.Dir(dir) {
		chromeSimDir := filepath.Join(dir, "shared", "chrome-sim")
		moduleSource := filepath.Join(chromeSimDir, "src", "index.ts")
		if info, err := os.Stat(moduleSource); err == nil && !info.IsDir() {
			return moduleSource, chromeSimDir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false
		}
	}
}

func collectNodePaths(sourceDir, chromeSimDir string) []string {
	paths := make([]string, 0, 4)
	seen := map[string]struct{}{}

	addPath := func(dir string) {
		trimmed := strings.TrimSpace(dir)
		if trimmed == "" {
			return
		}
		info, err := os.Stat(trimmed)
		if err != nil || !info.IsDir() {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		paths = append(paths, trimmed)
	}

	for dir := sourceDir; ; dir = filepath.Dir(dir) {
		addPath(filepath.Join(dir, "node_modules"))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	addPath(filepath.Join(chromeSimDir, "node_modules"))

	return paths
}
