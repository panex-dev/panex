// Package releasedesc builds the stable release descriptor emitted by package
// runs. The descriptor is the public bridge from Core release artifacts to
// future publish and Insights consumers.
package releasedesc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/panex-dev/panex/internal/capability"
	"github.com/panex-dev/panex/internal/graph"
	"github.com/panex-dev/panex/internal/manifest"
	"github.com/panex-dev/panex/internal/target"
	"github.com/panex-dev/panex/internal/verify"
)

const SchemaVersion = 1

// Descriptor is the public release descriptor schema.
type Descriptor struct {
	SchemaVersion    int                         `json:"schema_version"`
	ProjectID        string                      `json:"project_id"`
	ProjectName      string                      `json:"project_name"`
	Version          string                      `json:"version"`
	ProducedAt       string                      `json:"produced_at"`
	RunID            string                      `json:"run_id"`
	Targets          map[string]TargetDescriptor `json:"targets"`
	Verification     VerificationSummary         `json:"verification_summary"`
	PermissionDiff   PermissionDiff              `json:"permission_diff"`
	BuildFingerprint string                      `json:"build_fingerprint"`
	PublishMetadata  any                         `json:"publish_metadata"`
}

// TargetDescriptor is the target-specific descriptor payload.
type TargetDescriptor struct {
	ManifestFingerprint string            `json:"manifest_fingerprint"`
	Permissions         []string          `json:"permissions"`
	HostPermissions     []string          `json:"host_permissions"`
	CapabilityStates    map[string]string `json:"capability_resolutions"`
	Artifact            Artifact          `json:"artifact"`
}

// Artifact is the public artifact record shape embedded in the descriptor.
type Artifact struct {
	Type      string `json:"type"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
}

// VerificationSummary is the self-contained verification result subset.
type VerificationSummary struct {
	Status     string         `json:"status"`
	HardBlocks []verify.Block `json:"hard_blocks"`
	Warnings   []string       `json:"warnings"`
}

// PermissionDiff is the stable public permission-diff shape.
type PermissionDiff struct {
	Added        []string `json:"added"`
	Removed      []string `json:"removed"`
	HostExpanded []string `json:"host_expanded"`
	HostNarrowed []string `json:"host_narrowed"`
}

// BuildInput is all state needed to build a release descriptor.
type BuildInput struct {
	Graph              *graph.Graph
	Version            string
	RunID              string
	ProducedAt         string
	BuildFingerprint   string
	Matrix             *capability.TargetMatrix
	ManifestResult     *manifest.CompileResult
	Artifacts          []target.ArtifactRecord
	VerificationResult *verify.Result
}

// Build constructs a self-contained descriptor from package run evidence.
func Build(input BuildInput) (Descriptor, error) {
	if input.Graph == nil {
		return Descriptor{}, fmt.Errorf("graph is required")
	}
	if input.ManifestResult == nil {
		return Descriptor{}, fmt.Errorf("manifest result is required")
	}
	if input.VerificationResult == nil {
		return Descriptor{}, fmt.Errorf("verification result is required")
	}

	version := input.Version
	if version == "" {
		version = input.Graph.Project.Version
	}
	producedAt := input.ProducedAt
	if producedAt == "" {
		producedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	buildFingerprint := input.BuildFingerprint
	if buildFingerprint == "" {
		buildFingerprint = input.Graph.GraphHash
	}

	manifests := manifestsByTarget(input.ManifestResult.Outputs)
	artifacts := artifactsByTarget(input.Artifacts)
	targets := make(map[string]TargetDescriptor, len(input.Graph.TargetsResolved))
	for _, targetName := range input.Graph.TargetsResolved {
		manifestOutput, ok := manifests[targetName]
		if !ok {
			return Descriptor{}, fmt.Errorf("manifest output missing for target %q", targetName)
		}
		artifact, ok := artifacts[targetName]
		if !ok {
			return Descriptor{}, fmt.Errorf("artifact missing for target %q", targetName)
		}

		targets[targetName] = TargetDescriptor{
			ManifestFingerprint: manifestOutput.ManifestHash,
			Permissions:         cloneStrings(manifestOutput.Permissions),
			HostPermissions:     cloneStrings(manifestOutput.HostPermissions),
			CapabilityStates:    capabilityStates(input.Matrix, targetName),
			Artifact: Artifact{
				Type:      artifact.ArtifactType,
				Path:      artifact.FilePath,
				SHA256:    artifact.SHA256,
				SizeBytes: artifact.FileSize,
			},
		}
	}

	return Descriptor{
		SchemaVersion:    SchemaVersion,
		ProjectID:        input.Graph.Project.ID,
		ProjectName:      input.Graph.Project.Name,
		Version:          version,
		ProducedAt:       producedAt,
		RunID:            input.RunID,
		Targets:          targets,
		Verification:     verificationSummary(input.VerificationResult),
		PermissionDiff:   permissionDiff(input.VerificationResult.PermissionDiff),
		BuildFingerprint: buildFingerprint,
		PublishMetadata:  nil,
	}, nil
}

// WriteFile writes a release descriptor as stable, pretty JSON.
func WriteFile(path string, descriptor Descriptor) error {
	data, err := json.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal release descriptor: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create release descriptor dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write release descriptor: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace release descriptor: %w", err)
	}
	return nil
}

func manifestsByTarget(outputs []manifest.CompileOutput) map[string]manifest.CompileOutput {
	out := make(map[string]manifest.CompileOutput, len(outputs))
	for _, output := range outputs {
		out[output.Target] = output
	}
	return out
}

func artifactsByTarget(records []target.ArtifactRecord) map[string]target.ArtifactRecord {
	out := make(map[string]target.ArtifactRecord, len(records))
	for _, record := range records {
		out[record.Target] = record
	}
	return out
}

func capabilityStates(matrix *capability.TargetMatrix, targetName string) map[string]string {
	out := map[string]string{}
	if matrix == nil {
		return out
	}
	for _, resolution := range matrix.ResolutionsForTarget(targetName) {
		out[resolution.Capability] = resolution.State
	}
	return out
}

func verificationSummary(result *verify.Result) VerificationSummary {
	return VerificationSummary{
		Status:     result.Status,
		HardBlocks: append([]verify.Block(nil), result.HardBlocks...),
		Warnings:   cloneStrings(result.Warnings),
	}
}

func permissionDiff(diff *verify.PermissionDiff) PermissionDiff {
	if diff == nil {
		return PermissionDiff{
			Added:        []string{},
			Removed:      []string{},
			HostExpanded: []string{},
			HostNarrowed: []string{},
		}
	}
	return PermissionDiff{
		Added:        cloneStrings(diff.AddedPermissions),
		Removed:      cloneStrings(diff.RemovedPermissions),
		HostExpanded: cloneStrings(diff.AddedHostPermissions),
		HostNarrowed: cloneStrings(diff.RemovedHostPerms),
	}
}

func cloneStrings(values []string) []string {
	out := append([]string(nil), values...)
	if out == nil {
		return []string{}
	}
	sort.Strings(out)
	return out
}
