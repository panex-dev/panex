package releasedesc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/panex-dev/panex/internal/capability"
	"github.com/panex-dev/panex/internal/graph"
	"github.com/panex-dev/panex/internal/manifest"
	"github.com/panex-dev/panex/internal/target"
	"github.com/panex-dev/panex/internal/verify"
)

func TestBuildDescriptor(t *testing.T) {
	g := &graph.Graph{
		Project:         graph.ProjectIdentity{ID: "acme.ext", Name: "ext", Version: "1.2.3"},
		TargetsResolved: []string{"chrome"},
		GraphHash:       "graph_hash",
	}
	matrix := &capability.TargetMatrix{
		Resolutions: []capability.Resolution{
			{Capability: "storage", Target: "chrome", State: "native"},
		},
	}
	manifests := &manifest.CompileResult{Outputs: []manifest.CompileOutput{{
		Target:          "chrome",
		ManifestHash:    "manifest_hash",
		Permissions:     []string{"storage"},
		HostPermissions: []string{"https://example.com/*"},
	}}}
	verification := &verify.Result{Status: "passed", HardBlocks: []verify.Block{}, Warnings: []string{}}

	descriptor, err := Build(BuildInput{
		Graph:              g,
		RunID:              "run_001",
		ProducedAt:         "2026-05-20T00:00:00Z",
		Matrix:             matrix,
		ManifestResult:     manifests,
		Artifacts:          []target.ArtifactRecord{{Target: "chrome", ArtifactType: "chrome_zip", FilePath: ".panex/artifacts/chrome/ext.zip", SHA256: "abc", FileSize: 123}},
		VerificationResult: verification,
	})
	if err != nil {
		t.Fatal(err)
	}

	if descriptor.SchemaVersion != 1 {
		t.Fatalf("schema version: got %d", descriptor.SchemaVersion)
	}
	if descriptor.ProjectID != "acme.ext" || descriptor.Version != "1.2.3" {
		t.Fatalf("project identity: got %+v", descriptor)
	}
	chrome := descriptor.Targets["chrome"]
	if chrome.ManifestFingerprint != "manifest_hash" {
		t.Fatalf("manifest fingerprint: got %q", chrome.ManifestFingerprint)
	}
	if chrome.CapabilityStates["storage"] != "native" {
		t.Fatalf("capability state: got %+v", chrome.CapabilityStates)
	}
	if chrome.Artifact.Type != "chrome_zip" || chrome.Artifact.SHA256 != "abc" || chrome.Artifact.SizeBytes != 123 {
		t.Fatalf("artifact: got %+v", chrome.Artifact)
	}
}

func TestBuildDescriptorRequiresArtifact(t *testing.T) {
	_, err := Build(BuildInput{
		Graph: &graph.Graph{
			Project:         graph.ProjectIdentity{Name: "ext", Version: "1.0.0"},
			TargetsResolved: []string{"chrome"},
		},
		ManifestResult:     &manifest.CompileResult{Outputs: []manifest.CompileOutput{{Target: "chrome"}}},
		VerificationResult: &verify.Result{Status: "passed"},
	})
	if err == nil {
		t.Fatal("expected missing artifact error")
	}
}

func TestWriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "release.json")
	descriptor := Descriptor{
		SchemaVersion: 1,
		ProjectID:     "acme.ext",
		ProjectName:   "ext",
		Version:       "1.0.0",
		RunID:         "run_001",
		Targets:       map[string]TargetDescriptor{},
		Verification:  VerificationSummary{Status: "passed", HardBlocks: []verify.Block{}, Warnings: []string{}},
		PermissionDiff: PermissionDiff{
			Added:        []string{},
			Removed:      []string{},
			HostExpanded: []string{},
			HostNarrowed: []string{},
		},
	}

	if err := WriteFile(path, descriptor); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Descriptor
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ProjectID != "acme.ext" {
		t.Fatalf("project id: got %q", decoded.ProjectID)
	}
}
