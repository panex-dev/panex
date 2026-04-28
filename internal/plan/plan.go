// Package plan implements the plan/apply model for all project mutations.
// Plan computes proposed changes with a snapshot hash. Apply executes
// with project lock, step recording, and drift rejection. Spec section 21.
package plan

import (
	"context"
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
type Action interface {
	Kind() string
	Desc() string
	Execute(ctx context.Context, input ApplyInput) error
	Rollback(ctx context.Context, input ApplyInput) error
}

// GenerateManifestAction writes a target manifest to disk.
type GenerateManifestAction struct {
	Target   string         `json:"target"`
	Path     string         `json:"path"`
	Manifest map[string]any `json:"manifest"`
}

func (a *GenerateManifestAction) Kind() string { return "generate_manifest" }
func (a *GenerateManifestAction) Desc() string {
	return fmt.Sprintf("generate manifest.json for %s", a.Target)
}

func (a *GenerateManifestAction) MarshalJSON() ([]byte, error) {
	type Alias GenerateManifestAction
	return json.Marshal(&struct {
		Kind string `json:"kind"`
		*Alias
	}{
		Kind:  a.Kind(),
		Alias: (*Alias)(a),
	})
}

func (a *GenerateManifestAction) Execute(ctx context.Context, input ApplyInput) error {
	dir := filepath.Dir(a.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}
	return manifest.WriteManifest(a.Manifest, a.Path)
}

func (a *GenerateManifestAction) Rollback(ctx context.Context, input ApplyInput) error {
	return os.Remove(a.Path)
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

// UnmarshalJSON implements custom decoding for polymorphic actions.
func (p *Plan) UnmarshalJSON(data []byte) error {
	type Alias Plan
	aux := &struct {
		Actions []json.RawMessage `json:"actions"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	for _, raw := range aux.Actions {
		var typeHeader struct {
			Kind string `json:"kind"`
			Type string `json:"type"` // fallback for old plans
		}
		if err := json.Unmarshal(raw, &typeHeader); err != nil {
			return err
		}

		kind := typeHeader.Kind
		if kind == "" {
			kind = typeHeader.Type
		}

		var action Action
		switch kind {
		case "generate_manifest":
			var a GenerateManifestAction
			if err := json.Unmarshal(raw, &a); err != nil {
				return err
			}
			action = &a
		default:
			// For unknown actions or to maintain some compatibility with old plans
			// we could have a GenericAction, but here we'll just error or skip.
			return fmt.Errorf("unknown action kind: %s", kind)
		}
		p.Actions = append(p.Actions, action)
	}

	return nil
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
func Apply(ctx context.Context, mgr *lock.Manager, input ApplyInput) *ApplyResult {
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
	if mgr == nil {
		result.Status = "failed"
		result.Errors = append(result.Errors, "nil lock manager")
		return result
	}
	projectLock, err := mgr.Acquire(lock.ProjectMutation, "apply", "cli")
	if err != nil {
		result.Status = "failed"
		result.Errors = append(result.Errors, fmt.Sprintf("lock: %v", err))
		return result
	}
	defer func() { _ = mgr.Release(projectLock) }()

	// Create run
	run := ledger.NewRun("apply", ledger.Actor{Type: ledger.ActorAgent, Name: "panex-cli"})
	run.ProjectHash = input.Plan.ProjectHash
	if err := run.Transition(ledger.StatusRunning); err != nil {
		result.Status = "failed"
		result.Errors = append(result.Errors, fmt.Sprintf("transition to running: %v", err))
		return result
	}

	result.RunID = run.RunID

	var completed []Action

	// Execute actions
	for _, action := range input.Plan.Actions {
		step := run.AddStep("apply", action.Kind())

		err := action.Execute(ctx, input)
		if err != nil {
			step.Fail(err.Error())
			result.Failed = append(result.Failed, fmt.Sprintf("%s: %v", action.Desc(), err))
			result.Errors = append(result.Errors, err.Error())

			// Rollback
			_ = run.Transition(ledger.StatusRollingBack)
			for i := len(completed) - 1; i >= 0; i-- {
				rbAction := completed[i]
				rbStep := run.AddStep("rollback", rbAction.Kind())
				if rbErr := rbAction.Rollback(ctx, input); rbErr != nil {
					rbStep.Fail(rbErr.Error())
				} else {
					rbStep.Complete(nil)
				}
			}
			break
		} else {
			step.Complete(action)
			result.Applied = append(result.Applied, action.Desc())
			completed = append(completed, action)
		}
	}

	if len(result.Failed) > 0 {
		_ = run.Transition(ledger.StatusFailed)
		result.Status = "failed"
	} else {
		if err := run.Transition(ledger.StatusSucceeded); err != nil {
			result.Status = "failed"
			result.Errors = append(result.Errors, fmt.Sprintf("transition to succeeded: %v", err))
		} else {
			result.Status = "succeeded"
		}
	}

	// Write run to ledger
	runDir := filepath.Join(input.ProjectDir, ".panex", "runs", run.RunID)
	_ = run.WriteToDir(runDir)

	return result
}

// WritePlan persists a plan to disk.
func WritePlan(p *Plan, path string) error {
	data, err := json.Marshal(p)
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
