// Package configloader loads Panex project configuration.
// Phase 1 supports JSON only (panex.config.json).
// Future phases will add TypeScript evaluation (panex.config.ts).
// Spec section 11.
package configloader

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the resolved project configuration.
type Config struct {
	Project       ProjectConfig          `json:"project"`
	Entries       map[string]EntryConfig `json:"entries,omitempty"`
	Targets       TargetConfigMap        `json:"targets,omitempty"`
	Capabilities  map[string]any         `json:"capabilities,omitempty"`
	Runtime       RuntimeConfig          `json:"runtime,omitempty"`
	Packaging     PackagingConfig        `json:"packaging,omitempty"`
	Compatibility map[string]any         `json:"compatibility,omitempty"`
	Features      map[string]any         `json:"features,omitempty"`
	Publish       map[string]any         `json:"publish,omitempty"`
}

// ProjectConfig contains stable project identity.
type ProjectConfig struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	RepoRoot    string `json:"repo_root,omitempty"`
	Workspace   string `json:"workspace,omitempty"`
}

// EntryConfig declares an extension surface.
type EntryConfig struct {
	Path       string   `json:"path"`
	ModuleType string   `json:"module_type,omitempty"` // "esm", "cjs"
	Conditions []string `json:"conditions,omitempty"`
	Targets    []string `json:"targets,omitempty"` // empty = all targets
}

// TargetConfigMap holds per-target overrides.
type TargetConfigMap map[string]TargetConfig

// TargetConfig is per-target configuration.
type TargetConfig struct {
	Enabled  bool           `json:"enabled"`
	Manifest map[string]any `json:"manifest,omitempty"`
}

// RuntimeConfig defines dev bridge and session behavior.
type RuntimeConfig struct {
	BridgeAuth     string `json:"bridge_auth,omitempty"`
	ProfileReuse   bool   `json:"profile_reuse,omitempty"`
	LogVerbosity   string `json:"log_verbosity,omitempty"`
	TraceEnabled   bool   `json:"trace_enabled,omitempty"`
	ReloadStrategy string `json:"reload_strategy,omitempty"` // "full", "smart"
}

// PackagingConfig defines artifact naming and output preferences.
type PackagingConfig struct {
	OutputDir     string `json:"output_dir,omitempty"`
	ArtifactName  string `json:"artifact_name,omitempty"`
	VersionSource string `json:"version_source,omitempty"` // "manifest", "package_json", "config"
	Version       string `json:"version,omitempty"`
}

// Loaded is the result of loading a config file.
type Loaded struct {
	Config     *Config
	SourcePath string
	ConfigHash string // SHA-256 of raw config content
}

// ConfigFileNames in search order.
var ConfigFileNames = []string{
	"panex.config.json",
}

// Load searches for and loads a Panex config from the project directory.
// Returns nil Loaded (not an error) if no config file exists.
func Load(projectDir string) (*Loaded, error) {
	for _, name := range ConfigFileNames {
		path := filepath.Join(projectDir, name)
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}

		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}

		hash := fmt.Sprintf("sha256:%x", sha256.Sum256(data))

		return &Loaded{
			Config:     &cfg,
			SourcePath: path,
			ConfigHash: hash,
		}, nil
	}

	return nil, nil
}

// LoadFromFile loads config from a specific path.
func LoadFromFile(path string) (*Loaded, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	hash := fmt.Sprintf("sha256:%x", sha256.Sum256(data))

	return &Loaded{
		Config:     &cfg,
		SourcePath: path,
		ConfigHash: hash,
	}, nil
}

// Default returns a minimal default config with the given project name.
func Default(name string) *Config {
	return &Config{
		Project: ProjectConfig{
			Name: name,
			ID:   name,
		},
		Targets: TargetConfigMap{
			"chrome": {Enabled: true},
		},
	}
}

// WriteToFile writes config as JSON to the given path.
func WriteToFile(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return os.Rename(tmp, path)
}
