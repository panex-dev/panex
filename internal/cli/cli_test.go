package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panex-dev/panex/internal/configloader"
	"github.com/panex-dev/panex/internal/graph"
	"github.com/panex-dev/panex/internal/policy"
)

func TestCmdInspect_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	// Capture output by redirecting stdout
	code := captureExitCode(func() int { return CmdInspect(dir) })
	if code != ExitSuccess {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestCmdInspect_WithProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"test","dependencies":{"react":"^18"}}`)
	writeFile(t, filepath.Join(dir, "tsconfig.json"), `{}`)
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "lockfileVersion: 9")

	code := captureExitCode(func() int { return CmdInspect(dir) })
	if code != ExitSuccess {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestCmdInit_NewProject(t *testing.T) {
	dir := t.TempDir()

	code := captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test-ext", Targets: []string{"chrome"}})
	})
	if code != ExitSuccess {
		t.Errorf("expected exit 0, got %d", code)
	}

	// Verify .panex/ was created
	if _, err := os.Stat(filepath.Join(dir, ".panex", "state.json")); err != nil {
		t.Error("expected state.json")
	}
	if _, err := os.Stat(filepath.Join(dir, ".panex", "project.graph.json")); err != nil {
		t.Error("expected project.graph.json")
	}
}

func TestCmdInit_AlreadyInitialized(t *testing.T) {
	dir := t.TempDir()

	// First init
	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test"})
	})

	// Second init should be no-op
	code := captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test"})
	})
	if code != ExitSuccess {
		t.Errorf("expected exit 0 for already initialized, got %d", code)
	}
}

func TestCmdDoctor_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	code := captureExitCode(func() int { return CmdDoctor(dir, false) })
	// Should succeed even with issues (reports them, doesn't fail)
	if code != ExitOperationalFail && code != ExitSuccess {
		t.Errorf("unexpected exit code: %d", code)
	}
}

func TestCmdDoctor_Fix(t *testing.T) {
	dir := t.TempDir()

	code := captureExitCode(func() int { return CmdDoctor(dir, true) })
	_ = code

	// Verify .panex was created by fix
	if _, err := os.Stat(filepath.Join(dir, ".panex")); err != nil {
		t.Error("expected .panex/ after doctor --fix")
	}
}

func TestAddTarget_BootstrapsConfigAndUpdatesGraphAndPolicy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "background.js"), "// bg")

	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test-ext", Targets: []string{"chrome"}})
	})

	out, err := AddTarget(dir, "firefox")
	if err != nil {
		t.Fatalf("AddTarget: %v", err)
	}

	if out.Command != "add-target" {
		t.Fatalf("command: got %q", out.Command)
	}
	if len(out.Warnings) == 0 {
		t.Fatal("expected warning for unresolved firefox target")
	}

	loaded, err := configloader.Load(dir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded == nil || loaded.Config == nil {
		t.Fatal("expected bootstrapped config")
	}
	if !loaded.Config.Targets["chrome"].Enabled {
		t.Fatal("expected chrome target to stay enabled")
	}
	if !loaded.Config.Targets["firefox"].Enabled {
		t.Fatal("expected firefox target to be enabled")
	}

	g, err := graph.ReadFromFile(filepath.Join(dir, ".panex", "project.graph.json"))
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}
	if got, want := g.TargetsRequested, []string{"chrome", "firefox"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("targets requested: got %v, want %v", got, want)
	}
	if got, want := g.TargetsResolved, []string{"chrome"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("targets resolved: got %v, want %v", got, want)
	}

	pol, err := policy.LoadFromFile(filepath.Join(dir, "panex.policy.toml"))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	if got, want := pol.Targets.Allowed, []string{"chrome", "firefox"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("policy targets: got %v, want %v", got, want)
	}
}

func TestAddTarget_AlreadyEnabledIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "background.js"), "// bg")

	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test-ext", Targets: []string{"chrome"}})
	})

	out, err := AddTarget(dir, "chrome")
	if err != nil {
		t.Fatalf("AddTarget: %v", err)
	}
	if out.Summary == "" {
		t.Fatal("expected summary")
	}

	pol, err := policy.LoadFromFile(filepath.Join(dir, "panex.policy.toml"))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	if got := len(pol.Targets.Allowed); got != 1 {
		t.Fatalf("policy targets length: got %d, want 1", got)
	}
	if pol.Targets.Allowed[0] != "chrome" {
		t.Fatalf("policy target: got %v", pol.Targets.Allowed)
	}
}

func TestAddTarget_UnknownTargetFails(t *testing.T) {
	dir := t.TempDir()

	if _, err := AddTarget(dir, "opera"); err == nil {
		t.Fatal("expected unknown target error")
	}

	if _, err := os.Stat(filepath.Join(dir, "panex.config.json")); !os.IsNotExist(err) {
		t.Fatalf("panex.config.json should not be written on validation error: %v", err)
	}
}

func TestAddTarget_RefusesTypeScriptConfig(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not found in PATH")
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, configloader.TypeScriptConfigFileName), `
export default {
  project: { name: "ts-ext", id: "ts-ext" },
  targets: { chrome: { enabled: true } },
};
`)

	_, err := AddTarget(dir, "firefox")
	if err == nil {
		t.Fatal("expected add-target to reject TypeScript-authored config")
	}
	if !strings.Contains(err.Error(), configloader.TypeScriptConfigFileName) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVerify_InitializedProject(t *testing.T) {
	dir := t.TempDir()

	// Init first
	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test", Targets: []string{"chrome"}})
	})

	code := captureExitCode(func() int { return CmdVerify(dir) })
	// May pass or fail depending on entrypoints — just verify no crash
	if code == ExitInternalFault {
		t.Error("internal fault during verify")
	}
}

func TestCmdPackage(t *testing.T) {
	dir := t.TempDir()

	// Create a buildable project
	writeFile(t, filepath.Join(dir, "manifest.json"), `{"manifest_version":3,"name":"Test","version":"1.0.0"}`)
	writeFile(t, filepath.Join(dir, "background.js"), "// background")
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"test"}`)

	// Init
	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test", Targets: []string{"chrome"}})
	})

	// Package
	code := captureExitCode(func() int {
		return CmdPackage(dir, PackageOptions{SourceDir: dir, Version: "1.0.0"})
	})
	if code != ExitSuccess {
		t.Errorf("expected exit 0, got %d", code)
	}

	// Verify artifact was created
	artifactDir := filepath.Join(dir, ".panex", "artifacts", "chrome")
	entries, _ := os.ReadDir(artifactDir)
	if len(entries) == 0 {
		t.Error("expected artifact in .panex/artifacts/chrome/")
	}

	// Verify run was recorded
	runsDir := filepath.Join(dir, ".panex", "runs")
	runEntries, _ := os.ReadDir(runsDir)
	if len(runEntries) == 0 {
		t.Error("expected run record in .panex/runs/")
	}
}

func TestCmdPlan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "background.js"), "// bg")

	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test", Targets: []string{"chrome"}})
	})

	code := captureExitCode(func() int { return CmdPlan(dir) })
	if code != ExitSuccess {
		t.Errorf("expected exit 0, got %d", code)
	}

	// Plan should be saved
	if _, err := os.Stat(filepath.Join(dir, ".panex", "current.plan.json")); err != nil {
		t.Error("expected plan file")
	}
}

func TestCmdApply(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "background.js"), "// bg")

	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test", Targets: []string{"chrome"}})
	})
	captureExitCode(func() int { return CmdPlan(dir) })

	code := captureExitCode(func() int { return CmdApply(dir, ApplyOptions{}) })
	if code != ExitSuccess {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestCmdApply_NoPlan(t *testing.T) {
	dir := t.TempDir()
	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test", Targets: []string{"chrome"}})
	})

	code := captureExitCode(func() int { return CmdApply(dir, ApplyOptions{}) })
	if code == ExitSuccess {
		t.Error("expected failure when no plan exists")
	}
}

func TestCmdDev_NoLaunch(t *testing.T) {
	dir := t.TempDir()
	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test", Targets: []string{"chrome"}})
	})

	code := captureExitCode(func() int {
		return CmdDev(dir, DevOptions{NoLaunch: true})
	})
	if code != ExitSuccess {
		t.Errorf("expected exit 0, got %d", code)
	}

	// Session should be written
	sessionsDir := filepath.Join(dir, ".panex", "sessions")
	entries, _ := os.ReadDir(sessionsDir)
	if len(entries) == 0 {
		t.Error("expected session dir")
	}

	data, err := os.ReadFile(filepath.Join(sessionsDir, entries[0].Name(), "session.json"))
	if err != nil {
		t.Fatalf("read session.json: %v", err)
	}

	var stored struct {
		ExtensionID string `json:"extension_id"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("unmarshal session.json: %v", err)
	}
	if stored.ExtensionID != "test" {
		t.Fatalf("extension id: got %q, want %q", stored.ExtensionID, "test")
	}
}

func TestCmdTest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "background.js"), "// bg")

	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test", Targets: []string{"chrome"}})
	})

	code := captureExitCode(func() int { return CmdTest(dir) })
	// May fail or pass depending on entry validation
	if code == ExitInternalFault {
		t.Error("internal fault during test")
	}
}

func TestCmdReport_NoRuns(t *testing.T) {
	dir := t.TempDir()
	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test"})
	})

	code := captureExitCode(func() int { return CmdReport(dir, "") })
	if code == ExitSuccess {
		t.Error("expected failure with no runs")
	}
}

func TestCmdReport_AfterPackage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"), `{"manifest_version":3,"name":"Test","version":"1.0.0"}`)
	writeFile(t, filepath.Join(dir, "background.js"), "// bg")
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"test"}`)

	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test", Targets: []string{"chrome"}})
	})
	captureExitCode(func() int {
		return CmdPackage(dir, PackageOptions{SourceDir: dir, Version: "1.0.0"})
	})

	code := captureExitCode(func() int { return CmdReport(dir, "") })
	if code != ExitSuccess {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestCmdResume_NoRun(t *testing.T) {
	dir := t.TempDir()
	captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "test"})
	})

	code := captureExitCode(func() int { return CmdResume(dir, "") })
	if code == ExitSuccess {
		t.Error("expected failure with no run to resume")
	}
}

func TestFullWorkflow_Inspect_Plan_Apply_Verify_Package(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"), `{"manifest_version":3,"name":"Integration","version":"1.0.0"}`)
	writeFile(t, filepath.Join(dir, "background.js"), `console.log("bg")`)
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"integration-test"}`)
	if err := os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. Inspect
	code := captureExitCode(func() int { return CmdInspect(dir) })
	if code != ExitSuccess {
		t.Fatalf("inspect: exit %d", code)
	}

	// 2. Init
	code = captureExitCode(func() int {
		return CmdInit(dir, InitOptions{Name: "integration-test", Targets: []string{"chrome"}})
	})
	if code != ExitSuccess {
		t.Fatalf("init: exit %d", code)
	}

	// 3. Plan
	code = captureExitCode(func() int { return CmdPlan(dir) })
	if code != ExitSuccess {
		t.Fatalf("plan: exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, ".panex", "current.plan.json")); err != nil {
		t.Fatal("plan file not written")
	}

	// 4. Apply
	code = captureExitCode(func() int { return CmdApply(dir, ApplyOptions{}) })
	if code != ExitSuccess {
		t.Fatalf("apply: exit %d", code)
	}

	// 5. Verify
	code = captureExitCode(func() int { return CmdVerify(dir) })
	if code == ExitInternalFault {
		t.Fatal("verify: internal fault")
	}

	// 6. Package
	code = captureExitCode(func() int {
		return CmdPackage(dir, PackageOptions{SourceDir: dir, Version: "1.0.0"})
	})
	if code != ExitSuccess {
		t.Fatalf("package: exit %d", code)
	}

	// Verify artifacts
	entries, _ := os.ReadDir(filepath.Join(dir, ".panex", "artifacts", "chrome"))
	if len(entries) == 0 {
		t.Error("no artifacts produced")
	}

	// 7. Report (should read the latest run)
	code = captureExitCode(func() int { return CmdReport(dir, "") })
	if code != ExitSuccess {
		t.Fatalf("report: exit %d", code)
	}
}

func TestOutput_JSON(t *testing.T) {
	out := Output{
		Status:  "ok",
		Command: "inspect",
		Summary: "test summary",
		Data:    map[string]string{"key": "value"},
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]any
	if json.Unmarshal(data, &parsed) != nil {
		t.Fatal("output should be valid JSON")
	}
	if parsed["status"] != "ok" {
		t.Errorf("status: got %v", parsed["status"])
	}
}

// --- helpers ---

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// captureExitCode runs a function that returns an exit code,
// redirecting stdout to discard during test.
func captureExitCode(fn func() int) int {
	// Save and redirect stdout
	old := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = old }()
	return fn()
}
