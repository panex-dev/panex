//go:build windows

package lock

import (
	"os"

	"golang.org/x/sys/windows"
)

// We lock a single byte at a very high offset (4GB) to avoid interfering with
// reading/writing the actual file content (which is small JSON). Offset 0 is
// where the JSON starts, so locking it blocks ReadFile/io.ReadAll.
const (
	lockOffsetLow  = 0
	lockOffsetHigh = 1 // 1 << 32 = 4GB
)

func osAcquire(f *os.File) error {
	// Exclusive lock, non-blocking.
	var overlapped windows.Overlapped
	overlapped.Offset = lockOffsetLow
	overlapped.OffsetHigh = lockOffsetHigh
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
}

func osAcquireShared(f *os.File) error {
	// Shared lock, non-blocking.
	var overlapped windows.Overlapped
	overlapped.Offset = lockOffsetLow
	overlapped.OffsetHigh = lockOffsetHigh
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
}

func osRelease(f *os.File) error {
	var overlapped windows.Overlapped
	overlapped.Offset = lockOffsetLow
	overlapped.OffsetHigh = lockOffsetHigh
	// UnlockFileEx must match the byte range exactly.
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &overlapped)
}
