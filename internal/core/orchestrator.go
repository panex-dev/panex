package core

import (
	"context"
	"fmt"

	"github.com/panex-dev/panex/internal/capability"
	"github.com/panex-dev/panex/internal/fsmodel"
	"github.com/panex-dev/panex/internal/graph"
	"github.com/panex-dev/panex/internal/lock"
	"github.com/panex-dev/panex/internal/manifest"
	"github.com/panex-dev/panex/internal/plan"
	"github.com/panex-dev/panex/internal/target"
)

// Orchestrator provides a high-level API for Panex operations,
// ensuring consistent execution across CLI and MCP surfaces (L6).
type Orchestrator struct {
	ProjectDir string
	Registry   *target.Registry
}

// NewOrchestrator creates a new orchestrator for a project.
func NewOrchestrator(projectDir string, registry *target.Registry) *Orchestrator {
	return &Orchestrator{
		ProjectDir: projectDir,
		Registry:   registry,
	}
}

// ApplyInput is the input for the high-level Apply operation.
type ApplyInput struct {
	Graph *graph.Graph
	Plan  *plan.Plan
	Force bool
}

// Apply executes a plan with full orchestration: capability resolution,
// manifest compilation, and locked plan application (L6).
func (o *Orchestrator) Apply(ctx context.Context, input ApplyInput) (*plan.ApplyResult, error) {
	if input.Graph == nil {
		return nil, fmt.Errorf("nil graph")
	}
	if input.Plan == nil {
		return nil, fmt.Errorf("nil plan")
	}

	adapters := o.Registry.All()

	// 1. Resolve capabilities
	matrix, err := capability.Compile(capability.CompilerInput{
		Capabilities: input.Graph.Capabilities,
		Targets:      input.Graph.TargetsResolved,
		Adapters:     adapters,
	})
	if err != nil {
		return nil, fmt.Errorf("capability resolution: %w", err)
	}

	// 2. Compile manifests
	manifestResult := manifest.Compile(manifest.CompileInput{
		Graph:    input.Graph,
		Matrix:   matrix,
		Adapters: adapters,
	})

	// 3. Setup lock manager
	root, err := fsmodel.NewRoot(o.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("fs root: %w", err)
	}
	mgr := lock.NewManager(root.StateRoot())

	// 4. Execute plan
	result := plan.Apply(ctx, mgr, plan.ApplyInput{
		ProjectDir:     o.ProjectDir,
		Plan:           input.Plan,
		Graph:          input.Graph,
		ManifestResult: manifestResult,
		Force:          input.Force,
	})

	return result, nil
}
