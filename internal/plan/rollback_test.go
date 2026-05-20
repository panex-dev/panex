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

type rollbackTestAction struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

func (a *rollbackTestAction) Kind() string     { return "rollback_test_action" }
func (a *rollbackTestAction) Risk() string     { return "safe" }
func (a *rollbackTestAction) Reversible() bool { return true }
func (a *rollbackTestAction) Describe() string { return fmt.Sprintf("write %s", a.ID) }

func (a *rollbackTestAction) Execute(_ ExecContext) error {
	return os.WriteFile(a.Path, []byte(a.ID), 0o644)
}

func (a *rollbackTestAction) Rollback(_ ExecContext) error {
	return os.Remove(a.Path)
}

func TestRollback_ReversesSucceededApplySteps(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.txt")
	secondPath := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(firstPath, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &Plan{Actions: ActionList{
		&rollbackTestAction{ID: "first", Path: firstPath},
		&rollbackTestAction{ID: "second", Path: secondPath},
	}}
	run := failedRollbackRun(t, []ledger.Status{ledger.StatusSucceeded, ledger.StatusSucceeded})

	result := Rollback(context.Background(), lock.NewManager(dir), RollbackInput{
		ProjectDir: dir,
		Run:        run,
		Plan:       p,
	})

	if result.Status != "rolled_back" {
		t.Fatalf("status=%s errors=%v", result.Status, result.Errors)
	}
	if len(result.RolledBack) != 2 {
		t.Fatalf("rolled_back=%v, want both actions", result.RolledBack)
	}
	for _, path := range []string{firstPath, secondPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s removed, stat err=%v", path, err)
		}
	}
	if run.Status != ledger.StatusFailed {
		t.Fatalf("run status=%s, want failed terminal state after rollback", run.Status)
	}
}

func TestRollback_DoesNotRepeatSucceededRollbackSteps(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.txt")
	secondPath := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(firstPath, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &Plan{Actions: ActionList{
		&rollbackTestAction{ID: "first", Path: firstPath},
		&rollbackTestAction{ID: "second", Path: secondPath},
	}}
	run := failedRollbackRun(t, []ledger.Status{ledger.StatusSucceeded, ledger.StatusSucceeded})
	rollbackStep := run.AddStep("rollback", "rollback_test_action")
	rollbackStep.Complete(nil)

	result := Rollback(context.Background(), lock.NewManager(dir), RollbackInput{
		ProjectDir: dir,
		Run:        run,
		Plan:       p,
	})

	if result.Status != "rolled_back" {
		t.Fatalf("status=%s errors=%v", result.Status, result.Errors)
	}
	if len(result.RolledBack) != 1 || result.RolledBack[0] != "write first" {
		t.Fatalf("rolled_back=%v, want only remaining first action", result.RolledBack)
	}
	if _, err := os.Stat(firstPath); !os.IsNotExist(err) {
		t.Fatalf("expected first action rolled back, stat err=%v", err)
	}
}

func TestRollback_RejectsSucceededRun(t *testing.T) {
	dir := t.TempDir()
	run := ledger.NewRun("apply", ledger.Actor{Type: ledger.ActorAgent, Name: "test"})
	if err := run.Transition(ledger.StatusRunning); err != nil {
		t.Fatal(err)
	}
	if err := run.Transition(ledger.StatusSucceeded); err != nil {
		t.Fatal(err)
	}

	result := Rollback(context.Background(), lock.NewManager(dir), RollbackInput{
		ProjectDir: dir,
		Run:        run,
		Plan:       &Plan{},
	})

	if result.Status != "failed" {
		t.Fatalf("status=%s, want failed", result.Status)
	}
}

func failedRollbackRun(t *testing.T, statuses []ledger.Status) *ledger.Run {
	t.Helper()
	run := ledger.NewRun("apply", ledger.Actor{Type: ledger.ActorAgent, Name: "test"})
	if err := run.Transition(ledger.StatusRunning); err != nil {
		t.Fatal(err)
	}
	for _, status := range statuses {
		step := run.AddStep("apply", "rollback_test_action")
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
