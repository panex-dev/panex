// Package plan implements the plan/apply model for all project mutations.
// Plan computes proposed changes with a snapshot hash. Apply executes
// with project lock, step recording, and drift rejection. Spec section 21.
package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/panex-dev/panex/internal/graph"
	"github.com/panex-dev/panex/internal/ledger"
	"github.com/panex-dev/panex/internal/lock"
	"github.com/panex-dev/panex/internal/manifest"
)

// Action describes a single planned mutation.
type Action struct {
	Type        string `json:"type"`        // "create_file", "update_file", "delete_file", "generate_manifest", "install_dependency"
	Path        string `json:"path"`        // target path
	Description string `json:"description"` // human-readable
	Reversible  bool   `json:"reversible"`
	Risk        string `json:"risk"` // "safe", "low", "medium", "high"
}

// Plan is the result of computing proposed changes.
type Plan struct {
	PlanID         string                  `json:"plan_id"`
	CreatedAt      string                  `json:"created_at"`
	ProjectHash    string                  `json:"project_hash"` // snapshot at plan time
	ConfigHash     string                  `json:"config_hash"`  // config at plan time
	Actions        []Action                `json:"actions"`
	ManifestDiffs  map[string]ManifestDiff `json:"manifest_diffs,omitempty"`
	PermissionDiff *PermissionDiff         `json:"permission_diff,omitempty"`
	Warnings       []string                `json:"warnings,omitempty"`
	EstimatedSteps int                     `json:"estimated_steps"`
}

// ManifestDiff describes changes to a target's manifest.
type ManifestDiff struct {
	Target  string `json:"target"`
	IsNew   bool   `json:"is_new"`
	Changes int    `json:"changes"` // number of changed keys
}

// PermissionDiff tracks permission changes.
type PermissionDiff struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

// PlanInput is everything needed to compute a plan.
type PlanInput struct {
	ProjectDir     string
	Graph          *graph.Graph
	ManifestResult *manifest.CompileResult
	PreviousPerms  []string
}

// ComputePlan generates a plan from the current project state.
func ComputePlan(input PlanInput) (*Plan, error) {
	if input.Graph == nil {
		return nil, fmt.Errorf("nil graph")
	}

	graphHash, err := input.Graph.ComputeHash()
	if err != nil {
		return nil, fmt.Errorf("compute graph hash: %w", err)
	}

	plan := &Plan{
		PlanID:      ledger.NewRunID(), // reuse ID generator
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		ProjectHash: graphHash,
		ConfigHash:  input.Graph.ConfigHash,
	}

	// Plan manifest generation
	if input.ManifestResult != nil {
		plan.ManifestDiffs = make(map[string]ManifestDiff)
		for _, out := range input.ManifestResult.Outputs {
			manifestPath := filepath.Join(input.ProjectDir, ".panex", "runs", "generated", "manifests", out.Target, "manifest.json")
			isNew := true
			if _, err := os.Stat(manifestPath); err == nil {
				isNew = false
			}
			plan.ManifestDiffs[out.Target] = ManifestDiff{
				Target: out.Target,
				IsNew:  isNew,
			}
			plan.Actions = append(plan.Actions, Action{
				Type:        "generate_manifest",
				Path:        manifestPath,
				Description: fmt.Sprintf("generate manifest.json for %s", out.Target),
				Reversible:  true,
				Risk:        "safe",
			})
		}

		// Permission diff (compare when previous state is known, even if empty)
		if input.PreviousPerms != nil {
			diff := computePermissionDiff(input.PreviousPerms, collectAllPerms(input.ManifestResult))
			if len(diff.Added) > 0 || len(diff.Removed) > 0 {
				plan.PermissionDiff = diff
				if len(diff.Added) > 0 {
					plan.Warnings = append(plan.Warnings,
						fmt.Sprintf("new permissions requested: %v", diff.Added))
				}
			}
		}
	}

	plan.EstimatedSteps = len(plan.Actions)

	return plan, nil
}

// ApplyInput is everything needed to execute a plan.
type ApplyInput struct {
	ProjectDir     string
	Plan           *Plan
	Graph          *graph.Graph
	ManifestResult *manifest.CompileResult
	LockManager    *lock.Manager
	Force          bool // skip drift check
}

// ApplyResult is the outcome of applying a plan.
type ApplyResult struct {
	RunID   string   `json:"run_id"`
	Status  string   `json:"status"` // "succeeded", "failed", "drift_detected"
	Applied []string `json:"applied,omitempty"`
	Failed  []string `json:"failed,omitempty"`
	Errors  []string `json:"errors,omitempty"`
}

// Apply executes a plan with locking and drift detection.
func Apply(input ApplyInput) *ApplyResult {
	result := &ApplyResult{}

	if input.Plan == nil {
		result.Status = "failed"
		result.Errors = append(result.Errors, "nil plan")
		return result
	}

	// Drift detection: recompute graph hash and compare
	if !input.Force && input.Graph != nil {
		currentHash, err := input.Graph.ComputeHash()
		if err == nil && currentHash != input.Plan.ProjectHash {
			result.Status = "drift_detected"
			result.Errors = append(result.Errors,
				fmt.Sprintf("plan_drift_detected: graph hash changed from %s to %s", input.Plan.ProjectHash, currentHash))
			return result
		}
	}

	// Acquire project lock
	var projectLock *lock.Lock
	if input.LockManager != nil {
		var err error
		projectLock, err = input.LockManager.Acquire(lock.ProjectMutation, "apply", "cli")
		if err != nil {
			result.Status = "failed"
			result.Errors = append(result.Errors, fmt.Sprintf("lock: %v", err))
			return result
		}
		defer func() { _ = input.LockManager.Release(projectLock) }()
	}

	// Create run
	run := ledger.NewRun("apply", ledger.Actor{Type: ledger.ActorAgent, Name: "panex-cli"})
	run.ProjectHash = input.Plan.ProjectHash
	_ = run.Transition(ledger.StatusRunning)

	result.RunID = run.RunID

	// Execute actions
	for _, action := range input.Plan.Actions {
		step := run.AddStep("apply", action.Type+"_"+action.Path)

		var err error
		switch action.Type {
		case "generate_manifest":
			err = applyGenerateManifest(action, input)
		default:
			err = fmt.Errorf("unknown action type: %s", action.Type)
		}

		if err != nil {
			step.Fail(err.Error())
			result.Failed = append(result.Failed, fmt.Sprintf("%s: %v", action.Description, err))
			result.Errors = append(result.Errors, err.Error())
		} else {
			step.Complete(action)
			result.Applied = append(result.Applied, action.Description)
		}
	}

	if len(result.Failed) > 0 {
		_ = run.Transition(ledger.StatusFailed)
		result.Status = "failed"
	} else {
		_ = run.Transition(ledger.StatusSucceeded)
		result.Status = "succeeded"
	}

	// Write run to ledger
	runDir := filepath.Join(input.ProjectDir, ".panex", "runs", run.RunID)
	_ = run.WriteToDir(runDir)

	return result
}

// WritePlan persists a plan to disk.
func WritePlan(p *Plan, path string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadPlan loads a plan from disk.
func ReadPlan(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}
	return &p, nil
}

// --- helpers ---

func applyGenerateManifest(action Action, input ApplyInput) error {
	if input.ManifestResult == nil {
		return fmt.Errorf("no manifest result")
	}

	for _, out := range input.ManifestResult.Outputs {
		dir := filepath.Dir(action.Path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
		if err := manifest.WriteManifest(out.Manifest, action.Path); err != nil {
			return err
		}
	}
	return nil
}

func computePermissionDiff(previous, current []string) *PermissionDiff {
	prev := toSet(previous)
	curr := toSet(current)

	diff := &PermissionDiff{}
	for p := range curr {
		if !prev[p] {
			diff.Added = append(diff.Added, p)
		}
	}
	for p := range prev {
		if !curr[p] {
			diff.Removed = append(diff.Removed, p)
		}
	}
	return diff
}

func collectAllPerms(result *manifest.CompileResult) []string {
	seen := map[string]bool{}
	var perms []string
	for _, out := range result.Outputs {
		for _, p := range out.Permissions {
			if !seen[p] {
				seen[p] = true
				perms = append(perms, p)
			}
		}
	}
	return perms
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
