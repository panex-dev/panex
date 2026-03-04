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

	done := make(chan error, 1)
	go func() {
		done <- watcher.Run(ctx)
	}()

	time.Sleep(30 * time.Millisecond)

	if err := os.WriteFile(target, []byte("v2"), 0o600); err != nil {
		t.Fatalf("write file v2: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
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
			goto done
		}
	}

done:

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watcher shutdown")
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

	done := make(chan error, 1)
	go func() {
		done <- watcher.Run(ctx)
	}()

	time.Sleep(30 * time.Millisecond)

	subDir := filepath.Join(root, "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	newFile := filepath.Join(subDir, "entry.js")
	if err := os.WriteFile(newFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write nested file: %v", err)
	}

	event := waitForEvent(t, events, 2*time.Second)
	if !slices.Contains(event.Paths, "nested/entry.js") {
		t.Fatalf("expected nested file path in event, got %v", event.Paths)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watcher shutdown")
	}
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
	done := make(chan error, 1)
	go func() {
		done <- watcher.Run(ctx)
	}()

	time.Sleep(30 * time.Millisecond)
	if err := os.WriteFile(target, []byte("b"), 0o600); err != nil {
		t.Fatalf("write changed file: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
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
