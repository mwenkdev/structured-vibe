//go:build windows

package lock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// tryLock takes an exclusive advisory lock without blocking.
//
// Windows releases file locks when the handle closes, including when the
// process terminates abnormally, giving the same process-scoped ownership
// semantics as flock.
func tryLock(f *os.File) error {
	ol := new(windows.Overlapped)
	err := windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, ol,
	)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, windows.ERROR_LOCK_VIOLATION), errors.Is(err, windows.ERROR_IO_PENDING):
		return ErrHeld
	default:
		return err
	}
}

func unlock(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)
}
