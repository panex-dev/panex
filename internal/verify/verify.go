// Package verify implements the verification engine. It determines
// whether the project is structurally and release-wise sound.
// Spec section 31.
package verify

import (
	"github.com/panex-dev/panex/internal/capability"
	"github.com/panex-dev/panex/internal/graph"
)

// Result is the verification outcome.
type Result struct {
	Status           string          `json:"status"` // "passed" or "failed"
	HardBlocks       []Block         `json:"hard_blocks"`
	Warnings         []string        `json:"warnings"`
	Info             []string        `json:"info"`
	PermissionDiff   *PermissionDiff `json:"permission_diff,omitempty"`
	SuggestedRepairs []string        `json:"suggested_repairs,omitempty"`
}

// Block is a hard verification failure.
type Block struct {
	Code    string `json:"code"`
	Target  string `json:"target,omitempty"`
	Message string `json:"message"`
}

// PermissionDiff shows what changed since the last release.
type PermissionDiff struct {
	AddedPermissions     []string `json:"added_permissions"`
	RemovedPermissions   []string `json:"removed_permissions"`
	AddedHostPermissions []string `json:"added_host_permissions"`
	RemovedHostPerms     []string `json:"removed_host_permissions"`
}

// Input is everything the verifier needs.
type Input struct {
	Graph               *graph.Graph
	Matrix              *capability.TargetMatrix
	PreviousPermissions []string
	PreviousHostPerms   []string
}

// Verify runs all verification checks and returns a Result.
func Verify(input Input) *Result {
	r := &Result{
		Status:     "passed",
		HardBlocks: []Block{},
		Warnings:   []string{},
		Info:       []string{},
	}

	checkGraphCompleteness(input.Graph, r)
	checkCapabilityBlocks(input.Matrix, r)
	checkEntryCompleteness(input.Graph, r)
	computePermissionDiff(input, r)

	if len(r.HardBlocks) > 0 {
		r.Status = "failed"
	}

	return r
}

func checkGraphCompleteness(g *graph.Graph, r *Result) {
	if g == nil {
		r.HardBlocks = append(r.HardBlocks, Block{
			Code:    "GRAPH_MISSING",
			Message: "project graph is nil",
		})
		return
	}
	if g.Project.Name == "" && g.Project.ID == "" {
		r.Warnings = append(r.Warnings, "project identity not set (name and id are empty)")
	}
	if len(g.TargetsResolved) == 0 {
		r.HardBlocks = append(r.HardBlocks, Block{
			Code:    "NO_TARGETS",
			Message: "no targets resolved",
		})
	}
	if g.GraphHash == "" {
		r.Warnings = append(r.Warnings, "graph hash not computed — drift detection disabled")
	}
}

func checkCapabilityBlocks(m *capability.TargetMatrix, r *Result) {
	if m == nil {
		return
	}
	for _, res := range m.Resolutions {
		if res.State == "blocked" {
			r.HardBlocks = append(r.HardBlocks, Block{
				Code:    "CAPABILITY_BLOCKED_ON_TARGET",
				Target:  res.Target,
				Message: res.Capability + " is blocked on " + res.Target + ": " + res.Reason,
			})
		}
		if res.State == "degraded" {
			r.Warnings = append(r.Warnings,
				res.Capability+" is degraded on "+res.Target+": "+res.Reason)
		}
	}
}

func checkEntryCompleteness(g *graph.Graph, r *Result) {
	if g == nil {
		return
	}
	if _, ok := g.Entries["background"]; !ok {
		r.Warnings = append(r.Warnings, "no background entrypoint declared")
	}
	if len(g.Entries) == 0 {
		r.HardBlocks = append(r.HardBlocks, Block{
			Code:    "NO_ENTRIES",
			Message: "no extension entrypoints declared",
		})
	}
}

func computePermissionDiff(input Input, r *Result) {
	if input.Matrix == nil {
		return
	}

	currentPerms := make(map[string]bool)
	for _, p := range input.Matrix.Permissions {
		currentPerms[p] = true
	}
	prevPerms := make(map[string]bool)
	for _, p := range input.PreviousPermissions {
		prevPerms[p] = true
	}

	diff := &PermissionDiff{}

	for p := range currentPerms {
		if !prevPerms[p] {
			diff.AddedPermissions = append(diff.AddedPermissions, p)
		}
	}
	for p := range prevPerms {
		if !currentPerms[p] {
			diff.RemovedPermissions = append(diff.RemovedPermissions, p)
		}
	}

	// Host permissions
	currentHost := make(map[string]bool)
	for _, p := range input.Matrix.HostPerms {
		currentHost[p] = true
	}
	prevHost := make(map[string]bool)
	for _, p := range input.PreviousHostPerms {
		prevHost[p] = true
	}

	for p := range currentHost {
		if !prevHost[p] {
			diff.AddedHostPermissions = append(diff.AddedHostPermissions, p)
		}
	}
	for p := range prevHost {
		if !currentHost[p] {
			diff.RemovedHostPerms = append(diff.RemovedHostPerms, p)
		}
	}

	r.PermissionDiff = diff

	if len(diff.AddedPermissions) > 0 {
		r.Info = append(r.Info, "new permissions added — review recommended")
	}
	if len(diff.AddedHostPermissions) > 0 {
		r.Warnings = append(r.Warnings, "new host permissions added — policy review required")
	}
}
