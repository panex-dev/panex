package plan

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/panex-dev/panex/internal/capability"
	"github.com/panex-dev/panex/internal/graph"
	"github.com/panex-dev/panex/internal/lock"
	"github.com/panex-dev/panex/internal/manifest"
	"github.com/panex-dev/panex/internal/target"
)

// TestApply_MultiTarget_WritesDistinctManifests is the C1 regression.
// The previous flat-Action design wrote the last target's manifest to
// every per-target Path. Each target must end up with its own manifest.
func TestApply_MultiTarget_WritesDistinctManifests(t *testing.T) {
	dir := t.TempDir()
	setupPanexDir(t, dir)

	g := &graph.Graph{
		SchemaVersion:   1,
		Project:         graph.ProjectIdentity{Name: "multi", ID: "multi"},
		TargetsResolved: []string{"chrome"},
		Capabilities:    map[string]any{"tabs": true},
	}

	// Compile a real CompileResult, then synthesize a second target by
	// duplicating the chrome output with a distinct marker. This isolates
	// the per-target write path from adapter availability.
	base := manifest.Compile(manifest.CompileInput{
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
	if len(base.Outputs) == 0 {
		t.Fatalf("base compile produced no outputs: %v", base.Errors)
	}

	chromeOut := base.Outputs[0]
	chromeOut.Target = "chrome"
	chromeOut.Manifest["panex_marker"] = "chrome-only"

	firefoxManifest := map[string]any{}
	for k, v := range base.Outputs[0].Manifest {
		firefoxManifest[k] = v
	}
	firefoxManifest["panex_marker"] = "firefox-only"
	firefoxOut := manifest.CompileOutput{
		Target:   "firefox",
		Manifest: firefoxManifest,
	}

	manifestResult := &manifest.CompileResult{Outputs: []manifest.CompileOutput{chromeOut, firefoxOut}}

	p, err := ComputePlan(PlanInput{ProjectDir: dir, Graph: g, ManifestResult: manifestResult})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(p.Actions) != 2 {
		t.Fatalf("expected 2 actions (one per target), got %d", len(p.Actions))
	}

	ctx := context.Background()
	mgr := lock.NewManager(dir)
	result := Apply(ctx, mgr, ApplyInput{ProjectDir: dir, Plan: p, Graph: g, ManifestResult: manifestResult})
	if result.Status != "succeeded" {
		t.Fatalf("apply: status=%s errors=%v", result.Status, result.Errors)
	}

	chromePath := filepath.Join(dir, ".panex", "runs", "generated", "manifests", "chrome", "manifest.json")
	firefoxPath := filepath.Join(dir, ".panex", "runs", "generated", "manifests", "firefox", "manifest.json")

	chromeData := readManifest(t, chromePath)
	firefoxData := readManifest(t, firefoxPath)

	if chromeData["panex_marker"] != "chrome-only" {
		t.Errorf("chrome manifest marker = %v, want chrome-only", chromeData["panex_marker"])
	}
	if firefoxData["panex_marker"] != "firefox-only" {
		t.Errorf("firefox manifest marker = %v, want firefox-only", firefoxData["panex_marker"])
	}
}

// TestApply_RollbackOnFailure verifies H6: when a later action fails,
// earlier successful actions are reversed in reverse order.
func TestApply_RollbackOnFailure(t *testing.T) {
	dir := t.TempDir()
	setupPanexDir(t, dir)

	g := makeTestGraph()

	goodPath := filepath.Join(dir, "good", "manifest.json")
	// badPath is unwritable — the parent is a regular file, so MkdirAll fails.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(blocker, "manifest.json")

	p := &Plan{
		PlanID:      "test-plan",
		ProjectHash: "ignored", // drift skipped via Force
		Actions: ActionList{
			&GenerateManifestAction{Target: "good", Path: goodPath, Manifest: map[string]any{"k": "v"}},
			&GenerateManifestAction{Target: "bad", Path: badPath, Manifest: map[string]any{"k": "v"}},
		},
	}

	ctx := context.Background()
	mgr := lock.NewManager(dir)
	result := Apply(ctx, mgr, ApplyInput{ProjectDir: dir, Plan: p, Graph: g, Force: true})

	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s (errors=%v)", result.Status, result.Errors)
	}
	if len(result.Applied) != 1 {
		t.Errorf("expected 1 applied, got %d: %v", len(result.Applied), result.Applied)
	}
	if len(result.RolledBack) != 1 {
		t.Errorf("expected 1 rolled back, got %d: %v", len(result.RolledBack), result.RolledBack)
	}
	if _, err := os.Stat(goodPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("good manifest should have been removed by rollback, got err=%v", err)
	}
}

// TestActionList_RoundTrip verifies polymorphic actions survive JSON
// serialization through the registry.
func TestActionList_RoundTrip(t *testing.T) {
	original := ActionList{
		&GenerateManifestAction{Target: "chrome", Path: "/p/chrome.json", Manifest: map[string]any{"name": "x"}},
		&GenerateManifestAction{Target: "firefox", Path: "/p/firefox.json", Manifest: map[string]any{"name": "y"}},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var roundtrip ActionList
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(roundtrip) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(roundtrip))
	}
	for i, a := range roundtrip {
		gm, ok := a.(*GenerateManifestAction)
		if !ok {
			t.Fatalf("action %d: wrong type %T", i, a)
		}
		want := original[i].(*GenerateManifestAction)
		if gm.Target != want.Target || gm.Path != want.Path {
			t.Errorf("action %d: got %+v want %+v", i, gm, want)
		}
	}
}

// TestActionList_UnknownType rejects unregistered action kinds.
func TestActionList_UnknownType(t *testing.T) {
	data := []byte(`[{"type":"install_dependency","spec":{"name":"foo"}}]`)
	var al ActionList
	if err := al.UnmarshalJSON(data); err == nil {
		t.Fatal("expected error for unknown action type")
	}
}

func readManifest(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return m
}
