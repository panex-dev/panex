//go:build windows

package lock

import "syscall"

// PROCESS_QUERY_LIMITED_INFORMATION — minimal access to call GetExitCodeProcess.
const processQueryLimitedInformation = 0x1000

// STILL_ACTIVE — exit code reported for processes that have not yet terminated.
const stillActive = 259

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)

	var exitCode uint32
	if err := syscall.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode == stillActive
}
