package target

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Firefox implements the Adapter interface for the Firefox target.
type Firefox struct{}

var _ Adapter = (*Firefox)(nil)

func NewFirefox() *Firefox { return &Firefox{} }

func (f *Firefox) Name() string { return "firefox" }

func (f *Firefox) Catalog() CapabilityCatalog {
	return CapabilityCatalog{
		Target: "firefox",
		Capabilities: map[string]CapabilitySupport{
			"tabs":                {State: "native", Permission: "tabs"},
			"windows":             {State: "native", Permission: "windows"}, //nolint:misspell
			"storage":             {State: "native", Permission: "storage"},
			"scripting":           {State: "adapted", Permission: "tabs", Notes: "Firefox target uses MV2 content script and tabs primitives"},
			"content":             {State: "native"},
			"commands":            {State: "native"},
			"alarms":              {State: "native", Permission: "alarms"},
			"notifications":       {State: "native", Permission: "notifications"},
			"downloads":           {State: "native", Permission: "downloads"},
			"clipboard":           {State: "native", Permission: "clipboardRead"},
			"contextMenus":        {State: "native", Permission: "menus"},
			"identity":            {State: "degraded", Permission: "identity", Notes: "Firefox identity APIs differ from Chromium identity flows"},
			"networkRules":        {State: "blocked", Notes: "declarativeNetRequest is not available in the Firefox MV2 target"},
			"devtools":            {State: "native"},
			"omnibox":             {State: "native"},
			"sideSurface":         {State: "blocked", Notes: "Firefox uses sidebar_action, not Chrome sidePanel"},
			"sidebarSurface":      {State: "native"},
			"offscreenExecution":  {State: "blocked", Notes: "offscreen documents are Chrome-specific"},
			"nativeMessaging":     {State: "native", Permission: "nativeMessaging"},
			"hostAccess":          {State: "native"},
			"backgroundExecution": {State: "adapted", Notes: "Firefox target uses MV2 background scripts"},
			"sessionState":        {State: "native", Permission: "storage"},
			"capture":             {State: "native", Permission: "tabCapture"},
			"cookies":             {State: "native", Permission: "cookies"},
			"history":             {State: "native", Permission: "history"},
			"bookmarks":           {State: "native", Permission: "bookmarks"},
		},
	}
}

func (f *Firefox) InspectEnvironment(ctx context.Context) (EnvironmentInfo, Result) {
	info := EnvironmentInfo{}
	binary := findFirefoxBinary()
	if binary == "" {
		info.Reason = "no Firefox binary found"
		return info, Result{
			Adapter:     "firefox",
			Operation:   "inspectEnvironment",
			Outcome:     EnvironmentMissing,
			Reason:      "no Firefox binary found in standard locations",
			ReasonCode:  "firefox_not_found",
			Suggestions: []string{"install Firefox", "set FIREFOX_PATH environment variable"},
			Repairable:  false,
		}
	}

	info.Available = true
	info.BinaryPath = binary
	info.Launchable = true

	versionCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(versionCtx, binary, "--version")
	out, err := cmd.Output()
	if err == nil {
		info.Version = strings.TrimSpace(string(out))
	}

	return info, Result{
		Adapter:   "firefox",
		Operation: "inspectEnvironment",
		Outcome:   Success,
		Details:   info,
	}
}

func (f *Firefox) ResolveCapabilities(capabilities map[string]any) (map[string]CapabilityResolution, Result) {
	catalog := f.Catalog()
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
			res := CapabilityResolution{State: "degraded", Reason: support.Notes}
			if support.Permission != "" {
				res.Permissions = []string{support.Permission}
			}
			resolved[name] = res
		case "blocked":
			resolved[name] = CapabilityResolution{State: "blocked", Reason: support.Notes}
		}
	}

	return resolved, Result{
		Adapter:   "firefox",
		Operation: "resolveCapabilities",
		Outcome:   Success,
		Details:   resolved,
	}
}

func (f *Firefox) CompileManifest(opts ManifestCompileOptions) (ManifestOutput, Result) {
	manifest := map[string]any{
		"manifest_version": 2,
		"name":             opts.ProjectName,
		"version":          opts.ProjectVersion,
	}

	if entry, ok := opts.Entries["background"]; ok {
		manifest["background"] = map[string]any{
			"scripts":    []string{entry.Path},
			"persistent": false,
		}
	}

	if entry, ok := opts.Entries["popup"]; ok {
		manifest["browser_action"] = map[string]any{
			"default_popup": entry.Path,
		}
	}

	if entry, ok := opts.Entries["options"]; ok {
		manifest["options_ui"] = map[string]any{
			"page":        entry.Path,
			"open_in_tab": false,
		}
	}

	if entry, ok := opts.Entries["sidebar"]; ok {
		manifest["sidebar_action"] = map[string]any{
			"default_panel": entry.Path,
		}
	} else if entry, ok := opts.Entries["side_panel"]; ok {
		manifest["sidebar_action"] = map[string]any{
			"default_panel": entry.Path,
		}
	}

	if entry, ok := opts.Entries["content_script"]; ok {
		manifest["content_scripts"] = []map[string]any{
			{
				"js":      []string{entry.Path},
				"matches": []string{"<all_urls>"},
			},
		}
	}

	manifestPermissions := append([]string{}, opts.Permissions...)
	manifestPermissions = append(manifestPermissions, opts.HostPermissions...)
	if len(manifestPermissions) > 0 {
		manifest["permissions"] = manifestPermissions
	}

	output := ManifestOutput{
		Manifest:        manifest,
		Permissions:     opts.Permissions,
		HostPermissions: opts.HostPermissions,
	}

	return output, Result{
		Adapter:   "firefox",
		Operation: "compileManifest",
		Outcome:   Success,
	}
}

func (f *Firefox) PackageArtifact(ctx context.Context, opts PackageOptions) (ArtifactRecord, Result) {
	record := ArtifactRecord{
		Target:       "firefox",
		ArtifactType: "firefox_xpi",
		ProducedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}

	if _, err := os.Stat(opts.SourceDir); err != nil {
		return record, Result{
			Adapter:    "firefox",
			Operation:  "packageArtifact",
			Outcome:    Blocked,
			Reason:     fmt.Sprintf("source directory not found: %s", opts.SourceDir),
			ReasonCode: "source_dir_missing",
		}
	}

	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return record, Result{
			Adapter:    "firefox",
			Operation:  "packageArtifact",
			Outcome:    Blocked,
			Reason:     fmt.Sprintf("cannot create output dir: %v", err),
			ReasonCode: "output_dir_error",
		}
	}

	xpiName := opts.ArtifactName + "-" + opts.Version + ".xpi"
	xpiPath := filepath.Join(opts.OutputDir, xpiName)
	if err := createZip(opts.SourceDir, xpiPath); err != nil {
		return record, Result{
			Adapter:    "firefox",
			Operation:  "packageArtifact",
			Outcome:    Blocked,
			Reason:     fmt.Sprintf("xpi creation failed: %v", err),
			ReasonCode: "xpi_error",
		}
	}

	digest, size, err := fileDigest(xpiPath)
	if err != nil {
		return record, Result{
			Adapter:    "firefox",
			Operation:  "packageArtifact",
			Outcome:    Blocked,
			Reason:     fmt.Sprintf("digest computation failed: %v", err),
			ReasonCode: "digest_error",
		}
	}

	record.FilePath = xpiPath
	record.FileSize = size
	record.SHA256 = digest

	return record, Result{
		Adapter:   "firefox",
		Operation: "packageArtifact",
		Outcome:   Success,
		Details:   record,
	}
}

func findFirefoxBinary() string {
	if p := os.Getenv("FIREFOX_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications/Firefox.app/Contents/MacOS/firefox",
		}
	case "linux":
		candidates = []string{
			"firefox",
			"firefox-esr",
		}
	case "windows":
		candidates = []string{
			`C:\Program Files\Mozilla Firefox\firefox.exe`,
			`C:\Program Files (x86)\Mozilla Firefox\firefox.exe`,
		}
	}

	for _, candidate := range candidates {
		if filepath.IsAbs(candidate) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		} else if p, err := exec.LookPath(candidate); err == nil {
			return p
		}
	}
	return ""
}
