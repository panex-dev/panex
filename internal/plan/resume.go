package plan

import (
	"context"
	"fmt"

	"github.com/panex-dev/panex/internal/ledger"
	"github.com/panex-dev/panex/internal/lock"
)

// ResumeInput contains the durable run and plan records needed to replay a
// failed or incomplete apply operation.
type ResumeInput struct {
	ProjectDir string
	Run        *ledger.Run
	Plan       *Plan
}

// ResumeResult describes the steps replayed by resume.
type ResumeResult struct {
	RunID    string   `json:"run_id"`
	Status   string   `json:"status"`
	Skipped  []string `json:"skipped,omitempty"`
	Replayed []string `json:"replayed,omitempty"`
	Failed   []string `json:"failed,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

// Resume replays failed or incomplete apply steps from a saved plan. Previously
// succeeded steps are skipped unless the original run rolled them back.
func Resume(ctx context.Context, mgr *lock.Manager, input ResumeInput) *ResumeResult {
	result := &ResumeResult{}
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
	if !input.Run.Resumable || input.Run.Status == ledger.StatusSucceeded {
		result.Status = "failed"
		result.Errors = append(result.Errors,
			fmt.Sprintf("run %s is not resumable (status: %s)", input.Run.RunID, input.Run.Status))
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
		projectLock, err := mgr.Acquire(lock.ProjectMutation, "resume", "cli")
		if err != nil {
			result.Status = "failed"
			result.Errors = append(result.Errors, fmt.Sprintf("lock: %v", err))
			return result
		}
		defer func() { _ = mgr.Release(projectLock) }()
	}

	if input.Run.Status != ledger.StatusRunning {
		if err := input.Run.Transition(ledger.StatusRunning); err != nil {
			result.Status = "failed"
			result.Errors = append(result.Errors, err.Error())
			return result
		}
	}

	ctxExec := ExecContext{ProjectDir: input.ProjectDir}
	replayStart := replayStartIndex(input.Run, input.Plan.Actions)
	for i, action := range input.Plan.Actions {
		if i < replayStart {
			result.Skipped = append(result.Skipped, action.Describe())
			continue
		}
		if err := ctx.Err(); err != nil {
			result.Errors = append(result.Errors, err.Error())
			if transitionErr := input.Run.Transition(ledger.StatusFailed); transitionErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("transition ->failed: %v", transitionErr))
			}
			result.Status = "failed"
			return result
		}

		step := input.Run.AddStep("resume", action.Kind())
		if err := action.Execute(ctxExec); err != nil {
			step.Fail(err.Error())
			result.Failed = append(result.Failed, fmt.Sprintf("%s: %v", action.Describe(), err))
			result.Errors = append(result.Errors, err.Error())
			if transitionErr := input.Run.Transition(ledger.StatusFailed); transitionErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("transition ->failed: %v", transitionErr))
			}
			result.Status = "failed"
			return result
		}
		step.Complete(nil)
		result.Replayed = append(result.Replayed, action.Describe())
	}

	if err := input.Run.Transition(ledger.StatusSucceeded); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("transition ->succeeded: %v", err))
		result.Status = "failed"
		return result
	}
	result.Status = "succeeded"
	return result
}

func replayStartIndex(run *ledger.Run, actions ActionList) int {
	if rolledBack(run) {
		return 0
	}
	applySteps := make([]ledger.Step, 0, len(actions))
	for _, step := range run.Steps {
		if step.Component == "apply" {
			applySteps = append(applySteps, step)
		}
	}
	for i, action := range actions {
		if i >= len(applySteps) {
			return i
		}
		step := applySteps[i]
		if step.Action != action.Kind() || step.Status != ledger.StatusSucceeded {
			return i
		}
	}
	return len(actions)
}

func rolledBack(run *ledger.Run) bool {
	for _, step := range run.Steps {
		if step.Component == "rollback" && step.Status == ledger.StatusSucceeded {
			return true
		}
	}
	return false
}
