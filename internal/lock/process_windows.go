//go:build windows

package lock

import (
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// PROCESS_QUERY_LIMITED_INFORMATION — minimal access to call GetExitCodeProcess.
const processQueryLimitedInformation = 0x1000

// STILL_ACTIVE — exit code reported for processes that have not yet terminated.
const stillActive = 259

func osAcquire(f *os.File) error {
	// Use LockFileEx to acquire an exclusive lock without blocking (LOCKFILE_FAIL_IMMEDIATELY).
	// We lock the first byte of the file.
	var overlapped windows.Overlapped
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
}

func osRelease(f *os.File) error {
	var overlapped windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &overlapped)
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode == stillActive
}
