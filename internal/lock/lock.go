// Package lock provides an OS-managed advisory file lock.
//
// Requirements (architecture 13.6):
//
//   - the lock is held from the beginning of validation through successful
//     publication or failure cleanup;
//   - lock ownership is process-scoped, and process termination releases it;
//   - stale lock-file existence alone never implies ownership;
//   - lock metadata may be written for diagnostics only;
//   - if another process owns the lock, acquisition fails immediately;
//   - svibe does not wait, retry, or break locks based on elapsed time;
//   - unsupported or broken filesystem locking is an error rather than a
//     fallback to fragile PID/TTL semantics.
//
// The last point is why this package has no "force" or "steal" path. A
// timeout-based lock breaker cannot distinguish a crashed process from a slow
// one, and guessing wrong corrupts a published snapshot.
package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrHeld indicates another process currently owns the lock.
var ErrHeld = errors.New("lock is held by another process")

// Lock is an acquired advisory file lock.
type Lock struct {
	path string
	f    *os.File
}

// Acquire takes the lock at path, creating parent directories as needed.
//
// It never blocks. If the lock is already held, it returns ErrHeld
// immediately.
func Acquire(path string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating lock directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}

	if err := tryLock(f); err != nil {
		_ = f.Close()
		if errors.Is(err, ErrHeld) {
			return nil, ErrHeld
		}
		// Unsupported or broken locking is an error. Do not fall back to
		// inspecting the lock file's existence or contents.
		return nil, fmt.Errorf("filesystem locking unavailable for %s: %w", path, err)
	}

	l := &Lock{path: path, f: f}
	l.writeDiagnostics()
	return l, nil
}

// writeDiagnostics records who holds the lock. This is for humans reading the
// file during troubleshooting and is never consulted to determine ownership.
func (l *Lock) writeDiagnostics() {
	if err := l.f.Truncate(0); err != nil {
		return
	}
	if _, err := l.f.Seek(0, 0); err != nil {
		return
	}
	host, _ := os.Hostname()
	fmt.Fprintf(l.f,
		"# Diagnostics only. Ownership is determined by the OS advisory lock,\n"+
			"# never by this file's existence or contents.\npid: %d\nhost: %s\n",
		os.Getpid(), host)
	_ = l.f.Sync() // diagnostics only
}

// Release drops the lock.
//
// The lock file is deliberately left in place. Removing it would race with
// another process that has already opened it, and its presence means nothing.
func (l *Lock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := unlock(l.f)
	closeErr := l.f.Close()
	l.f = nil
	if err != nil {
		return err
	}
	return closeErr
}

// Path returns the lock file path.
func (l *Lock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}
