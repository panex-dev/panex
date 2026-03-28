package panexerr

import (
	"errors"
	"strings"
	"testing"
)

func TestError_Format(t *testing.T) {
	e := New("CONFIG_INVALID", CatConfig, "config_loader", "missing project.name field")
	got := e.Error()
	if !strings.Contains(got, "CONFIG_INVALID") {
		t.Errorf("expected error to contain code: %s", got)
	}
	if !strings.Contains(got, "config") {
		t.Errorf("expected error to contain category: %s", got)
	}
	if !strings.Contains(got, "missing project.name") {
		t.Errorf("expected error to contain message: %s", got)
	}
}

func TestError_WithCause(t *testing.T) {
	cause := errors.New("file not found")
	e := Wrap("CONFIG_MISSING", CatConfig, "config_loader", "panex.config.ts not found", cause)

	got := e.Error()
	if !strings.Contains(got, "file not found") {
		t.Errorf("expected error to contain cause: %s", got)
	}

	if !errors.Is(e, cause) {
		t.Error("expected Unwrap to return cause")
	}
}

func TestConvenienceConstructors(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		code     string
		category Category
	}{
		{"ConfigInvalid", ConfigInvalid("loader", "bad"), "CONFIG_INVALID", CatConfig},
		{"ConfigMissing", ConfigMissing("loader", "missing"), "CONFIG_MISSING", CatConfig},
		{"EnvironmentMissing", EnvironmentMissing("env", "no chrome"), "ENVIRONMENT_MISSING", CatEnvironment},
		{"InspectionAmbiguity", InspectionAmbiguity("inspector", "two frameworks"), "INSPECTION_AMBIGUITY", CatInspection},
		{"CapabilityBlocked", CapabilityBlocked("compiler", "firefox", "sidePanel", "not supported"), "CAPABILITY_BLOCKED_ON_TARGET", CatCapability},
		{"ManifestInvalid", ManifestInvalid("manifest", "bad key"), "MANIFEST_INVALID", CatManifest},
		{"PolicyDenied", PolicyDenied("policy", "mutation.allow_file_deletion", "delete src/"), "POLICY_DENIED", CatPolicy},
		{"PlanDrift", PlanDrift("apply"), "PLAN_DRIFT_DETECTED", CatGraph},
		{"DependencyMissing", DependencyMissing("deps", "react"), "DEPENDENCY_MISSING", CatDependency},
		{"PermissionExpansion", PermissionExpansionOutsideCompiler("manifest", "tabs added outside compiler"), "PERMISSION_EXPANSION_OUTSIDE_CAPABILITY_COMPILER", CatManifest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("code: got %s, want %s", tt.err.Code, tt.code)
			}
			if tt.err.Category != tt.category {
				t.Errorf("category: got %s, want %s", tt.err.Category, tt.category)
			}
		})
	}
}

func TestRepairableErrors(t *testing.T) {
	e := ManifestInvalid("manifest", "bad key")
	if !e.Repairable {
		t.Error("ManifestInvalid should be repairable")
	}
	if len(e.Recipes) == 0 {
		t.Error("ManifestInvalid should have suggested recipes")
	}
}

func TestPolicyDenied_Flag(t *testing.T) {
	e := PolicyDenied("policy", "mutation.allow_file_deletion", "delete")
	if !e.PolicyFlag {
		t.Error("PolicyDenied should have policy_related=true")
	}
}
