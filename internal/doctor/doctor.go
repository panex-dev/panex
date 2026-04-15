// Package doctor implements diagnostics and repair recipes for
// known failure classes. Spec sections 29-30.
package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const staleLockThreshold = 1 * time.Hour

// Diagnosis is a single detected issue.
type Diagnosis struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"` // "error", "warning", "info"
	Message    string `json:"message"`
	Component  string `json:"component"`
	Repairable bool   `json:"repairable"`
	RecipeID   string `json:"recipe_id,omitempty"`
}

// Report is the full doctor output.
type Report struct {
	Status    string      `json:"status"` // "healthy", "issues_found", "repaired"
	Diagnoses []Diagnosis `json:"diagnoses"`
	Repaired  []string    `json:"repaired,omitempty"`
	Failed    []string    `json:"failed_repairs,omitempty"`
}

// Options controls doctor behavior.
type Options struct {
	ProjectDir string
	Fix        bool // auto-apply safe repairs
}

// Run executes all diagnostic checks.
func Run(opts Options) *Report {
	r := &Report{
		Status:    "healthy",
		Diagnoses: []Diagnosis{},
	}

	checkPanexDir(opts.ProjectDir, r)
	checkManifestJSON(opts.ProjectDir, r)
	checkPackageJSON(opts.ProjectDir, r)
	checkNodeModules(opts.ProjectDir, r)
	checkStateIntegrity(opts.ProjectDir, r)
	checkStaleLocks(opts.ProjectDir, r)

	if len(r.Diagnoses) > 0 {
		r.Status = "issues_found"
	}

	if opts.Fix {
		applyRepairs(opts, r)
		if len(r.Repaired) > 0 && len(r.Failed) == 0 {
			r.Status = "repaired"
		}
	}

	return r
}

// --- diagnostic checks ---

func checkPanexDir(projectDir string, r *Report) {
	panexDir := filepath.Join(projectDir, ".panex")
	if _, err := os.Stat(panexDir); os.IsNotExist(err) {
		r.Diagnoses = append(r.Diagnoses, Diagnosis{
			Code:       "PANEX_NOT_INITIALIZED",
			Severity:   "error",
			Message:    ".panex/ directory does not exist — run panex init",
			Component:  "fsmodel",
			Repairable: true,
			RecipeID:   "init_state_dir",
		})
	}
}

func checkManifestJSON(projectDir string, r *Report) {
	// Check common locations
	locations := []string{
		filepath.Join(projectDir, "manifest.json"),
		filepath.Join(projectDir, "src", "manifest.json"),
		filepath.Join(projectDir, "public", "manifest.json"),
	}

	found := false
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			found = true
			// Validate JSON
			data, err := os.ReadFile(loc)
			if err == nil {
				var m map[string]any
				if json.Unmarshal(data, &m) != nil {
					r.Diagnoses = append(r.Diagnoses, Diagnosis{
						Code:       "MANIFEST_INVALID_JSON",
						Severity:   "error",
						Message:    fmt.Sprintf("%s is not valid JSON", loc),
						Component:  "manifest",
						Repairable: false,
					})
				} else {
					// Check manifest_version
					if _, ok := m["manifest_version"]; !ok {
						r.Diagnoses = append(r.Diagnoses, Diagnosis{
							Code:       "MANIFEST_MISSING_VERSION",
							Severity:   "warning",
							Message:    "manifest.json missing manifest_version field",
							Component:  "manifest",
							Repairable: true,
							RecipeID:   "add_manifest_version",
						})
					}
				}
			}
			break
		}
	}

	if !found {
		r.Diagnoses = append(r.Diagnoses, Diagnosis{
			Code:       "MANIFEST_NOT_FOUND",
			Severity:   "info",
			Message:    "no manifest.json found — Panex will generate one",
			Component:  "manifest",
			Repairable: false,
		})
	}
}

func checkPackageJSON(projectDir string, r *Report) {
	pkgPath := filepath.Join(projectDir, "package.json")
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		r.Diagnoses = append(r.Diagnoses, Diagnosis{
			Code:       "PACKAGE_JSON_MISSING",
			Severity:   "warning",
			Message:    "no package.json found",
			Component:  "inspector",
			Repairable: true,
			RecipeID:   "create_package_json",
		})
	}
}

func checkNodeModules(projectDir string, r *Report) {
	pkgPath := filepath.Join(projectDir, "package.json")
	nmPath := filepath.Join(projectDir, "node_modules")

	if _, err := os.Stat(pkgPath); err == nil {
		if _, err := os.Stat(nmPath); os.IsNotExist(err) {
			r.Diagnoses = append(r.Diagnoses, Diagnosis{
				Code:       "DEPENDENCIES_NOT_INSTALLED",
				Severity:   "warning",
				Message:    "package.json exists but node_modules/ missing — run npm/pnpm install",
				Component:  "dependency",
				Repairable: true,
				RecipeID:   "install_dependencies",
			})
		}
	}
}

func checkStateIntegrity(projectDir string, r *Report) {
	statePath := filepath.Join(projectDir, ".panex", "state.json")
	if _, err := os.Stat(statePath); err != nil {
		return // not initialized, already reported
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		r.Diagnoses = append(r.Diagnoses, Diagnosis{
			Code:       "STATE_UNREADABLE",
			Severity:   "error",
			Message:    fmt.Sprintf("cannot read state.json: %v", err),
			Component:  "fsmodel",
			Repairable: true,
			RecipeID:   "reset_state",
		})
		return
	}

	var state map[string]any
	if json.Unmarshal(data, &state) != nil {
		r.Diagnoses = append(r.Diagnoses, Diagnosis{
			Code:       "STATE_CORRUPT",
			Severity:   "error",
			Message:    "state.json is not valid JSON",
			Component:  "fsmodel",
			Repairable: true,
			RecipeID:   "reset_state",
		})
	}
}

func checkStaleLocks(projectDir string, r *Report) {
	locksDir := filepath.Join(projectDir, ".panex", "locks")
	entries, err := os.ReadDir(locksDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			info, err := e.Info()
			if err != nil {
				continue
			}
			age := time.Since(info.ModTime())
			if age < staleLockThreshold {
				continue // recent lock, not stale
			}
			r.Diagnoses = append(r.Diagnoses, Diagnosis{
				Code:       "STALE_LOCK",
				Severity:   "warning",
				Message:    fmt.Sprintf("lock file %s is %s old (size: %d bytes) — likely stale", e.Name(), age.Truncate(time.Second), info.Size()),
				Component:  "concurrency",
				Repairable: true,
				RecipeID:   "remove_stale_lock",
			})
		}
	}
}

// --- repair execution ---

func applyRepairs(opts Options, r *Report) {
	for _, d := range r.Diagnoses {
		if !d.Repairable || d.RecipeID == "" {
			continue
		}

		var err error
		switch d.RecipeID {
		case "init_state_dir":
			err = repairInitStateDir(opts.ProjectDir)
		case "remove_stale_lock":
			err = repairRemoveStaleLock(opts.ProjectDir, d.Message)
		case "reset_state":
			err = repairResetState(opts.ProjectDir)
		default:
			continue // unknown recipe, skip
		}

		if err != nil {
			r.Failed = append(r.Failed, fmt.Sprintf("%s: %v", d.RecipeID, err))
		} else {
			r.Repaired = append(r.Repaired, d.RecipeID)
		}
	}
}

func repairInitStateDir(projectDir string) error {
	dirs := []string{
		filepath.Join(projectDir, ".panex"),
		filepath.Join(projectDir, ".panex", "runs"),
		filepath.Join(projectDir, ".panex", "sessions"),
		filepath.Join(projectDir, ".panex", "reports"),
		filepath.Join(projectDir, ".panex", "cache"),
		filepath.Join(projectDir, ".panex", "artifacts"),
		filepath.Join(projectDir, ".panex", "locks"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	// Write minimal state.json
	state := map[string]any{"schema_version": 1}
	data, _ := json.MarshalIndent(state, "", "  ")
	return os.WriteFile(filepath.Join(projectDir, ".panex", "state.json"), data, 0o644)
}

func repairRemoveStaleLock(projectDir, _ string) error {
	locksDir := filepath.Join(projectDir, ".panex", "locks")
	entries, err := os.ReadDir(locksDir)
	if err != nil {
		return err
	}
	var errs []string
	for _, e := range entries {
		if !e.IsDir() {
			if err := os.Remove(filepath.Join(locksDir, e.Name())); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to remove locks: %s", errs[0])
	}
	return nil
}

func repairResetState(projectDir string) error {
	state := map[string]any{"schema_version": 1}
	data, _ := json.MarshalIndent(state, "", "  ")
	return os.WriteFile(filepath.Join(projectDir, ".panex", "state.json"), data, 0o644)
}
