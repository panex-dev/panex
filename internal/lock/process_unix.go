//go:build !windows

package lock

import (
	"os"
	"syscall"
)

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Signal 0 probes liveness without
	// actually delivering a signal.
	return process.Signal(syscall.Signal(0)) == nil
}
