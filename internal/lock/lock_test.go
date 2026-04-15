package lock

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireRelease(t *testing.T) {
	dir := t.TempDir()
	panexDir := filepath.Join(dir, ".panex")
	if err := os.MkdirAll(panexDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(panexDir)

	l, err := mgr.Acquire(ProjectMutation, "apply", "cli")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	// Lock file should exist
	if _, err := os.Stat(l.Path); err != nil {
		t.Error("lock file should exist")
	}

	// Should be held
	held, info := mgr.IsHeld(ProjectMutation)
	if !held {
		t.Error("lock should be held")
	}
	if info.Operation != "apply" {
		t.Errorf("operation: got %s", info.Operation)
	}

	// Release
	if err := mgr.Release(l); err != nil {
		t.Errorf("release: %v", err)
	}

	held, _ = mgr.IsHeld(ProjectMutation)
	if held {
		t.Error("lock should not be held after release")
	}
}

func TestDoubleAcquire(t *testing.T) {
	dir := t.TempDir()
	panexDir := filepath.Join(dir, ".panex")
	if err := os.MkdirAll(panexDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(panexDir)

	l, err := mgr.Acquire(ProjectMutation, "apply", "cli")
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer func() { _ = mgr.Release(l) }()

	// Second acquire should fail (same process holds it)
	_, err = mgr.Acquire(ProjectMutation, "plan", "cli")
	if err == nil {
		t.Error("expected error on double acquire")
	}
}

func TestDifferentLockTypes(t *testing.T) {
	dir := t.TempDir()
	panexDir := filepath.Join(dir, ".panex")
	if err := os.MkdirAll(panexDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(panexDir)

	l1, err := mgr.Acquire(ProjectMutation, "apply", "cli")
	if err != nil {
		t.Fatalf("project lock: %v", err)
	}
	defer func() { _ = mgr.Release(l1) }()

	// Different lock type should succeed
	l2, err := mgr.Acquire(DevSession, "dev", "cli")
	if err != nil {
		t.Fatalf("session lock: %v", err)
	}
	defer func() { _ = mgr.Release(l2) }()
}

func TestRecoverStale(t *testing.T) {
	dir := t.TempDir()
	panexDir := filepath.Join(dir, ".panex")
	locksDir := filepath.Join(panexDir, "locks")
	if err := os.MkdirAll(locksDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a lock with a dead PID
	if err := os.WriteFile(filepath.Join(locksDir, "project.lock"), []byte(`{"pid":999999,"operation":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(panexDir)
	recovered := mgr.RecoverStale()

	if len(recovered) != 1 {
		t.Errorf("expected 1 recovered, got %d", len(recovered))
	}
	if len(recovered) > 0 && recovered[0] != ProjectMutation {
		t.Errorf("recovered type: got %s", recovered[0])
	}

	// Lock file should be gone
	if _, err := os.Stat(filepath.Join(locksDir, "project.lock")); !os.IsNotExist(err) {
		t.Error("stale lock should have been removed")
	}
}

func TestRecoverCorruptLock(t *testing.T) {
	dir := t.TempDir()
	panexDir := filepath.Join(dir, ".panex")
	locksDir := filepath.Join(panexDir, "locks")
	if err := os.MkdirAll(locksDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write corrupt lock
	if err := os.WriteFile(filepath.Join(locksDir, "session.lock"), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(panexDir)
	recovered := mgr.RecoverStale()

	if len(recovered) != 1 {
		t.Errorf("expected 1 recovered, got %d", len(recovered))
	}
}

func TestLegacyPidFormat(t *testing.T) {
	dir := t.TempDir()
	panexDir := filepath.Join(dir, ".panex")
	locksDir := filepath.Join(panexDir, "locks")
	if err := os.MkdirAll(locksDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Legacy format from old doctor tests
	if err := os.WriteFile(filepath.Join(locksDir, "project.lock"), []byte("pid:999999"), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(panexDir)

	// Should detect as stale (pid 999999 is almost certainly dead)
	held, _ := mgr.IsHeld(ProjectMutation)
	if held {
		t.Error("legacy lock with dead pid should not be held")
	}
}

func TestReleaseNil(t *testing.T) {
	dir := t.TempDir()
	panexDir := filepath.Join(dir, ".panex")

	mgr := NewManager(panexDir)
	if err := mgr.Release(nil); err != nil {
		t.Errorf("release nil should be no-op: %v", err)
	}
}

func TestStaleInfo(t *testing.T) {
	dir := t.TempDir()
	panexDir := filepath.Join(dir, ".panex")
	locksDir := filepath.Join(panexDir, "locks")
	if err := os.MkdirAll(locksDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(locksDir, "project.lock"), []byte(`{"pid":999999,"operation":"apply","holder":"cli"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(panexDir)
	stale := mgr.StaleInfo()

	if len(stale) != 1 {
		t.Fatalf("expected 1 stale, got %d", len(stale))
	}
	if stale[0].Operation != "apply" {
		t.Errorf("operation: got %s", stale[0].Operation)
	}
}
