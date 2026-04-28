package daemon

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestFileWatcherBasic(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "index.ts")
	if err := os.WriteFile(target, []byte("a"), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	events := make(chan FileChangeEvent, 4)
	watcher, err := NewFileWatcher(root, 100*time.Millisecond, func(event FileChangeEvent) {
		events <- event
	})
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startWatcher(t, watcher, ctx)

	// Update existing file
	if err := os.WriteFile(target, []byte("b"), 0o600); err != nil {
		t.Fatalf("write update: %v", err)
	}

	select {
	case event := <-events:
		if !slices.Contains(event.Paths, "index.ts") {
			t.Errorf("expected index.ts in event, got %v", event.Paths)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for file event")
	}
}

func TestFileWatcherBatching(t *testing.T) {
	root := t.TempDir()
	events := make(chan FileChangeEvent, 4)
	watcher, err := NewFileWatcher(root, 200*time.Millisecond, func(event FileChangeEvent) {
		events <- event
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startWatcher(t, watcher, ctx)

	// Rapidly create multiple files
	files := []string{"a.js", "b.js", "c.js"}
	for _, f := range files {
		_ = os.WriteFile(filepath.Join(root, f), []byte("x"), 0o600)
	}

	select {
	case event := <-events:
		sort.Strings(event.Paths)
		if !slices.Equal(event.Paths, files) {
			t.Errorf("expected %v, got %v", files, event.Paths)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for batched event")
	}
}

func TestFileWatcherIgnoresInfrastructure(t *testing.T) {
	root := t.TempDir()
	events := make(chan FileChangeEvent, 4)
	watcher, err := NewFileWatcher(root, 100*time.Millisecond, func(event FileChangeEvent) {
		events <- event
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startWatcher(t, watcher, ctx)

	// Create file in .git (should be ignored)
	gitDir := filepath.Join(root, ".git")
	_ = os.MkdirAll(gitDir, 0o755)
	_ = os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main"), 0o600)

	// Create visible file
	_ = os.WriteFile(filepath.Join(root, "visible.js"), []byte("x"), 0o600)

	select {
	case event := <-events:
		for _, p := range event.Paths {
			if strings.HasPrefix(p, ".git") {
				t.Errorf("infrastructure file %s was not ignored", p)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}
}

func TestFileWatcherWatchesNewDirectories(t *testing.T) {
	root := t.TempDir()
	events := make(chan FileChangeEvent, 4)
	// Use shorter debounce for test speed
	watcher, err := NewFileWatcher(root, 100*time.Millisecond, func(event FileChangeEvent) {
		events <- event
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startWatcher(t, watcher, ctx)

	subDir := filepath.Join(root, "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	// Wait a bit for the syncTree ticker or the eager Create event to register the watch.
	// Windows CI can be very slow.
	time.Sleep(1 * time.Second)

	newFile := filepath.Join(subDir, "entry.js")
	if err := os.WriteFile(newFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write nested file: %v", err)
	}

	// If we miss the event, we poll-write to eventually trigger it after syncTree catches up.
	// We use a long deadline for Windows CI (15s).
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		_ = os.WriteFile(newFile, []byte("y"), 0o600)
		select {
		case event := <-events:
			if slices.Contains(event.Paths, "nested/entry.js") {
				return
			}
		case <-time.After(200 * time.Millisecond):
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
	// Long debounce to ensure it hasn't flushed yet
	watcher, err := NewFileWatcher(root, 5*time.Second, func(event FileChangeEvent) {
		events <- event
	})
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	startWatcher(t, watcher, ctx)

	// Trigger change
	_ = os.WriteFile(target, []byte("b"), 0o600)
	time.Sleep(200 * time.Millisecond)

	// Cancel before debounce expires — should trigger immediate flush
	cancel()

	select {
	case event := <-events:
		if !slices.Contains(event.Paths, "index.ts") {
			t.Errorf("expected index.ts in flush event, got %v", event.Paths)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for cancel flush")
	}
}

// --- helpers ---

func startWatcher(t *testing.T, w *FileWatcher, ctx context.Context) {
	t.Helper()
	ready := make(chan struct{})
	w.ready = ready
	go func() {
		if err := w.Run(ctx); err != nil {
			// May fail if ctx is cancelled during startup
		}
	}()

	select {
	case <-ready:
		// Give it a tiny bit more time for the OS to actually register watches
		time.Sleep(200 * time.Millisecond)
	case <-time.After(10 * time.Second):
		t.Fatal("watcher never became ready")
	}
}
