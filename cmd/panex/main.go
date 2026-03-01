package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	panexconfig "github.com/panex-dev/panex/internal/config"
	"github.com/panex-dev/panex/internal/daemon"
)

const usageText = `panex - development runtime for Chrome extensions

Usage:
  panex version
  panex dev [--config path/to/panex.toml]
`

// This is overridden in release builds via -ldflags "-X main.version=<semver>".
var version = "dev"

var startDev = startDevServer

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		var cliErr *cliError
		if errors.As(err, &cliErr) {
			_, _ = fmt.Fprintln(os.Stderr, cliErr.msg)
			os.Exit(cliErr.code)
		}

		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type cliError struct {
	code int
	msg  string
}

func (e *cliError) Error() string {
	return e.msg
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return &cliError{
			code: 2,
			msg:  usageText,
		}
	}

	switch args[0] {
	case "version":
		return writef(stdout, "panex %s\n", version)
	case "dev":
		return runDev(args[1:], stdout)
	case "help", "-h", "--help":
		return writeString(stdout, usageText)
	default:
		return &cliError{
			code: 2,
			msg:  fmt.Sprintf("unknown command %q\n\n%s", args[0], usageText),
		}
	}
}

func runDev(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	// Suppress default flag package output so all user-facing errors stay in our format.
	fs.SetOutput(io.Discard)

	configPath := fs.String("config", panexconfig.DefaultPath, "Path to panex configuration file")
	if err := fs.Parse(args); err != nil {
		return &cliError{
			code: 2,
			msg:  fmt.Sprintf("invalid dev flags: %v", err),
		}
	}

	if fs.NArg() > 0 {
		return &cliError{
			code: 2,
			msg:  fmt.Sprintf("unexpected arguments for dev: %v", fs.Args()),
		}
	}

	cfg, err := panexconfig.Load(*configPath)
	if err != nil {
		return &cliError{
			code: 2,
			msg:  fmt.Sprintf("failed to load config %q: %v", *configPath, err),
		}
	}

	return startDev(cfg, stdout)
}

func writef(w io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(w, format, args...)
	return err
}

func writeString(w io.Writer, value string) error {
	_, err := io.WriteString(w, value)
	return err
}

func startDevServer(cfg panexconfig.Config, stdout io.Writer) error {
	server, err := daemon.NewWebSocketServer(daemon.WebSocketConfig{
		Port:          cfg.Server.Port,
		AuthToken:     cfg.Server.AuthToken,
		ServerVersion: version,
		DaemonID:      "daemon-1",
	})
	if err != nil {
		return err
	}

	if err := writef(stdout, "panex dev\nws_url=ws://127.0.0.1:%d/ws\n", cfg.Server.Port); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return server.Run(ctx)
}
