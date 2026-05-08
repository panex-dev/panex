// Package capability implements the capability system and compiler.
// It resolves semantic capability declarations into concrete permissions,
// manifest fragments, and resolution states per target. Spec sections 15-16.
package capability

import (
	"sort"

	"github.com/panex-dev/panex/internal/target"
)

// Resolution is the resolved state of a single capability for a single target.
type Resolution struct {
	Capability  string   `json:"capability"`
	Target      string   `json:"target"`
	State       string   `json:"state"` // native, adapted, degraded, blocked, optional-fallback
	Reason      string   `json:"reason,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// TargetMatrix is the full resolution matrix across all capabilities and targets.
type TargetMatrix struct {
	Resolutions       []Resolution        `json:"resolutions"`
	Permissions       []string            `json:"permissions"`
	HostPerms         []string            `json:"host_permissions"`
	HostPermsByTarget map[string][]string `json:"host_permissions_by_target,omitempty"`
	Warnings          []string            `json:"warnings"`
	Errors            []string            `json:"errors"`
}

// CompilerInput is everything the compiler needs.
type CompilerInput struct {
	Capabilities            map[string]any
	Targets                 []string
	Adapters                map[string]target.Adapter
	HostPermissions         []string
	HostPermissionsByTarget map[string][]string
}

// Compile runs the full capability compilation pipeline:
// normalize → expand → intersect with catalogs → resolve → generate.
func Compile(input CompilerInput) (*TargetMatrix, error) {
	matrix := &TargetMatrix{
		HostPermsByTarget: make(map[string][]string),
	}
	permSet := make(map[string]bool)
	hostPermSet := make(map[string]bool)

	for _, tgt := range input.Targets {
		adapter, ok := input.Adapters[tgt]
		if !ok {
			matrix.Errors = append(matrix.Errors,
				"no adapter registered for target: "+tgt)
			continue
		}

		resolved, result := adapter.ResolveCapabilities(input.Capabilities)
		if result.Outcome != target.Success {
			matrix.Errors = append(matrix.Errors,
				"capability resolution failed for "+tgt+": "+result.Reason)
			continue
		}

		hostPerms := hostPermissionsForTargetInput(input, tgt)
		if len(hostPerms) > 0 {
			matrix.HostPermsByTarget[tgt] = hostPerms
			for _, p := range hostPerms {
				hostPermSet[p] = true
			}
		}

		for capName, res := range resolved {
			matrix.Resolutions = append(matrix.Resolutions, Resolution{
				Capability:  capName,
				Target:      tgt,
				State:       res.State,
				Reason:      res.Reason,
				Permissions: res.Permissions,
			})

			// Collect permissions from native/adapted capabilities
			if res.State == "native" || res.State == "adapted" {
				for _, p := range res.Permissions {
					permSet[p] = true
				}
			}

			// Warn on degraded
			if res.State == "degraded" {
				matrix.Warnings = append(matrix.Warnings,
					capName+" is degraded on "+tgt+": "+res.Reason)
			}

			// Error on blocked (unless optional-fallback)
			if res.State == "blocked" {
				matrix.Errors = append(matrix.Errors,
					capName+" is blocked on "+tgt+": "+res.Reason)
			}
		}
	}

	// Deduplicate and sort permissions
	for p := range permSet {
		matrix.Permissions = append(matrix.Permissions, p)
	}
	sort.Strings(matrix.Permissions)
	for p := range hostPermSet {
		matrix.HostPerms = append(matrix.HostPerms, p)
	}
	sort.Strings(matrix.HostPerms)
	if len(matrix.HostPermsByTarget) == 0 {
		matrix.HostPermsByTarget = nil
	}

	return matrix, nil
}

// HasBlockedCapabilities returns true if any resolution is blocked.
func (m *TargetMatrix) HasBlockedCapabilities() bool {
	for _, r := range m.Resolutions {
		if r.State == "blocked" {
			return true
		}
	}
	return false
}

// PermissionsForTarget returns the permission set for a specific target.
func (m *TargetMatrix) PermissionsForTarget(tgt string) []string {
	permSet := make(map[string]bool)
	for _, r := range m.Resolutions {
		if r.Target != tgt {
			continue
		}
		if r.State == "native" || r.State == "adapted" {
			for _, p := range r.Permissions {
				permSet[p] = true
			}
		}
	}
	out := make([]string, 0, len(permSet))
	for p := range permSet {
		out = append(out, p)
	}
	return out
}

// ResolutionsForTarget returns all resolutions for a specific target.
func (m *TargetMatrix) ResolutionsForTarget(tgt string) []Resolution {
	var out []Resolution
	for _, r := range m.Resolutions {
		if r.Target == tgt {
			out = append(out, r)
		}
	}
	return out
}

// HostPermissionsForTarget returns the host permission set for a specific target.
func (m *TargetMatrix) HostPermissionsForTarget(tgt string) []string {
	if m == nil {
		return nil
	}
	if len(m.HostPermsByTarget) > 0 {
		return append([]string(nil), m.HostPermsByTarget[tgt]...)
	}
	return append([]string(nil), m.HostPerms...)
}

func hostPermissionsForTargetInput(input CompilerInput, tgt string) []string {
	hostPerms := input.HostPermissions
	if perTarget, ok := input.HostPermissionsByTarget[tgt]; ok {
		hostPerms = perTarget
	}
	return dedupeSortedStrings(hostPerms)
}

func dedupeSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
