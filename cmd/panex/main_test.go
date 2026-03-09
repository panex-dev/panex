package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panex-dev/panex/internal/build"
	panexconfig "github.com/panex-dev/panex/internal/config"
	"github.com/panex-dev/panex/internal/daemon"
	"github.com/panex-dev/panex/internal/protocol"
)

func TestRunVersion(t *testing.T) {
	var out bytes.Buffer

	err := run([]string{"version"}, &out)
	if err != nil {
		t.Fatalf("run(version) returned error: %v", err)
	}

	const want = "panex dev\n"
	if out.String() != want {
		t.Fatalf("unexpected version output: got %q, want %q", out.String(), want)
	}
}

func TestRunHelpAliases(t *testing.T) {
	testCases := []struct {
		name string
		args []string
	}{
		{name: "help command", args: []string{"help"}},
		{name: "short help flag", args: []string{"-h"}},
		{name: "long help flag", args: []string{"--help"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			err := run(tc.args, &out)
			if err != nil {
				t.Fatalf("run(%v) returned error: %v", tc.args, err)
			}

			if out.String() != usageText {
				t.Fatalf("unexpected help output: got %q, want %q", out.String(), usageText)
			}
		})
	}
}

func TestRunNoArgsReturnsUsageError(t *testing.T) {
	var out bytes.Buffer

	err := run(nil, &out)
	cliErr := requireCLIError(t, err)

	if cliErr.code != 2 {
		t.Fatalf("unexpected error code: got %d, want 2", cliErr.code)
	}
	if cliErr.msg != usageText {
		t.Fatalf("unexpected usage message: got %q, want %q", cliErr.msg, usageText)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
}

func TestRunUnknownCommandReturnsUsageError(t *testing.T) {
	var out bytes.Buffer

	err := run([]string{"nope"}, &out)
	cliErr := requireCLIError(t, err)

	if cliErr.code != 2 {
		t.Fatalf("unexpected error code: got %d, want 2", cliErr.code)
	}
	if !strings.Contains(cliErr.msg, `unknown command "nope"`) {
		t.Fatalf("missing unknown command message: %q", cliErr.msg)
	}
	if !strings.Contains(cliErr.msg, "Usage:") {
		t.Fatalf("missing usage text in error: %q", cliErr.msg)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
}

func TestRunDevDefaultConfig(t *testing.T) {
	tempDir := t.TempDir()
	writePanexConfig(t, filepath.Join(tempDir, "panex.toml"), `
[extension]
source_dir = "./src"
out_dir = "./dist"

[server]
port = 3000
auth_token = "token-123"
`)

	var out bytes.Buffer
	var captured panexconfig.Config
	withStubbedStartDev(t, func(cfg panexconfig.Config, stdout io.Writer) error {
		captured = cfg
		_, err := io.WriteString(stdout, "dev started\n")
		return err
	})

	err := withWorkingDir(tempDir, func() error {
		return run([]string{"dev"}, &out)
	})
	if err != nil {
		t.Fatalf("run(dev) returned error: %v", err)
	}

	const want = "dev started\n"
	if out.String() != want {
		t.Fatalf("unexpected dev output: got %q, want %q", out.String(), want)
	}
	if captured.Server.Port != 3000 {
		t.Fatalf("unexpected server port: got %d", captured.Server.Port)
	}
	if captured.Server.AuthToken != "token-123" {
		t.Fatalf("unexpected auth token: got %q", captured.Server.AuthToken)
	}
}

func TestRunDevCustomConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "custom.toml")
	writePanexConfig(t, configPath, `
[extension]
source_dir = "./extension-src"
out_dir = "./build"

[server]
port = 4317
auth_token = "custom-token"
`)

	var out bytes.Buffer
	var captured panexconfig.Config
	withStubbedStartDev(t, func(cfg panexconfig.Config, stdout io.Writer) error {
		captured = cfg
		_, err := io.WriteString(stdout, "dev started\n")
		return err
	})

	err := run([]string{"dev", "--config", configPath}, &out)
	if err != nil {
		t.Fatalf("run(dev --config) returned error: %v", err)
	}

	want := "dev started\n"
	if out.String() != want {
		t.Fatalf("unexpected dev output: got %q, want %q", out.String(), want)
	}
	if captured.Extension.SourceDir != "./extension-src" {
		t.Fatalf("unexpected source_dir: got %q", captured.Extension.SourceDir)
	}
	if captured.Extension.OutDir != "./build" {
		t.Fatalf("unexpected out_dir: got %q", captured.Extension.OutDir)
	}
	if captured.Server.Port != 4317 {
		t.Fatalf("unexpected port: got %d", captured.Server.Port)
	}
	if captured.Server.AuthToken != "custom-token" {
		t.Fatalf("unexpected auth token: got %q", captured.Server.AuthToken)
	}
}

func TestRunDevEnvAuthTokenOverride(t *testing.T) {
	tempDir := t.TempDir()
	writePanexConfig(t, filepath.Join(tempDir, "panex.toml"), `
[extension]
source_dir = "./src"
out_dir = "./dist"

[server]
port = 3000
auth_token = "config-token"
`)

	withStubbedLookupEnv(t, func(key string) (string, bool) {
		if key != "PANEX_AUTH_TOKEN" {
			return "", false
		}
		return "  env-token  ", true
	})

	var out bytes.Buffer
	var captured panexconfig.Config
	withStubbedStartDev(t, func(cfg panexconfig.Config, stdout io.Writer) error {
		captured = cfg
		_, err := io.WriteString(stdout, "dev started\n")
		return err
	})

	err := withWorkingDir(tempDir, func() error {
		return run([]string{"dev"}, &out)
	})
	if err != nil {
		t.Fatalf("run(dev) returned error: %v", err)
	}

	if captured.Server.AuthToken != "env-token" {
		t.Fatalf("unexpected auth token: got %q, want %q", captured.Server.AuthToken, "env-token")
	}
}

func TestRunDevRejectsEmptyEnvAuthTokenOverride(t *testing.T) {
	tempDir := t.TempDir()
	writePanexConfig(t, filepath.Join(tempDir, "panex.toml"), `
[extension]
source_dir = "./src"
out_dir = "./dist"

[server]
port = 3000
auth_token = "config-token"
`)

	withStubbedLookupEnv(t, func(key string) (string, bool) {
		if key != "PANEX_AUTH_TOKEN" {
			return "", false
		}
		return "   ", true
	})

	var out bytes.Buffer
	err := withWorkingDir(tempDir, func() error {
		return run([]string{"dev"}, &out)
	})
	cliErr := requireCLIError(t, err)

	if cliErr.code != 2 {
		t.Fatalf("unexpected error code: got %d, want 2", cliErr.code)
	}
	if !strings.Contains(cliErr.msg, "PANEX_AUTH_TOKEN must not be empty when set") {
		t.Fatalf("unexpected error message: %q", cliErr.msg)
	}
}

func TestRunDevUnexpectedPositionalArg(t *testing.T) {
	var out bytes.Buffer

	err := run([]string{"dev", "extra"}, &out)
	cliErr := requireCLIError(t, err)

	if cliErr.code != 2 {
		t.Fatalf("unexpected error code: got %d, want 2", cliErr.code)
	}
	if !strings.Contains(cliErr.msg, "unexpected arguments for dev") {
		t.Fatalf("missing positional-arg validation error: %q", cliErr.msg)
	}
}

func TestRunDevInvalidFlag(t *testing.T) {
	var out bytes.Buffer

	err := run([]string{"dev", "--bad-flag"}, &out)
	cliErr := requireCLIError(t, err)

	if cliErr.code != 2 {
		t.Fatalf("unexpected error code: got %d, want 2", cliErr.code)
	}
	if !strings.Contains(cliErr.msg, "invalid dev flags") {
		t.Fatalf("missing invalid-flag message: %q", cliErr.msg)
	}
}

func TestRunDevMissingConfig(t *testing.T) {
	var out bytes.Buffer

	err := run([]string{"dev", "--config", filepath.Join(t.TempDir(), "missing.toml")}, &out)
	cliErr := requireCLIError(t, err)

	if cliErr.code != 2 {
		t.Fatalf("unexpected error code: got %d, want 2", cliErr.code)
	}
	if !strings.Contains(cliErr.msg, "failed to load config") {
		t.Fatalf("missing load failure message: %q", cliErr.msg)
	}
	if !strings.Contains(cliErr.msg, "config file not found") {
		t.Fatalf("missing file-not-found detail: %q", cliErr.msg)
	}
}

func TestRunWriteFailurePropagates(t *testing.T) {
	err := run([]string{"version"}, failingWriter{})
	if err == nil {
		t.Fatal("expected write failure error, got nil")
	}

	var cliErr *cliError
	if errors.As(err, &cliErr) {
		t.Fatalf("expected raw write error, got cliError: %+v", cliErr)
	}
}

func TestCLIErrorErrorReturnsMessage(t *testing.T) {
	err := (&cliError{msg: "boom"}).Error()
	if err != "boom" {
		t.Fatalf("unexpected error string: got %q, want %q", err, "boom")
	}
}

func TestStartDevServerCoordinatesStartupLifecycle(t *testing.T) {
	cfg := newCLIConfigFixture(t)

	signalCtx, signalCancel := context.WithCancel(context.Background())
	t.Cleanup(signalCancel)

	var serverCfg daemon.WebSocketConfig
	fakeServer := &fakeDevServer{
		fakeBroadcaster: &fakeBroadcaster{},
		run: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
	}
	withStubbedNewWebSocketServer(t, func(cfg daemon.WebSocketConfig) (devRuntimeServer, error) {
		serverCfg = cfg
		return fakeServer, nil
	})

	var builderSourceDir string
	var builderOutDir string
	var builderOptionCount int
	var buildCalls [][]string
	withStubbedNewEsbuildBuilder(t, func(sourceDir, outDir string, opts ...build.Option) (buildRunner, error) {
		builderSourceDir = sourceDir
		builderOutDir = outDir
		builderOptionCount = len(opts)

		return fakeBuildRunner{
			build: func(_ context.Context, changedPaths []string) (build.Result, error) {
				buildCalls = append(buildCalls, append([]string(nil), changedPaths...))
				signalCancel()
				return build.Result{
					BuildID:      "build-startup",
					Success:      true,
					DurationMS:   12,
					ChangedFiles: changedPaths,
				}, nil
			},
		}, nil
	})

	var watchRoot string
	var watchDebounce time.Duration
	var watchEmit func(daemon.FileChangeEvent)
	withStubbedNewFileWatcher(t, func(
		root string,
		debounce time.Duration,
		emit func(daemon.FileChangeEvent),
	) (runtimeRunner, error) {
		watchRoot = root
		watchDebounce = debounce
		watchEmit = emit
		return fakeRunComponent{
			run: func(ctx context.Context) error {
				<-ctx.Done()
				return nil
			},
		}, nil
	})

	withStubbedSignalContext(t, func() (context.Context, context.CancelFunc) {
		return signalCtx, signalCancel
	})

	var out bytes.Buffer
	if err := startDevServer(cfg, &out); err != nil {
		t.Fatalf("startDevServer() returned error: %v", err)
	}

	const wantBanner = "panex dev\nws_url=ws://127.0.0.1:4317/ws\n"
	if out.String() != wantBanner {
		t.Fatalf("unexpected startup banner: got %q, want %q", out.String(), wantBanner)
	}
	if serverCfg.Port != 4317 {
		t.Fatalf("unexpected websocket port: got %d, want %d", serverCfg.Port, 4317)
	}
	if serverCfg.AuthToken != "dev-token" {
		t.Fatalf("unexpected auth token: got %q, want %q", serverCfg.AuthToken, "dev-token")
	}
	if serverCfg.EventStorePath != cfg.Server.EventStorePath {
		t.Fatalf("unexpected event store path: got %q, want %q", serverCfg.EventStorePath, cfg.Server.EventStorePath)
	}
	if builderSourceDir != cfg.Extension.SourceDir {
		t.Fatalf("unexpected builder source dir: got %q, want %q", builderSourceDir, cfg.Extension.SourceDir)
	}
	if builderOutDir != cfg.Extension.OutDir {
		t.Fatalf("unexpected builder out dir: got %q, want %q", builderOutDir, cfg.Extension.OutDir)
	}
	if builderOptionCount != 0 {
		t.Fatalf("expected no build options for empty source dir, got %d", builderOptionCount)
	}
	if len(buildCalls) != 1 {
		t.Fatalf("expected one startup build, got %d", len(buildCalls))
	}
	if len(buildCalls[0]) != 0 {
		t.Fatalf("expected startup build with no changed paths, got %v", buildCalls[0])
	}
	if watchRoot != cfg.Extension.SourceDir {
		t.Fatalf("unexpected watch root: got %q, want %q", watchRoot, cfg.Extension.SourceDir)
	}
	if watchDebounce != daemon.DefaultWatchDebounce {
		t.Fatalf("unexpected watch debounce: got %s, want %s", watchDebounce, daemon.DefaultWatchDebounce)
	}
	if watchEmit == nil {
		t.Fatal("expected file watcher emit callback")
	}

	startupBuildEvent := waitForBroadcast(t, fakeServer.fakeBroadcaster, 2*time.Second)
	if startupBuildEvent.Name != protocol.MessageBuildComplete {
		t.Fatalf("unexpected startup message name: got %q, want %q", startupBuildEvent.Name, protocol.MessageBuildComplete)
	}
	startupReloadEvent := waitForBroadcast(t, fakeServer.fakeBroadcaster, 2*time.Second)
	if startupReloadEvent.Name != protocol.MessageCommandReload {
		t.Fatalf("unexpected startup reload name: got %q, want %q", startupReloadEvent.Name, protocol.MessageCommandReload)
	}
}

func TestStartDevServerReturnsBuilderConfigurationError(t *testing.T) {
	cfg := newCLIConfigFixture(t)

	withStubbedNewWebSocketServer(t, func(cfg daemon.WebSocketConfig) (devRuntimeServer, error) {
		return &fakeDevServer{fakeBroadcaster: &fakeBroadcaster{}}, nil
	})
	withStubbedNewEsbuildBuilder(t, func(string, string, ...build.Option) (buildRunner, error) {
		return nil, errors.New("builder boom")
	})

	var out bytes.Buffer
	err := startDevServer(cfg, &out)
	if err == nil {
		t.Fatal("expected builder configuration error, got nil")
	}
	if !strings.Contains(err.Error(), `configure esbuild: builder boom`) {
		t.Fatalf("unexpected builder error: %v", err)
	}
}

func TestRunBuildLoopBroadcastsBuildComplete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes := make(chan daemon.FileChangeEvent, 1)
	broadcaster := &fakeBroadcaster{}
	var buildCalls int
	builder := fakeBuildRunner{
		build: func(_ context.Context, changedPaths []string) (build.Result, error) {
			buildCalls++
			return build.Result{
				BuildID:      fmt.Sprintf("build-123-%d", buildCalls),
				Success:      true,
				DurationMS:   21,
				ChangedFiles: changedPaths,
			}, nil
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- runBuildLoop(ctx, builder, broadcaster, changes)
	}()

	startupBuildEvent := waitForBroadcast(t, broadcaster, 2*time.Second)
	if startupBuildEvent.Name != protocol.MessageBuildComplete {
		t.Fatalf("unexpected startup message name: got %q, want %q", startupBuildEvent.Name, protocol.MessageBuildComplete)
	}

	startupPayload, ok := startupBuildEvent.Data.(protocol.BuildComplete)
	if !ok {
		t.Fatalf("unexpected startup payload type: got %T", startupBuildEvent.Data)
	}
	if startupPayload.BuildID != "build-123-1" {
		t.Fatalf("unexpected startup build id: got %q, want %q", startupPayload.BuildID, "build-123-1")
	}
	if len(startupPayload.ChangedFiles) != 0 {
		t.Fatalf("expected no startup changed files, got %v", startupPayload.ChangedFiles)
	}

	startupReloadEvent := waitForBroadcast(t, broadcaster, 2*time.Second)
	if startupReloadEvent.Name != protocol.MessageCommandReload {
		t.Fatalf("unexpected startup reload message name: got %q, want %q", startupReloadEvent.Name, protocol.MessageCommandReload)
	}

	startupReloadPayload, ok := startupReloadEvent.Data.(protocol.CommandReload)
	if !ok {
		t.Fatalf("unexpected startup reload payload type: got %T", startupReloadEvent.Data)
	}
	if startupReloadPayload.Reason != "startup" {
		t.Fatalf("unexpected startup reload reason: got %q, want %q", startupReloadPayload.Reason, "startup")
	}
	if startupReloadPayload.BuildID != "build-123-1" {
		t.Fatalf("unexpected startup reload build id: got %q, want %q", startupReloadPayload.BuildID, "build-123-1")
	}

	changes <- daemon.FileChangeEvent{Paths: []string{"src/index.ts"}, OccurredAt: time.Now()}

	buildEvent := waitForBroadcast(t, broadcaster, 2*time.Second)
	if buildEvent.Name != protocol.MessageBuildComplete {
		t.Fatalf("unexpected message name: got %q, want %q", buildEvent.Name, protocol.MessageBuildComplete)
	}

	payload, ok := buildEvent.Data.(protocol.BuildComplete)
	if !ok {
		t.Fatalf("unexpected payload type: got %T", buildEvent.Data)
	}
	if !payload.Success {
		t.Fatal("expected successful payload")
	}
	if payload.BuildID != "build-123-2" {
		t.Fatalf("unexpected build id: got %q, want %q", payload.BuildID, "build-123-2")
	}
	if len(payload.ChangedFiles) != 1 || payload.ChangedFiles[0] != "src/index.ts" {
		t.Fatalf("unexpected changed files: %v", payload.ChangedFiles)
	}

	reloadEvent := waitForBroadcast(t, broadcaster, 2*time.Second)
	if reloadEvent.Name != protocol.MessageCommandReload {
		t.Fatalf("unexpected message name: got %q, want %q", reloadEvent.Name, protocol.MessageCommandReload)
	}

	reloadPayload, ok := reloadEvent.Data.(protocol.CommandReload)
	if !ok {
		t.Fatalf("unexpected payload type: got %T", reloadEvent.Data)
	}
	if reloadPayload.Reason != "build.complete" {
		t.Fatalf("unexpected reload reason: got %q, want %q", reloadPayload.Reason, "build.complete")
	}
	if reloadPayload.BuildID != "build-123-2" {
		t.Fatalf("unexpected reload build id: got %q, want %q", reloadPayload.BuildID, "build-123-2")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runBuildLoop() returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for build loop shutdown")
	}
}

func TestRunBuildLoopBuilderErrorStillBroadcastsFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes := make(chan daemon.FileChangeEvent, 1)
	broadcaster := &fakeBroadcaster{}
	var buildCalls int
	builder := fakeBuildRunner{
		build: func(_ context.Context, changedPaths []string) (build.Result, error) {
			buildCalls++
			if buildCalls == 1 {
				return build.Result{
					BuildID:      "build-startup-1",
					Success:      true,
					DurationMS:   12,
					ChangedFiles: changedPaths,
				}, nil
			}
			return build.Result{}, errors.New("boom")
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- runBuildLoop(ctx, builder, broadcaster, changes)
	}()

	startupBuildEvent := waitForBroadcast(t, broadcaster, 2*time.Second)
	if startupBuildEvent.Name != protocol.MessageBuildComplete {
		t.Fatalf("unexpected startup message name: got %q, want %q", startupBuildEvent.Name, protocol.MessageBuildComplete)
	}
	startupReloadEvent := waitForBroadcast(t, broadcaster, 2*time.Second)
	if startupReloadEvent.Name != protocol.MessageCommandReload {
		t.Fatalf("unexpected startup reload name: got %q, want %q", startupReloadEvent.Name, protocol.MessageCommandReload)
	}

	changes <- daemon.FileChangeEvent{Paths: []string{"src/invalid.ts"}, OccurredAt: time.Now()}

	event := waitForBroadcast(t, broadcaster, 2*time.Second)
	if event.Name != protocol.MessageBuildComplete {
		t.Fatalf("unexpected message name: got %q, want %q", event.Name, protocol.MessageBuildComplete)
	}

	payload, ok := event.Data.(protocol.BuildComplete)
	if !ok {
		t.Fatalf("unexpected payload type: got %T", event.Data)
	}
	if payload.Success {
		t.Fatal("expected failure payload")
	}
	if !strings.HasPrefix(payload.BuildID, "build-failed-") {
		t.Fatalf("unexpected failure build id: %q", payload.BuildID)
	}
	if len(payload.ChangedFiles) != 1 || payload.ChangedFiles[0] != "src/invalid.ts" {
		t.Fatalf("unexpected changed files: %v", payload.ChangedFiles)
	}
	if countBroadcastsByName(broadcaster, protocol.MessageCommandReload) != 0 {
		t.Fatal("did not expect command.reload broadcast for failed build")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runBuildLoop() returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for build loop shutdown")
	}
}

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func requireCLIError(t *testing.T, err error) *cliError {
	t.Helper()

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cliErr *cliError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected cliError, got %T (%v)", err, err)
	}

	return cliErr
}

func writePanexConfig(t *testing.T, path, content string) {
	t.Helper()

	err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600)
	if err != nil {
		t.Fatalf("failed to write config fixture: %v", err)
	}
}

func withWorkingDir(dir string, fn func() error) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := os.Chdir(dir); err != nil {
		return err
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	return fn()
}

func withStubbedStartDev(t *testing.T, stub func(cfg panexconfig.Config, stdout io.Writer) error) {
	t.Helper()

	original := startDev
	startDev = stub
	t.Cleanup(func() {
		startDev = original
	})
}

func withStubbedLookupEnv(t *testing.T, stub func(string) (string, bool)) {
	t.Helper()

	original := lookupEnv
	lookupEnv = stub
	t.Cleanup(func() {
		lookupEnv = original
	})
}

func withStubbedNewWebSocketServer(
	t *testing.T,
	stub func(cfg daemon.WebSocketConfig) (devRuntimeServer, error),
) {
	t.Helper()

	original := newWebSocketServer
	newWebSocketServer = stub
	t.Cleanup(func() {
		newWebSocketServer = original
	})
}

func withStubbedNewEsbuildBuilder(
	t *testing.T,
	stub func(sourceDir, outDir string, opts ...build.Option) (buildRunner, error),
) {
	t.Helper()

	original := newEsbuildBuilder
	newEsbuildBuilder = stub
	t.Cleanup(func() {
		newEsbuildBuilder = original
	})
}

func withStubbedNewFileWatcher(
	t *testing.T,
	stub func(root string, debounce time.Duration, emit func(daemon.FileChangeEvent)) (runtimeRunner, error),
) {
	t.Helper()

	original := newFileWatcher
	newFileWatcher = stub
	t.Cleanup(func() {
		newFileWatcher = original
	})
}

func withStubbedSignalContext(t *testing.T, stub func() (context.Context, context.CancelFunc)) {
	t.Helper()

	original := newSignalContext
	newSignalContext = stub
	t.Cleanup(func() {
		newSignalContext = original
	})
}

func newCLIConfigFixture(t *testing.T) panexconfig.Config {
	t.Helper()

	root := t.TempDir()
	sourceDir := filepath.Join(root, "src")
	outDir := filepath.Join(root, "dist")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create out dir: %v", err)
	}

	return panexconfig.Config{
		Extension: panexconfig.Extension{
			SourceDir: sourceDir,
			OutDir:    outDir,
		},
		Server: panexconfig.Server{
			Port:           4317,
			AuthToken:      "dev-token",
			EventStorePath: filepath.Join(root, "events.db"),
		},
	}
}

type fakeBuildRunner struct {
	build func(ctx context.Context, changedPaths []string) (build.Result, error)
}

func (f fakeBuildRunner) Build(ctx context.Context, changedPaths []string) (build.Result, error) {
	return f.build(ctx, changedPaths)
}

type fakeRunComponent struct {
	run func(ctx context.Context) error
}

func (f fakeRunComponent) Run(ctx context.Context) error {
	if f.run == nil {
		return nil
	}
	return f.run(ctx)
}

type fakeDevServer struct {
	*fakeBroadcaster
	run func(ctx context.Context) error
}

func (f *fakeDevServer) Run(ctx context.Context) error {
	if f.run == nil {
		return nil
	}
	return f.run(ctx)
}

type fakeBroadcaster struct {
	mu      sync.Mutex
	events  []protocol.Envelope
	eventCh chan struct{}
}

func (f *fakeBroadcaster) Broadcast(_ context.Context, message protocol.Envelope) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.eventCh == nil {
		f.eventCh = make(chan struct{}, 1)
	}
	f.events = append(f.events, message)
	select {
	case f.eventCh <- struct{}{}:
	default:
	}

	return nil
}

func waitForBroadcast(t *testing.T, broadcaster *fakeBroadcaster, timeout time.Duration) protocol.Envelope {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		broadcaster.mu.Lock()
		if len(broadcaster.events) > 0 {
			event := broadcaster.events[0]
			broadcaster.events = broadcaster.events[1:]
			broadcaster.mu.Unlock()
			return event
		}

		eventCh := broadcaster.eventCh
		broadcaster.mu.Unlock()

		if eventCh == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		select {
		case <-eventCh:
		case <-time.After(10 * time.Millisecond):
		}
	}

	t.Fatalf("timed out waiting for broadcast after %s", timeout)
	return protocol.Envelope{}
}

func countBroadcastsByName(broadcaster *fakeBroadcaster, name protocol.MessageName) int {
	broadcaster.mu.Lock()
	defer broadcaster.mu.Unlock()

	count := 0
	for _, event := range broadcaster.events {
		if event.Name == name {
			count++
		}
	}

	return count
}
