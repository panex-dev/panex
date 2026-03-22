package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/panex-dev/panex/internal/build"
	panexconfig "github.com/panex-dev/panex/internal/config"
	"github.com/panex-dev/panex/internal/daemon"
	"github.com/panex-dev/panex/internal/protocol"
)

const usageText = `panex - development runtime for Chrome extensions

Usage:
  panex version
  panex init [--force]
  panex dev [--config path/to/panex.toml] [--open]
  panex doctor
  panex paths
`

// This is overridden in release builds via -ldflags "-X main.version=<semver>".
var version = "dev"
var lookupEnv = os.LookupEnv

var startDev = startDevServer
var buildFailureSeq uint64
var newWebSocketServer = func(cfg daemon.WebSocketConfig) (devRuntimeServer, error) {
	return daemon.NewWebSocketServer(cfg)
}
var newEsbuildBuilder = func(sourceDir, outDir string, opts ...build.Option) (buildRunner, error) {
	return build.NewEsbuildBuilder(sourceDir, outDir, opts...)
}
var newFileWatcher = func(
	root string,
	debounce time.Duration,
	emit func(daemon.FileChangeEvent),
) (runtimeRunner, error) {
	return daemon.NewFileWatcher(root, debounce, emit)
}
var newSignalContext = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

type buildRunner interface {
	Build(ctx context.Context, changedPaths []string) (build.Result, error)
}

type runtimeRunner interface {
	Run(ctx context.Context) error
}

type extensionTarget struct {
	ID        string
	SourceDir string
	OutDir    string
}

type envelopeBroadcaster interface {
	Broadcast(ctx context.Context, message protocol.Envelope) error
}

type devRuntimeServer interface {
	runtimeRunner
	envelopeBroadcaster
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
	case "init":
		return runInit(args[1:], stdout)
	case "dev":
		return runDev(args[1:], stdout)
	case "doctor":
		return runDoctor(stdout)
	case "paths":
		return runPaths(stdout)
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
	openFlag := fs.Bool("open", false, "Open chrome://extensions in the default browser")
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
		if errors.Is(err, panexconfig.ErrConfigFileNotFound) && *configPath == panexconfig.DefaultPath {
			inferred, inferErr := panexconfig.Infer(".")
			if inferErr != nil {
				return &cliError{
					code: 2,
					msg:  fmt.Sprintf("failed to load config %q: %v\n\nRun `panex init` in the current directory to scaffold a starter config and extension.", *configPath, err),
				}
			}
			cfg = inferred
			if writeErr := writef(stdout, "no panex.toml found, using manifest.json in current directory\n"); writeErr != nil {
				return writeErr
			}
		} else {
			return &cliError{
				code: 2,
				msg:  fmt.Sprintf("failed to load config %q: %v", *configPath, err),
			}
		}
	}
	cfg, err = applyEnvironmentOverrides(cfg)
	if err != nil {
		return &cliError{
			code: 2,
			msg:  err.Error(),
		}
	}

	if *openFlag {
		if openErr := openBrowser("chrome://extensions"); openErr != nil {
			_ = writef(stdout, "note: could not open browser: %v\n", openErr)
		}
	}

	return startDev(cfg, stdout)
}

func applyEnvironmentOverrides(cfg panexconfig.Config) (panexconfig.Config, error) {
	authToken, ok := lookupEnv("PANEX_AUTH_TOKEN")
	if !ok {
		return cfg, nil
	}

	authToken = strings.TrimSpace(authToken)
	if authToken == "" {
		return panexconfig.Config{}, errors.New("PANEX_AUTH_TOKEN must not be empty when set")
	}

	cfg.Server.AuthToken = authToken
	return cfg, nil
}

func writef(w io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(w, format, args...)
	return err
}

func writeString(w io.Writer, value string) error {
	_, err := io.WriteString(w, value)
	return err
}

func writeStartupGuide(w io.Writer, extensions []panexconfig.Extension) error {
	for _, ext := range extensions {
		absOutDir, err := filepath.Abs(ext.OutDir)
		if err != nil {
			absOutDir = ext.OutDir
		}
		if len(extensions) == 1 {
			if err := writef(w, "out_dir=%s\n", absOutDir); err != nil {
				return err
			}
		} else {
			if err := writef(w, "out_dir[%s]=%s\n", ext.ID, absOutDir); err != nil {
				return err
			}
		}
	}

	if len(extensions) == 1 {
		absOutDir, _ := filepath.Abs(extensions[0].OutDir)
		return writef(w, "\nLoad your extension in Chrome:\n  1. Open chrome://extensions\n  2. Enable \"Developer mode\"\n  3. Click \"Load unpacked\" → select %s\n", absOutDir)
	}
	return nil
}

func startDevServer(cfg panexconfig.Config, stdout io.Writer) error {
	server, err := newWebSocketServer(daemon.WebSocketConfig{
		Port:           cfg.Server.Port,
		BindAddress:    cfg.Server.BindAddress,
		AuthToken:      cfg.Server.AuthToken,
		EventStorePath: cfg.Server.EventStorePath,
		ServerVersion:  version,
		DaemonID:       "daemon-1",
	})
	if err != nil {
		return err
	}

	if err := writef(stdout, "panex dev\nws_url=ws://%s:%d/ws\n", cfg.Server.BindAddress, cfg.Server.Port); err != nil {
		return err
	}
	if err := writeStartupGuide(stdout, cfg.Extensions); err != nil {
		return err
	}

	ctx, stop := newSignalContext()
	defer stop()

	targets := make([]extensionTarget, 0, len(cfg.Extensions))
	for _, extension := range cfg.Extensions {
		targets = append(targets, extensionTarget{
			ID:        extension.ID,
			SourceDir: extension.SourceDir,
			OutDir:    extension.OutDir,
		})
	}

	type extensionRuntime struct {
		target       extensionTarget
		builder      buildRunner
		watcher      runtimeRunner
		changeEvents chan daemon.FileChangeEvent
	}

	runtimes := make([]extensionRuntime, 0, len(targets))
	daemonURL := fmt.Sprintf("ws://%s:%d/ws", cfg.Server.BindAddress, cfg.Server.Port)
	for _, target := range targets {
		builderOptions := []build.Option{}
		if injection, ok := build.AutoDetectChromeSimInjection(
			target.SourceDir,
			daemonURL,
			cfg.Server.AuthToken,
			target.ID,
		); ok {
			builderOptions = append(builderOptions, build.WithChromeSimInjection(injection))
		}

		builder, err := newEsbuildBuilder(target.SourceDir, target.OutDir, builderOptions...)
		if err != nil {
			return fmt.Errorf("configure esbuild for extension %q: %w", target.ID, err)
		}

		changeEvents := make(chan daemon.FileChangeEvent, 64)
		watcher, err := newFileWatcher(
			target.SourceDir,
			daemon.DefaultWatchDebounce,
			func(event daemon.FileChangeEvent) {
				select {
				case changeEvents <- event:
				default:
				}
			},
		)
		if err != nil {
			return fmt.Errorf("configure file watcher for extension %q: %w", target.ID, err)
		}

		runtimes = append(runtimes, extensionRuntime{
			target:       target,
			builder:      builder,
			watcher:      watcher,
			changeEvents: changeEvents,
		})
	}

	runErrCh := make(chan error, 1+(len(runtimes)*2))

	go func() {
		runErrCh <- server.Run(ctx)
	}()
	for _, runtime := range runtimes {
		go func(runtime extensionRuntime) {
			runErrCh <- runtime.watcher.Run(ctx)
		}(runtime)
		go func(runtime extensionRuntime) {
			runErrCh <- runBuildLoop(ctx, runtime.target, runtime.builder, server, runtime.changeEvents)
		}(runtime)
	}

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
	target extensionTarget,
	builder buildRunner,
	server envelopeBroadcaster,
	changeEvents <-chan daemon.FileChangeEvent,
) error {
	runBuild := func(changedPaths []string, reloadReason string) {
		result, err := builder.Build(ctx, changedPaths)
		if err != nil {
			result = build.Result{
				BuildID:      fmt.Sprintf("build-failed-%d", atomic.AddUint64(&buildFailureSeq, 1)),
				Success:      false,
				DurationMS:   0,
				ChangedFiles: changedPaths,
				Errors:       []string{err.Error()},
			}
		}

		if broadcastErr := server.Broadcast(ctx,
			protocol.NewBuildComplete(
				protocol.Source{
					Role: protocol.SourceDaemon,
					ID:   "daemon-1",
				},
				protocol.BuildComplete{
					BuildID:      result.BuildID,
					Success:      result.Success,
					DurationMS:   result.DurationMS,
					ExtensionID:  target.ID,
					ChangedFiles: result.ChangedFiles,
				},
			),
		); broadcastErr != nil && !errors.Is(ctx.Err(), context.Canceled) {
			// Broadcast failures are non-fatal to the daemon loop; disconnected clients should not stop builds.
			_ = writef(os.Stderr, "broadcast build.complete failed: %v\n", broadcastErr)
		}

		if result.Success {
			// Reload commands are emitted only after successful builds so clients can treat reload as a
			// strong signal that new artifacts exist in the output directory.
			if broadcastErr := server.Broadcast(ctx,
				protocol.NewCommandReload(
					protocol.Source{
						Role: protocol.SourceDaemon,
						ID:   "daemon-1",
					},
					protocol.CommandReload{
						Reason:      reloadReason,
						BuildID:     result.BuildID,
						ExtensionID: target.ID,
					},
				),
			); broadcastErr != nil && !errors.Is(ctx.Err(), context.Canceled) {
				_ = writef(os.Stderr, "broadcast command.reload failed: %v\n", broadcastErr)
			}
		}

		// Keep diagnostics available in daemon logs until inspector/event-store steps are implemented.
		for _, diagnostic := range result.Errors {
			_ = writef(os.Stderr, "build %s error: %s\n", result.BuildID, diagnostic)
		}
	}

	runBuild(nil, "startup")

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-changeEvents:
			if !ok {
				return nil
			}

			runBuild(event.Paths, "build.complete")
		}
	}
}
