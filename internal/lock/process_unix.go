//go:build !windows

package lock

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func osAcquire(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}

func osRelease(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
