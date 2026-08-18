package resolve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/hostskills"
	"github.com/mwenkdev/structured-vibe/internal/pack"
	"github.com/mwenkdev/structured-vibe/internal/scope"
	"github.com/mwenkdev/structured-vibe/internal/skill"
)

// mkPack builds an in-memory pack with the named skills.
func mkPack(name string, sc scope.Scope, skillNames ...string) *pack.Pack {
	p := &pack.Pack{
		Manifest: pack.Manifest{Name: name, Version: "0.1.0"},
		Root:     "/packs/" + name,
		Scope:    sc,
	}
	for _, sn := range skillNames {
		p.Skills = append(p.Skills, skill.Skill{
			Name:        sn,
			Description: "description for " + sn,
			Dir:         "/packs/" + name + "/skills/" + sn,
			Path:        "/packs/" + name + "/skills/" + sn + "/SKILL.md",
		})
	}
	return p
}

func TestPrecedenceProjectWinsOverUserOverCore(t *testing.T) {
	in := Input{Packs: []*pack.Pack{
		mkPack("core-pack", scope.Core, "security-review"),
		mkPack("user-pack", scope.User, "security-review"),
		mkPack("project-pack", scope.Project, "security-review"),
	}}

	r, d := Resolve(in)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if len(r.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(r.Skills))
	}

	got := r.Skills[0]
	if got.Origin.Scope != "project" || got.Origin.Pack != "project-pack" {
		t.Errorf("winner = %+v, want project-pack", got.Origin)
	}

	// The replacement is complete, but the shadowed definitions are recorded.
	if len(got.Shadowed) != 2 {
		t.Fatalf("expected 2 shadowed, got %+v", got.Shadowed)
	}
	if got.Shadowed[0].Scope != "user" || got.Shadowed[1].Scope != "core" {
		t.Errorf("shadowed order = %+v, want user then core", got.Shadowed)
	}
}

func TestUserWinsOverCore(t *testing.T) {
	in := Input{Packs: []*pack.Pack{
		mkPack("core-pack", scope.Core, "sv-plan"),
		mkPack("user-pack", scope.User, "sv-plan"),
	}}
	r, d := Resolve(in)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if r.Skills[0].Origin.Scope != "user" {
		t.Errorf("winner scope = %q", r.Skills[0].Origin.Scope)
	}
	// Replacing a core workflow skill is normal and warrants no warning.
	if len(d.Warnings()) != 0 {
		t.Errorf("unexpected warnings: %+v", d.Warnings())
	}
}

// TestSameScopeAmbiguityIsHardError is the invariant that stops svibe from
// ever resolving a collision by filesystem, alphabetical, or last-write order.
func TestSameScopeAmbiguityIsHardError(t *testing.T) {
	in := Input{Packs: []*pack.Pack{
		mkPack("mike-general", scope.User, "java-backend"),
		mkPack("mike-experiments", scope.User, "java-backend"),
	}}

	r, d := Resolve(in)
	if r != nil {
		t.Error("expected no resolution")
	}
	if !hasCode(d.Errors(), "resolve.ambiguous") {
		t.Errorf("want resolve.ambiguous, got %+v", d.Errors())
	}
}

// TestAmbiguityInShadowedScopeStillFails: an ambiguous lower scope is a
// broken environment even when a higher scope would have won.
func TestAmbiguityInShadowedScopeStillFails(t *testing.T) {
	in := Input{Packs: []*pack.Pack{
		mkPack("user-a", scope.User, "thing"),
		mkPack("user-b", scope.User, "thing"),
		mkPack("project-pack", scope.Project, "thing"),
	}}

	r, d := Resolve(in)
	if r != nil {
		t.Error("expected no resolution")
	}
	if !hasCode(d.Errors(), "resolve.ambiguous") {
		t.Errorf("want resolve.ambiguous, got %+v", d.Errors())
	}
}

func TestDuplicatePackNameIsError(t *testing.T) {
	in := Input{Packs: []*pack.Pack{
		mkPack("same-name", scope.User, "a"),
		mkPack("same-name", scope.Project, "b"),
	}}

	r, d := Resolve(in)
	if r != nil {
		t.Error("expected no resolution")
	}
	if !hasCode(d.Errors(), "resolve.pack.duplicate") {
		t.Errorf("want resolve.pack.duplicate, got %+v", d.Errors())
	}
}

func TestSkillsAreSortedByName(t *testing.T) {
	in := Input{Packs: []*pack.Pack{
		mkPack("p", scope.Core, "zebra", "alpha", "middle"),
	}}
	r, d := Resolve(in)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	want := []string{"alpha", "middle", "zebra"}
	for i, w := range want {
		if r.Skills[i].Name != w {
			t.Errorf("skills[%d] = %q, want %q", i, r.Skills[i].Name, w)
		}
	}
}

func TestDisjointSkillsAllSurvive(t *testing.T) {
	in := Input{Packs: []*pack.Pack{
		mkPack("core-pack", scope.Core, "a", "b"),
		mkPack("user-pack", scope.User, "c"),
		mkPack("project-pack", scope.Project, "d"),
	}}
	r, d := Resolve(in)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if len(r.Skills) != 4 {
		t.Errorf("expected 4 skills, got %d", len(r.Skills))
	}
	for _, s := range r.Skills {
		if len(s.Shadowed) != 0 {
			t.Errorf("skill %q should shadow nothing", s.Name)
		}
	}
}

// TestHostCollisionWarns covers the host behavior that makes duplicate skill
// names nondeterministic: the loader warns and overwrites at unbounded
// concurrency, so svibe surfaces the collision rather than resolving it.
func TestHostCollisionWarns(t *testing.T) {
	projectRoot := t.TempDir()

	// A skill the host discovers on its own, outside any svibe pack.
	agentSkill := filepath.Join(projectRoot, ".agents", "skills", "java-backend")
	if err := os.MkdirAll(agentSkill, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: java-backend\ndescription: A competing definition.\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(agentSkill, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	in := Input{
		Packs:       []*pack.Pack{mkPack("core-pack", scope.Core, "java-backend", "testing")},
		ProjectRoot: projectRoot,
	}

	r, d := Resolve(in)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if r == nil {
		t.Fatal("collision must be a warning, not a failure")
	}
	if !hasCode(d.Warnings(), "resolve.host-collision") {
		t.Errorf("want resolve.host-collision warning, got %+v", d.Warnings())
	}
	// Only the colliding skill warns.
	if n := len(d.Warnings()); n != 1 {
		t.Errorf("expected exactly 1 warning, got %d: %+v", n, d.Warnings())
	}
}

func TestHostCollisionExcludesGeneratedSnapshot(t *testing.T) {
	projectRoot := t.TempDir()

	// The generated snapshot is svibe's own output and must not be reported
	// as colliding with itself.
	generated := filepath.Join(projectRoot, ".structured-vibe", "generated")
	snapshot := filepath.Join(generated, "opencode", "skills", "java-backend")
	if err := os.MkdirAll(snapshot, 0o755); err != nil {
		t.Fatal(err)
	}

	in := Input{
		Packs:        []*pack.Pack{mkPack("core-pack", scope.Core, "java-backend")},
		ProjectRoot:  projectRoot,
		ExcludeRoots: []string{generated},
	}

	_, d := Resolve(in)
	if len(d.Warnings()) != 0 {
		t.Errorf("unexpected warnings: %+v", d.Warnings())
	}
}

func TestNoPacksResolvesEmpty(t *testing.T) {
	r, d := Resolve(Input{})
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if len(r.Skills) != 0 {
		t.Errorf("expected no skills")
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

// TestCollisionAcrossConfiguredSkillPaths covers a second skills.paths entry
// serving the same skill name.
//
// The host scans every configured path and resolves duplicate names by race,
// so a collision between two configured paths is exactly as nondeterministic
// as one against .claude or .agents.
func TestCollisionAcrossConfiguredSkillPaths(t *testing.T) {
	other := t.TempDir()

	// The host scans configured paths recursively, so a skill nested below
	// the configured root still collides.
	dir := filepath.Join(other, "group", "java-backend")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: java-backend\ndescription: A competing definition.\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	in := Input{
		Packs:       []*pack.Pack{mkPack("core-pack", scope.Core, "java-backend", "testing")},
		ProjectRoot: t.TempDir(),
		ExtraSkillDirs: []hostskills.ExtraDir{
			{Dir: other, Origin: "opencode skills.paths entry", Recursive: true},
		},
	}

	r, d := Resolve(in)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if r == nil {
		t.Fatal("a collision must warn, not fail")
	}
	if !hasCode(d.Warnings(), "resolve.host-collision") {
		t.Errorf("want resolve.host-collision, got %+v", d.Warnings())
	}
	if n := len(d.Warnings()); n != 1 {
		t.Errorf("only the colliding skill should warn, got %d: %+v", n, d.Warnings())
	}
}

// TestNoCollisionWhenConfiguredPathIsDisjoint keeps the warning from becoming
// noise.
func TestNoCollisionWhenConfiguredPathIsDisjoint(t *testing.T) {
	other := t.TempDir()
	dir := filepath.Join(other, "unrelated-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: unrelated-skill\ndescription: Does not collide.\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	in := Input{
		Packs:       []*pack.Pack{mkPack("core-pack", scope.Core, "java-backend")},
		ProjectRoot: t.TempDir(),
		ExtraSkillDirs: []hostskills.ExtraDir{
			{Dir: other, Origin: "opencode skills.paths entry", Recursive: true},
		},
	}

	_, d := Resolve(in)
	if len(d.Warnings()) != 0 {
		t.Errorf("unexpected warnings: %+v", d.Warnings())
	}
}
