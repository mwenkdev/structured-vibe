package lock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireAndRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "dir", "test.lock")

	l, err := Acquire(path)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if l.Path() != path {
		t.Errorf("path = %q", l.Path())
	}
	if err := l.Release(); err != nil {
		t.Errorf("release: %v", err)
	}
}

// TestSecondAcquireFailsImmediately is the core contract: svibe does not
// wait, retry, or break locks.
func TestSecondAcquireFailsImmediately(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	first, err := Acquire(path)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer first.Release()

	start := time.Now()
	second, err := Acquire(path)
	elapsed := time.Since(start)

	if err == nil {
		second.Release()
		t.Fatal("second acquire should have failed")
	}
	if !errors.Is(err, ErrHeld) {
		t.Errorf("err = %v, want ErrHeld", err)
	}
	// "Immediately" means it must not be waiting on the lock.
	if elapsed > time.Second {
		t.Errorf("acquire blocked for %v; it must fail immediately", elapsed)
	}
}

func TestReleaseAllowsReacquire(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}

	second, err := Acquire(path)
	if err != nil {
		t.Fatalf("reacquire after release failed: %v", err)
	}
	second.Release()
}

// TestStaleLockFileDoesNotImplyOwnership: a leftover lock file from a crashed
// process must not block anyone. Ownership is the OS lock, never the file.
func TestStaleLockFileDoesNotImplyOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	// Simulate a lock file left behind by a process that died.
	content := "pid: 999999\nhost: some-dead-machine\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	l, err := Acquire(path)
	if err != nil {
		t.Fatalf("a stale lock file must not block acquisition: %v", err)
	}
	l.Release()
}

// TestLockFileSurvivesRelease: removing it would race with another process
// that has already opened it.
func TestLockFileSurvivesRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	l, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	l.Release()

	if _, err := os.Stat(path); err != nil {
		t.Errorf("lock file should remain after release: %v", err)
	}
}

// TestDiagnosticsAreLabelledAsSuch guards against anyone later treating the
// file contents as authoritative.
func TestDiagnosticsAreLabelledAsSuch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	l, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if len(got) == 0 {
		t.Fatal("expected diagnostics to be written")
	}
	if want := "Diagnostics only"; !contains(got, want) {
		t.Errorf("lock file should state it is diagnostic only, got:\n%s", got)
	}
}

func TestReleaseOnNilIsSafe(t *testing.T) {
	var l *Lock
	if err := l.Release(); err != nil {
		t.Errorf("release on nil lock: %v", err)
	}
	if l.Path() != "" {
		t.Error("path on nil lock should be empty")
	}
}

func TestDoubleReleaseIsSafe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")
	l, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	if err := l.Release(); err != nil {
		t.Errorf("second release should be a no-op, got %v", err)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
