package target

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Chrome implements the Adapter interface for the Chrome target.
type Chrome struct{}

var _ Adapter = (*Chrome)(nil)

func NewChrome() *Chrome { return &Chrome{} }

func (c *Chrome) Name() string { return "chrome" }

func (c *Chrome) Catalog() CapabilityCatalog {
	return CapabilityCatalog{
		Target: "chrome",
		Capabilities: map[string]CapabilitySupport{
			"tabs":                {State: "native", Permission: "tabs"},
			"windows":             {State: "native", Permission: "windows"}, //nolint:misspell
			"storage":             {State: "native", Permission: "storage"},
			"scripting":           {State: "native", Permission: "scripting"},
			"content":             {State: "native"},
			"commands":            {State: "native"},
			"alarms":              {State: "native", Permission: "alarms"},
			"notifications":       {State: "native", Permission: "notifications"},
			"downloads":           {State: "native", Permission: "downloads"},
			"clipboard":           {State: "native", Permission: "clipboardRead"},
			"contextMenus":        {State: "native", Permission: "contextMenus"},
			"identity":            {State: "native", Permission: "identity"},
			"networkRules":        {State: "native", Permission: "declarativeNetRequest"},
			"devtools":            {State: "native", Permission: "devtools"},
			"omnibox":             {State: "native"},
			"sideSurface":         {State: "native", Permission: "sidePanel"},
			"sidebarSurface":      {State: "blocked", Notes: "Chrome uses sidePanel, not sidebar"},
			"offscreenExecution":  {State: "native", Permission: "offscreen"},
			"nativeMessaging":     {State: "native", Permission: "nativeMessaging"},
			"hostAccess":          {State: "native"},
			"backgroundExecution": {State: "native", Notes: "service worker model (MV3)"},
			"sessionState":        {State: "native", Permission: "storage"},
			"capture":             {State: "native", Permission: "tabCapture"},
			"cookies":             {State: "native", Permission: "cookies"},
			"history":             {State: "native", Permission: "history"},
			"bookmarks":           {State: "native", Permission: "bookmarks"},
		},
	}
}

func (c *Chrome) InspectEnvironment(ctx context.Context) (EnvironmentInfo, Result) {
	info := EnvironmentInfo{}
	binary := findChromeBinary()
	if binary == "" {
		info.Reason = "no Chrome binary found"
		return info, Result{
			Adapter:     "chrome",
			Operation:   "inspectEnvironment",
			Outcome:     EnvironmentMissing,
			Reason:      "no Chrome binary found in standard locations",
			ReasonCode:  "chrome_not_found",
			Suggestions: []string{"install Google Chrome", "set CHROME_PATH environment variable"},
			Repairable:  false,
		}
	}

	info.Available = true
	info.BinaryPath = binary
	info.Launchable = true

	// Try to get version
	cmd := exec.CommandContext(ctx, binary, "--version")
	out, err := cmd.Output()
	if err == nil {
		info.Version = strings.TrimSpace(string(out))
	}

	return info, Result{
		Adapter:   "chrome",
		Operation: "inspectEnvironment",
		Outcome:   Success,
		Details:   info,
	}
}

func (c *Chrome) ResolveCapabilities(capabilities map[string]any) (map[string]CapabilityResolution, Result) {
	catalog := c.Catalog()
	resolved := make(map[string]CapabilityResolution, len(capabilities))

	for name := range capabilities {
		support, known := catalog.Capabilities[name]
		if !known {
			resolved[name] = CapabilityResolution{
				State:  "blocked",
				Reason: fmt.Sprintf("unknown capability %q", name),
			}
			continue
		}

		switch support.State {
		case "native":
			res := CapabilityResolution{State: "native"}
			if support.Permission != "" {
				res.Permissions = []string{support.Permission}
			}
			resolved[name] = res
		case "adapted":
			res := CapabilityResolution{State: "adapted", Reason: support.Notes}
			if support.Permission != "" {
				res.Permissions = []string{support.Permission}
			}
			resolved[name] = res
		case "degraded":
			resolved[name] = CapabilityResolution{State: "degraded", Reason: support.Notes}
		case "blocked":
			resolved[name] = CapabilityResolution{State: "blocked", Reason: support.Notes}
		}
	}

	return resolved, Result{
		Adapter:   "chrome",
		Operation: "resolveCapabilities",
		Outcome:   Success,
		Details:   resolved,
	}
}

func (c *Chrome) CompileManifest(opts ManifestCompileOptions) (ManifestOutput, Result) {
	manifest := map[string]any{
		"manifest_version": 3,
		"name":             opts.ProjectName,
		"version":          opts.ProjectVersion,
	}

	// Background
	if entry, ok := opts.Entries["background"]; ok {
		manifest["background"] = map[string]any{
			"service_worker": entry.Path,
			"type":           "module",
		}
	}

	// Action (popup)
	if entry, ok := opts.Entries["popup"]; ok {
		manifest["action"] = map[string]any{
			"default_popup": entry.Path,
		}
	}

	// Options
	if entry, ok := opts.Entries["options"]; ok {
		manifest["options_ui"] = map[string]any{
			"page":        entry.Path,
			"open_in_tab": false,
		}
	}

	// Side panel
	if entry, ok := opts.Entries["side_panel"]; ok {
		manifest["side_panel"] = map[string]any{
			"default_path": entry.Path,
		}
	}

	// Content scripts
	if entry, ok := opts.Entries["content_script"]; ok {
		manifest["content_scripts"] = []map[string]any{
			{
				"js":      []string{entry.Path},
				"matches": []string{"<all_urls>"},
			},
		}
	}

	// Permissions
	if len(opts.Permissions) > 0 {
		manifest["permissions"] = opts.Permissions
	}

	// Host permissions
	if len(opts.HostPermissions) > 0 {
		manifest["host_permissions"] = opts.HostPermissions
	}

	output := ManifestOutput{
		Manifest:        manifest,
		Permissions:     opts.Permissions,
		HostPermissions: opts.HostPermissions,
	}

	return output, Result{
		Adapter:   "chrome",
		Operation: "compileManifest",
		Outcome:   Success,
	}
}

func (c *Chrome) PackageArtifact(ctx context.Context, opts PackageOptions) (ArtifactRecord, Result) {
	record := ArtifactRecord{
		Target:       "chrome",
		ArtifactType: "chrome_zip",
		ProducedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	// Verify source directory exists
	if _, err := os.Stat(opts.SourceDir); err != nil {
		return record, Result{
			Adapter:    "chrome",
			Operation:  "packageArtifact",
			Outcome:    Blocked,
			Reason:     fmt.Sprintf("source directory not found: %s", opts.SourceDir),
			ReasonCode: "source_dir_missing",
		}
	}

	// Create output directory
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return record, Result{
			Adapter:    "chrome",
			Operation:  "packageArtifact",
			Outcome:    Blocked,
			Reason:     fmt.Sprintf("cannot create output dir: %v", err),
			ReasonCode: "output_dir_error",
		}
	}

	zipName := opts.ArtifactName + "-" + opts.Version + ".zip"
	zipPath := filepath.Join(opts.OutputDir, zipName)

	// Create zip
	if err := createZip(opts.SourceDir, zipPath); err != nil {
		return record, Result{
			Adapter:    "chrome",
			Operation:  "packageArtifact",
			Outcome:    Blocked,
			Reason:     fmt.Sprintf("zip creation failed: %v", err),
			ReasonCode: "zip_error",
		}
	}

	// Compute digest
	digest, size, err := fileDigest(zipPath)
	if err != nil {
		return record, Result{
			Adapter:    "chrome",
			Operation:  "packageArtifact",
			Outcome:    Blocked,
			Reason:     fmt.Sprintf("digest computation failed: %v", err),
			ReasonCode: "digest_error",
		}
	}

	record.FilePath = zipPath
	record.FileSize = size
	record.SHA256 = digest

	return record, Result{
		Adapter:   "chrome",
		Operation: "packageArtifact",
		Outcome:   Success,
		Details:   record,
	}
}

// --- helpers ---

func findChromeBinary() string {
	// Check environment variable first
	if p := os.Getenv("CHROME_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	case "linux":
		candidates = []string{
			"google-chrome",
			"google-chrome-stable",
			"chromium",
			"chromium-browser",
		}
	case "windows":
		candidates = []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		}
	}

	for _, c := range candidates {
		if filepath.IsAbs(c) {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		} else {
			if p, err := exec.LookPath(c); err == nil {
				return p
			}
		}
	}
	return ""
}

func createZip(sourceDir, zipPath string) (err error) {
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	w := zip.NewWriter(f)
	defer func() {
		if cerr := w.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	return filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		// Normalize to forward slashes for zip
		rel = filepath.ToSlash(rel)

		entry, err := w.Create(rel)
		if err != nil {
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = src.Close() }()

		_, err = io.Copy(entry, src)
		return err
	})
}

func fileDigest(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), size, nil
}
