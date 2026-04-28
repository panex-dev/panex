// Package graph builds and manages the normalized project graph.
// The project graph merges authored config with inspector findings
// into a single machine-readable model. Spec section 13.
package graph

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// Graph is the normalized internal representation of the extension project.
type Graph struct {
	SchemaVersion    int               `json:"schema_version"`
	Project          ProjectIdentity   `json:"project"`
	SourceRoot       string            `json:"source_root"`
	PackageManager   string            `json:"package_manager"`
	WorkspaceType    string            `json:"workspace_type"`
	Framework        DetectedFact      `json:"framework"`
	Bundler          DetectedFact      `json:"bundler"`
	Language         DetectedFact      `json:"language"`
	Entries          map[string]Entry  `json:"entries"`
	TargetsRequested []string          `json:"targets_requested"`
	TargetsResolved  []string          `json:"targets_resolved"`
	Capabilities     map[string]any    `json:"capabilities"`
	Dependencies     map[string]string `json:"dependencies"`
	StateDir         string            `json:"state_dir"`
	ConfigHash       string            `json:"config_hash"`
	GraphHash        string            `json:"graph_hash"`
}

// ProjectIdentity is the stable project identity.
type ProjectIdentity struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// DetectedFact is a value with detection provenance.
type DetectedFact struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
}

// Entry is a resolved extension surface entry.
type Entry struct {
	Path   string `json:"path"`
	Type   string `json:"type"`
	Source string `json:"source"`
}

// hashView is the projection of Graph that participates in the content hash.
//
// It deliberately excludes:
//   - GraphHash (the hash output itself; including it would self-reference)
//   - SourceRoot (an absolute path that varies per machine; including it
//     means two devs on the same commit produce different ProjectHash and
//     a plan made on one machine always reports drift on another)
//
// Field order and JSON tags mirror Graph for the fields that remain. Map
// keys are sorted by encoding/json so the marshaled output is deterministic.
type hashView struct {
	SchemaVersion    int               `json:"schema_version"`
	Project          ProjectIdentity   `json:"project"`
	PackageManager   string            `json:"package_manager"`
	WorkspaceType    string            `json:"workspace_type"`
	Framework        DetectedFact      `json:"framework"`
	Bundler          DetectedFact      `json:"bundler"`
	Language         DetectedFact      `json:"language"`
	Entries          map[string]Entry  `json:"entries"`
	TargetsRequested []string          `json:"targets_requested"`
	TargetsResolved  []string          `json:"targets_resolved"`
	Capabilities     map[string]any    `json:"capabilities"`
	Dependencies     map[string]string `json:"dependencies"`
	StateDir         string            `json:"state_dir"`
	ConfigHash       string            `json:"config_hash"`
}

// ComputeHash computes a stable SHA-256 hash of the graph content.
//
// The hash is portable across machines (independent of SourceRoot) and the
// receiver is never mutated, so concurrent calls are safe. Used for drift
// detection between plan and apply.
func (g *Graph) ComputeHash() (string, error) {
	view := hashView{
		SchemaVersion:    g.SchemaVersion,
		Project:          g.Project,
		PackageManager:   g.PackageManager,
		WorkspaceType:    g.WorkspaceType,
		Framework:        g.Framework,
		Bundler:          g.Bundler,
		Language:         g.Language,
		Entries:          g.Entries,
		TargetsRequested: g.TargetsRequested,
		TargetsResolved:  g.TargetsResolved,
		Capabilities:     g.Capabilities,
		Dependencies:     g.Dependencies,
		StateDir:         g.StateDir,
		ConfigHash:       g.ConfigHash,
	}
	data, err := json.Marshal(view)
	if err != nil {
		return "", fmt.Errorf("marshal graph for hash: %w", err)
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", h), nil
}
