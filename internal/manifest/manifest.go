// Package manifest compiles target-specific manifest files from resolved
// capabilities. Permission authority flows exclusively from the capability
// compiler — no independent permission reasoning. Spec section 18.
package manifest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/panex-dev/panex/internal/capability"
	"github.com/panex-dev/panex/internal/graph"
	"github.com/panex-dev/panex/internal/target"
)

// CompileInput is everything the manifest compiler needs.
type CompileInput struct {
	Graph    *graph.Graph
	Matrix   *capability.TargetMatrix
	Adapters map[string]target.Adapter
	Version  string // explicit version override, or empty
}

// CompileOutput is the result of manifest compilation for one target.
type CompileOutput struct {
	Target          string         `json:"target"`
	Manifest        map[string]any `json:"manifest"`
	Permissions     []string       `json:"permissions"`
	HostPermissions []string       `json:"host_permissions,omitempty"`
	ManifestHash    string         `json:"manifest_hash"`
	Warnings        []string       `json:"warnings,omitempty"`
}

// CompileResult is the full manifest compilation result.
type CompileResult struct {
	Outputs []CompileOutput `json:"outputs"`
	Errors  []string        `json:"errors,omitempty"`
}

// Compile produces manifest files for all resolved targets.
func Compile(input CompileInput) *CompileResult {
	result := &CompileResult{}

	if input.Graph == nil {
		result.Errors = append(result.Errors, "nil graph")
		return result
	}

	version := resolveVersion(input)

	for _, tgt := range input.Graph.TargetsResolved {
		adapter, ok := input.Adapters[tgt]
		if !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("no adapter for target %q", tgt))
			continue
		}

		// Get permissions from capability compiler output only
		perms := permissionsFromMatrix(input.Matrix, tgt)
		hostPerms := hostPermsFromMatrix(input.Matrix, tgt)

		// Build entries map for adapter
		entries := make(map[string]target.EntrySpec, len(input.Graph.Entries))
		for name, e := range input.Graph.Entries {
			entries[name] = target.EntrySpec{Path: e.Path, Type: e.Type}
		}

		mOutput, adapterResult := adapter.CompileManifest(target.ManifestCompileOptions{
			ProjectName:     input.Graph.Project.Name,
			ProjectVersion:  version,
			Entries:         entries,
			Permissions:     perms,
			HostPermissions: hostPerms,
		})

		if adapterResult.Outcome != target.Success {
			result.Errors = append(result.Errors, fmt.Sprintf("%s manifest: %s", tgt, adapterResult.Reason))
			continue
		}

		// Verify no permission expansion outside capability compiler
		if err := validatePermissionAuthority(mOutput.Permissions, perms); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", tgt, err))
			continue
		}

		hash := computeManifestHash(mOutput.Manifest)

		output := CompileOutput{
			Target:          tgt,
			Manifest:        mOutput.Manifest,
			Permissions:     mOutput.Permissions,
			HostPermissions: mOutput.HostPermissions,
			ManifestHash:    hash,
		}

		result.Outputs = append(result.Outputs, output)
	}

	return result
}

// WriteManifest writes a manifest.json to the given path with deterministic key ordering.
func WriteManifest(manifest map[string]any, path string) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return os.Rename(tmp, path)
}

// --- internal ---

func resolveVersion(input CompileInput) string {
	if input.Version != "" {
		return input.Version
	}
	if input.Graph != nil && input.Graph.Project.Version != "" {
		return input.Graph.Project.Version
	}
	return "0.0.1"
}

func permissionsFromMatrix(matrix *capability.TargetMatrix, tgt string) []string {
	if matrix == nil {
		return nil
	}
	seen := map[string]bool{}
	var perms []string
	for _, r := range matrix.Resolutions {
		if r.Target != tgt {
			continue
		}
		if r.State != "native" && r.State != "adapted" {
			continue
		}
		for _, p := range r.Permissions {
			if !seen[p] {
				seen[p] = true
				perms = append(perms, p)
			}
		}
	}
	sort.Strings(perms)
	return perms
}

func hostPermsFromMatrix(matrix *capability.TargetMatrix, tgt string) []string {
	if matrix == nil {
		return nil
	}
	return matrix.HostPermissionsForTarget(tgt)
}

func validatePermissionAuthority(manifestPerms, capabilityPerms []string) error {
	allowed := map[string]bool{}
	for _, p := range capabilityPerms {
		allowed[p] = true
	}
	for _, p := range manifestPerms {
		if !allowed[p] {
			return fmt.Errorf("permission_expansion_outside_capability_compiler: %q not in resolved capability set", p)
		}
	}
	return nil
}

func computeManifestHash(manifest map[string]any) string {
	// Deterministic serialization for hash stability
	data, _ := json.Marshal(manifest)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", hash)
}
