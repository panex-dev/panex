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

// Plan is the result of computing proposed changes.
type Plan struct {
	PlanID         string                  `json:"plan_id"`
	CreatedAt      string                  `json:"created_at"`
	ProjectHash    string                  `json:"project_hash"` // snapshot at plan time
	ConfigHash     string                  `json:"config_hash"`  // config at plan time
	Actions        ActionList              `json:"actions"`
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
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		ProjectHash: graphHash,
		ConfigHash:  input.Graph.ConfigHash,
	}

	// Plan manifest generation — one action per target, each carrying its
	// own destination path and rendered manifest body.
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
			plan.Actions = append(plan.Actions, &GenerateManifestAction{
				Target:   out.Target,
				Path:     manifestPath,
				Manifest: out.Manifest,
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
	RunID      string   `json:"run_id"`
	Status     string   `json:"status"` // "succeeded", "failed", "drift_detected"
	Applied    []string `json:"applied,omitempty"`
	Failed     []string `json:"failed,omitempty"`
	RolledBack []string `json:"rolled_back,omitempty"`
	Errors     []string `json:"errors,omitempty"`
}

// Apply executes a plan with locking, drift detection, and reverse-order
// rollback on failure.
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
	if err := run.Transition(ledger.StatusRunning); err != nil {
		result.Status = "failed"
		result.Errors = append(result.Errors, fmt.Sprintf("transition created→running: %v", err))
		return result
	}

	result.RunID = run.RunID

	ctx := ExecContext{ProjectDir: input.ProjectDir}
	executed := make([]Action, 0, len(input.Plan.Actions))

	// Execute actions in order, tracking which succeeded for rollback.
	for _, action := range input.Plan.Actions {
		step := run.AddStep("apply", action.Kind())

		if err := action.Execute(ctx); err != nil {
			step.Fail(err.Error())
			result.Failed = append(result.Failed, fmt.Sprintf("%s: %v", action.Describe(), err))
			result.Errors = append(result.Errors, err.Error())
			rollbackExecuted(run, ctx, executed, result)
			finalize(run, ledger.StatusFailed, result)
			writeRun(input.ProjectDir, run)
			return result
		}

		step.Complete(nil)
		result.Applied = append(result.Applied, action.Describe())
		executed = append(executed, action)
	}

	finalize(run, ledger.StatusSucceeded, result)
	writeRun(input.ProjectDir, run)
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

// rollbackExecuted reverses executed actions in reverse order. Errors are
// recorded on the run + result but do not abort the rollback — best-effort
// undo gives the operator a chance to recover even from partial failures.
func rollbackExecuted(run *ledger.Run, ctx ExecContext, executed []Action, result *ApplyResult) {
	if len(executed) == 0 {
		return
	}
	if err := run.Transition(ledger.StatusRollingBack); err != nil {
		// State machine should allow running→rolling-back; if not, surface
		// the bug rather than continuing in the wrong state.
		result.Errors = append(result.Errors, fmt.Sprintf("transition running→rolling-back: %v", err))
	}
	for i := len(executed) - 1; i >= 0; i-- {
		action := executed[i]
		if !action.Reversible() {
			continue
		}
		step := run.AddStep("rollback", action.Kind())
		if err := action.Rollback(ctx); err != nil {
			step.Fail(err.Error())
			result.Errors = append(result.Errors, fmt.Sprintf("rollback %s: %v", action.Describe(), err))
			continue
		}
		step.Complete(nil)
		result.RolledBack = append(result.RolledBack, action.Describe())
	}
}

// finalize transitions the run to its terminal state and sets result.Status.
// Transition errors are surfaced rather than swallowed — they indicate a
// programmer bug in the state machine, not a runtime condition.
func finalize(run *ledger.Run, terminal ledger.Status, result *ApplyResult) {
	if err := run.Transition(terminal); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("transition →%s: %v", terminal, err))
	}
	switch terminal {
	case ledger.StatusSucceeded:
		result.Status = "succeeded"
	case ledger.StatusFailed:
		result.Status = "failed"
	default:
		result.Status = string(terminal)
	}
}

func writeRun(projectDir string, run *ledger.Run) {
	runDir := filepath.Join(projectDir, ".panex", "runs", run.RunID)
	_ = run.WriteToDir(runDir)
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
