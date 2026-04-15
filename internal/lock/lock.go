// Package lock provides file-based mutual exclusion for Panex operations.
// Spec section 36.
package lock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

	// Check for existing lock
	if existing, err := m.readLock(path); err == nil {
		if m.isAlive(existing.PID) {
			return nil, fmt.Errorf("lock %s held by pid %d (%s) since %s",
				lt, existing.PID, existing.Operation, existing.AcquiredAt.Format(time.RFC3339))
		}
		// Stale lock — remove it
		_ = os.Remove(path)
	}

	info := Info{
		PID:        os.Getpid(),
		Operation:  operation,
		AcquiredAt: time.Now().UTC(),
		Holder:     holder,
	}

	data, _ := json.MarshalIndent(info, "", "  ")

	// Atomic write: tmp + rename
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return nil, fmt.Errorf("write lock: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("rename lock: %w", err)
	}

	return &Lock{Type: lt, Path: path, Info: info}, nil
}

// Release removes the lock file.
func (m *Manager) Release(l *Lock) error {
	if l == nil {
		return nil
	}
	return os.Remove(l.Path)
}

// IsHeld checks if a lock type is currently held by a live process.
func (m *Manager) IsHeld(lt Type) (bool, *Info) {
	info, err := m.readLock(m.lockPath(lt))
	if err != nil {
		return false, nil
	}
	if !m.isAlive(info.PID) {
		return false, &info // stale
	}
	return true, &info
}

// RecoverStale removes locks whose holder PIDs are no longer running.
// Returns the list of recovered lock types.
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
		info, err := m.readLock(path)
		if err != nil {
			// Corrupt lock file — remove it
			_ = os.Remove(path)
			lt := Type(strings.TrimSuffix(e.Name(), ".lock"))
			recovered = append(recovered, lt)
			continue
		}
		if !m.isAlive(info.PID) {
			_ = os.Remove(path)
			lt := Type(strings.TrimSuffix(e.Name(), ".lock"))
			recovered = append(recovered, lt)
		}
	}

	return recovered
}

// StaleInfo returns info about stale locks (for doctor diagnostics).
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
		info, err := m.readLock(path)
		if err != nil {
			continue
		}
		if !m.isAlive(info.PID) {
			stale = append(stale, info)
		}
	}

	return stale
}

func (m *Manager) lockPath(lt Type) string {
	return filepath.Join(m.locksDir, string(lt)+".lock")
}

func (m *Manager) readLock(path string) (Info, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Info{}, err
	}

	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		// Try legacy format: "pid:1234"
		if strings.HasPrefix(string(data), "pid:") {
			pidStr := strings.TrimPrefix(strings.TrimSpace(string(data)), "pid:")
			pid, _ := strconv.Atoi(pidStr)
			return Info{PID: pid}, nil
		}
		return Info{}, err
	}
	return info, nil
}

func (m *Manager) isAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Signal 0 checks liveness.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
