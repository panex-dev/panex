// Package fsmodel defines the canonical .panex/ directory structure.
// All Panex components must use these paths rather than constructing
// paths ad hoc. This is the filesystem contract from spec section 10.
package fsmodel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	StateDir     = ".panex"
	ConfigFile   = "panex.config.ts"
	PolicyFile   = "panex.policy.yaml"
	StateName    = "state.json"
	ConfigLock   = "config.lock.json"
	ProjectGraph = "project.graph.json"
	Environment  = "environment.json"
	RunsDir      = "runs"
	SessionsDir  = "sessions"
	ReportsDir   = "reports"
	CacheDir     = "cache"
	ArtifactsDir = "artifacts"
	LocksDir     = "locks"
	GeneratedDir = "generated"
	ManifestsDir = "manifests"
	TraceDir     = "trace"
	ProjectLock  = "project.lock"
	DevLock      = "dev.lock"
	PublishLock  = "publish.lock"
)

// Root represents a Panex-managed project root.
type Root struct {
	ProjectDir string // absolute path to the project directory
}

// NewRoot creates a Root anchored at the given project directory.
// The directory must exist. It does not need to be initialized yet.
func NewRoot(projectDir string) (*Root, error) {
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve project dir: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat project dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", abs)
	}
	return &Root{ProjectDir: abs}, nil
}

// IsInitialized returns true if the .panex/ state directory exists.
func (r *Root) IsInitialized() bool {
	info, err := os.Stat(r.StateRoot())
	return err == nil && info.IsDir()
}

// Init creates the .panex/ directory structure. It is idempotent —
// existing directories are not modified or recreated.
func (r *Root) Init() error {
	dirs := []string{
		r.StateRoot(),
		r.RunsRoot(),
		r.SessionsRoot(),
		r.ReportsRoot(),
		r.CacheRoot(),
		filepath.Join(r.CacheRoot(), "downloads"),
		filepath.Join(r.CacheRoot(), "package-manager"),
		filepath.Join(r.CacheRoot(), "browser-profiles"),
		filepath.Join(r.CacheRoot(), "launch-artifacts"),
		r.ArtifactsRoot(),
		r.LocksRoot(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	// Write initial state.json if it doesn't exist
	statePath := r.StatePath()
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		initial := State{
			SchemaVersion:  1,
			InitializedAt:  time.Now().UTC().Format(time.RFC3339Nano),
			LatestRunID:    "",
			LatestReportAt: "",
			ActiveSession:  "",
		}
		if err := writeJSON(statePath, initial); err != nil {
			return fmt.Errorf("write state.json: %w", err)
		}
	}

	return nil
}

// --- Path accessors ---

func (r *Root) StateRoot() string {
	return filepath.Join(r.ProjectDir, StateDir)
}

func (r *Root) ConfigFilePath() string {
	return filepath.Join(r.ProjectDir, ConfigFile)
}

func (r *Root) PolicyFilePath() string {
	return filepath.Join(r.ProjectDir, PolicyFile)
}

func (r *Root) StatePath() string {
	return filepath.Join(r.StateRoot(), StateName)
}

func (r *Root) ConfigLockPath() string {
	return filepath.Join(r.StateRoot(), ConfigLock)
}

func (r *Root) ProjectGraphPath() string {
	return filepath.Join(r.StateRoot(), ProjectGraph)
}

func (r *Root) EnvironmentPath() string {
	return filepath.Join(r.StateRoot(), Environment)
}

func (r *Root) RunsRoot() string {
	return filepath.Join(r.StateRoot(), RunsDir)
}

func (r *Root) SessionsRoot() string {
	return filepath.Join(r.StateRoot(), SessionsDir)
}

func (r *Root) ReportsRoot() string {
	return filepath.Join(r.StateRoot(), ReportsDir)
}

func (r *Root) CacheRoot() string {
	return filepath.Join(r.StateRoot(), CacheDir)
}

func (r *Root) ArtifactsRoot() string {
	return filepath.Join(r.StateRoot(), ArtifactsDir)
}

func (r *Root) LocksRoot() string {
	return filepath.Join(r.StateRoot(), LocksDir)
}

// RunDir returns the directory for a specific run.
func (r *Root) RunDir(runID string) string {
	return filepath.Join(r.RunsRoot(), runID)
}

// RunManifestDir returns the generated manifest directory for a target within a run.
func (r *Root) RunManifestDir(runID, target string) string {
	return filepath.Join(r.RunDir(runID), GeneratedDir, ManifestsDir, target)
}

// RunTracePath returns the trace events path for a run.
func (r *Root) RunTracePath(runID string) string {
	return filepath.Join(r.RunDir(runID), TraceDir, "events.jsonl")
}

// SessionDir returns the directory for a specific session.
func (r *Root) SessionDir(sessionID string) string {
	return filepath.Join(r.SessionsRoot(), sessionID)
}

// ArtifactDir returns the artifact directory for a specific target.
func (r *Root) ArtifactDir(target string) string {
	return filepath.Join(r.ArtifactsRoot(), target)
}

// LockPath returns the path for a named lock file.
func (r *Root) LockPath(name string) string {
	return filepath.Join(r.LocksRoot(), name)
}

// --- State file ---

// State is the small top-level state pointer file (.panex/state.json).
type State struct {
	SchemaVersion  int    `json:"schema_version"`
	InitializedAt  string `json:"initialized_at"`
	LatestRunID    string `json:"latest_run_id,omitempty"`
	LatestReportAt string `json:"latest_report_at,omitempty"`
	ActiveSession  string `json:"active_session,omitempty"`
}

// ReadState reads .panex/state.json.
func (r *Root) ReadState() (State, error) {
	var s State
	data, err := os.ReadFile(r.StatePath())
	if err != nil {
		return s, fmt.Errorf("read state.json: %w", err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("parse state.json: %w", err)
	}
	return s, nil
}

// WriteState writes .panex/state.json atomically.
func (r *Root) WriteState(s State) error {
	return writeJSON(r.StatePath(), s)
}

// --- helpers ---

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	// Atomic write: write to temp, then rename
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
