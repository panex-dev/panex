//go:build !windows

package lock

import (
	"os"

	"golang.org/x/sys/unix"
)

func osAcquire(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}

func osAcquireShared(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_SH|unix.LOCK_NB)
}

func osRelease(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
