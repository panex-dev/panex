package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/panex-dev/panex/internal/cli"
	panexconfig "github.com/panex-dev/panex/internal/config"
	"github.com/panex-dev/panex/internal/doctor"
)

var readProcVersion = defaultReadProcVersion
var currentGOOS = runtime.GOOS

func runDoctor(stdout io.Writer) error {
	return runDoctorInProject(projectDir(), stdout, false)
}

func runDoctorInProject(projectDir string, stdout io.Writer, jsonOutput bool) error {
	if jsonOutput {
		report := doctor.Run(doctor.Options{ProjectDir: projectDir})
		out := cli.Output{
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
		return writeJSONEnvelope(stdout, out)
	}

	if err := writeString(stdout, "panex doctor\n\n"); err != nil {
		return err
	}

	issues := 0

	cfg, configSource, configErr := detectConfig(projectDir)
	if configErr != nil {
		issues++
		if errors.Is(configErr, panexconfig.ErrConfigFileNotFound) || errors.Is(configErr, panexconfig.ErrManifestNotFound) {
			if err := writeString(stdout, "config: not found\n  Run `panex init` in the project directory, or pass `--cwd` to a directory with manifest.json.\n"); err != nil {
				return err
			}
		} else {
			if err := writef(stdout, "config: %s (error: %v)\n", panexconfig.DefaultPath, configErr); err != nil {
				return err
			}
		}
		return writeDoctorSummary(stdout, issues)
	}

	if err := writef(stdout, "config: %s\n", configSource); err != nil {
		return err
	}

	for _, ext := range cfg.Extensions {
		label := ""
		if len(cfg.Extensions) > 1 {
			label = " [" + ext.ID + "]"
		}

		absSource, _ := filepath.Abs(ext.SourceDir)
		if info, err := os.Stat(absSource); err != nil || !info.IsDir() {
			issues++
			if err := writef(stdout, "source_dir%s: %s (not found)\n", label, absSource); err != nil {
				return err
			}
		} else {
			if err := writef(stdout, "source_dir%s: %s (ok)\n", label, absSource); err != nil {
				return err
			}
		}

		absOut, _ := filepath.Abs(ext.OutDir)
		if _, err := os.Stat(absOut); err != nil {
			issues++
			if err := writef(stdout, "out_dir%s: %s (not built yet)\n  Run `panex dev` to build the extension.\n", label, absOut); err != nil {
				return err
			}
		} else if _, err := os.Stat(filepath.Join(absOut, "manifest.json")); err != nil {
			issues++
			if err := writef(stdout, "out_dir%s: %s (exists, but no manifest.json)\n  The build may have failed. Run `panex dev` and check for errors.\n", label, absOut); err != nil {
				return err
			}
		} else {
			if err := writef(stdout, "out_dir%s: %s (ok, manifest.json found)\n", label, absOut); err != nil {
				return err
			}
		}
	}

	if cfg.Server.AuthToken == panexconfig.DefaultAuthToken {
		issues++
		if err := writeString(stdout, "auth_token: using default \"dev-token\" — run `panex init --force` to generate a unique token\n"); err != nil {
			return err
		}
	}

	if isWSL() {
		for _, ext := range cfg.Extensions {
			absOut, _ := filepath.Abs(ext.OutDir)
			if !strings.HasPrefix(absOut, "/mnt/") {
				issues++
				if err := writef(stdout, "\nwarning: WSL detected — output at %s may not be visible to Windows Chrome\n  Work under /mnt/c/... or copy output to a Windows-visible path.\n", absOut); err != nil {
					return err
				}
				break
			}
		}
	}

	return writeDoctorSummary(stdout, issues)
}

func detectConfig(projectDir string) (panexconfig.Config, string, error) {
	configPath := filepath.Join(projectDir, panexconfig.DefaultPath)

	cfg, loadErr := panexconfig.Load(configPath)
	if loadErr == nil {
		return resolveConfigPaths(cfg, projectDir), panexconfig.DefaultPath, nil
	}

	if !errors.Is(loadErr, panexconfig.ErrConfigFileNotFound) {
		return panexconfig.Config{}, "", loadErr
	}

	cfg, inferErr := panexconfig.Infer(projectDir)
	if inferErr == nil {
		return resolveConfigPaths(cfg, projectDir), "manifest.json (inferred)", nil
	}

	return panexconfig.Config{}, "", inferErr
}

func writeDoctorSummary(w io.Writer, issues int) error {
	if issues == 0 {
		return writeString(w, "\nNo issues found.\n")
	}
	return writef(w, "\n%d issue(s) found.\n", issues)
}

func isWSL() bool {
	if currentGOOS != "linux" {
		return false
	}
	data := readProcVersion()
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

func defaultReadProcVersion() []byte {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return nil
	}
	return data
}
