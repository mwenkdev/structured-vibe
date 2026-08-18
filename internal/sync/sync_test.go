package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/lock"
	"github.com/mwenkdev/structured-vibe/internal/resolve"
)

// srcSkill builds a source skill directory and returns a Resolved for it.
func srcSkill(t *testing.T, root, name string, files map[string]string) resolve.Resolved {
	t.Helper()
	dir := filepath.Join(root, name)
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return resolve.Resolved{
		Name:   name,
		Origin: resolve.Origin{Scope: "core", Pack: "core-pack"},
		Dir:    dir,
	}
}

func okIntegration() diag.Diagnostics { return diag.Diagnostics{} }

func TestPublishesSnapshot(t *testing.T) {
	src := t.TempDir()
	snapshot := filepath.Join(t.TempDir(), "opencode")

	r := &resolve.Resolution{Skills: []resolve.Resolved{
		srcSkill(t, src, "sv-plan", map[string]string{"SKILL.md": "plan"}),
		srcSkill(t, src, "sv-review", map[string]string{"SKILL.md": "review"}),
	}}

	res, d := Run(Request{
		SnapshotRoot:      snapshot,
		Resolution:        r,
		Fingerprint:       "fp-1",
		SvibeVersion:      "1.0.0",
		VerifyIntegration: okIntegration,
	})
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if res.SkillCount != 2 {
		t.Errorf("skill count = %d", res.SkillCount)
	}

	for _, name := range []string{"sv-plan", "sv-review"} {
		p := filepath.Join(snapshot, SkillsDirName, name, "SKILL.md")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s not published: %v", name, err)
		}
	}

	state, err := ReadState(snapshot)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.Fingerprint != "fp-1" || state.SvibeVersion != "1.0.0" {
		t.Errorf("state = %+v", state)
	}
}

// TestCopiesSupportingFiles: sync copies complete skill directories.
func TestCopiesSupportingFiles(t *testing.T) {
	src := t.TempDir()
	snapshot := filepath.Join(t.TempDir(), "opencode")

	r := &resolve.Resolution{Skills: []resolve.Resolved{
		srcSkill(t, src, "thing", map[string]string{
			"SKILL.md":              "body",
			"references/notes.md":   "notes",
			"templates/example.txt": "template",
			"scripts/run.sh":        "#!/bin/sh\n",
		}),
	}}

	if _, d := Run(Request{
		SnapshotRoot: snapshot, Resolution: r, Fingerprint: "fp",
		SvibeVersion: "1.0.0", VerifyIntegration: okIntegration,
	}); d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}

	for _, rel := range []string{
		"SKILL.md", "references/notes.md", "templates/example.txt", "scripts/run.sh",
	} {
		p := filepath.Join(snapshot, SkillsDirName, "thing", filepath.FromSlash(rel))
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s not materialized: %v", rel, err)
		}
	}
}

// TestDoesNotSymlinkSources: the snapshot is a point-in-time copy, so editing
// the source afterwards must not change what the host sees.
func TestDoesNotSymlinkSources(t *testing.T) {
	src := t.TempDir()
	snapshot := filepath.Join(t.TempDir(), "opencode")

	skill := srcSkill(t, src, "thing", map[string]string{"SKILL.md": "original"})
	r := &resolve.Resolution{Skills: []resolve.Resolved{skill}}

	if _, d := Run(Request{
		SnapshotRoot: snapshot, Resolution: r, Fingerprint: "fp",
		SvibeVersion: "1.0.0", VerifyIntegration: okIntegration,
	}); d.HasErrors() {
		t.Fatal(d.Errors())
	}

	published := filepath.Join(snapshot, SkillsDirName, "thing", "SKILL.md")

	info, err := os.Lstat(published)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("published skill must not be a symlink")
	}

	// Mutate the source; the snapshot must be unaffected.
	if err := os.WriteFile(filepath.Join(skill.Dir, "SKILL.md"), []byte("mutated"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(published)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "original" {
		t.Errorf("snapshot tracked the source: %q", raw)
	}
}

// TestFailedIntegrationLeavesPreviousSnapshotIntact is the transactional
// invariant: a failed sync must not change live state.
func TestFailedIntegrationLeavesPreviousSnapshotIntact(t *testing.T) {
	src := t.TempDir()
	snapshot := filepath.Join(t.TempDir(), "opencode")

	first := &resolve.Resolution{Skills: []resolve.Resolved{
		srcSkill(t, src, "original-skill", map[string]string{"SKILL.md": "v1"}),
	}}
	if _, d := Run(Request{
		SnapshotRoot: snapshot, Resolution: first, Fingerprint: "fp-1",
		SvibeVersion: "1.0.0", VerifyIntegration: okIntegration,
	}); d.HasErrors() {
		t.Fatal(d.Errors())
	}

	// A second sync whose integration precondition fails.
	second := &resolve.Resolution{Skills: []resolve.Resolved{
		srcSkill(t, src, "replacement-skill", map[string]string{"SKILL.md": "v2"}),
	}}
	res, d := Run(Request{
		SnapshotRoot: snapshot, Resolution: second, Fingerprint: "fp-2",
		SvibeVersion: "1.0.0",
		VerifyIntegration: func() diag.Diagnostics {
			var vd diag.Diagnostics
			vd.Errorf("hostint.missing", "", "integration not installed")
			return vd
		},
	})
	if res != nil || !d.HasErrors() {
		t.Fatal("sync should have failed")
	}

	// The previous snapshot must be untouched.
	if _, err := os.Stat(filepath.Join(snapshot, SkillsDirName, "original-skill", "SKILL.md")); err != nil {
		t.Errorf("previous snapshot was damaged: %v", err)
	}
	if _, err := os.Stat(filepath.Join(snapshot, SkillsDirName, "replacement-skill")); err == nil {
		t.Error("failed sync published content")
	}

	state, err := ReadState(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if state.Fingerprint != "fp-1" {
		t.Errorf("state advanced on a failed sync: %+v", state)
	}
}

// TestFailedCopyLeavesPreviousSnapshotIntact covers failure during
// materialization rather than during preconditions.
func TestFailedCopyLeavesPreviousSnapshotIntact(t *testing.T) {
	src := t.TempDir()
	snapshot := filepath.Join(t.TempDir(), "opencode")

	first := &resolve.Resolution{Skills: []resolve.Resolved{
		srcSkill(t, src, "original-skill", map[string]string{"SKILL.md": "v1"}),
	}}
	if _, d := Run(Request{
		SnapshotRoot: snapshot, Resolution: first, Fingerprint: "fp-1",
		SvibeVersion: "1.0.0", VerifyIntegration: okIntegration,
	}); d.HasErrors() {
		t.Fatal(d.Errors())
	}

	// A skill whose source directory does not exist.
	broken := &resolve.Resolution{Skills: []resolve.Resolved{{
		Name:   "broken",
		Origin: resolve.Origin{Scope: "core", Pack: "core-pack"},
		Dir:    filepath.Join(src, "does-not-exist"),
	}}}
	res, d := Run(Request{
		SnapshotRoot: snapshot, Resolution: broken, Fingerprint: "fp-2",
		SvibeVersion: "1.0.0", VerifyIntegration: okIntegration,
	})
	if res != nil || !d.HasErrors() {
		t.Fatal("sync should have failed")
	}

	if _, err := os.Stat(filepath.Join(snapshot, SkillsDirName, "original-skill", "SKILL.md")); err != nil {
		t.Errorf("previous snapshot was damaged: %v", err)
	}
}

func TestReplacesPreviousSnapshotContents(t *testing.T) {
	src := t.TempDir()
	snapshot := filepath.Join(t.TempDir(), "opencode")

	first := &resolve.Resolution{Skills: []resolve.Resolved{
		srcSkill(t, src, "removed-later", map[string]string{"SKILL.md": "gone"}),
	}}
	if _, d := Run(Request{
		SnapshotRoot: snapshot, Resolution: first, Fingerprint: "fp-1",
		SvibeVersion: "1.0.0", VerifyIntegration: okIntegration,
	}); d.HasErrors() {
		t.Fatal(d.Errors())
	}

	second := &resolve.Resolution{Skills: []resolve.Resolved{
		srcSkill(t, src, "kept", map[string]string{"SKILL.md": "here"}),
	}}
	if _, d := Run(Request{
		SnapshotRoot: snapshot, Resolution: second, Fingerprint: "fp-2",
		SvibeVersion: "1.0.0", VerifyIntegration: okIntegration,
	}); d.HasErrors() {
		t.Fatal(d.Errors())
	}

	// A skill that no longer resolves must not linger in the snapshot.
	if _, err := os.Stat(filepath.Join(snapshot, SkillsDirName, "removed-later")); err == nil {
		t.Error("stale skill survived republication")
	}
	if _, err := os.Stat(filepath.Join(snapshot, SkillsDirName, "kept", "SKILL.md")); err != nil {
		t.Errorf("new skill missing: %v", err)
	}
}

// TestHeldLockFailsImmediately: a concurrent sync fails rather than waiting.
func TestHeldLockFailsImmediately(t *testing.T) {
	src := t.TempDir()
	snapshot := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(snapshot, 0o755); err != nil {
		t.Fatal(err)
	}

	held, err := lock.Acquire(filepath.Join(snapshot, LockFileName))
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()

	r := &resolve.Resolution{Skills: []resolve.Resolved{
		srcSkill(t, src, "thing", map[string]string{"SKILL.md": "x"}),
	}}
	res, d := Run(Request{
		SnapshotRoot: snapshot, Resolution: r, Fingerprint: "fp",
		SvibeVersion: "1.0.0", VerifyIntegration: okIntegration,
	})
	if res != nil {
		t.Fatal("sync should have failed while the lock was held")
	}
	if !hasCode(d.Errors(), "sync.locked") {
		t.Errorf("want sync.locked, got %+v", d.Errors())
	}
}

// TestIntegrationVerifiedBeforePublication: the precondition must run before
// anything is staged or published.
func TestIntegrationVerifiedBeforePublication(t *testing.T) {
	src := t.TempDir()
	snapshot := filepath.Join(t.TempDir(), "opencode")

	called := false
	r := &resolve.Resolution{Skills: []resolve.Resolved{
		srcSkill(t, src, "thing", map[string]string{"SKILL.md": "x"}),
	}}
	Run(Request{
		SnapshotRoot: snapshot, Resolution: r, Fingerprint: "fp",
		SvibeVersion: "1.0.0",
		VerifyIntegration: func() diag.Diagnostics {
			called = true
			// Nothing may be published at this point.
			if _, err := os.Stat(filepath.Join(snapshot, SkillsDirName)); err == nil {
				t.Error("skills were published before the integration was verified")
			}
			return diag.Diagnostics{}
		},
	})
	if !called {
		t.Error("integration verification was skipped")
	}
}

func TestNoStagingResiduePersists(t *testing.T) {
	src := t.TempDir()
	snapshot := filepath.Join(t.TempDir(), "opencode")

	r := &resolve.Resolution{Skills: []resolve.Resolved{
		srcSkill(t, src, "thing", map[string]string{"SKILL.md": "x"}),
	}}
	if _, d := Run(Request{
		SnapshotRoot: snapshot, Resolution: r, Fingerprint: "fp",
		SvibeVersion: "1.0.0", VerifyIntegration: okIntegration,
	}); d.HasErrors() {
		t.Fatal(d.Errors())
	}

	entries, err := os.ReadDir(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if len(name) >= 9 && name[:9] == ".staging-" {
			t.Errorf("staging directory left behind: %s", name)
		}
		if len(name) >= 9 && name[:9] == ".retired-" {
			t.Errorf("retired directory left behind: %s", name)
		}
	}
}

func TestNilResolutionIsAnError(t *testing.T) {
	_, d := Run(Request{SnapshotRoot: t.TempDir(), SvibeVersion: "1.0.0"})
	if !d.HasErrors() {
		t.Error("expected an error for a nil resolution")
	}
}

func TestEmptyResolutionPublishesEmptySnapshot(t *testing.T) {
	snapshot := filepath.Join(t.TempDir(), "opencode")
	res, d := Run(Request{
		SnapshotRoot: snapshot, Resolution: &resolve.Resolution{},
		Fingerprint: "fp", SvibeVersion: "1.0.0", VerifyIntegration: okIntegration,
	})
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if res.SkillCount != 0 {
		t.Errorf("skill count = %d", res.SkillCount)
	}
	if _, err := os.Stat(SkillsDir(snapshot)); err != nil {
		t.Errorf("skills directory should exist even when empty: %v", err)
	}
}

func hasCode(d diag.Diagnostics, code string) bool {
	for _, x := range d {
		if x.Code == code {
			return true
		}
	}
	return false
}
