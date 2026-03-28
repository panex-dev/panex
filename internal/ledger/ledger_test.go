package ledger

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRunID(t *testing.T) {
	id := NewRunID()
	if !strings.HasPrefix(id, "run_") {
		t.Errorf("expected run_ prefix, got %s", id)
	}
	// Should be unique
	id2 := NewRunID()
	if id == id2 {
		t.Error("expected unique run IDs")
	}
}

func TestNewRun(t *testing.T) {
	r := NewRun("apply", Actor{Type: ActorAgent, Name: "claude-code"})

	if r.Operation != "apply" {
		t.Errorf("operation: got %s", r.Operation)
	}
	if r.Status != StatusCreated {
		t.Errorf("status: got %s, want created", r.Status)
	}
	if r.Actor.Type != ActorAgent {
		t.Errorf("actor type: got %s", r.Actor.Type)
	}
	if r.StartedAt == "" {
		t.Error("expected started_at")
	}
	if !r.Resumable {
		t.Error("new runs should be resumable")
	}
}

func TestRun_Transition_Valid(t *testing.T) {
	tests := []struct {
		from Status
		to   Status
	}{
		{StatusCreated, StatusPlanned},
		{StatusCreated, StatusRunning},
		{StatusPlanned, StatusRunning},
		{StatusRunning, StatusSucceeded},
		{StatusRunning, StatusFailed},
		{StatusRunning, StatusPaused},
		{StatusRunning, StatusAwaitingPolicy},
		{StatusRunning, StatusRollingBack},
		{StatusPaused, StatusRunning},
		{StatusAwaitingPolicy, StatusRunning},
		{StatusRollingBack, StatusFailed},
		{StatusRollingBack, StatusSucceeded},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			r := &Run{Status: tt.from}
			if err := r.Transition(tt.to); err != nil {
				t.Errorf("expected valid transition: %v", err)
			}
			if r.Status != tt.to {
				t.Errorf("status: got %s, want %s", r.Status, tt.to)
			}
		})
	}
}

func TestRun_Transition_Invalid(t *testing.T) {
	tests := []struct {
		from Status
		to   Status
	}{
		{StatusSucceeded, StatusRunning},
		{StatusFailed, StatusRunning},
		{StatusCancelled, StatusRunning},
		{StatusCreated, StatusSucceeded},
		{StatusPlanned, StatusSucceeded},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			r := &Run{Status: tt.from}
			if err := r.Transition(tt.to); err == nil {
				t.Error("expected error for invalid transition")
			}
		})
	}
}

func TestRun_Transition_SetsCompletedAt(t *testing.T) {
	r := &Run{Status: StatusRunning}
	_ = r.Transition(StatusSucceeded)
	if r.CompletedAt == "" {
		t.Error("terminal transition should set completed_at")
	}
}

func TestRun_AddStep(t *testing.T) {
	r := NewRun("apply", Actor{Type: ActorHuman, Name: "user"})

	s1 := r.AddStep("inspector", "inspect")
	if s1.Seq != 1 {
		t.Errorf("seq: got %d, want 1", s1.Seq)
	}
	if s1.Component != "inspector" {
		t.Errorf("component: got %s", s1.Component)
	}
	if s1.Status != StatusRunning {
		t.Errorf("status: got %s, want running", s1.Status)
	}

	s2 := r.AddStep("manifest", "compile")
	if s2.Seq != 2 {
		t.Errorf("seq: got %d, want 2", s2.Seq)
	}

	if len(r.Steps) != 2 {
		t.Errorf("steps: got %d, want 2", len(r.Steps))
	}
}

func TestStep_Complete(t *testing.T) {
	r := NewRun("apply", Actor{Type: ActorAgent, Name: "test"})
	s := r.AddStep("inspector", "inspect")

	s.Complete(map[string]string{"entries": "2"})

	if s.Status != StatusSucceeded {
		t.Errorf("status: got %s, want succeeded", s.Status)
	}
	if s.CompletedAt == "" {
		t.Error("expected completed_at")
	}
	if s.Outputs == nil {
		t.Error("expected outputs")
	}
}

func TestStep_Fail(t *testing.T) {
	r := NewRun("apply", Actor{Type: ActorAgent, Name: "test"})
	s := r.AddStep("manifest", "compile")

	s.Fail("permission conflict")

	if s.Status != StatusFailed {
		t.Errorf("status: got %s, want failed", s.Status)
	}
	if s.Error != "permission conflict" {
		t.Errorf("error: got %s", s.Error)
	}
}

func TestRun_WriteAndRead(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "run_001")

	r := NewRun("apply", Actor{Type: ActorAgent, Name: "claude-code"})
	r.ProjectHash = "sha256:abc"
	r.ConfigHash = "sha256:def"

	s := r.AddStep("inspector", "inspect")
	s.Complete("ok")

	if err := r.WriteToDir(dir); err != nil {
		t.Fatalf("WriteToDir: %v", err)
	}

	got, err := ReadFromDir(dir)
	if err != nil {
		t.Fatalf("ReadFromDir: %v", err)
	}

	if got.RunID != r.RunID {
		t.Errorf("run_id: got %s, want %s", got.RunID, r.RunID)
	}
	if got.ProjectHash != "sha256:abc" {
		t.Errorf("project_hash: got %s", got.ProjectHash)
	}
	if len(got.Steps) != 1 {
		t.Errorf("steps: got %d, want 1", len(got.Steps))
	}
	if got.Steps[0].Component != "inspector" {
		t.Errorf("step component: got %s", got.Steps[0].Component)
	}
}
