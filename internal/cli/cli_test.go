package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mwenkdev/structured-vibe/internal/managed"
	"github.com/mwenkdev/structured-vibe/internal/pack"
	"github.com/mwenkdev/structured-vibe/internal/paths"
)

type runOutput struct {
	stdout string
	stderr string
	err    error
}

// runCLI executes a command with an isolated config root and working dir.
//
// An empty manifest is injected so integrity checks pass trivially; the
// managed-payload policy has its own dedicated tests below.
func runCLI(t *testing.T, configHome, cwd string, args ...string) runOutput {
	t.Helper()
	return runCLIWithManifest(t, configHome, cwd, managed.Manifest{}, args...)
}

// runCLIWithManifest executes a command against an explicit managed manifest.
func runCLIWithManifest(t *testing.T, configHome, cwd string, m managed.Manifest, args ...string) runOutput {
	t.Helper()
	t.Setenv(paths.ConfigHomeEnv, configHome)

	var out, errw bytes.Buffer
	e := &Env{Stdout: &out, Stderr: &errw, Cwd: cwd, Manifest: m}
	err := Run(e, args)
	return runOutput{stdout: out.String(), stderr: errw.String(), err: err}
}

// manifestFor computes a manifest describing the managed files currently
// present beneath configHome, so a constructed installation is self-consistent.
func manifestFor(t *testing.T, configHome string, relPaths ...string) managed.Manifest {
	t.Helper()
	m := managed.Manifest{}
	for _, rel := range relPaths {
		raw, err := os.ReadFile(filepath.Join(configHome, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("manifestFor: %v", err)
		}
		sum := sha256.Sum256(raw)
		m[rel] = hex.EncodeToString(sum[:])
	}
	return m
}

// newRepo creates a directory that ProjectRoot will treat as a repository.
func newRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Resolve symlinks so comparisons match what ProjectRoot returns.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return root
	}
	return resolved
}

// newCore writes a minimal core pack into the config root and returns a
// manifest describing it, so core membership is closed over exactly these
// skills.
func newCore(t *testing.T, configHome string, skillNames ...string) managed.Manifest {
	t.Helper()
	core := paths.CoreDir(configHome)
	if err := os.MkdirAll(core, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "name: structured-vibe-core\nversion: 0.1.0\n"
	if err := os.WriteFile(filepath.Join(core, pack.ManifestName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, n := range skillNames {
		dir := filepath.Join(core, pack.SkillsDir, n)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + n + "\ndescription: Core skill " + n + " for testing.\n---\nbody\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rel := []string{"core/" + pack.ManifestName}
	for _, n := range skillNames {
		rel = append(rel, "core/"+pack.SkillsDir+"/"+n+"/SKILL.md")
	}
	return manifestFor(t, configHome, rel...)
}

func TestInitCreatesProjectPack(t *testing.T) {
	configHome := t.TempDir()
	repo := newRepo(t)

	got := runCLI(t, configHome, repo, "init")
	if got.err != nil {
		t.Fatalf("init failed: %v\n%s", got.err, got.stderr)
	}

	manifest := filepath.Join(repo, paths.ProjectDirName, pack.ManifestName)
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("manifest not created: %v", err)
	}
	// Pack name is derived from the repository directory, starting at 0.1.0.
	if !strings.Contains(string(raw), "version: 0.1.0") {
		t.Errorf("manifest = %q", raw)
	}
	if _, err := os.Stat(filepath.Join(repo, paths.ProjectDirName, pack.SkillsDir)); err != nil {
		t.Errorf("skills directory not created: %v", err)
	}
}

func TestInitAddsGitignoreEntry(t *testing.T) {
	configHome := t.TempDir()
	repo := newRepo(t)

	if got := runCLI(t, configHome, repo, "init"); got.err != nil {
		t.Fatalf("init failed: %v", got.err)
	}

	raw, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil {
		t.Fatalf("gitignore not written: %v", err)
	}
	if !strings.Contains(string(raw), gitignoreEntry) {
		t.Errorf("gitignore = %q, want entry %q", raw, gitignoreEntry)
	}
}

func TestInitPreservesExistingGitignore(t *testing.T) {
	configHome := t.TempDir()
	repo := newRepo(t)

	existing := "node_modules/\nbin/\n"
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := runCLI(t, configHome, repo, "init"); got.err != nil {
		t.Fatalf("init failed: %v", got.err)
	}

	raw, _ := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if !strings.Contains(string(raw), "node_modules/") {
		t.Error("existing gitignore content was lost")
	}
	if !strings.Contains(string(raw), gitignoreEntry) {
		t.Error("generated entry not appended")
	}
}

func TestInitIsIdempotent(t *testing.T) {
	configHome := t.TempDir()
	repo := newRepo(t)

	if got := runCLI(t, configHome, repo, "init"); got.err != nil {
		t.Fatalf("first init failed: %v", got.err)
	}
	second := runCLI(t, configHome, repo, "init", "--json")
	if second.err != nil {
		t.Fatalf("second init failed: %v", second.err)
	}

	var env struct {
		Result struct {
			AlreadyDone bool `json:"already_initialized"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(second.stdout), &env); err != nil {
		t.Fatal(err)
	}
	if !env.Result.AlreadyDone {
		t.Error("second init should report the pack already exists")
	}

	// The gitignore entry must not be duplicated.
	raw, _ := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if n := strings.Count(string(raw), gitignoreEntry); n != 1 {
		t.Errorf("gitignore entry appears %d times", n)
	}
}

// TestInitFailsOutsideRepository: init requires a project root (architecture 6).
func TestInitFailsOutsideRepository(t *testing.T) {
	configHome := t.TempDir()
	notARepo := t.TempDir()

	got := runCLI(t, configHome, notARepo, "init")
	if got.err == nil {
		t.Fatal("expected init to fail outside a git repository")
	}
	if !strings.Contains(got.stderr, "init.no-project") {
		t.Errorf("stderr = %q", got.stderr)
	}
}

// TestResolveWorksOutsideRepository: resolve still works with core and user
// scopes when project scope does not exist.
func TestResolveWorksOutsideRepository(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")
	notARepo := t.TempDir()

	got := runCLIWithManifest(t, configHome, notARepo, m, "resolve", "--json")
	if got.err != nil {
		t.Fatalf("resolve failed: %v\n%s", got.err, got.stderr)
	}

	var env struct {
		OK     bool `json:"ok"`
		Result struct {
			ProjectRoot string `json:"project_root"`
			Skills      []struct {
				Name string `json:"name"`
			} `json:"skills"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &env); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, got.stdout)
	}
	if !env.OK {
		t.Error("expected ok")
	}
	if env.Result.ProjectRoot != "" {
		t.Errorf("project_root = %q, want empty", env.Result.ProjectRoot)
	}
	if len(env.Result.Skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(env.Result.Skills))
	}
}

func TestResolveProjectOverridesCore(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan", "sv-review")
	repo := newRepo(t)

	if got := runCLIWithManifest(t, configHome, repo, m, "init"); got.err != nil {
		t.Fatalf("init failed: %v", got.err)
	}

	// Project-scope replacement of a core workflow skill is normal.
	dir := filepath.Join(repo, paths.ProjectDirName, pack.SkillsDir, "sv-plan")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: sv-plan\ndescription: Project-specific planning skill.\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runCLIWithManifest(t, configHome, repo, m, "resolve", "--json")
	if got.err != nil {
		t.Fatalf("resolve failed: %v\n%s", got.err, got.stderr)
	}

	var env struct {
		Result struct {
			Skills []struct {
				Name     string                   `json:"name"`
				Origin   struct{ Scope string }   `json:"origin"`
				Shadowed []struct{ Scope string } `json:"shadowed"`
			} `json:"skills"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Result.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(env.Result.Skills))
	}
	for _, s := range env.Result.Skills {
		if s.Name != "sv-plan" {
			continue
		}
		if s.Origin.Scope != "project" {
			t.Errorf("sv-plan scope = %q, want project", s.Origin.Scope)
		}
		if len(s.Shadowed) != 1 || s.Shadowed[0].Scope != "core" {
			t.Errorf("sv-plan shadowed = %+v, want core", s.Shadowed)
		}
	}
}

func TestValidateSinglePack(t *testing.T) {
	configHome := t.TempDir()
	packRoot := t.TempDir()

	manifest := "name: standalone\nversion: 1.2.3\n"
	if err := os.WriteFile(filepath.Join(packRoot, pack.ManifestName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(packRoot, pack.SkillsDir, "thing")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: thing\ndescription: A standalone skill.\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runCLI(t, configHome, t.TempDir(), "validate", packRoot, "--json")
	if got.err != nil {
		t.Fatalf("validate failed: %v\n%s", got.err, got.stderr)
	}
	if !strings.Contains(got.stdout, "\"ok\": true") {
		t.Errorf("stdout = %s", got.stdout)
	}
}

func TestValidateSinglePackReportsErrors(t *testing.T) {
	configHome := t.TempDir()
	packRoot := t.TempDir()

	// Invalid SemVer, and a skill whose name does not match its directory.
	if err := os.WriteFile(filepath.Join(packRoot, pack.ManifestName), []byte("name: bad\nversion: 1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(packRoot, pack.SkillsDir, "thing")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: mismatched\ndescription: Wrong name.\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runCLI(t, configHome, t.TempDir(), "validate", packRoot)
	if got.err == nil {
		t.Fatal("expected validation to fail")
	}
	if !strings.Contains(got.stderr, "pack.version.invalid") {
		t.Errorf("missing version error: %q", got.stderr)
	}
	if !strings.Contains(got.stderr, "skill.name.mismatch") {
		t.Errorf("missing name mismatch error: %q", got.stderr)
	}
}

func TestValidateRejectsMultiplePathArguments(t *testing.T) {
	got := runCLI(t, t.TempDir(), t.TempDir(), "validate", "a", "b")
	var exit *ExitError
	if !errors.As(got.err, &exit) || exit.Code != 2 {
		t.Errorf("expected usage exit 2, got %v", got.err)
	}
}

func TestUnknownCommandExitsTwo(t *testing.T) {
	got := runCLI(t, t.TempDir(), t.TempDir(), "definitely-not-a-command")
	var exit *ExitError
	if !errors.As(got.err, &exit) || exit.Code != 2 {
		t.Errorf("expected exit 2, got %v", got.err)
	}
	if !strings.Contains(got.stderr, "unknown command") {
		t.Errorf("stderr = %q", got.stderr)
	}
}

// TestNoWorkflowCommands guards architecture 4.1: the CLI is infrastructure
// and must not grow workflow commands.
func TestNoWorkflowCommands(t *testing.T) {
	forbidden := []string{"plan", "review", "beads", "finalize", "execute", "verify"}
	for _, c := range commands {
		for _, f := range forbidden {
			if c.Name == f {
				t.Errorf("CLI exposes workflow command %q; the workflow belongs in host skills", f)
			}
		}
	}
}

func TestVersionCommand(t *testing.T) {
	got := runCLI(t, t.TempDir(), t.TempDir(), "version")
	if got.err != nil {
		t.Fatalf("version failed: %v", got.err)
	}
	if strings.TrimSpace(got.stdout) == "" {
		t.Error("expected a version on stdout")
	}
}

// TestIntegrityCheckedOnEveryInvocation guards architecture 16.2: the check
// runs before executing any requested command, deliberately including
// innocuous ones.
func TestIntegrityCheckedOnEveryInvocation(t *testing.T) {
	for _, cmd := range []string{"version", "resolve", "validate", "init", "advise"} {
		t.Run(cmd, func(t *testing.T) {
			configHome := t.TempDir()

			// A manifest describing a file that was never installed.
			m := managed.Manifest{"config/models.yaml": strings.Repeat("0", 64)}

			args := []string{cmd}
			if cmd == "advise" {
				args = append(args, "--skill", "x")
			}
			got := runCLIWithManifest(t, configHome, newRepo(t), m, args...)

			if got.err == nil {
				t.Fatalf("%s ran despite an incomplete installation", cmd)
			}
			if !strings.Contains(got.stderr, "managed.missing") {
				t.Errorf("stderr = %q", got.stderr)
			}
			if got.stdout != "" {
				t.Errorf("no output should be produced: %q", got.stdout)
			}
		})
	}
}

// TestModifiedManagedFileWarnsInEnvelope: modification warns but the command
// still runs, and the warning reaches the JSON envelope.
func TestModifiedManagedFileWarnsInEnvelope(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")

	// Tamper with a managed file after the manifest was computed.
	skillPath := filepath.Join(configHome, "core", pack.SkillsDir, "sv-plan", "SKILL.md")
	tampered := "---\nname: sv-plan\ndescription: Locally modified description.\n---\nchanged\n"
	if err := os.WriteFile(skillPath, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runCLIWithManifest(t, configHome, newRepo(t), m, "resolve", "--json")
	if got.err != nil {
		t.Fatalf("a modified managed file must not fail the command: %v", got.err)
	}

	var env struct {
		OK       bool `json:"ok"`
		Warnings []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Error("ok should remain true: warnings are advisory")
	}

	var found bool
	for _, w := range env.Warnings {
		if w.Code == "managed.modified" {
			found = true
			if !strings.Contains(w.Message, "unsupported") {
				t.Errorf("warning should say local changes are unsupported: %q", w.Message)
			}
		}
	}
	if !found {
		t.Errorf("managed.modified warning missing from envelope: %+v", env.Warnings)
	}
}

// TestExtraCoreSkillDirectoryNotDiscovered guards closed core membership:
// dropping a SKILL.md directory into the installed core tree creates nothing.
func TestExtraCoreSkillDirectoryNotDiscovered(t *testing.T) {
	configHome := t.TempDir()
	m := newCore(t, configHome, "sv-plan")

	intruder := filepath.Join(configHome, "core", pack.SkillsDir, "sneaky-skill")
	if err := os.MkdirAll(intruder, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: sneaky-skill\ndescription: Added by hand to the installed core tree.\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(intruder, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runCLIWithManifest(t, configHome, newRepo(t), m, "resolve", "--json")
	if got.err != nil {
		t.Fatalf("resolve failed: %v\n%s", got.err, got.stderr)
	}

	var env struct {
		Result struct {
			Skills []struct {
				Name string `json:"name"`
			} `json:"skills"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &env); err != nil {
		t.Fatal(err)
	}
	for _, s := range env.Result.Skills {
		if s.Name == "sneaky-skill" {
			t.Fatal("an unlisted directory must not become a core skill")
		}
	}
	if len(env.Result.Skills) != 1 {
		t.Errorf("expected only sv-plan, got %+v", env.Result.Skills)
	}
}
