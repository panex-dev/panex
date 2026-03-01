package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/panex-dev/panex/internal/build"
	panexconfig "github.com/panex-dev/panex/internal/config"
	"github.com/panex-dev/panex/internal/daemon"
	"github.com/panex-dev/panex/internal/protocol"
)

const usageText = `panex - development runtime for Chrome extensions

Usage:
  panex version
  panex dev [--config path/to/panex.toml]
`

// This is overridden in release builds via -ldflags "-X main.version=<semver>".
var version = "dev"

var startDev = startDevServer
var buildFailureSeq uint64

type buildRunner interface {
	Build(ctx context.Context, changedPaths []string) (build.Result, error)
}

type envelopeBroadcaster interface {
	Broadcast(message protocol.Envelope) error
}

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

	builder, err := build.NewEsbuildBuilder(cfg.Extension.SourceDir, cfg.Extension.OutDir)
	if err != nil {
		return fmt.Errorf("configure esbuild: %w", err)
	}

	changeEvents := make(chan daemon.FileChangeEvent, 64)
	watcher, err := daemon.NewFileWatcher(
		cfg.Extension.SourceDir,
		daemon.DefaultWatchDebounce,
		func(event daemon.FileChangeEvent) {
			select {
			case changeEvents <- event:
			default:
			}
		},
	)
	if err != nil {
		return fmt.Errorf("configure file watcher: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runErrCh := make(chan error, 3)

	go func() {
		runErrCh <- server.Run(ctx)
	}()
	go func() {
		runErrCh <- watcher.Run(ctx)
	}()
	go func() {
		runErrCh <- runBuildLoop(ctx, builder, server, changeEvents)
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case runErr := <-runErrCh:
			if runErr == nil {
				continue
			}

			stop()
			return runErr
		}
	}
}

func runBuildLoop(
	ctx context.Context,
	builder buildRunner,
	server envelopeBroadcaster,
	changeEvents <-chan daemon.FileChangeEvent,
) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-changeEvents:
			result, err := builder.Build(ctx, event.Paths)
			if err != nil {
				result = build.Result{
					BuildID:      fmt.Sprintf("build-failed-%d", atomic.AddUint64(&buildFailureSeq, 1)),
					Success:      false,
					DurationMS:   0,
					ChangedFiles: event.Paths,
					Errors:       []string{err.Error()},
				}
			}

			if broadcastErr := server.Broadcast(
				protocol.NewBuildComplete(
					protocol.Source{
						Role: protocol.SourceDaemon,
						ID:   "daemon-1",
					},
					protocol.BuildComplete{
						BuildID:      result.BuildID,
						Success:      result.Success,
						DurationMS:   result.DurationMS,
						ChangedFiles: result.ChangedFiles,
					},
				),
			); broadcastErr != nil && !errors.Is(ctx.Err(), context.Canceled) {
				// Broadcast failures are non-fatal to the daemon loop; disconnected clients should not stop builds.
				_ = writef(os.Stderr, "broadcast build.complete failed: %v\n", broadcastErr)
			}

			// Keep diagnostics available in daemon logs until inspector/event-store steps are implemented.
			for _, diagnostic := range result.Errors {
				_ = writef(os.Stderr, "build %s error: %s\n", result.BuildID, diagnostic)
			}
		}
	}
}
