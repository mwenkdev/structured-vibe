package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/hostint"
	"github.com/mwenkdev/structured-vibe/internal/managed"
	"github.com/mwenkdev/structured-vibe/internal/paths"
	"github.com/mwenkdev/structured-vibe/internal/sync"
)

// runHost executes a command with the host integration reported as installed.
func runHost(t *testing.T, configHome, cwd string, m managed.Manifest, args ...string) runOutput {
	t.Helper()
	t.Setenv(paths.ConfigHomeEnv, configHome)

	var out, errw strings.Builder
	e := &Env{
		Stdout: &out, Stderr: &errw, Cwd: cwd, Manifest: m,
		VerifyIntegration: func() diag.Diagnostics { return diag.Diagnostics{} },
	}
	err := Run(e, args)
	return runOutput{stdout: out.String(), stderr: errw.String(), err: err}
}

func snapshotSkills(projectRoot string) string {
	return sync.SkillsDir(paths.OpenCodeSnapshotDir(projectRoot))
}

func TestSyncPublishesSnapshot(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan", "sv-review")
	repo := newRepo(t)

	if got := runHost(t, configHome, repo, m, "init"); got.err != nil {
		t.Fatalf("init: %v\n%s", got.err, got.stderr)
	}

	got := runHost(t, configHome, repo, m, "sync", "--json")
	if got.err != nil {
		t.Fatalf("sync: %v\n%s", got.err, got.stderr)
	}

	var env struct {
		OK     bool `json:"ok"`
		Result struct {
			SkillCount      int    `json:"skill_count"`
			SkillsDir       string `json:"skills_dir"`
			RestartRequired bool   `json:"restart_required"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &env); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, got.stdout)
	}
	if !env.OK || env.Result.SkillCount != 2 {
		t.Errorf("result = %+v", env.Result)
	}

	// The snapshot must land where the project config points the host.
	for _, name := range []string{"sv-plan", "sv-review"} {
		p := filepath.Join(snapshotSkills(repo), name, "SKILL.md")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s not published: %v", name, err)
		}
	}
}

// TestSyncTellsUserToRestart: the host memoizes skill discovery at startup
// and never hot-reloads, so a mid-session sync cannot take effect.
func TestSyncTellsUserToRestart(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")
	repo := newRepo(t)

	runHost(t, configHome, repo, m, "init")
	got := runHost(t, configHome, repo, m, "sync")
	if got.err != nil {
		t.Fatalf("sync: %v\n%s", got.err, got.stderr)
	}
	if !strings.Contains(strings.ToLower(got.stdout), "restart") {
		t.Errorf("sync must tell the user to restart OpenCode:\n%s", got.stdout)
	}
}

// TestSyncFailsWithoutIntegration: a missing integration is a hard failure.
func TestSyncFailsWithoutIntegration(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")
	repo := newRepo(t)
	home := t.TempDir() // no plugin installed

	t.Setenv(paths.ConfigHomeEnv, configHome)
	runHost(t, configHome, repo, m, "init")

	var out, errw strings.Builder
	e := &Env{
		Stdout: &out, Stderr: &errw, Cwd: repo, Manifest: m,
		VerifyIntegration: func() diag.Diagnostics { return hostint.VerifyIntegration(home) },
	}
	if err := Run(e, []string{"sync"}); err == nil {
		t.Fatal("sync should fail without the OpenCode integration")
	}
	if !strings.Contains(errw.String(), "hostint.missing") {
		t.Errorf("stderr = %q", errw.String())
	}
	if _, err := os.Stat(snapshotSkills(repo)); err == nil {
		t.Error("nothing should have been published")
	}
}

func TestSyncSnapshotIsRegisteredPath(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")
	repo := newRepo(t)

	runHost(t, configHome, repo, m, "init")
	runHost(t, configHome, repo, m, "sync")

	// What init registered must be exactly where sync published.
	registered := filepath.Join(repo, filepath.FromSlash(paths.OpenCodeSkillsRelPath))
	published := snapshotSkills(repo)
	if registered != published {
		t.Errorf("registered %q but published %q", registered, published)
	}
	if _, err := os.Stat(filepath.Join(registered, "sv-plan", "SKILL.md")); err != nil {
		t.Errorf("host would not find the skill: %v", err)
	}
}

func TestStatusMissingBeforeSync(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")
	repo := newRepo(t)

	got := runHost(t, configHome, repo, m, "status", "--json")
	if got.err != nil {
		t.Fatalf("status: %v\n%s", got.err, got.stderr)
	}
	if state := statusState(t, got.stdout); state != stateMissing {
		t.Errorf("state = %q, want %q", state, stateMissing)
	}
}

func TestStatusCurrentAfterSync(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")
	repo := newRepo(t)

	runHost(t, configHome, repo, m, "init")
	runHost(t, configHome, repo, m, "sync")

	got := runHost(t, configHome, repo, m, "status", "--json")
	if got.err != nil {
		t.Fatalf("status: %v\n%s", got.err, got.stderr)
	}
	if state := statusState(t, got.stdout); state != stateCurrent {
		t.Errorf("state = %q, want %q", state, stateCurrent)
	}
}

// TestStatusStaleAfterSourceEdit uses content, never timestamps.
func TestStatusStaleAfterSourceEdit(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")
	repo := newRepo(t)

	runHost(t, configHome, repo, m, "init")
	runHost(t, configHome, repo, m, "sync")

	// Edit the source skill. The manifest must be recomputed to match, since
	// integrity is separate from freshness.
	skillPath := filepath.Join(configHome, "core", "skills", "sv-plan", "SKILL.md")
	body := "---\nname: sv-plan\ndescription: An edited description for staleness testing.\n---\nnew body\n"
	if err := os.WriteFile(skillPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m2 := manifestFor(t, configHome, "core/structured-vibe.yaml", "core/skills/sv-plan/SKILL.md")

	got := runHost(t, configHome, repo, m2, "status", "--json")
	if got.err != nil {
		t.Fatalf("status: %v\n%s", got.err, got.stderr)
	}
	if state := statusState(t, got.stdout); state != stateStale {
		t.Errorf("state = %q, want %q", state, stateStale)
	}
}

// TestStatusStaleIsNotFailure: staleness is informational state.
func TestStatusStaleIsNotFailure(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")
	repo := newRepo(t)

	runHost(t, configHome, repo, m, "init")
	runHost(t, configHome, repo, m, "sync")

	skillPath := filepath.Join(configHome, "core", "skills", "sv-plan", "SKILL.md")
	body := "---\nname: sv-plan\ndescription: Another edited description here.\n---\nx\n"
	os.WriteFile(skillPath, []byte(body), 0o644)
	m2 := manifestFor(t, configHome, "core/structured-vibe.yaml", "core/skills/sv-plan/SKILL.md")

	if got := runHost(t, configHome, repo, m2, "status"); got.err != nil {
		t.Errorf("stale status must not fail the command: %v", got.err)
	}
}

// TestStatusInvalidWhenStateFileMissing: skills published with no state file
// means freshness cannot be established.
func TestStatusInvalidWhenStateFileMissing(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")
	repo := newRepo(t)

	runHost(t, configHome, repo, m, "init")
	runHost(t, configHome, repo, m, "sync")

	stateFile := filepath.Join(paths.OpenCodeSnapshotDir(repo), sync.StateFileName)
	if err := os.Remove(stateFile); err != nil {
		t.Fatal(err)
	}

	got := runHost(t, configHome, repo, m, "status", "--json")
	if got.err != nil {
		t.Fatalf("status: %v", got.err)
	}
	if state := statusState(t, got.stdout); state != stateInvalid {
		t.Errorf("state = %q, want %q", state, stateInvalid)
	}
}

// TestStatusReportsVersionDrift: the CLI version is the rules version, so a
// snapshot from a different svibe is stale even with identical inputs.
func TestStatusReportsVersionDrift(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")
	repo := newRepo(t)

	runHost(t, configHome, repo, m, "init")
	runHost(t, configHome, repo, m, "sync")

	// Rewrite the state as if produced by a different svibe.
	root := paths.OpenCodeSnapshotDir(repo)
	st, err := sync.ReadState(root)
	if err != nil {
		t.Fatal(err)
	}
	st.SvibeVersion = "9.9.9-other"
	raw, _ := json.MarshalIndent(st, "", "  ")
	if err := os.WriteFile(filepath.Join(root, sync.StateFileName), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	got := runHost(t, configHome, repo, m, "status", "--json")
	if got.err != nil {
		t.Fatalf("status: %v", got.err)
	}

	var env struct {
		Result struct {
			State        string `json:"state"`
			VersionDrift bool   `json:"version_drift"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &env); err != nil {
		t.Fatal(err)
	}
	if !env.Result.VersionDrift {
		t.Error("version drift not reported")
	}
	if env.Result.State != stateStale {
		t.Errorf("state = %q, want %q: a rules-version change can change resolution",
			env.Result.State, stateStale)
	}
}

// TestStatusWarnsWhenSnapshotNotRegistered: a synced snapshot the host is not
// pointed at is invisible.
func TestStatusWarnsWhenSnapshotNotRegistered(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")
	repo := newRepo(t)

	// Sync without init, so nothing registers the path.
	runHost(t, configHome, repo, m, "sync")

	got := runHost(t, configHome, repo, m, "status", "--json")
	if got.err != nil {
		t.Fatalf("status: %v", got.err)
	}
	if !strings.Contains(got.stderr, "status.not-registered") {
		t.Errorf("expected a not-registered warning, stderr = %q", got.stderr)
	}
}

func TestSyncRejectsPositionalArguments(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")
	got := runHost(t, configHome, newRepo(t), m, "sync", "extra")
	if got.err == nil {
		t.Error("expected usage failure")
	}
}

func statusState(t *testing.T, stdout string) string {
	t.Helper()
	var env struct {
		Result struct {
			State string `json:"state"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout)
	}
	return env.Result.State
}

// TestSyncBeforeInitDoesNotBreakLaterCommands is a regression test.
//
// sync creates .structured-vibe/ to hold generated output. If the presence of
// that directory were taken to mean "a project pack exists", every later
// command would fail on the missing manifest, so running sync before init
// would brick the tool for that repository.
func TestSyncBeforeInitDoesNotBreakLaterCommands(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")
	repo := newRepo(t)

	if got := runHost(t, configHome, repo, m, "sync"); got.err != nil {
		t.Fatalf("sync before init failed: %v\n%s", got.err, got.stderr)
	}

	// The directory now exists but contains no manifest.
	packDir := filepath.Join(repo, paths.ProjectDirName)
	if _, err := os.Stat(packDir); err != nil {
		t.Fatalf("expected %s to exist after sync: %v", packDir, err)
	}

	for _, cmd := range []string{"resolve", "validate", "status", "sync"} {
		got := runHost(t, configHome, repo, m, cmd)
		if got.err != nil {
			t.Errorf("%s failed after sync-before-init: %v\n%s", cmd, got.err, got.stderr)
		}
	}

	// init must still work afterwards.
	if got := runHost(t, configHome, repo, m, "init"); got.err != nil {
		t.Errorf("init after sync failed: %v\n%s", got.err, got.stderr)
	}
}

// TestEmptyProjectDirIsNotAPack covers the same rule directly.
func TestEmptyProjectDirIsNotAPack(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")
	repo := newRepo(t)

	if err := os.MkdirAll(filepath.Join(repo, paths.ProjectDirName), 0o755); err != nil {
		t.Fatal(err)
	}

	got := runHost(t, configHome, repo, m, "resolve", "--json")
	if got.err != nil {
		t.Fatalf("a manifest-less .structured-vibe must not be treated as a pack: %v\n%s",
			got.err, got.stderr)
	}
}
