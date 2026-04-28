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

func runCoreInspect() error {
	code := cli.CmdInspect(projectDir())
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCorePlan() error {
	code := cli.CmdPlan(projectDir())
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCoreApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	force := fs.Bool("force", false, "Skip drift check")
	if err := fs.Parse(args); err != nil {
		return &cliError{code: 2, msg: fmt.Sprintf("invalid apply flags: %v", err)}
	}

	code := cli.CmdApply(projectDir(), cli.ApplyOptions{Force: *force})
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCoreTest() error {
	code := cli.CmdTest(projectDir())
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCoreVerify() error {
	code := cli.CmdVerify(projectDir())
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCorePackage(args []string) error {
	fs := flag.NewFlagSet("package", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ver := fs.String("version", "", "Package version (defaults to project version)")
	sourceDir := fs.String("source-dir", "", "Extension source directory")
	if err := fs.Parse(args); err != nil {
		return &cliError{code: 2, msg: fmt.Sprintf("invalid package flags: %v", err)}
	}

	code := cli.CmdPackage(projectDir(), cli.PackageOptions{
		Version:   *ver,
		SourceDir: *sourceDir,
	})
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCoreReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	runID := fs.String("run-id", "", "Specific run ID")
	if err := fs.Parse(args); err != nil {
		return &cliError{code: 2, msg: fmt.Sprintf("invalid report flags: %v", err)}
	}

	code := cli.CmdReport(projectDir(), *runID)
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runCoreResume(args []string) error {
	fs := flag.NewFlagSet("resume", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	runID := fs.String("run-id", "", "Run ID to resume")
	if err := fs.Parse(args); err != nil {
		return &cliError{code: 2, msg: fmt.Sprintf("invalid resume flags: %v", err)}
	}

	code := cli.CmdResume(projectDir(), *runID)
	if code != 0 {
		return &cliError{code: code, msg: ""}
	}
	return nil
}

func runMCP() error {
	srv := mcp.NewServer(projectDir())
	return srv.Run(context.Background())
}
