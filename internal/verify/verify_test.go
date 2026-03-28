package verify

import (
	"testing"

	"github.com/panex-dev/panex/internal/capability"
	"github.com/panex-dev/panex/internal/graph"
)

func TestVerify_HealthyProject(t *testing.T) {
	g := &graph.Graph{
		Project:         graph.ProjectIdentity{Name: "test", ID: "dev.test"},
		TargetsResolved: []string{"chrome"},
		Entries:         map[string]graph.Entry{"background": {Path: "bg.ts", Type: "service-worker"}},
		GraphHash:       "sha256:abc",
	}
	m := &capability.TargetMatrix{
		Resolutions: []capability.Resolution{
			{Capability: "tabs", Target: "chrome", State: "native", Permissions: []string{"tabs"}},
		},
		Permissions: []string{"tabs"},
	}

	r := Verify(Input{Graph: g, Matrix: m})

	if r.Status != "passed" {
		t.Errorf("expected passed, got %s", r.Status)
	}
	if len(r.HardBlocks) > 0 {
		t.Errorf("unexpected hard blocks: %v", r.HardBlocks)
	}
}

func TestVerify_BlockedCapability(t *testing.T) {
	g := &graph.Graph{
		TargetsResolved: []string{"chrome"},
		Entries:         map[string]graph.Entry{"background": {Path: "bg.ts"}},
		GraphHash:       "sha256:abc",
	}
	m := &capability.TargetMatrix{
		Resolutions: []capability.Resolution{
			{Capability: "sidebarSurface", Target: "chrome", State: "blocked", Reason: "not supported"},
		},
	}

	r := Verify(Input{Graph: g, Matrix: m})

	if r.Status != "failed" {
		t.Error("expected failed for blocked capability")
	}
	if len(r.HardBlocks) == 0 {
		t.Error("expected hard block")
	}
	if r.HardBlocks[0].Code != "CAPABILITY_BLOCKED_ON_TARGET" {
		t.Errorf("code: got %s", r.HardBlocks[0].Code)
	}
}

func TestVerify_NoTargets(t *testing.T) {
	g := &graph.Graph{
		TargetsResolved: []string{},
		Entries:         map[string]graph.Entry{"background": {Path: "bg.ts"}},
	}

	r := Verify(Input{Graph: g})

	if r.Status != "failed" {
		t.Error("expected failed for no targets")
	}
}

func TestVerify_NoEntries(t *testing.T) {
	g := &graph.Graph{
		TargetsResolved: []string{"chrome"},
		Entries:         map[string]graph.Entry{},
	}

	r := Verify(Input{Graph: g})

	if r.Status != "failed" {
		t.Error("expected failed for no entries")
	}
}

func TestVerify_PermissionDiff(t *testing.T) {
	g := &graph.Graph{
		TargetsResolved: []string{"chrome"},
		Entries:         map[string]graph.Entry{"background": {Path: "bg.ts"}},
		GraphHash:       "sha256:abc",
	}
	m := &capability.TargetMatrix{
		Resolutions: []capability.Resolution{
			{Capability: "tabs", Target: "chrome", State: "native", Permissions: []string{"tabs"}},
			{Capability: "cookies", Target: "chrome", State: "native", Permissions: []string{"cookies"}},
		},
		Permissions: []string{"tabs", "cookies"},
		HostPerms:   []string{"https://*.example.com/*"},
	}

	r := Verify(Input{
		Graph:               g,
		Matrix:              m,
		PreviousPermissions: []string{"tabs"},
		PreviousHostPerms:   []string{},
	})

	if r.PermissionDiff == nil {
		t.Fatal("expected permission diff")
	}

	if len(r.PermissionDiff.AddedPermissions) != 1 || r.PermissionDiff.AddedPermissions[0] != "cookies" {
		t.Errorf("added permissions: got %v", r.PermissionDiff.AddedPermissions)
	}
	if len(r.PermissionDiff.AddedHostPermissions) != 1 {
		t.Errorf("added host permissions: got %v", r.PermissionDiff.AddedHostPermissions)
	}
	if len(r.PermissionDiff.RemovedPermissions) != 0 {
		t.Errorf("removed permissions should be empty: got %v", r.PermissionDiff.RemovedPermissions)
	}
}

func TestVerify_DegradedWarning(t *testing.T) {
	g := &graph.Graph{
		TargetsResolved: []string{"chrome"},
		Entries:         map[string]graph.Entry{"background": {Path: "bg.ts"}},
		GraphHash:       "sha256:abc",
	}
	m := &capability.TargetMatrix{
		Resolutions: []capability.Resolution{
			{Capability: "foo", Target: "chrome", State: "degraded", Reason: "partial support"},
		},
	}

	r := Verify(Input{Graph: g, Matrix: m})

	if r.Status != "passed" {
		t.Error("degraded should warn, not block")
	}
	if len(r.Warnings) == 0 {
		t.Error("expected warning for degraded capability")
	}
}

func TestVerify_NilGraph(t *testing.T) {
	r := Verify(Input{})
	if r.Status != "failed" {
		t.Error("nil graph should fail")
	}
}
