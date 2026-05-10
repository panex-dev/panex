// Package configloader loads Panex project configuration.
// It supports TypeScript-authored panex.config.ts evaluation via
// esbuild + a Node subprocess, and JSON fallback via panex.config.json.
// Spec section 11.
package configloader

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/evanw/esbuild/pkg/api"
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
	TypeScriptConfigFileName,
	JSONConfigFileName,
}

const (
	TypeScriptConfigFileName = "panex.config.ts"
	JSONConfigFileName       = "panex.config.json"
)

var (
	execLookPath = exec.LookPath
	commandExec  = exec.CommandContext
)

// Load searches for and loads a Panex config from the project directory.
// Returns nil Loaded (not an error) if no config file exists.
func Load(projectDir string) (*Loaded, error) {
	for _, name := range ConfigFileNames {
		path := filepath.Join(projectDir, name)
		loaded, err := LoadFromFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", name, err)
		}
		if loaded != nil {
			return loaded, nil
		}
	}

	return nil, nil
}

// LoadFromFile loads config from a specific path.
func LoadFromFile(path string) (*Loaded, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg, err := decodeConfig(path, data)
	if err != nil {
		return nil, err
	}

	hash := fmt.Sprintf("sha256:%x", sha256.Sum256(data))

	return &Loaded{
		Config:     cfg,
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

func decodeConfig(path string, data []byte) (*Config, error) {
	switch filepath.Ext(path) {
	case ".json":
		return decodeJSONConfig(data)
	case ".ts":
		return decodeTypeScriptConfig(path)
	default:
		return nil, fmt.Errorf("unsupported config file extension: %s", filepath.Ext(path))
	}
}

func decodeJSONConfig(data []byte) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

func decodeTypeScriptConfig(path string) (*Config, error) {
	nodePath, err := execLookPath("node")
	if err != nil {
		return nil, fmt.Errorf("evaluate config: node binary not found in PATH")
	}

	bundle, err := transpileTypeScriptConfig(path)
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "panex-config-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	bundlePath := filepath.Join(tempDir, "panex.config.cjs")
	if err := os.WriteFile(bundlePath, bundle, 0o600); err != nil {
		return nil, fmt.Errorf("write temp config bundle: %w", err)
	}

	evaluated, err := evaluateBundledConfig(nodePath, bundlePath)
	if err != nil {
		return nil, err
	}

	cfg, err := decodeJSONConfig(evaluated)
	if err != nil {
		return nil, fmt.Errorf("parse evaluated config: %w", err)
	}

	return cfg, nil
}

func transpileTypeScriptConfig(path string) ([]byte, error) {
	result := api.Build(api.BuildOptions{
		EntryPoints: []string{path},
		Bundle:      true,
		Platform:    api.PlatformNode,
		Format:      api.FormatCommonJS,
		Write:       false,
		LogLevel:    api.LogLevelSilent,
		Outfile:     "panex.config.cjs",
	})
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("transpile config: %s", joinEsbuildMessages(result.Errors))
	}
	if len(result.OutputFiles) == 0 {
		return nil, fmt.Errorf("transpile config: no output produced")
	}
	return result.OutputFiles[0].Contents, nil
}

func evaluateBundledConfig(nodePath string, bundlePath string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	script := strings.Join([]string{
		"const mod = require(process.argv[1]);",
		"const raw = mod && Object.prototype.hasOwnProperty.call(mod, 'default') ? mod.default : mod;",
		"Promise.resolve(raw).then((value) => {",
		"  if (value === undefined) throw new Error('config module exported undefined');",
		"  const encoded = JSON.stringify(value);",
		"  if (encoded === undefined) throw new Error('config module did not evaluate to a JSON-serializable value');",
		"  process.stdout.write(encoded);",
		"}).catch((err) => {",
		"  const message = err && err.stack ? err.stack : String(err);",
		"  process.stderr.write(message);",
		"  process.exit(1);",
		"});",
	}, "\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := commandExec(ctx, nodePath, "-e", script, bundlePath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("evaluate config: %s", message)
	}
	return stdout.Bytes(), nil
}

func joinEsbuildMessages(messages []api.Message) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		text := strings.TrimSpace(message.Text)
		if text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return "unknown esbuild error"
	}
	return strings.Join(parts, "; ")
}
