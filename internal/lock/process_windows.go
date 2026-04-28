//go:build windows

package lock

import (
	"os"

	"golang.org/x/sys/windows"
)

func osAcquire(f *os.File) error {
	// Use LockFileEx to acquire an exclusive lock without blocking (LOCKFILE_FAIL_IMMEDIATELY).
	// We lock a single byte at a very high offset (4GB) to avoid interfering with
	// reading/writing the actual file content (which is small JSON).
	var overlapped windows.Overlapped
	overlapped.OffsetHigh = 1 // 4GB offset
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
}

func osRelease(f *os.File) error {
	var overlapped windows.Overlapped
	overlapped.OffsetHigh = 1
	// UnlockFileEx must match the byte range exactly.
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &overlapped)
}
