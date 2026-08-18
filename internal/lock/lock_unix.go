//go:build !windows

package lock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// tryLock takes an exclusive advisory lock without blocking.
//
// flock ownership is tied to the open file description and is released by the
// kernel when the process exits, including on crash. That is what makes
// "stale lock file" impossible to confuse with "locked".
func tryLock(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, unix.EWOULDBLOCK):
		return ErrHeld
	default:
		return err
	}
}

func unlock(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
