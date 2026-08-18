// Package sync materializes the resolved skill set into a host-facing
// snapshot.
//
// The transaction (architecture 13.4, 13.5):
//
//	acquire lock
//	validate
//	resolve
//	build desired snapshot
//	verify OpenCode integration
//	stage filesystem publication
//	publish atomically
//	write minimal sync metadata
//	release lock
//
// If publication cannot complete, the previous generated snapshot remains
// intact. Resolution and planning happen in memory; the filesystem is only
// touched once every precondition has passed.
package sync

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/lock"
	"github.com/mwenkdev/structured-vibe/internal/resolve"
)

// StateFileName is the hidden sync-state file inside the snapshot root.
//
// It holds only what is needed to compare the previous snapshot to current
// inputs. It deliberately does not cache a redundant full resolution
// manifest; "svibe resolve --json" remains the source for current resolution
// details (architecture 14).
const StateFileName = ".svibe-sync.json"

// LockFileName is the advisory lock guarding the transaction.
const LockFileName = ".svibe-sync.lock"

// SkillsDirName is the directory the host is pointed at.
const SkillsDirName = "skills"

// State is the persisted sync metadata.
type State struct {
	SvibeVersion string `json:"svibe_version"`
	Fingerprint  string `json:"fingerprint"`
	SyncedAt     string `json:"synced_at"`
}

// Request describes one synchronization.
type Request struct {
	// SnapshotRoot is the directory that will contain skills/ and the state
	// file, for example <git-root>/.structured-vibe/generated/opencode.
	SnapshotRoot string
	// Resolution is the already-resolved winning skill set.
	Resolution *resolve.Resolution
	// Fingerprint identifies the inputs that produced Resolution.
	Fingerprint string
	// SvibeVersion is the CLI version, which is also the rules version.
	SvibeVersion string
	// VerifyIntegration runs after the snapshot is planned and before
	// anything is published. A returned error diagnostic aborts the
	// transaction with the previous snapshot untouched.
	VerifyIntegration func() diag.Diagnostics
}

// Result reports what a successful sync published.
type Result struct {
	SnapshotRoot string `json:"snapshot_root"`
	SkillsDir    string `json:"skills_dir"`
	SkillCount   int    `json:"skill_count"`
	Fingerprint  string `json:"fingerprint"`
	SyncedAt     string `json:"synced_at"`
}

// Run executes the sync transaction.
func Run(req Request) (*Result, diag.Diagnostics) {
	var d diag.Diagnostics

	if req.Resolution == nil {
		d.Errorf("sync.no-resolution", "", "cannot sync without a resolution")
		return nil, d
	}

	if err := os.MkdirAll(req.SnapshotRoot, 0o755); err != nil {
		d.Errorf("sync.mkdir", req.SnapshotRoot, "cannot create snapshot root: %v", err)
		return nil, d
	}

	// The lock is held across the entire transaction.
	lk, err := lock.Acquire(filepath.Join(req.SnapshotRoot, LockFileName))
	if err != nil {
		if err == lock.ErrHeld {
			d.Errorf("sync.locked", req.SnapshotRoot,
				"another svibe sync is in progress for this snapshot; "+
					"svibe does not wait, retry, or break locks")
			return nil, d
		}
		d.Errorf("sync.lock", req.SnapshotRoot, "cannot acquire sync lock: %v", err)
		return nil, d
	}
	defer func() { _ = lk.Release() }()

	// Preconditions must all pass before anything is published.
	if req.VerifyIntegration != nil {
		vd := req.VerifyIntegration()
		d.Extend(vd)
		if vd.HasErrors() {
			return nil, d
		}
	}

	// Stage beside the destination so the final swap is a same-filesystem
	// rename. A temp directory elsewhere could be on another filesystem,
	// turning the atomic swap into a slow copy that can fail halfway.
	staging, err := os.MkdirTemp(req.SnapshotRoot, ".staging-")
	if err != nil {
		d.Errorf("sync.staging", req.SnapshotRoot, "cannot create staging directory: %v", err)
		return nil, d
	}
	defer func() { _ = os.RemoveAll(staging) }()

	stagedSkills := filepath.Join(staging, SkillsDirName)
	if err := os.MkdirAll(stagedSkills, 0o755); err != nil {
		d.Errorf("sync.staging", stagedSkills, "cannot create staging skills directory: %v", err)
		return nil, d
	}

	// Copy complete winning skill directories, including supporting files.
	// Source directories are never symlinked: the snapshot is a point-in-time
	// copy of the resolved state (architecture 13.3).
	for _, s := range req.Resolution.Skills {
		dst := filepath.Join(stagedSkills, s.Name)
		if err := copyTree(s.Dir, dst); err != nil {
			d.Errorf("sync.copy", s.Dir, "cannot materialize skill %q: %v", s.Name, err)
			return nil, d
		}
	}

	syncedAt := time.Now().UTC().Format(time.RFC3339)
	state := State{
		SvibeVersion: req.SvibeVersion,
		Fingerprint:  req.Fingerprint,
		SyncedAt:     syncedAt,
	}
	if err := writeState(filepath.Join(staging, StateFileName), state); err != nil {
		d.Errorf("sync.state", staging, "cannot write sync state: %v", err)
		return nil, d
	}

	// Publish. Everything below this point is designed so a failure leaves
	// the previous snapshot in place.
	liveSkills := filepath.Join(req.SnapshotRoot, SkillsDirName)
	if err := swapDir(stagedSkills, liveSkills); err != nil {
		d.Errorf("sync.publish", liveSkills, "cannot publish snapshot: %v", err)
		return nil, d
	}

	// The state file is written only after the skills are live, so a crash
	// between the two leaves state describing the older snapshot. That reads
	// as stale, which is recoverable; the reverse would claim currency for a
	// snapshot that was never published.
	if err := os.Rename(
		filepath.Join(staging, StateFileName),
		filepath.Join(req.SnapshotRoot, StateFileName),
	); err != nil {
		d.Errorf("sync.state", req.SnapshotRoot, "cannot publish sync state: %v", err)
		return nil, d
	}

	return &Result{
		SnapshotRoot: req.SnapshotRoot,
		SkillsDir:    liveSkills,
		SkillCount:   len(req.Resolution.Skills),
		Fingerprint:  req.Fingerprint,
		SyncedAt:     syncedAt,
	}, d
}

// swapDir replaces live with staged as close to atomically as the platform
// allows.
//
// os.Rename cannot replace a non-empty directory on Windows, and on Unix
// rename(2) only replaces an empty one. So the live tree is renamed aside
// first, the new tree moved into place, and the old tree removed afterwards.
// If the second rename fails, the original is restored.
func swapDir(staged, live string) error {
	parent := filepath.Dir(live)
	retired := ""

	if _, err := os.Lstat(live); err == nil {
		retired = filepath.Join(parent, fmt.Sprintf(".retired-%d", time.Now().UnixNano()))
		if err := os.Rename(live, retired); err != nil {
			return fmt.Errorf("moving previous snapshot aside: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(staged, live); err != nil {
		if retired != "" {
			// Restore the previous snapshot; the caller must see it intact.
			if restoreErr := os.Rename(retired, live); restoreErr != nil {
				return fmt.Errorf(
					"publishing failed (%v) and the previous snapshot could not be "+
						"restored (%v); it remains at %s", err, restoreErr, retired)
			}
		}
		return fmt.Errorf("moving new snapshot into place: %w", err)
	}

	if retired != "" {
		// Best effort: the new snapshot is already live.
		_ = os.RemoveAll(retired)
	}
	return nil
}

func writeState(path string, s State) error {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

// ReadState loads the sync state from a snapshot root.
func ReadState(snapshotRoot string) (*State, error) {
	raw, err := os.ReadFile(filepath.Join(snapshotRoot, StateFileName))
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SkillsDir is the published skills directory for a snapshot root.
func SkillsDir(snapshotRoot string) string {
	return filepath.Join(snapshotRoot, SkillsDirName)
}

// copyTree copies a directory recursively, preserving the executable bit.
func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		t := filepath.Join(dst, e.Name())

		switch {
		case e.IsDir():
			if err := copyTree(s, t); err != nil {
				return err
			}
		case e.Type()&os.ModeSymlink != 0:
			// Skill validation rejects symlinks that escape the skill
			// directory. Materialize the content so the snapshot stands alone.
			if err := copyFile(s, t); err != nil {
				return err
			}
		case e.Type().IsRegular():
			if err := copyFile(s, t); err != nil {
				return err
			}
		default:
			// Devices, sockets and pipes have no meaning in a skill snapshot.
			continue
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	mode := os.FileMode(0o644)
	if info.Mode()&0o111 != 0 {
		mode = 0o755
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	// Close is checked rather than deferred and discarded: on a written file
	// it can report a deferred write error, which would silently corrupt the
	// published snapshot.
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
