// Package cli implements the new Panex CLI surface. Non-interactive
// by default, JSON-first, stable exit codes. Spec section 34.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/panex-dev/panex/internal/capability"
	"github.com/panex-dev/panex/internal/doctor"
	"github.com/panex-dev/panex/internal/fsmodel"
	"github.com/panex-dev/panex/internal/graph"
	"github.com/panex-dev/panex/internal/inspector"
	"github.com/panex-dev/panex/internal/ledger"
	"github.com/panex-dev/panex/internal/policy"
	"github.com/panex-dev/panex/internal/target"
	"github.com/panex-dev/panex/internal/verify"
)

// Exit codes (spec section 34.4)
const (
	ExitSuccess           = 0
	ExitOperationalFail   = 1
	ExitConfigError       = 2
	ExitPolicyDenied      = 3
	ExitEnvUnsupported    = 4
	ExitPlanDrift         = 5
	ExitVerifyFailed      = 6
	ExitRuntimeFail       = 7
	ExitPublishFail       = 8
	ExitInternalFault     = 9
)

// Output is the uniform CLI output envelope.
type Output struct {
	Status   string   `json:"status"`
	Command  string   `json:"command"`
	RunID    string   `json:"run_id,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	Data     any      `json:"data,omitempty"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Next     []string `json:"next_actions,omitempty"`
}

// Emit writes the output as JSON to stdout and returns the exit code.
func Emit(out Output) int {
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(data))

	if len(out.Errors) > 0 {
		return ExitOperationalFail
	}
	return ExitSuccess
}

// --- Commands ---

// CmdInspect runs project inspection.
func CmdInspect(projectDir string) int {
	ins := inspector.New(projectDir)
	report, err := ins.Inspect()
	if err != nil {
		return Emit(Output{
			Status:  "error",
			Command: "inspect",
			Errors:  []string{err.Error()},
		})
	}

	return Emit(Output{
		Status:  "ok",
		Command: "inspect",
		Summary: summarizeInspection(report),
		Data:    report,
	})
}

// CmdInit initializes a Panex project.
func CmdInit(projectDir string, opts InitOptions) int {
	root, err := fsmodel.NewRoot(projectDir)
	if err != nil {
		return Emit(Output{
			Status:  "error",
			Command: "init",
			Errors:  []string{err.Error()},
		})
	}

	if root.IsInitialized() {
		return Emit(Output{
			Status:  "already_initialized",
			Command: "init",
			Summary: "project already initialized",
			Next:    []string{"panex inspect", "panex plan"},
		})
	}

	// Run inspector
	ins := inspector.New(projectDir)
	report, err := ins.Inspect()
	if err != nil {
		return Emit(Output{
			Status:  "error",
			Command: "init",
			Errors:  []string{fmt.Sprintf("inspection failed: %v", err)},
		})
	}

	// Build graph from inspection
	builder := graph.NewBuilder(projectDir)
	g, err := builder.BuildFromInspection(report)
	if err != nil {
		return Emit(Output{
			Status:  "error",
			Command: "init",
			Errors:  []string{fmt.Sprintf("graph build failed: %v", err)},
		})
	}

	// Apply opts overrides
	if opts.Name != "" {
		g.Project.Name = opts.Name
		g.Project.ID = opts.Name
	}
	if len(opts.Targets) > 0 {
		g.TargetsRequested = opts.Targets
		g.TargetsResolved = opts.Targets
	}

	// Initialize .panex/
	if err := root.Init(); err != nil {
		return Emit(Output{
			Status:  "error",
			Command: "init",
			Errors:  []string{fmt.Sprintf("init failed: %v", err)},
		})
	}

	// Write project graph
	if err := graph.WriteToFile(g, root.ProjectGraphPath()); err != nil {
		return Emit(Output{
			Status:  "error",
			Command: "init",
			Errors:  []string{fmt.Sprintf("write graph failed: %v", err)},
		})
	}

	// Write default policy
	pol := policy.Default()
	if len(opts.Targets) > 0 {
		pol.Targets.Allowed = opts.Targets
	}
	writePolicyFile(root.PolicyFilePath(), pol)

	// Write environment.json
	envInfo := detectEnvironment(projectDir)
	writeJSONFile(root.EnvironmentPath(), envInfo)

	created := []string{
		".panex/state.json",
		".panex/project.graph.json",
		root.PolicyFilePath(),
		".panex/environment.json",
	}

	return Emit(Output{
		Status:  "initialized",
		Command: "init",
		Summary: fmt.Sprintf("initialized project with %d entrypoints, targets: %v", len(g.Entries), g.TargetsResolved),
		Data: map[string]any{
			"project_id":    g.Project.ID,
			"created_files": created,
			"graph":         g,
		},
		Next: []string{"panex plan", "panex dev"},
	})
}

// InitOptions are flags for panex init.
type InitOptions struct {
	Name    string
	Targets []string
}

// CmdDoctor runs diagnostics and optionally applies repairs.
func CmdDoctor(projectDir string, fix bool) int {
	report := doctor.Run(doctor.Options{
		ProjectDir: projectDir,
		Fix:        fix,
	})

	out := Output{
		Status:  report.Status,
		Command: "doctor",
		Data:    report,
	}

	switch report.Status {
	case "healthy":
		out.Summary = "no issues found"
	case "issues_found":
		out.Summary = fmt.Sprintf("%d issues found", len(report.Diagnoses))
		out.Next = []string{"panex doctor --fix"}
	case "repaired":
		out.Summary = fmt.Sprintf("%d issues repaired", len(report.Repaired))
	}

	for _, d := range report.Diagnoses {
		if d.Severity == "error" {
			out.Errors = append(out.Errors, d.Message)
		} else if d.Severity == "warning" {
			out.Warnings = append(out.Warnings, d.Message)
		}
	}

	return Emit(out)
}

// CmdVerify runs verification checks.
func CmdVerify(projectDir string) int {
	root, err := fsmodel.NewRoot(projectDir)
	if err != nil {
		return Emit(Output{Status: "error", Command: "verify", Errors: []string{err.Error()}})
	}

	g, err := graph.ReadFromFile(root.ProjectGraphPath())
	if err != nil {
		return Emit(Output{Status: "error", Command: "verify", Errors: []string{"cannot read project graph: " + err.Error()}, Next: []string{"panex init"}})
	}

	// Resolve capabilities through Chrome adapter
	chrome := target.NewChrome()
	adapters := map[string]target.Adapter{"chrome": chrome}

	matrix, err := capability.Compile(capability.CompilerInput{
		Capabilities: g.Capabilities,
		Targets:      g.TargetsResolved,
		Adapters:     adapters,
	})
	if err != nil {
		return Emit(Output{Status: "error", Command: "verify", Errors: []string{err.Error()}})
	}

	result := verify.Verify(verify.Input{
		Graph:  g,
		Matrix: matrix,
	})

	out := Output{
		Status:  result.Status,
		Command: "verify",
		Data:    result,
	}

	if result.Status == "passed" {
		out.Summary = "all checks passed"
		out.Next = []string{"panex package"}
	} else {
		out.Summary = fmt.Sprintf("%d hard blocks", len(result.HardBlocks))
		for _, b := range result.HardBlocks {
			out.Errors = append(out.Errors, b.Message)
		}
		out.Warnings = result.Warnings
		out.Next = []string{"panex doctor --fix"}
	}

	return Emit(out)
}

// CmdPackage creates distributable artifacts.
func CmdPackage(projectDir string, opts PackageOptions) int {
	root, err := fsmodel.NewRoot(projectDir)
	if err != nil {
		return Emit(Output{Status: "error", Command: "package", Errors: []string{err.Error()}})
	}

	g, err := graph.ReadFromFile(root.ProjectGraphPath())
	if err != nil {
		return Emit(Output{Status: "error", Command: "package", Errors: []string{"read graph: " + err.Error()}})
	}

	// Create a run
	run := ledger.NewRun("package", ledger.Actor{Type: ledger.ActorAgent, Name: "panex-cli"})
	run.ProjectHash = g.GraphHash
	_ = run.Transition(ledger.StatusRunning)

	runDir := root.RunDir(run.RunID)

	chrome := target.NewChrome()
	adapters := map[string]target.Adapter{"chrome": chrome}

	var artifacts []target.ArtifactRecord
	var errors []string

	for _, tgt := range g.TargetsResolved {
		adapter, ok := adapters[tgt]
		if !ok {
			errors = append(errors, "no adapter for target: "+tgt)
			continue
		}

		step := run.AddStep("packager", "package_"+tgt)

		sourceDir := opts.SourceDir
		if sourceDir == "" {
			sourceDir = projectDir
		}

		record, result := adapter.PackageArtifact(context.Background(), target.PackageOptions{
			SourceDir:    sourceDir,
			OutputDir:    root.ArtifactDir(tgt),
			ArtifactName: g.Project.Name,
			Version:      opts.Version,
		})

		if result.Outcome != target.Success {
			step.Fail(result.Reason)
			errors = append(errors, fmt.Sprintf("%s: %s", tgt, result.Reason))
			continue
		}

		step.Complete(record)
		artifacts = append(artifacts, record)
	}

	if len(errors) > 0 {
		_ = run.Transition(ledger.StatusFailed)
	} else {
		_ = run.Transition(ledger.StatusSucceeded)
	}

	_ = run.WriteToDir(runDir)

	// Update state
	state, _ := root.ReadState()
	state.LatestRunID = run.RunID
	_ = root.WriteState(state)

	out := Output{
		Command: "package",
		RunID:   run.RunID,
		Data: map[string]any{
			"artifacts": artifacts,
			"run_id":    run.RunID,
		},
	}

	if len(errors) > 0 {
		out.Status = "failed"
		out.Errors = errors
		out.Summary = fmt.Sprintf("%d/%d targets failed", len(errors), len(g.TargetsResolved))
		return Emit(out)
	}

	out.Status = "ok"
	out.Summary = fmt.Sprintf("%d artifacts created", len(artifacts))
	return Emit(out)
}

// PackageOptions are flags for panex package.
type PackageOptions struct {
	SourceDir string
	Version   string
}

// --- helpers ---

func summarizeInspection(r *inspector.Report) string {
	parts := []string{}
	if r.Framework != nil {
		parts = append(parts, "framework="+r.Framework.Value)
	}
	if r.Bundler != nil {
		parts = append(parts, "bundler="+r.Bundler.Value)
	}
	if r.Language != nil {
		parts = append(parts, "lang="+r.Language.Value)
	}
	if r.PackageManager != nil {
		parts = append(parts, "pm="+r.PackageManager.Value)
	}
	entries := fmt.Sprintf("entries=%d", len(r.Entrypoints))
	parts = append(parts, entries)

	s := ""
	for i, p := range parts {
		if i > 0 {
			s += ", "
		}
		s += p
	}
	return s
}

func detectEnvironment(projectDir string) map[string]any {
	chrome := target.NewChrome()
	info, _ := chrome.InspectEnvironment(context.Background())
	return map[string]any{
		"project_dir": projectDir,
		"chrome":      info,
	}
}

func writePolicyFile(path string, p *policy.Policy) {
	content := `# Panex policy — constrains agent and CI behavior
version = 1

[mutation]
allow_file_creation = true
allow_file_update = true
allow_file_deletion = false
allow_dependency_install = true
allow_lockfile_changes = true
allow_bundler_rewrite = false

[targets]
allowed = ` + fmt.Sprintf("%q", p.Targets.Allowed) + `

[permissions]
allow_new_permissions = true
allow_new_host_permissions = false
require_permission_diff_review = true

[runtime]
allow_loopback_bridge = true
allow_native_messaging = false

[repairs]
auto_apply_safe_repairs = true
max_attempts = 3

[publishing]
allow_publish = false
require_verify_pass = true
`
	os.WriteFile(path, []byte(content), 0o644)
}

func writeJSONFile(path string, v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	data = append(data, '\n')
	tmp := path + ".tmp"
	os.WriteFile(tmp, data, 0o644)
	os.Rename(tmp, path)
}
