//go:build windows

package lock

import (
	"os"

	"golang.org/x/sys/windows"
)

func osAcquire(f *os.File) error {
	// Use LockFileEx to acquire an exclusive lock without blocking (LOCKFILE_FAIL_IMMEDIATELY).
	// We lock a large range to cover the whole file (0xffffffff, 0xffffffff).
	var overlapped windows.Overlapped
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
}

func osRelease(f *os.File) error {
	var overlapped windows.Overlapped
	// UnlockFileEx must match the byte range exactly.
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &overlapped)
}
