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
  panex [--cwd path] version
  panex [--cwd path] init [--force]
  panex [--cwd path] add-target <target>
  panex [--cwd path] inspect
  panex [--cwd path] plan
  panex [--cwd path] apply [--force]
  panex [--cwd path] dev [--config path/to/panex.toml] [--open]
  panex [--cwd path] test
  panex [--cwd path] verify
  panex [--cwd path] package [--version v0.1.0]
  panex [--cwd path] report [--run-id id]
  panex [--cwd path] resume [--run-id id]
  panex [--cwd path] doctor [--fix]
  panex [--cwd path] paths
  panex [--cwd path] mcp

Global flags:
  --cwd path  Override working directory for project resolution
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

type globalOptions struct {
	projectDir string
}

func (e *cliError) Error() string {
	return e.msg
}

func run(args []string, stdout io.Writer) error {
	opts, args, err := parseGlobalOptions(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return writeString(stdout, usageText)
		}
		return &cliError{
			code: 2,
			msg:  fmt.Sprintf("invalid global flags: %v", err),
		}
	}

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
		return runInitInProject(opts.projectDir, args[1:], stdout)
	case "add-target":
		return runCoreAddTargetInProject(opts.projectDir, args[1:])
	case "inspect":
		return runCoreInspectInProject(opts.projectDir)
	case "plan":
		return runCorePlanInProject(opts.projectDir)
	case "apply":
		return runCoreApplyInProject(opts.projectDir, args[1:])
	case "dev":
		return runDevInProject(opts.projectDir, args[1:], stdout)
	case "test":
		return runCoreTestInProject(opts.projectDir)
	case "verify":
		return runCoreVerifyInProject(opts.projectDir)
	case "package":
		return runCorePackageInProject(opts.projectDir, args[1:])
	case "report":
		return runCoreReportInProject(opts.projectDir, args[1:])
	case "resume":
		return runCoreResumeInProject(opts.projectDir, args[1:])
	case "doctor":
		return runDoctorInProject(opts.projectDir, stdout)
	case "paths":
		return runPathsInProject(opts.projectDir, stdout)
	case "mcp":
		return runMCPInProject(opts.projectDir)
	case "help", "-h", "--help":
		return writeString(stdout, usageText)
	default:
		return &cliError{
			code: 2,
			msg:  fmt.Sprintf("unknown command %q\n\n%s", args[0], usageText),
		}
	}
}

func runDevInProject(projectDir string, args []string, stdout io.Writer) error {
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

	cfg, inferred, err := loadDevConfig(projectDir, *configPath)
	if err != nil {
		return &cliError{
			code: 2,
			msg:  err.Error(),
		}
	}
	if inferred {
		if writeErr := writef(stdout, "no panex.toml found, using manifest.json in project directory\n"); writeErr != nil {
			return writeErr
		}
	}

	cfg, err = applyEnvironmentOverrides(cfg)
	if err != nil {
		return &cliError{
			code: 2,
			msg:  err.Error(),
		}
	}

	if isWSL() {
		for _, ext := range cfg.Extensions {
			absOut, _ := filepath.Abs(ext.OutDir)
			if !strings.HasPrefix(absOut, "/mnt/") {
				_ = writef(stdout, "warning: WSL detected — output directory %s is not on a Windows-mounted path.\nChrome cannot load extensions from Linux filesystem paths.\nRun 'panex doctor' for details, or set out_dir to a path under /mnt/.\n", absOut)
				break
			}
		}
	}

	if *openFlag {
		if openErr := openBrowser("chrome://extensions"); openErr != nil {
			_ = writef(stdout, "note: could not open browser: %v\n", openErr)
		}
	}

	return startDev(cfg, stdout)
}

func parseGlobalOptions(args []string) (globalOptions, []string, error) {
	fs := flag.NewFlagSet("panex", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cwd := fs.String("cwd", "", "Override working directory for project resolution")
	if err := fs.Parse(args); err != nil {
		return globalOptions{}, nil, err
	}

	projectDir, err := resolveProjectDir(*cwd)
	if err != nil {
		return globalOptions{}, nil, err
	}

	return globalOptions{projectDir: projectDir}, fs.Args(), nil
}

func resolveProjectDir(cwd string) (string, error) {
	if strings.TrimSpace(cwd) == "" {
		return projectDir(), nil
	}

	absDir, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve --cwd %q: %w", cwd, err)
	}

	info, err := os.Stat(absDir)
	if err != nil {
		return "", fmt.Errorf("stat --cwd %q: %w", cwd, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("--cwd %q is not a directory", cwd)
	}

	return absDir, nil
}

func loadDevConfig(projectDir string, configPath string) (panexconfig.Config, bool, error) {
	resolvedConfigPath := configPath
	if !filepath.IsAbs(resolvedConfigPath) {
		resolvedConfigPath = filepath.Join(projectDir, configPath)
	}

	cfg, err := panexconfig.Load(resolvedConfigPath)
	if err != nil {
		if errors.Is(err, panexconfig.ErrConfigFileNotFound) && configPath == panexconfig.DefaultPath {
			inferred, inferErr := panexconfig.Infer(projectDir)
			if inferErr != nil {
				return panexconfig.Config{}, false, fmt.Errorf(
					"failed to load config %q: %v\n\nRun `panex init` in the project directory to scaffold a starter config and extension.",
					configPath,
					err,
				)
			}
			return resolveConfigPaths(inferred, projectDir), true, nil
		}
		return panexconfig.Config{}, false, fmt.Errorf("failed to load config %q: %v", configPath, err)
	}

	return resolveConfigPaths(cfg, filepath.Dir(resolvedConfigPath)), false, nil
}

func resolveConfigPaths(cfg panexconfig.Config, root string) panexconfig.Config {
	resolved := cfg
	resolved.Extensions = make([]panexconfig.Extension, 0, len(cfg.Extensions))
	for _, ext := range cfg.Extensions {
		resolved.Extensions = append(resolved.Extensions, panexconfig.Extension{
			ID:        ext.ID,
			SourceDir: resolveConfigPath(root, ext.SourceDir),
			OutDir:    resolveConfigPath(root, ext.OutDir),
		})
	}
	if len(resolved.Extensions) > 0 {
		resolved.Extension = resolved.Extensions[0]
	}
	resolved.Server.EventStorePath = resolveConfigPath(root, cfg.Server.EventStorePath)
	return resolved
}

func resolveConfigPath(root string, path string) string {
	if strings.TrimSpace(path) == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
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

	if len(cfg.Extensions) > 1 {
		_ = writef(os.Stderr, "warning: multi-extension mode is experimental — storage and event isolation between extensions is not complete yet\n")
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
		dirty        *atomic.Bool
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

		var dirtyFlag atomic.Bool
		changeEvents := make(chan daemon.FileChangeEvent, 64)
		watcher, err := newFileWatcher(
			target.SourceDir,
			daemon.DefaultWatchDebounce,
			func(event daemon.FileChangeEvent) {
				select {
				case changeEvents <- event:
				default:
					dirtyFlag.Store(true)
					_ = writef(os.Stderr, "warning: file change event dropped (build in progress), will rebuild after current build\n")
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
			dirty:        &dirtyFlag,
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
			runErrCh <- runBuildLoop(ctx, runtime.target, runtime.builder, server, runtime.changeEvents, runtime.dirty)
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
	dirty *atomic.Bool,
) error {
	runBuild := func(changedPaths []string, reloadReason string) {
		result, err := builder.Build(ctx, changedPaths)
		if err != nil {
			result = build.Result{
				BuildID:         fmt.Sprintf("build-failed-%d", atomic.AddUint64(&buildFailureSeq, 1)),
				Success:         false,
				DurationMS:      0,
				TriggeringFiles: changedPaths,
				Errors:          []string{err.Error()},
			}
		}

		if broadcastErr := server.Broadcast(ctx,
			protocol.NewBuildComplete(
				protocol.Source{
					Role: protocol.SourceDaemon,
					ID:   "daemon-1",
				},
				protocol.BuildComplete{
					BuildID:         result.BuildID,
					Success:         result.Success,
					DurationMS:      result.DurationMS,
					ExtensionID:     target.ID,
					TriggeringFiles: result.TriggeringFiles,
					Diagnostics:     result.Errors,
				},
			),
		); broadcastErr != nil && !errors.Is(ctx.Err(), context.Canceled) {
			// Broadcast failures are non-fatal to the daemon loop; disconnected clients should not stop builds.
			_ = writef(os.Stderr, "broadcast build.complete failed: %v\n", broadcastErr)
		}

		if result.Success {
			// Verify output directory contains manifest.json before signaling reload.
			manifestPath := filepath.Join(target.OutDir, "manifest.json")
			if _, statErr := os.Stat(manifestPath); statErr != nil {
				_ = writef(os.Stderr, "warning: build succeeded but %s not found, skipping reload\n", manifestPath)
			} else if broadcastErr := server.Broadcast(ctx,
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
	if dirty.CompareAndSwap(true, false) {
		runBuild(nil, "missed-changes")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-changeEvents:
			if !ok {
				return nil
			}

			runBuild(event.Paths, "build.complete")
			if dirty.CompareAndSwap(true, false) {
				runBuild(nil, "missed-changes")
			}
		}
	}
}
