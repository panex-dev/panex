// Package lock provides file-based mutual exclusion for Panex operations.
// Spec section 36.
package lock

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Type is a named lock category.
type Type string

const (
	ProjectMutation Type = "project"
	DevSession      Type = "session"
	Publish         Type = "publish"
)

// Info is persisted inside each lock file.
type Info struct {
	PID        int       `json:"pid"`
	Operation  string    `json:"operation"`
	AcquiredAt time.Time `json:"acquired_at"`
	Holder     string    `json:"holder"` // "cli", "mcp", "agent"
}

// Lock represents an acquired lock.
type Lock struct {
	Type Type
	Path string
	Info Info
	file *os.File
}

// Manager manages lock files under .panex/locks/.
type Manager struct {
	locksDir string
}

// NewManager creates a lock manager for the given .panex/locks/ directory.
func NewManager(panexDir string) *Manager {
	return &Manager{locksDir: filepath.Join(panexDir, "locks")}
}

// Acquire attempts to take a lock. Returns the lock if successful, error if held.
func (m *Manager) Acquire(lt Type, operation, holder string) (*Lock, error) {
	if err := os.MkdirAll(m.locksDir, 0o755); err != nil {
		return nil, fmt.Errorf("create locks dir: %w", err)
	}

	path := m.lockPath(lt)
	// Open with RDWR so we can write info after acquiring the lock.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	// Try to acquire OS advisory lock (C4, L5)
	if err := osAcquire(f); err != nil {
		// Lock is held by another process.
		info, readErr := m.readLock(f)
		_ = f.Close()
		if readErr == nil {
			return nil, fmt.Errorf("lock %s held by pid %d (%s) since %s",
				lt, info.PID, info.Operation, info.AcquiredAt.Format(time.RFC3339Nano))
		}
		return nil, fmt.Errorf("lock %s is held", lt)
	}

	info := Info{
		PID:        os.Getpid(),
		Operation:  operation,
		AcquiredAt: time.Now().UTC(),
		Holder:     holder,
	}

	// Write info to the file for diagnostics.
	data, _ := json.MarshalIndent(info, "", "  ")
	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = f.Write(data)

	return &Lock{Type: lt, Path: path, Info: info, file: f}, nil
}

// Release removes the lock file hold and closes the handle.
func (m *Manager) Release(l *Lock) error {
	if l == nil || l.file == nil {
		return nil
	}
	_ = osRelease(l.file)
	err := l.file.Close()
	l.file = nil // prevent double close/use after free
	_ = os.Remove(l.Path)
	return err
}

// IsHeld checks if a lock type is currently held by a live process.
func (m *Manager) IsHeld(lt Type) (bool, *Info) {
	path := m.lockPath(lt)
	// On Windows, if one process has an exclusive lock, another process
	// can often still open the file for reading if it's already created.
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return true, nil // Cannot even open for reading -> definitely held
	}
	defer func() { _ = f.Close() }()

	// Try to acquire a shared lock. If it fails, someone has an exclusive lock.
	if err := osAcquireShared(f); err == nil {
		_ = osRelease(f)
		return false, nil
	}

	// Held. Try to read info.
	info, err := m.readLock(f)
	if err != nil {
		return true, nil
	}
	return true, &info
}

// RecoverStale cleans up orphan files that are NOT held by any process.
func (m *Manager) RecoverStale() []Type {
	var recovered []Type

	entries, err := os.ReadDir(m.locksDir)
	if err != nil {
		return nil
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".lock") {
			continue
		}
		path := filepath.Join(m.locksDir, e.Name())
		f, err := os.OpenFile(path, os.O_RDWR, 0o644)
		if err != nil {
			continue
		}
		if err := osAcquire(f); err == nil {
			// Not held — safe to delete orphan file
			_ = osRelease(f)
			_ = f.Close()
			_ = os.Remove(path)
			lt := Type(strings.TrimSuffix(e.Name(), ".lock"))
			recovered = append(recovered, lt)
		} else {
			_ = f.Close()
		}
	}

	return recovered
}

// StaleInfo returns info about orphans (diagnostics).
func (m *Manager) StaleInfo() []Info {
	var stale []Info

	entries, err := os.ReadDir(m.locksDir)
	if err != nil {
		return nil
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".lock") {
			continue
		}
		path := filepath.Join(m.locksDir, e.Name())
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		if err := osAcquireShared(f); err == nil {
			_ = osRelease(f)
			info, err := m.readLock(f)
			_ = f.Close()
			if err == nil {
				stale = append(stale, info)
			}
		} else {
			_ = f.Close()
		}
	}

	return stale
}

func (m *Manager) lockPath(lt Type) string {
	return filepath.Join(m.locksDir, string(lt)+".lock")
}

func (m *Manager) readLock(f *os.File) (Info, error) {
	_, _ = f.Seek(0, 0)
	data, err := io.ReadAll(f)
	if err != nil {
		return Info{}, err
	}

	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return Info{}, err
	}
	return info, nil
}
