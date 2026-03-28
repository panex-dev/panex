package capability

import (
	"testing"

	"github.com/panex-dev/panex/internal/target"
)

func TestCompile_SingleTarget(t *testing.T) {
	chrome := target.NewChrome()
	input := CompilerInput{
		Capabilities: map[string]any{
			"tabs":    "read-write",
			"storage": map[string]string{"mode": "sync"},
		},
		Targets:  []string{"chrome"},
		Adapters: map[string]target.Adapter{"chrome": chrome},
	}

	matrix, err := Compile(input)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if len(matrix.Resolutions) != 2 {
		t.Errorf("expected 2 resolutions, got %d", len(matrix.Resolutions))
	}

	// Both should be native on Chrome
	for _, r := range matrix.Resolutions {
		if r.State != "native" {
			t.Errorf("capability %s should be native on chrome, got %s", r.Capability, r.State)
		}
		if r.Target != "chrome" {
			t.Errorf("target: got %s, want chrome", r.Target)
		}
	}

	// Should have collected permissions
	if len(matrix.Permissions) == 0 {
		t.Error("expected permissions to be collected")
	}

	// Should have no errors or warnings
	if len(matrix.Errors) > 0 {
		t.Errorf("unexpected errors: %v", matrix.Errors)
	}
	if len(matrix.Warnings) > 0 {
		t.Errorf("unexpected warnings: %v", matrix.Warnings)
	}
}

func TestCompile_BlockedCapability(t *testing.T) {
	chrome := target.NewChrome()
	input := CompilerInput{
		Capabilities: map[string]any{
			"sidebarSurface": "preferred", // blocked on Chrome
		},
		Targets:  []string{"chrome"},
		Adapters: map[string]target.Adapter{"chrome": chrome},
	}

	matrix, err := Compile(input)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if !matrix.HasBlockedCapabilities() {
		t.Error("expected blocked capabilities")
	}

	if len(matrix.Errors) == 0 {
		t.Error("expected error for blocked capability")
	}
}

func TestCompile_MixedCapabilities(t *testing.T) {
	chrome := target.NewChrome()
	input := CompilerInput{
		Capabilities: map[string]any{
			"tabs":           "read",
			"sideSurface":    "preferred",
			"sidebarSurface": "preferred",
		},
		Targets:  []string{"chrome"},
		Adapters: map[string]target.Adapter{"chrome": chrome},
	}

	matrix, err := Compile(input)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	// Should have 3 resolutions
	if len(matrix.Resolutions) != 3 {
		t.Errorf("expected 3 resolutions, got %d", len(matrix.Resolutions))
	}

	// tabs and sideSurface should be native
	// sidebarSurface should be blocked
	stateMap := make(map[string]string)
	for _, r := range matrix.Resolutions {
		stateMap[r.Capability] = r.State
	}

	if stateMap["tabs"] != "native" {
		t.Errorf("tabs: got %s, want native", stateMap["tabs"])
	}
	if stateMap["sideSurface"] != "native" {
		t.Errorf("sideSurface: got %s, want native", stateMap["sideSurface"])
	}
	if stateMap["sidebarSurface"] != "blocked" {
		t.Errorf("sidebarSurface: got %s, want blocked", stateMap["sidebarSurface"])
	}
}

func TestCompile_HostPermissions(t *testing.T) {
	chrome := target.NewChrome()
	input := CompilerInput{
		Capabilities:    map[string]any{"tabs": "read"},
		Targets:         []string{"chrome"},
		Adapters:        map[string]target.Adapter{"chrome": chrome},
		HostPermissions: []string{"https://*.example.com/*"},
	}

	matrix, err := Compile(input)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if len(matrix.HostPerms) != 1 || matrix.HostPerms[0] != "https://*.example.com/*" {
		t.Errorf("host_permissions: got %v", matrix.HostPerms)
	}
}

func TestCompile_MissingAdapter(t *testing.T) {
	input := CompilerInput{
		Capabilities: map[string]any{"tabs": "read"},
		Targets:      []string{"firefox"},
		Adapters:     map[string]target.Adapter{},
	}

	matrix, err := Compile(input)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if len(matrix.Errors) == 0 {
		t.Error("expected error for missing adapter")
	}
}

func TestPermissionsForTarget(t *testing.T) {
	chrome := target.NewChrome()
	input := CompilerInput{
		Capabilities: map[string]any{
			"tabs":    "read",
			"storage": "sync",
			"cookies": true,
		},
		Targets:  []string{"chrome"},
		Adapters: map[string]target.Adapter{"chrome": chrome},
	}

	matrix, _ := Compile(input)
	perms := matrix.PermissionsForTarget("chrome")

	permSet := make(map[string]bool)
	for _, p := range perms {
		permSet[p] = true
	}

	if !permSet["tabs"] {
		t.Error("expected tabs permission")
	}
	if !permSet["storage"] {
		t.Error("expected storage permission")
	}
	if !permSet["cookies"] {
		t.Error("expected cookies permission")
	}
}

func TestResolutionsForTarget(t *testing.T) {
	chrome := target.NewChrome()
	input := CompilerInput{
		Capabilities: map[string]any{"tabs": "read", "storage": "sync"},
		Targets:      []string{"chrome"},
		Adapters:     map[string]target.Adapter{"chrome": chrome},
	}

	matrix, _ := Compile(input)
	res := matrix.ResolutionsForTarget("chrome")

	if len(res) != 2 {
		t.Errorf("expected 2 resolutions for chrome, got %d", len(res))
	}

	// Non-existent target should return empty
	res2 := matrix.ResolutionsForTarget("firefox")
	if len(res2) != 0 {
		t.Errorf("expected 0 resolutions for firefox, got %d", len(res2))
	}
}
