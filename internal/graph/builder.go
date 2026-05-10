package graph

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/panex-dev/panex/internal/configloader"
	"github.com/panex-dev/panex/internal/inspector"
	"github.com/panex-dev/panex/internal/target"
)

// Builder constructs a project graph from inspector findings and config.
type Builder struct {
	sourceRoot string
	registry   *target.Registry
}

// NewBuilder creates a graph builder for the given project root.
func NewBuilder(sourceRoot string) *Builder {
	return &Builder{
		sourceRoot: sourceRoot,
		registry:   target.DefaultRegistry(),
	}
}

// BuildFromInspection creates a graph solely from inspector findings.
// Used when no panex.config.ts exists yet (e.g., during init).
func (b *Builder) BuildFromInspection(report *inspector.Report) (*Graph, error) {
	g := &Graph{
		SchemaVersion: 1,
		Project: ProjectIdentity{
			ID:      "",
			Name:    "",
			Version: "0.0.1",
		},
		SourceRoot:   b.sourceRoot,
		Entries:      make(map[string]Entry),
		Capabilities: make(map[string]any),
		Dependencies: make(map[string]string),
		StateDir:     ".panex",
	}

	if report.PackageManager != nil {
		g.PackageManager = report.PackageManager.Value
	}
	if report.WorkspaceType != nil {
		g.WorkspaceType = report.WorkspaceType.Value
	}
	if report.Framework != nil {
		g.Framework = DetectedFact{
			Name:       report.Framework.Value,
			Confidence: report.Framework.Confidence,
		}
	}
	if report.Bundler != nil {
		g.Bundler = DetectedFact{
			Name:       report.Bundler.Value,
			Confidence: report.Bundler.Confidence,
		}
	}
	if report.Language != nil {
		g.Language = DetectedFact{
			Name:       report.Language.Value,
			Confidence: report.Language.Confidence,
		}
	}

	for name, ep := range report.Entrypoints {
		g.Entries[name] = Entry{
			Path:   ep.Path,
			Type:   ep.Type,
			Source: ep.Source,
		}
	}

	for _, t := range report.Targets {
		g.TargetsRequested = append(g.TargetsRequested, t.Value)
		if _, ok := b.registry.Get(t.Value); ok {
			g.TargetsResolved = append(g.TargetsResolved, t.Value)
		}
	}

	hash, err := g.ComputeHash()
	if err != nil {
		return nil, err
	}
	g.GraphHash = hash

	return g, nil
}

// BuildFromConfig creates a graph by merging config with inspector findings.
// Config values take precedence over inspector findings (spec section 7).
func (b *Builder) BuildFromConfig(config *ProjectConfig, report *inspector.Report) (*Graph, error) {
	g, err := b.BuildFromInspection(report)
	if err != nil {
		return nil, err
	}

	// Config overrides inspector findings (source-of-truth hierarchy)
	g.Project = ProjectIdentity{
		ID:      config.Project.ID,
		Name:    config.Project.Name,
		Version: config.Project.Version,
	}

	if g.Project.Version == "" {
		g.Project.Version = "0.0.1"
	}

	if len(config.Targets) > 0 {
		g.TargetsRequested = config.Targets
		g.TargetsResolved = []string{}
		for _, t := range config.Targets {
			if _, ok := b.registry.Get(t); ok {
				g.TargetsResolved = append(g.TargetsResolved, t)
			}
		}
	}

	// Config entries override detected entries
	for name, entry := range config.Entries {
		g.Entries[name] = Entry{
			Path:   entry.Path,
			Type:   entry.Type,
			Source: "config",
		}
	}

	if config.Capabilities != nil {
		g.Capabilities = config.Capabilities
	}

	g.ConfigHash = config.Hash

	hash, err := g.ComputeHash()
	if err != nil {
		return nil, err
	}
	g.GraphHash = hash

	return g, nil
}

// WriteToFile writes the graph as JSON to the given path.
func WriteToFile(g *Graph, path string) error {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal graph: %w", err)
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write graph: %w", err)
	}
	return os.Rename(tmp, path)
}

// ReadFromFile reads a graph from a JSON file.
func ReadFromFile(path string) (*Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read graph: %w", err)
	}
	var g Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("parse graph: %w", err)
	}
	return &g, nil
}

// --- Config types (simplified for Phase 1) ---

// ProjectConfig is the parsed representation of panex.config.ts.
// In Phase 1 this is loaded from JSON; TS evaluation comes later.
type ProjectConfig struct {
	Project      ProjectConfigBlock     `json:"project"`
	Entries      map[string]EntryConfig `json:"entries"`
	Targets      []string               `json:"targets"`
	Capabilities map[string]any         `json:"capabilities"`
	Hash         string                 `json:"-"`
}

type ProjectConfigBlock struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Version string `json:"version"`
}

type EntryConfig struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

// ProjectConfigFromLoaded converts a configloader.Loaded into the
// graph.ProjectConfig that BuildFromConfig expects. This is the single
// conversion point between the two type hierarchies.
func ProjectConfigFromLoaded(loaded *configloader.Loaded) *ProjectConfig {
	if loaded == nil || loaded.Config == nil {
		return nil
	}
	cfg := loaded.Config

	gc := &ProjectConfig{
		Project: ProjectConfigBlock{
			Name: cfg.Project.Name,
			ID:   cfg.Project.ID,
		},
		Targets:      make([]string, 0),
		Capabilities: cfg.Capabilities,
		Entries:      make(map[string]EntryConfig),
		Hash:         loaded.ConfigHash,
	}

	enabledTargets := make([]string, 0, len(cfg.Targets))
	for t, tc := range cfg.Targets {
		if tc.Enabled {
			enabledTargets = append(enabledTargets, t)
		}
	}
	sort.Strings(enabledTargets)
	gc.Targets = append(gc.Targets, enabledTargets...)
	for name, e := range cfg.Entries {
		gc.Entries[name] = EntryConfig{Path: e.Path, Type: e.ModuleType}
	}

	return gc
}

// LoadProjectConfig reads a panex config from a JSON file.
// Phase 1: loads JSON directly. Later: evaluates panex.config.ts.
func LoadProjectConfig(path string) (*ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	h := sha256.Sum256(data)
	cfg.Hash = fmt.Sprintf("sha256:%x", h)

	return &cfg, nil
}
