package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/panex-dev/panex/internal/cli"
	"github.com/panex-dev/panex/internal/mcp"
)

func projectDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func runCoreInspectInProject(rootDir string) error {
	code := cli.CmdInspect(rootDir)
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCoreAddTargetInProject(rootDir string, args []string, stdout io.Writer, jsonOutput bool) error {
	if len(args) != 1 {
		if jsonOutput {
			return writeJSONCommandError(stdout, 2, "add-target", "add-target requires exactly one target argument", nil, nil)
		}
		return &cliError{code: 2, msg: "add-target requires exactly one target argument"}
	}

	code := cli.CmdAddTarget(rootDir, args[0])
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCorePlanInProject(rootDir string) error {
	code := cli.CmdPlan(rootDir)
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCoreApplyInProject(rootDir string, args []string, stdout io.Writer, jsonOutput bool) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	force := fs.Bool("force", false, "Skip drift check")
	if err := fs.Parse(args); err != nil {
		if jsonOutput {
			return writeJSONCommandError(stdout, 2, "apply", fmt.Sprintf("invalid apply flags: %v", err), nil, nil)
		}
		return &cliError{code: 2, msg: fmt.Sprintf("invalid apply flags: %v", err)}
	}

	code := cli.CmdApply(rootDir, cli.ApplyOptions{Force: *force})
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCoreTestInProject(rootDir string) error {
	code := cli.CmdTest(rootDir)
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCoreVerifyInProject(rootDir string) error {
	code := cli.CmdVerify(rootDir)
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCorePackageInProject(rootDir string, args []string, stdout io.Writer, jsonOutput bool) error {
	fs := flag.NewFlagSet("package", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ver := fs.String("version", "", "Package version (defaults to project version)")
	sourceDir := fs.String("source-dir", "", "Extension source directory")
	if err := fs.Parse(args); err != nil {
		if jsonOutput {
			return writeJSONCommandError(stdout, 2, "package", fmt.Sprintf("invalid package flags: %v", err), nil, nil)
		}
		return &cliError{code: 2, msg: fmt.Sprintf("invalid package flags: %v", err)}
	}

	code := cli.CmdPackage(rootDir, cli.PackageOptions{
		Version:   *ver,
		SourceDir: *sourceDir,
	})
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCoreReportInProject(rootDir string, args []string, stdout io.Writer, jsonOutput bool) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	runID := fs.String("run-id", "", "Specific run ID")
	if err := fs.Parse(args); err != nil {
		if jsonOutput {
			return writeJSONCommandError(stdout, 2, "report", fmt.Sprintf("invalid report flags: %v", err), nil, nil)
		}
		return &cliError{code: 2, msg: fmt.Sprintf("invalid report flags: %v", err)}
	}

	code := cli.CmdReport(rootDir, *runID)
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCoreResumeInProject(rootDir string, args []string, stdout io.Writer, jsonOutput bool) error {
	fs := flag.NewFlagSet("resume", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	runID := fs.String("run-id", "", "Run ID to resume")
	if err := fs.Parse(args); err != nil {
		if jsonOutput {
			return writeJSONCommandError(stdout, 2, "resume", fmt.Sprintf("invalid resume flags: %v", err), nil, nil)
		}
		return &cliError{code: 2, msg: fmt.Sprintf("invalid resume flags: %v", err)}
	}

	code := cli.CmdResume(rootDir, *runID)
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runMCPInProject(rootDir string) error {
	srv := mcp.NewServer(rootDir)
	return srv.Run(context.Background())
}
