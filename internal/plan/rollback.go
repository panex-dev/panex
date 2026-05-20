package plan

import (
	"context"
	"fmt"

	"github.com/panex-dev/panex/internal/ledger"
	"github.com/panex-dev/panex/internal/lock"
)

// RollbackInput contains the durable run and plan records needed to roll back
// previously applied mutation steps.
type RollbackInput struct {
	ProjectDir string
	Run        *ledger.Run
	Plan       *Plan
}

// RollbackResult describes a manual rollback attempt.
type RollbackResult struct {
	RunID      string   `json:"run_id"`
	Status     string   `json:"status"`
	RolledBack []string `json:"rolled_back,omitempty"`
	Skipped    []string `json:"skipped,omitempty"`
	Failed     []string `json:"failed,omitempty"`
	Errors     []string `json:"errors,omitempty"`
}

// Rollback reverses recorded successful apply steps in reverse plan order. It
// is intended for failed or interrupted apply runs whose automatic rollback did
// not complete.
func Rollback(ctx context.Context, mgr *lock.Manager, input RollbackInput) *RollbackResult {
	result := &RollbackResult{}
	if input.Run == nil {
		result.Status = "failed"
		result.Errors = append(result.Errors, "nil run")
		return result
	}
	result.RunID = input.Run.RunID
	if input.Plan == nil {
		result.Status = "failed"
		result.Errors = append(result.Errors, "nil plan")
		return result
	}
	if input.Run.Operation != "apply" {
		result.Status = "failed"
		result.Errors = append(result.Errors, fmt.Sprintf("run %s is %q, not apply", input.Run.RunID, input.Run.Operation))
		return result
	}
	if input.Run.Status == ledger.StatusSucceeded {
		result.Status = "failed"
		result.Errors = append(result.Errors, fmt.Sprintf("cannot rollback succeeded run %s", input.Run.RunID))
		return result
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		result.Status = "failed"
		result.Errors = append(result.Errors, err.Error())
		return result
	}

	if mgr != nil {
		projectLock, err := mgr.Acquire(lock.ProjectMutation, "rollback", "mcp")
		if err != nil {
			result.Status = "failed"
			result.Errors = append(result.Errors, fmt.Sprintf("lock: %v", err))
			return result
		}
		defer func() { _ = mgr.Release(projectLock) }()
	}

	targets := rollbackTargets(input.Run, input.Plan.Actions)
	if len(targets) == 0 {
		result.Status = "noop"
		return result
	}

	if input.Run.Status != ledger.StatusRollingBack {
		if err := input.Run.Transition(ledger.StatusRollingBack); err != nil {
			result.Status = "failed"
			result.Errors = append(result.Errors, err.Error())
			return result
		}
	}

	ctxExec := ExecContext{ProjectDir: input.ProjectDir}
	for _, action := range targets {
		if err := ctx.Err(); err != nil {
			result.Errors = append(result.Errors, err.Error())
			result.Status = "failed"
			finalizeRollbackRun(input.Run, result)
			return result
		}
		if !action.Reversible() {
			result.Skipped = append(result.Skipped, action.Describe())
			continue
		}

		step := input.Run.AddStep("rollback", action.Kind())
		if err := action.Rollback(ctxExec); err != nil {
			step.Fail(err.Error())
			result.Failed = append(result.Failed, fmt.Sprintf("%s: %v", action.Describe(), err))
			result.Errors = append(result.Errors, fmt.Sprintf("rollback %s: %v", action.Describe(), err))
			continue
		}
		step.Complete(nil)
		result.RolledBack = append(result.RolledBack, action.Describe())
	}

	if len(result.Errors) > 0 {
		result.Status = "failed"
	} else if len(result.RolledBack) == 0 {
		result.Status = "noop"
	} else {
		result.Status = "rolled_back"
	}
	finalizeRollbackRun(input.Run, result)
	return result
}

func rollbackTargets(run *ledger.Run, actions ActionList) []Action {
	applySteps := make([]ledger.Step, 0, len(actions))
	rollbackSteps := 0
	for _, step := range run.Steps {
		switch step.Component {
		case "apply":
			applySteps = append(applySteps, step)
		case "rollback":
			if step.Status == ledger.StatusSucceeded {
				rollbackSteps++
			}
		}
	}

	targets := make([]Action, 0, len(actions))
	for i, action := range actions {
		if i >= len(applySteps) {
			break
		}
		step := applySteps[i]
		if step.Action == action.Kind() && step.Status == ledger.StatusSucceeded {
			targets = append(targets, action)
		}
	}

	reverse(targets)
	if rollbackSteps >= len(targets) {
		return nil
	}
	return targets[rollbackSteps:]
}

func reverse(actions []Action) {
	for i, j := 0, len(actions)-1; i < j; i, j = i+1, j-1 {
		actions[i], actions[j] = actions[j], actions[i]
	}
}

func finalizeRollbackRun(run *ledger.Run, result *RollbackResult) {
	if run.Status != ledger.StatusRollingBack {
		return
	}
	if err := run.Transition(ledger.StatusFailed); err != nil {
		result.Status = "failed"
		result.Errors = append(result.Errors, fmt.Sprintf("transition rolling-back->failed: %v", err))
	}
}
