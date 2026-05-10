package main

import (
	"io"
	"path/filepath"

	"github.com/panex-dev/panex/internal/cli"
)

func runPaths(stdout io.Writer) error {
	return runPathsInProject(projectDir(), stdout, false)
}

func runPathsInProject(projectDir string, stdout io.Writer, jsonOutput bool) error {
	cfg, _, err := detectConfig(projectDir)
	if err != nil {
		if jsonOutput {
			return writeJSONCommandError(
				stdout,
				2,
				"paths",
				"no panex.toml or manifest.json found",
				[]string{"panex init"},
				nil,
			)
		}
		return &cliError{
			code: 2,
			msg:  "no panex.toml or manifest.json found\n\nRun `panex init` in the project directory, or pass `--cwd` to a directory with manifest.json.",
		}
	}

	if jsonOutput {
		return writeJSONEnvelope(stdout, cli.Output{
			Status:  "ok",
			Command: "paths",
			Summary: "resolved extension paths",
			Data: map[string]any{
				"extensions": configPathEntries(cfg),
			},
		})
	}

	for _, ext := range cfg.Extensions {
		absSource, absErr := filepath.Abs(ext.SourceDir)
		if absErr != nil {
			absSource = ext.SourceDir
		}
		absOut, absErr := filepath.Abs(ext.OutDir)
		if absErr != nil {
			absOut = ext.OutDir
		}

		if len(cfg.Extensions) > 1 {
			if err := writef(stdout, "source_dir[%s]=%s\n", ext.ID, absSource); err != nil {
				return err
			}
			if err := writef(stdout, "out_dir[%s]=%s\n", ext.ID, absOut); err != nil {
				return err
			}
		} else {
			if err := writef(stdout, "source_dir=%s\n", absSource); err != nil {
				return err
			}
			if err := writef(stdout, "out_dir=%s\n", absOut); err != nil {
				return err
			}
		}
	}

	return nil
}
