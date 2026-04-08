package daemon

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNewFileWatcherValidation(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	validEmit := func(FileChangeEvent) {}

	testCases := []struct {
		name      string
		root      string
		debounce  time.Duration
		emit      func(FileChangeEvent)
		wantError string
	}{
		{
			name:      "missing root",
			root:      "",
			debounce:  DefaultWatchDebounce,
			emit:      validEmit,
			wantError: "watch root is required",
		},
		{
			name:      "non-positive debounce",
			root:      tmpDir,
			debounce:  0,
			emit:      validEmit,
			wantError: "debounce must be > 0",
		},
		{
			name:      "missing emit callback",
			root:      tmpDir,
			debounce:  DefaultWatchDebounce,
			emit:      nil,
			wantError: "emit callback is required",
		},
		{
			name:      "root is not a directory",
			root:      filePath,
			debounce:  DefaultWatchDebounce,
			emit:      validEmit,
			wantError: "watch root must be a directory",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewFileWatcher(tc.root, tc.debounce, tc.emit)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("unexpected error: got %v, want contains %q", err, tc.wantError)
			}
		})
	}
}

func TestFileWatcherDebouncesRapidFileChanges(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "app.js")
	if err := os.WriteFile(target, []byte("v1"), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	events := make(chan FileChangeEvent, 4)
	watcher, err := NewFileWatcher(root, 50*time.Millisecond, func(event FileChangeEvent) {
		events <- event
	})
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startWatcher(t, watcher, ctx)

	if err := os.WriteFile(target, []byte("v2"), 0o600); err != nil {
		t.Fatalf("write file v2: %v", err)
	}
	time.Sleep(10 * time.Millisecond) // rapid writes within debounce window
	if err := os.WriteFile(target, []byte("v3"), 0o600); err != nil {
		t.Fatalf("write file v3: %v", err)
	}

	event := waitForEvent(t, events, 2*time.Second)
	if len(event.Paths) != 1 {
		t.Fatalf("expected one deduplicated path, got %v", event.Paths)
	}
	if event.Paths[0] != "app.js" {
		t.Fatalf("unexpected path: got %q, want %q", event.Paths[0], "app.js")
	}

	// Under -race and slower CI schedulers, fsnotify can surface one trailing write
	// notification after the first debounce flush. Allow at most one extra identical batch.
	extraBatches := 0
	waitForExtras := time.NewTimer(250 * time.Millisecond)
	defer waitForExtras.Stop()

	for {
		select {
		case extra := <-events:
			extraBatches++
			if len(extra.Paths) != 1 || extra.Paths[0] != "app.js" {
				t.Fatalf("unexpected extra debounced batch: %+v", extra)
			}
			if extraBatches > 1 {
				t.Fatalf("expected at most one trailing debounced batch, got %d", extraBatches)
			}
		case <-waitForExtras.C:
			return
		}
	}
}

func TestFileWatcherWatchesNewDirectories(t *testing.T) {
	root := t.TempDir()

	events := make(chan FileChangeEvent, 8)
	watcher, err := NewFileWatcher(root, 50*time.Millisecond, func(event FileChangeEvent) {
		events <- event
	})
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startWatcher(t, watcher, ctx)

	subDir := filepath.Join(root, "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	// Poll-write into the new directory until fsnotify registers its watch.
	// Windows needs extra time for ReadDirectoryChangesW to pick up new watches.
	newFile := filepath.Join(subDir, "entry.js")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_ = os.WriteFile(newFile, []byte("x"), 0o600)
		select {
		case event := <-events:
			if slices.Contains(event.Paths, "nested/entry.js") {
				return
			}
		case <-time.After(100 * time.Millisecond):
		}
	}
	t.Fatal("timed out waiting for nested file event")
}

func TestFileWatcherFlushesPendingChangesOnCancel(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "index.ts")
	if err := os.WriteFile(target, []byte("a"), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	events := make(chan FileChangeEvent, 4)
	watcher, err := NewFileWatcher(root, 500*time.Millisecond, func(event FileChangeEvent) {
		events <- event
	})
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := startWatcher(t, watcher, ctx)

	if err := os.WriteFile(target, []byte("b"), 0o600); err != nil {
		t.Fatalf("write changed file: %v", err)
	}

	// Brief pause for fsnotify to deliver the inotify event to Run().
	// This must be shorter than the 500ms debounce so the event stays pending.
	time.Sleep(50 * time.Millisecond)
	cancel()

	event := waitForEvent(t, events, 2*time.Second)
	if !slices.Contains(event.Paths, "index.ts") {
		t.Fatalf("expected index.ts in flush event, got %v", event.Paths)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watcher shutdown")
	}
}

func TestFileWatcherSkipsInfrastructureDirs(t *testing.T) {
	root := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, ".git", "objects"), 0o755); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "dep"), 0o755); err != nil {
		t.Fatalf("create node_modules: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".panex", "dist"), 0o755); err != nil {
		t.Fatalf("create .panex: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatalf("create src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "app.js"), []byte("v1"), 0o600); err != nil {
		t.Fatalf("write app.js: %v", err)
	}

	events := make(chan FileChangeEvent, 8)
	watcher, err := NewFileWatcher(root, 50*time.Millisecond, func(event FileChangeEvent) {
		events <- event
	})
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startWatcher(t, watcher, ctx)

	// Write to infrastructure directories — these should not trigger events
	// because the watcher never added those directories.
	if err := os.WriteFile(filepath.Join(root, ".git", "index"), []byte("git-data"), 0o600); err != nil {
		t.Fatalf("write .git/index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "dep", "index.js"), []byte("dep"), 0o600); err != nil {
		t.Fatalf("write node_modules file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".panex", "dist", "output.js"), []byte("out"), 0o600); err != nil {
		t.Fatalf("write .panex file: %v", err)
	}

	// Write to a watched directory — this should trigger an event.
	if err := os.WriteFile(filepath.Join(root, "src", "app.js"), []byte("v2"), 0o600); err != nil {
		t.Fatalf("write src/app.js: %v", err)
	}

	event := waitForEvent(t, events, 2*time.Second)
	for _, path := range event.Paths {
		if strings.HasPrefix(path, ".git") || strings.HasPrefix(path, "node_modules") || strings.HasPrefix(path, ".panex") {
			t.Fatalf("unexpected infrastructure path in event: %q", path)
		}
	}
	if !slices.Contains(event.Paths, "src/app.js") {
		t.Fatalf("expected src/app.js in event, got %v", event.Paths)
	}
}

func TestFileWatcherSkipsNewInfrastructureDirs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.js"), []byte("v1"), 0o600); err != nil {
		t.Fatalf("write index.js: %v", err)
	}

	events := make(chan FileChangeEvent, 8)
	watcher, err := NewFileWatcher(root, 50*time.Millisecond, func(event FileChangeEvent) {
		events <- event
	})
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startWatcher(t, watcher, ctx)

	// Create a new infrastructure directory after the watcher starts.
	if err := os.MkdirAll(filepath.Join(root, ".panex", "dist"), 0o755); err != nil {
		t.Fatalf("create .panex: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".panex", "dist", "bundle.js"), []byte("out"), 0o600); err != nil {
		t.Fatalf("write .panex bundle: %v", err)
	}

	// Write to root to confirm the watcher is still alive.
	if err := os.WriteFile(filepath.Join(root, "index.js"), []byte("v2"), 0o600); err != nil {
		t.Fatalf("write index.js v2: %v", err)
	}

	event := waitForEvent(t, events, 2*time.Second)

	// The root-level .panex creation may appear as a directory event from the parent
	// watcher, but files inside .panex/dist/ must not appear because the subtree
	// was never added to fsnotify.
	for _, path := range event.Paths {
		if strings.HasPrefix(path, ".panex/dist/") {
			t.Fatalf("unexpected .panex/dist/ path in event: %q", path)
		}
	}
}

func TestIsInfrastructureDir(t *testing.T) {
	testCases := []struct {
		name string
		want bool
	}{
		{"node_modules", true},
		{".git", true},
		{".panex", true},
		{".vscode", true},
		{"src", false},
		{"dist", false},
		{"nested", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isInfrastructureDir(tc.name)
			if got != tc.want {
				t.Fatalf("isInfrastructureDir(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func waitForEvent(t *testing.T, events <-chan FileChangeEvent, timeout time.Duration) FileChangeEvent {
	t.Helper()

	select {
	case event := <-events:
		return event
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for file event after %s", timeout)
		return FileChangeEvent{}
	}
}

// startWatcher launches the watcher in a goroutine and blocks until it is
// ready to process filesystem events. This replaces fixed-duration sleeps
// that are fragile on slow CI runners.
func startWatcher(t *testing.T, watcher *FileWatcher, ctx context.Context) <-chan error {
	t.Helper()
	watcher.ready = make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- watcher.Run(ctx)
	}()
	select {
	case <-watcher.ready:
	case <-time.After(2 * time.Second):
		t.Fatal("file watcher did not become ready within 2s")
	}
	return done
}
