package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/panex-dev/panex/internal/cli"
	panexconfig "github.com/panex-dev/panex/internal/config"
)

const (
	defaultScaffoldSourceDir = "panex-extension"
	defaultScaffoldOutDir    = ".panex/dist"
)

type scaffoldFile struct {
	relativePath string
	contents     string
	perm         os.FileMode
}

func runInitInProject(projectRoot string, args []string, stdout io.Writer, jsonOutput bool) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	force := fs.Bool("force", false, "Overwrite scaffolded Panex starter files if they already exist")
	if err := fs.Parse(args); err != nil {
		if jsonOutput {
			return writeJSONCommandError(stdout, 2, "init", fmt.Sprintf("invalid init flags: %v", err), nil, nil)
		}
		return &cliError{
			code: 2,
			msg:  fmt.Sprintf("invalid init flags: %v", err),
		}
	}
	if fs.NArg() > 0 {
		if jsonOutput {
			return writeJSONCommandError(stdout, 2, "init", fmt.Sprintf("unexpected arguments for init: %v", fs.Args()), nil, nil)
		}
		return &cliError{
			code: 2,
			msg:  fmt.Sprintf("unexpected arguments for init: %v", fs.Args()),
		}
	}

	result, err := scaffoldStarterProject(projectRoot, *force)
	if err != nil {
		if jsonOutput {
			return writeJSONCommandError(stdout, 2, "init", err.Error(), nil, nil)
		}
		return &cliError{
			code: 2,
			msg:  err.Error(),
		}
	}

	if jsonOutput {
		return writeJSONEnvelope(stdout, cli.Output{
			Status:  "ok",
			Command: "init",
			Summary: "initialized starter project",
			Data: map[string]any{
				"config":     result.configPath,
				"source_dir": result.sourceDir,
				"out_dir":    result.outDir,
			},
			Next: []string{"panex dev", fmt.Sprintf("Load unpacked from %s in chrome://extensions", result.outDir)},
		})
	}

	return writef(
		stdout,
		"panex init\nconfig=%s\nsource_dir=%s\nout_dir=%s\n\nNext:\n  panex dev\n  Load unpacked from %s in chrome://extensions\n",
		result.configPath,
		result.sourceDir,
		result.outDir,
		result.outDir,
	)
}

type scaffoldResult struct {
	configPath string
	sourceDir  string
	outDir     string
}

func generateAuthToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate auth token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func scaffoldStarterProject(root string, force bool) (scaffoldResult, error) {
	configPath := filepath.Join(root, panexconfig.DefaultPath)
	sourceDir := filepath.Join(root, defaultScaffoldSourceDir)
	if err := ensureWritableScaffoldTargets(force, configPath, sourceDir); err != nil {
		return scaffoldResult{}, err
	}

	token, err := generateAuthToken()
	if err != nil {
		return scaffoldResult{}, err
	}

	files := []scaffoldFile{
		{
			relativePath: panexconfig.DefaultPath,
			contents: fmt.Sprintf(`[extension]
source_dir = "panex-extension"
out_dir = ".panex/dist"

[server]
port = 4317
auth_token = %q
event_store_path = ".panex/events.db"
`, token),
			perm: 0o600,
		},
		{
			relativePath: filepath.Join(defaultScaffoldSourceDir, "manifest.json"),
			contents: `{
  "manifest_version": 3,
  "name": "Panex Starter Extension",
  "version": "0.0.1",
  "background": {
    "service_worker": "background.js"
  },
  "action": {
    "default_popup": "popup.html",
    "default_title": "Panex Starter Extension"
  }
}
`,
			perm: 0o644,
		},
		{
			relativePath: filepath.Join(defaultScaffoldSourceDir, "background.js"),
			contents: `console.log("Panex starter extension loaded");
`,
			perm: 0o644,
		},
		{
			relativePath: filepath.Join(defaultScaffoldSourceDir, "popup.html"),
			contents: `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width,initial-scale=1">
    <title>Panex Starter</title>
  </head>
  <body>
    <main>
      <h1>Panex Starter</h1>
      <p>Panex built and loaded this extension.</p>
    </main>
    <script type="module" src="./popup.js"></script>
  </body>
</html>
`,
			perm: 0o644,
		},
		{
			relativePath: filepath.Join(defaultScaffoldSourceDir, "popup.js"),
			contents: `console.log("Panex starter popup ready");
`,
			perm: 0o644,
		},
	}

	for _, file := range files {
		outPath := filepath.Join(root, file.relativePath)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return scaffoldResult{}, fmt.Errorf("create scaffold directory %q: %w", filepath.Dir(outPath), err)
		}
		if err := os.WriteFile(outPath, []byte(file.contents), file.perm); err != nil {
			return scaffoldResult{}, fmt.Errorf("write scaffold file %q: %w", file.relativePath, err)
		}
	}

	return scaffoldResult{
		configPath: panexconfig.DefaultPath,
		sourceDir:  defaultScaffoldSourceDir,
		outDir:     defaultScaffoldOutDir,
	}, nil
}

func ensureWritableScaffoldTargets(force bool, configPath string, sourceDir string) error {
	for _, target := range []string{configPath, sourceDir} {
		_, err := os.Stat(target)
		switch {
		case err == nil:
			if force {
				continue
			}
			return fmt.Errorf(
				"refusing to overwrite existing scaffold path %q (rerun with --force to replace generated files)",
				target,
			)
		case os.IsNotExist(err):
			continue
		default:
			return fmt.Errorf("stat scaffold target %q: %w", target, err)
		}
	}

	return nil
}
