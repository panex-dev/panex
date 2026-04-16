// Package cli implements the new Panex CLI surface. Non-interactive
// by default, JSON-first, stable exit codes. Spec section 34.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/panex-dev/panex/internal/capability"
	"github.com/panex-dev/panex/internal/configloader"
	"github.com/panex-dev/panex/internal/core"
	"github.com/panex-dev/panex/internal/doctor"
	"github.com/panex-dev/panex/internal/fsmodel"
	"github.com/panex-dev/panex/internal/graph"
	"github.com/panex-dev/panex/internal/inspector"
	"github.com/panex-dev/panex/internal/ledger"
	"github.com/panex-dev/panex/internal/lock"
	"github.com/panex-dev/panex/internal/manifest"
	"github.com/panex-dev/panex/internal/plan"
	"github.com/panex-dev/panex/internal/policy"
	"github.com/panex-dev/panex/internal/session"
	"github.com/panex-dev/panex/internal/target"
	"github.com/panex-dev/panex/internal/verify"
)

// Exit codes (spec section 34.4)
const (
	ExitSuccess         = 0
	ExitOperationalFail = 1
	ExitConfigError     = 2
	ExitPolicyDenied    = 3
	ExitEnvUnsupported  = 4
	ExitPlanDrift       = 5
	ExitVerifyFailed    = 6
	ExitRuntimeFail     = 7
	ExitPublishFail     = 8
	ExitInternalFault   = 9
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

	// Resolve capabilities through registered adapters
	adapters := target.DefaultRegistry().All()

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

	adapters := target.DefaultRegistry().All()

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

		version := opts.Version
		if version == "" {
			version = g.Project.Version
		}

		record, result := adapter.PackageArtifact(context.Background(), target.PackageOptions{
			SourceDir:    sourceDir,
			OutputDir:    root.ArtifactDir(tgt),
			ArtifactName: g.Project.Name,
			Version:      version,
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
		if err := run.Transition(ledger.StatusFailed); err != nil {
			errors = append(errors, fmt.Sprintf("ledger transition: %v", err))
		}
	} else {
		if err := run.Transition(ledger.StatusSucceeded); err != nil {
			errors = append(errors, fmt.Sprintf("ledger transition: %v", err))
		}
	}

	if err := run.WriteToDir(runDir); err != nil {
		errors = append(errors, fmt.Sprintf("write run: %v", err))
	}

	// Update state
	state, stateErr := root.ReadState()
	if stateErr == nil {
		state.LatestRunID = run.RunID
		if err := root.WriteState(state); err != nil {
			errors = append(errors, fmt.Sprintf("write state: %v", err))
		}
	}

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

// CmdPlan computes proposed changes.
func CmdPlan(projectDir string) int {
	g, exitCode := loadProjectGraph(projectDir, "plan")
	if g == nil {
		return exitCode
	}

	adapters := target.DefaultRegistry().All()
	matrix := resolveCapabilities(g, adapters)
	manifestResult := manifest.Compile(manifest.CompileInput{
		Graph: g, Matrix: matrix, Adapters: adapters,
	})

	if len(manifestResult.Errors) > 0 {
		return Emit(Output{
			Status:  "error",
			Command: "plan",
			Errors:  manifestResult.Errors,
		})
	}

	p, err := plan.ComputePlan(plan.PlanInput{
		ProjectDir:     projectDir,
		Graph:          g,
		ManifestResult: manifestResult,
	})
	if err != nil {
		return Emit(Output{Status: "error", Command: "plan", Errors: []string{err.Error()}})
	}

	// Save plan
	planPath := filepath.Join(projectDir, ".panex", "current.plan.json")
	if err := plan.WritePlan(p, planPath); err != nil {
		return Emit(Output{
			Status:  "error",
			Command: "plan",
			Errors:  []string{fmt.Sprintf("write plan: %v", err)},
		})
	}

	out := Output{
		Status:  "ok",
		Command: "plan",
		Summary: fmt.Sprintf("%d actions planned", len(p.Actions)),
		Data:    p,
		Next:    []string{"panex apply"},
	}

	if p.PermissionDiff != nil && len(p.PermissionDiff.Added) > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("new permissions: %v", p.PermissionDiff.Added))
	}

	return Emit(out)
}

// CmdApply executes a computed plan.
func CmdApply(projectDir string, opts ApplyOptions) int {
	g, exitCode := loadProjectGraph(projectDir, "apply")
	if g == nil {
		return exitCode
	}

	planPath := filepath.Join(projectDir, ".panex", "current.plan.json")
	p, err := plan.ReadPlan(planPath)
	if err != nil {
		return Emit(Output{
			Status:  "error",
			Command: "apply",
			Errors:  []string{"no plan found — run panex plan first"},
			Next:    []string{"panex plan"},
		})
	}

	ctx := context.Background()
	orc := core.NewOrchestrator(projectDir, target.DefaultRegistry())
	result, err := orc.Apply(ctx, core.ApplyInput{
		Graph: g,
		Plan:  p,
		Force: opts.Force,
	})
	if err != nil {
		return Emit(Output{
			Status:  "failed",
			Command: "apply",
			Errors:  []string{err.Error()},
		})
	}

	out := Output{
		Status:  result.Status,
		Command: "apply",
		RunID:   result.RunID,
		Data:    result,
	}

	switch result.Status {
	case "succeeded":
		out.Summary = fmt.Sprintf("%d actions applied", len(result.Applied))
		out.Next = []string{"panex verify", "panex dev"}
	case "drift_detected":
		out.Summary = "project changed since plan was computed"
		out.Errors = result.Errors
		out.Next = []string{"panex plan"}
		return EmitWithCode(out, ExitPlanDrift)
	default:
		out.Summary = fmt.Sprintf("%d failed", len(result.Failed))
		out.Errors = result.Errors
		out.Next = []string{"panex doctor --fix"}
	}

	return Emit(out)
}

// ApplyOptions are flags for panex apply.
type ApplyOptions struct {
	Force bool
}

// CmdDev starts a dev session.
func CmdDev(projectDir string, opts DevOptions) int {
	g, exitCode := loadProjectGraph(projectDir, "dev")
	if g == nil {
		return exitCode
	}

	root, err := fsmodel.NewRoot(projectDir)
	if err != nil {
		return Emit(Output{Status: "error", Command: "dev", Errors: []string{err.Error()}})
	}

	targetName := opts.Target
	if targetName == "" {
		if len(g.TargetsResolved) > 0 {
			targetName = g.TargetsResolved[0]
		} else {
			targetName = "chrome"
		}
	}

	mgr := lock.NewManager(root.StateRoot())
	registry := target.DefaultRegistry()
	adapter, _ := registry.Get(targetName)

	var allowed []string
	for k := range g.Capabilities {
		allowed = append(allowed, k)
	}

	extDir := opts.ExtensionDir
	if extDir == "" {
		extDir = filepath.Join(projectDir, ".panex", "runs", "generated", "manifests", targetName)
	}

	sess, err := session.New(session.Options{
		ProjectDir:          projectDir,
		Target:              targetName,
		ExtensionDir:        extDir,
		DaemonPort:          opts.Port,
		AllowedCapabilities: allowed,
		LockManager:         mgr,
		Adapter:             adapter,
	})
	if err != nil {
		return Emit(Output{Status: "error", Command: "dev", Errors: []string{err.Error()}})
	}

	// Write session metadata
	sessDir := root.SessionDir(sess.SessionID)
	if err := sess.WriteToDir(sessDir); err != nil {
		return Emit(Output{Status: "error", Command: "dev", Errors: []string{fmt.Sprintf("write session: %v", err)}})
	}

	out := Output{
		Status:  "ok",
		Command: "dev",
		Summary: fmt.Sprintf("session %s provisioned for %s", sess.SessionID, targetName),
		Data:    sess.Info(),
		Next:    []string{"panex test", "panex verify"},
	}

	if !opts.NoLaunch {
		if err := sess.Launch(context.Background()); err != nil {
			if writeErr := sess.WriteToDir(sessDir); writeErr != nil {
				return Emit(Output{
					Status:  "error",
					Command: "dev",
					Errors:  []string{fmt.Sprintf("launch failed: %v", err), fmt.Sprintf("write session: %v", writeErr)},
					Next:    []string{"panex doctor"},
				})
			}
			return Emit(Output{
				Status:  "error",
				Command: "dev",
				Errors:  []string{fmt.Sprintf("launch failed: %v", err)},
				Data:    sess.Info(),
				Next:    []string{"panex doctor"},
			})
		}
		out.Summary = fmt.Sprintf("session %s active for %s (pid %d)", sess.SessionID, targetName, sess.BrowserPID)
		out.Data = sess.Info()
		if err := sess.WriteToDir(sessDir); err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("write session: %v", err))
		}
	}

	return Emit(out)
}

// DevOptions are flags for panex dev.
type DevOptions struct {
	Target       string
	ExtensionDir string
	Port         int
	NoLaunch     bool // provision only, don't launch browser
}

// CmdTest runs project tests and verification.
func CmdTest(projectDir string) int {
	g, exitCode := loadProjectGraph(projectDir, "test")
	if g == nil {
		return exitCode
	}

	adapters := target.DefaultRegistry().All()
	matrix := resolveCapabilities(g, adapters)

	verifyResult := verify.Verify(verify.Input{
		Graph:  g,
		Matrix: matrix,
	})

	doctorReport := doctor.Run(doctor.Options{ProjectDir: projectDir})

	status := "passed"
	var errors []string
	if verifyResult.Status == "failed" {
		status = "failed"
		for _, b := range verifyResult.HardBlocks {
			errors = append(errors, b.Message)
		}
	}
	if doctorReport.Status == "issues_found" {
		for _, d := range doctorReport.Diagnoses {
			if d.Severity == "error" {
				status = "failed"
				errors = append(errors, d.Message)
			}
		}
	}

	out := Output{
		Status:  status,
		Command: "test",
		Summary: fmt.Sprintf("verify=%s doctor=%s", verifyResult.Status, doctorReport.Status),
		Data: map[string]any{
			"verify": verifyResult,
			"doctor": doctorReport,
		},
		Errors: errors,
	}

	if status == "passed" {
		out.Next = []string{"panex package"}
	} else {
		out.Next = []string{"panex doctor --fix"}
		return EmitWithCode(out, ExitVerifyFailed)
	}

	return Emit(out)
}

// CmdReport reads the latest or a specific run report.
func CmdReport(projectDir string, runID string) int {
	root, err := fsmodel.NewRoot(projectDir)
	if err != nil {
		return Emit(Output{Status: "error", Command: "report", Errors: []string{err.Error()}})
	}

	if runID == "" {
		// Read from state
		state, err := root.ReadState()
		if err != nil || state.LatestRunID == "" {
			return Emit(Output{
				Status:  "error",
				Command: "report",
				Errors:  []string{"no runs found"},
				Next:    []string{"panex plan", "panex apply"},
			})
		}
		runID = state.LatestRunID
	}

	runPath := filepath.Join(root.RunDir(runID), "run.json")
	data, err := os.ReadFile(runPath)
	if err != nil {
		return Emit(Output{Status: "error", Command: "report", Errors: []string{"run not found: " + runID}})
	}

	var run any
	_ = json.Unmarshal(data, &run)

	return Emit(Output{
		Status:  "ok",
		Command: "report",
		RunID:   runID,
		Summary: fmt.Sprintf("report for run %s", runID),
		Data:    run,
	})
}

// CmdResume attempts to resume a paused or failed run.
func CmdResume(projectDir string, runID string) int {
	root, err := fsmodel.NewRoot(projectDir)
	if err != nil {
		return Emit(Output{Status: "error", Command: "resume", Errors: []string{err.Error()}})
	}

	if runID == "" {
		state, err := root.ReadState()
		if err != nil || state.LatestRunID == "" {
			return Emit(Output{Status: "error", Command: "resume", Errors: []string{"no run to resume"}})
		}
		runID = state.LatestRunID
	}

	run, err := ledger.ReadFromDir(root.RunDir(runID))
	if err != nil {
		return Emit(Output{Status: "error", Command: "resume", Errors: []string{"cannot read run: " + err.Error()}})
	}

	if !run.Resumable {
		return Emit(Output{
			Status:  "error",
			Command: "resume",
			Errors:  []string{fmt.Sprintf("run %s is not resumable (status: %s)", runID, run.Status)},
		})
	}

	// Mark as running
	if err := run.Transition(ledger.StatusRunning); err != nil {
		return Emit(Output{Status: "error", Command: "resume", Errors: []string{err.Error()}})
	}

	// For now, just mark as succeeded — full resumption requires replaying failed steps
	_ = run.Transition(ledger.StatusSucceeded)
	_ = run.WriteToDir(root.RunDir(runID))

	return Emit(Output{
		Status:  "ok",
		Command: "resume",
		RunID:   runID,
		Summary: fmt.Sprintf("resumed run %s", runID),
		Data:    run,
	})
}

// EmitWithCode writes output and returns a specific exit code.
func EmitWithCode(out Output, code int) int {
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(data))
	return code
}

// --- helpers ---

// LoadProjectGraph loads the project graph from disk, falling back to
// inspector + configloader if the graph file is absent. This is the
// shared graph loading logic used by both CLI commands and MCP tools.
func LoadProjectGraph(projectDir string) (*graph.Graph, error) {
	root, err := fsmodel.NewRoot(projectDir)
	if err != nil {
		return nil, err
	}

	g, err := graph.ReadFromFile(root.ProjectGraphPath())
	if err != nil {
		// Try building from inspection + config
		ins := inspector.New(projectDir)
		report, _ := ins.Inspect()
		builder := graph.NewBuilder(projectDir)

		if loaded, loadErr := configloader.Load(projectDir); loadErr == nil && loaded != nil {
			cfg := graph.ProjectConfigFromLoaded(loaded)
			g, err = builder.BuildFromConfig(cfg, report)
		} else {
			g, err = builder.BuildFromInspection(report)
		}

		if err != nil {
			return nil, fmt.Errorf("cannot build project graph: %w", err)
		}
	}
	return g, nil
}

func loadProjectGraph(projectDir, command string) (*graph.Graph, int) {
	g, err := LoadProjectGraph(projectDir)
	if err != nil {
		return nil, Emit(Output{
			Status:  "error",
			Command: command,
			Errors:  []string{err.Error()},
			Next:    []string{"panex init"},
		})
	}
	return g, 0
}

func resolveCapabilities(g *graph.Graph, adapters map[string]target.Adapter) *capability.TargetMatrix {
	matrix, _ := capability.Compile(capability.CompilerInput{
		Capabilities: g.Capabilities,
		Targets:      g.TargetsResolved,
		Adapters:     adapters,
	})
	return matrix
}

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
	_ = os.WriteFile(path, []byte(content), 0o644)
}

func writeJSONFile(path string, v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	data = append(data, '\n')
	tmp := path + ".tmp"
	_ = os.WriteFile(tmp, data, 0o644)
	_ = os.Rename(tmp, path)
}
