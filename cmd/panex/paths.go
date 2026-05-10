package main

import (
	"io"
	"path/filepath"
)

func runPaths(stdout io.Writer) error {
	return runPathsInProject(projectDir(), stdout)
}

func runPathsInProject(projectDir string, stdout io.Writer) error {
	cfg, _, err := detectConfig(projectDir)
	if err != nil {
		return &cliError{
			code: 2,
			msg:  "no panex.toml or manifest.json found\n\nRun `panex init` in the project directory, or pass `--cwd` to a directory with manifest.json.",
		}
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
