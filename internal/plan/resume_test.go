package plan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/panex-dev/panex/internal/ledger"
	"github.com/panex-dev/panex/internal/lock"
)

type resumeTestAction struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Fail bool   `json:"fail,omitempty"`
}

func (a *resumeTestAction) Kind() string     { return "resume_test_action" }
func (a *resumeTestAction) Risk() string     { return "safe" }
func (a *resumeTestAction) Reversible() bool { return true }
func (a *resumeTestAction) Describe() string { return fmt.Sprintf("write %s", a.ID) }

func (a *resumeTestAction) Execute(_ ExecContext) error {
	if a.Fail {
		return fmt.Errorf("forced failure for %s", a.ID)
	}
	return os.WriteFile(a.Path, []byte(a.ID), 0o644)
}

func (a *resumeTestAction) Rollback(_ ExecContext) error {
	return os.Remove(a.Path)
}

func registerResumeTestAction(t *testing.T) {
	t.Helper()
	previous, hadPrevious := actionRegistry["resume_test_action"]
	actionRegistry["resume_test_action"] = func() Action { return &resumeTestAction{} }
	t.Cleanup(func() {
		if hadPrevious {
			actionRegistry["resume_test_action"] = previous
		} else {
			delete(actionRegistry, "resume_test_action")
		}
	})
}

func TestResume_ReplaysFailedAndIncompleteSteps(t *testing.T) {
	registerResumeTestAction(t)
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.txt")
	secondPath := filepath.Join(dir, "second.txt")
	p := &Plan{Actions: ActionList{
		&resumeTestAction{ID: "first", Path: firstPath},
		&resumeTestAction{ID: "second", Path: secondPath},
	}}
	run := failedApplyRun(t, []ledger.Status{ledger.StatusSucceeded, ledger.StatusFailed})

	result := Resume(context.Background(), lock.NewManager(dir), ResumeInput{
		ProjectDir: dir,
		Run:        run,
		Plan:       p,
	})

	if result.Status != "succeeded" {
		t.Fatalf("status=%s errors=%v", result.Status, result.Errors)
	}
	if len(result.Skipped) != 1 || result.Skipped[0] != "write first" {
		t.Fatalf("skipped=%v, want first action skipped", result.Skipped)
	}
	if len(result.Replayed) != 1 || result.Replayed[0] != "write second" {
		t.Fatalf("replayed=%v, want second action replayed", result.Replayed)
	}
	if _, err := os.Stat(firstPath); !os.IsNotExist(err) {
		t.Fatalf("first action should not have been replayed, stat err=%v", err)
	}
	if got, err := os.ReadFile(secondPath); err != nil || string(got) != "second" {
		t.Fatalf("second action replay: got %q err=%v", string(got), err)
	}
}

func TestResume_ReplaysAllActionsAfterRollback(t *testing.T) {
	registerResumeTestAction(t)
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.txt")
	secondPath := filepath.Join(dir, "second.txt")
	p := &Plan{Actions: ActionList{
		&resumeTestAction{ID: "first", Path: firstPath},
		&resumeTestAction{ID: "second", Path: secondPath},
	}}
	run := failedApplyRun(t, []ledger.Status{ledger.StatusSucceeded, ledger.StatusFailed})
	rollbackStep := run.AddStep("rollback", "resume_test_action")
	rollbackStep.Complete(nil)

	result := Resume(context.Background(), lock.NewManager(dir), ResumeInput{
		ProjectDir: dir,
		Run:        run,
		Plan:       p,
	})

	if result.Status != "succeeded" {
		t.Fatalf("status=%s errors=%v", result.Status, result.Errors)
	}
	if len(result.Skipped) != 0 {
		t.Fatalf("skipped=%v, want rollback run to replay all actions", result.Skipped)
	}
	if len(result.Replayed) != 2 {
		t.Fatalf("replayed=%v, want both actions replayed", result.Replayed)
	}
	for _, path := range []string{firstPath, secondPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected replayed file %s: %v", path, err)
		}
	}
}

func TestResume_RejectsSucceededRun(t *testing.T) {
	dir := t.TempDir()
	run := ledger.NewRun("apply", ledger.Actor{Type: ledger.ActorAgent, Name: "test"})
	if err := run.Transition(ledger.StatusRunning); err != nil {
		t.Fatal(err)
	}
	if err := run.Transition(ledger.StatusSucceeded); err != nil {
		t.Fatal(err)
	}

	result := Resume(context.Background(), lock.NewManager(dir), ResumeInput{
		ProjectDir: dir,
		Run:        run,
		Plan:       &Plan{},
	})

	if result.Status != "failed" {
		t.Fatalf("status=%s, want failed", result.Status)
	}
}

func failedApplyRun(t *testing.T, statuses []ledger.Status) *ledger.Run {
	t.Helper()
	run := ledger.NewRun("apply", ledger.Actor{Type: ledger.ActorAgent, Name: "test"})
	if err := run.Transition(ledger.StatusRunning); err != nil {
		t.Fatal(err)
	}
	for _, status := range statuses {
		step := run.AddStep("apply", "resume_test_action")
		switch status {
		case ledger.StatusSucceeded:
			step.Complete(nil)
		case ledger.StatusFailed:
			step.Fail("boom")
		default:
			step.Status = status
		}
	}
	if err := run.Transition(ledger.StatusFailed); err != nil {
		t.Fatal(err)
	}
	return run
}
