// Package ledger implements the run ledger — the durable, structured
// record of every Panex operation. Spec section 22.
package ledger

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Status is the canonical run status.
type Status string

const (
	StatusCreated        Status = "created"
	StatusPlanned        Status = "planned"
	StatusRunning        Status = "running"
	StatusPaused         Status = "paused"
	StatusAwaitingPolicy Status = "awaiting-policy"
	StatusRollingBack    Status = "rolling-back"
	StatusSucceeded      Status = "succeeded"
	StatusFailed         Status = "failed"
	StatusCancelled      Status = "cancelled"
	StatusExpired        Status = "expired"
)

// ActorType identifies who initiated the run.
type ActorType string

const (
	ActorHuman ActorType = "human"
	ActorAgent ActorType = "agent"
	ActorCI    ActorType = "ci"
)

// Run is the durable record of a Panex operation.
type Run struct {
	RunID       string   `json:"run_id"`
	Operation   string   `json:"operation"`
	Status      Status   `json:"status"`
	ProjectHash string   `json:"project_hash"`
	ConfigHash  string   `json:"config_hash"`
	PolicyHash  string   `json:"policy_hash"`
	StartedAt   string   `json:"started_at"`
	CompletedAt string   `json:"completed_at,omitempty"`
	Actor       Actor    `json:"actor"`
	Steps       []Step   `json:"steps"`
	Artifacts   []string `json:"artifacts"`
	Reports     []string `json:"reports"`
	Resumable   bool     `json:"resumable"`
	Error       *RunError `json:"error,omitempty"`
}

// Actor is who initiated the run.
type Actor struct {
	Type ActorType `json:"type"`
	Name string    `json:"name"`
}

// Step is a single step within a run.
type Step struct {
	Seq         int    `json:"seq"`
	Component   string `json:"component"`
	Action      string `json:"action"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at,omitempty"`
	Status      Status `json:"status"`
	Inputs      any    `json:"inputs,omitempty"`
	Outputs     any    `json:"outputs,omitempty"`
	Error       string `json:"error,omitempty"`
	Rollback    string `json:"rollback,omitempty"`
}

// RunError is a structured error attached to a failed run.
type RunError struct {
	Code     string `json:"code"`
	Category string `json:"category"`
	Message  string `json:"message"`
}

// NewRunID generates a unique run ID.
func NewRunID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "run_" + hex.EncodeToString(b)
}

// NewRun creates a new run record in the created state.
func NewRun(operation string, actor Actor) *Run {
	return &Run{
		RunID:     NewRunID(),
		Operation: operation,
		Status:    StatusCreated,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Actor:     actor,
		Steps:     []Step{},
		Artifacts: []string{},
		Reports:   []string{},
		Resumable: true,
	}
}

// Transition changes the run status. It returns an error if the
// transition is not valid.
func (r *Run) Transition(to Status) error {
	if !validTransition(r.Status, to) {
		return fmt.Errorf("invalid transition: %s -> %s", r.Status, to)
	}
	r.Status = to
	if isTerminal(to) {
		r.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return nil
}

// AddStep appends a new step to the run.
func (r *Run) AddStep(component, action string) *Step {
	s := Step{
		Seq:       len(r.Steps) + 1,
		Component: component,
		Action:    action,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Status:    StatusRunning,
	}
	r.Steps = append(r.Steps, s)
	return &r.Steps[len(r.Steps)-1]
}

// CompleteStep marks the latest matching step as succeeded.
func (s *Step) Complete(outputs any) {
	s.Status = StatusSucceeded
	s.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	s.Outputs = outputs
}

// FailStep marks a step as failed.
func (s *Step) Fail(errMsg string) {
	s.Status = StatusFailed
	s.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	s.Error = errMsg
}

// --- Persistence ---

// WriteToDir writes the run record to a directory.
func (r *Run) WriteToDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create run dir: %w", err)
	}
	path := filepath.Join(dir, "run.json")
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run: %w", err)
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadFromDir reads a run record from a directory.
func ReadFromDir(dir string) (*Run, error) {
	path := filepath.Join(dir, "run.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read run: %w", err)
	}
	var r Run
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse run: %w", err)
	}
	return &r, nil
}

// --- state machine ---

func validTransition(from, to Status) bool {
	allowed := map[Status][]Status{
		StatusCreated:        {StatusPlanned, StatusRunning, StatusFailed, StatusCancelled},
		StatusPlanned:        {StatusRunning, StatusFailed, StatusCancelled},
		StatusRunning:        {StatusSucceeded, StatusFailed, StatusPaused, StatusAwaitingPolicy, StatusRollingBack},
		StatusPaused:         {StatusRunning, StatusFailed, StatusCancelled},
		StatusAwaitingPolicy: {StatusRunning, StatusFailed, StatusCancelled},
		StatusRollingBack:    {StatusFailed, StatusSucceeded},
	}
	valid, ok := allowed[from]
	if !ok {
		return false
	}
	for _, s := range valid {
		if s == to {
			return true
		}
	}
	return false
}

func isTerminal(s Status) bool {
	return s == StatusSucceeded || s == StatusFailed || s == StatusCancelled || s == StatusExpired
}
