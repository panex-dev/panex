package plan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/panex-dev/panex/internal/capability"
	"github.com/panex-dev/panex/internal/graph"
	"github.com/panex-dev/panex/internal/lock"
	"github.com/panex-dev/panex/internal/manifest"
	"github.com/panex-dev/panex/internal/target"
)

func makeTestGraph() *graph.Graph {
	return &graph.Graph{
		SchemaVersion:   1,
		Project:         graph.ProjectIdentity{Name: "test-ext", ID: "test-ext"},
		TargetsResolved: []string{"chrome"},
		Entries: map[string]graph.Entry{
			"background": {Path: "background.js", Type: "service-worker"},
		},
		Capabilities: map[string]any{"tabs": true},
	}
}

func makeManifestResult() *manifest.CompileResult {
	g := makeTestGraph()
	return manifest.Compile(manifest.CompileInput{
		Graph: g,
		Matrix: &capability.TargetMatrix{
			Resolutions: []capability.Resolution{
				{Capability: "tabs", Target: "chrome", State: "native", Permissions: []string{"tabs"}},
			},
			Permissions: []string{"tabs"},
		},
		Adapters: map[string]target.Adapter{"chrome": target.NewChrome()},
		Version:  "1.0.0",
	})
}

func TestComputePlan_Basic(t *testing.T) {
	dir := t.TempDir()
	g := makeTestGraph()

	p, err := ComputePlan(PlanInput{
		ProjectDir:     dir,
		Graph:          g,
		ManifestResult: makeManifestResult(),
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if p.PlanID == "" {
		t.Error("expected plan ID")
	}
	if p.ProjectHash == "" {
		t.Error("expected project hash")
	}
	if len(p.Actions) == 0 {
		t.Error("expected actions")
	}
	if p.EstimatedSteps != len(p.Actions) {
		t.Errorf("estimated steps: got %d, want %d", p.EstimatedSteps, len(p.Actions))
	}
}

func TestComputePlan_NilGraph(t *testing.T) {
	_, err := ComputePlan(PlanInput{})
	if err == nil {
		t.Error("expected error for nil graph")
	}
}

func TestComputePlan_PermissionDiff(t *testing.T) {
	dir := t.TempDir()
	g := makeTestGraph()

	p, err := ComputePlan(PlanInput{
		ProjectDir:     dir,
		Graph:          g,
		ManifestResult: makeManifestResult(),
		PreviousPerms:  []string{}, // no previous permissions
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if p.PermissionDiff == nil {
		t.Fatal("expected permission diff")
	}
	if len(p.PermissionDiff.Added) == 0 {
		t.Error("expected added permissions")
	}
}

func TestApply_Basic(t *testing.T) {
	dir := t.TempDir()
	setupPanexDir(t, dir)

	g := makeTestGraph()

	p, _ := ComputePlan(PlanInput{
		ProjectDir:     dir,
		Graph:          g,
		ManifestResult: makeManifestResult(),
	})

	result := Apply(ApplyInput{
		ProjectDir:     dir,
		Plan:           p,
		Graph:          g,
		ManifestResult: makeManifestResult(),
	})

	if result.Status != "succeeded" {
		t.Errorf("status: got %s, errors: %v", result.Status, result.Errors)
	}
	if result.RunID == "" {
		t.Error("expected run ID")
	}
	if len(result.Applied) == 0 {
		t.Error("expected applied actions")
	}
}

func TestApply_DriftDetection(t *testing.T) {
	dir := t.TempDir()
	setupPanexDir(t, dir)

	g := makeTestGraph()

	p, _ := ComputePlan(PlanInput{
		ProjectDir: dir,
		Graph:      g,
	})

	// Mutate the graph after planning
	g.Entries["popup"] = graph.Entry{Path: "popup.html"}

	result := Apply(ApplyInput{
		ProjectDir: dir,
		Plan:       p,
		Graph:      g,
	})

	if result.Status != "drift_detected" {
		t.Errorf("expected drift_detected, got %s", result.Status)
	}
}

func TestApply_DriftForceSkip(t *testing.T) {
	dir := t.TempDir()
	setupPanexDir(t, dir)

	g := makeTestGraph()

	p, _ := ComputePlan(PlanInput{
		ProjectDir: dir,
		Graph:      g,
	})

	// Mutate graph
	g.Entries["popup"] = graph.Entry{Path: "popup.html"}

	result := Apply(ApplyInput{
		ProjectDir: dir,
		Plan:       p,
		Graph:      g,
		Force:      true, // skip drift check
	})

	// Should proceed (may succeed or fail on actions, but not drift)
	if result.Status == "drift_detected" {
		t.Error("force should skip drift detection")
	}
}

func TestApply_WithLock(t *testing.T) {
	dir := t.TempDir()
	setupPanexDir(t, dir)

	g := makeTestGraph()
	mgr := lock.NewManager(filepath.Join(dir, ".panex"))

	p, _ := ComputePlan(PlanInput{
		ProjectDir: dir,
		Graph:      g,
	})

	result := Apply(ApplyInput{
		ProjectDir:  dir,
		Plan:        p,
		Graph:       g,
		LockManager: mgr,
	})

	if result.Status != "succeeded" {
		t.Errorf("status: got %s, errors: %v", result.Status, result.Errors)
	}

	// Lock should be released
	held, _ := mgr.IsHeld(lock.ProjectMutation)
	if held {
		t.Error("lock should be released after apply")
	}
}

func TestApply_NilPlan(t *testing.T) {
	result := Apply(ApplyInput{})
	if result.Status != "failed" {
		t.Error("expected failed for nil plan")
	}
}

func TestWriteReadPlan(t *testing.T) {
	dir := t.TempDir()
	g := makeTestGraph()

	p, _ := ComputePlan(PlanInput{
		ProjectDir: dir,
		Graph:      g,
	})

	path := filepath.Join(dir, "plan.json")
	if err := WritePlan(p, path); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := ReadPlan(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if loaded.PlanID != p.PlanID {
		t.Errorf("plan ID: got %s, want %s", loaded.PlanID, p.PlanID)
	}
	if loaded.ProjectHash != p.ProjectHash {
		t.Errorf("project hash: got %s, want %s", loaded.ProjectHash, p.ProjectHash)
	}
}

func TestApply_RunRecorded(t *testing.T) {
	dir := t.TempDir()
	setupPanexDir(t, dir)

	g := makeTestGraph()

	p, _ := ComputePlan(PlanInput{
		ProjectDir:     dir,
		Graph:          g,
		ManifestResult: makeManifestResult(),
	})

	result := Apply(ApplyInput{
		ProjectDir:     dir,
		Plan:           p,
		Graph:          g,
		ManifestResult: makeManifestResult(),
	})

	// Verify run was written
	runsDir := filepath.Join(dir, ".panex", "runs")
	runDir := filepath.Join(runsDir, result.RunID)
	if _, err := os.Stat(filepath.Join(runDir, "run.json")); err != nil {
		t.Error("expected run.json in run dir")
	}
}

// --- helpers ---

func setupPanexDir(t *testing.T, dir string) {
	t.Helper()
	for _, d := range []string{".panex", ".panex/runs", ".panex/locks"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}
