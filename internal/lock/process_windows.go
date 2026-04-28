//go:build windows

package lock

import (
	"os"

	"golang.org/x/sys/windows"
)

func osAcquire(f *os.File) error {
	// Exclusive lock, non-blocking.
	var overlapped windows.Overlapped
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
}

func osAcquireShared(f *os.File) error {
	// Shared lock, non-blocking.
	var overlapped windows.Overlapped
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
}

func osRelease(f *os.File) error {
	var overlapped windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &overlapped)
}
