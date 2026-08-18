package fingerprint

import (
	"os"
	"path/filepath"
	"testing"
)

// skillDir builds a skill directory containing the given files.
func skillDir(t *testing.T, root, name string, files map[string]string) string {
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
	return dir
}

func baseInput(t *testing.T) (Input, string) {
	t.Helper()
	root := t.TempDir()
	dir := skillDir(t, root, "alpha", map[string]string{
		"SKILL.md":            "alpha body",
		"references/notes.md": "notes",
	})
	return Input{
		SvibeVersion: "1.0.0",
		Scopes:       []string{"core", "user", "project"},
		Packs:        []string{"core:core-pack@0.1.0:/packs/core"},
		Skills:       []SkillInput{{Name: "alpha", Scope: "core", Pack: "core-pack", Dir: dir}},
	}, dir
}

func compute(t *testing.T, in Input) string {
	t.Helper()
	fp, err := Compute(in)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	return fp
}

func TestDeterministic(t *testing.T) {
	in, _ := baseInput(t)
	first := compute(t, in)
	second := compute(t, in)
	if first != second {
		t.Error("fingerprint is not deterministic")
	}
}

// TestContentChangeChangesFingerprint covers the primary case: a skill file
// was edited.
func TestContentChangeChangesFingerprint(t *testing.T) {
	in, dir := baseInput(t)
	before := compute(t, in)

	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("edited body"), 0o644); err != nil {
		t.Fatal(err)
	}
	if compute(t, in) == before {
		t.Error("editing a skill file must change the fingerprint")
	}
}

// TestSupportingFileChangeChangesFingerprint: the whole directory is
// materialized, so supporting files count.
func TestSupportingFileChangeChangesFingerprint(t *testing.T) {
	in, dir := baseInput(t)
	before := compute(t, in)

	p := filepath.Join(dir, "references", "notes.md")
	if err := os.WriteFile(p, []byte("different notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if compute(t, in) == before {
		t.Error("editing a supporting file must change the fingerprint")
	}
}

func TestAddedFileChangesFingerprint(t *testing.T) {
	in, dir := baseInput(t)
	before := compute(t, in)

	if err := os.WriteFile(filepath.Join(dir, "extra.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if compute(t, in) == before {
		t.Error("adding a file must change the fingerprint")
	}
}

// TestVersionChangeChangesFingerprint: the CLI version is the rules version,
// so identical inputs can resolve differently under a different svibe.
func TestVersionChangeChangesFingerprint(t *testing.T) {
	in, _ := baseInput(t)
	before := compute(t, in)

	in.SvibeVersion = "1.0.1"
	if compute(t, in) == before {
		t.Error("a svibe version change must change the fingerprint")
	}
}

// TestWinnerChangeChangesFingerprint: two byte-identical definitions from
// different packs are not the same resolution.
func TestWinnerChangeChangesFingerprint(t *testing.T) {
	in, _ := baseInput(t)
	before := compute(t, in)

	in.Skills[0].Scope = "project"
	in.Skills[0].Pack = "project-pack"
	if compute(t, in) == before {
		t.Error("a change of winning scope/pack must change the fingerprint")
	}
}

func TestPackSetChangeChangesFingerprint(t *testing.T) {
	in, _ := baseInput(t)
	before := compute(t, in)

	in.Packs = append(in.Packs, "user:extra@0.1.0:/packs/extra")
	if compute(t, in) == before {
		t.Error("a change in loaded packs must change the fingerprint")
	}
}

// TestPackOrderDoesNotMatter: discovery order must not affect the result.
func TestPackOrderDoesNotMatter(t *testing.T) {
	in, _ := baseInput(t)
	in.Packs = []string{"a", "b", "c"}
	first := compute(t, in)

	in.Packs = []string{"c", "a", "b"}
	if compute(t, in) != first {
		t.Error("pack ordering must not change the fingerprint")
	}
}

func TestSkillOrderDoesNotMatter(t *testing.T) {
	root := t.TempDir()
	a := skillDir(t, root, "alpha", map[string]string{"SKILL.md": "a"})
	b := skillDir(t, root, "beta", map[string]string{"SKILL.md": "b"})

	in := Input{SvibeVersion: "1.0.0"}
	in.Skills = []SkillInput{{Name: "alpha", Dir: a}, {Name: "beta", Dir: b}}
	first := compute(t, in)

	in.Skills = []SkillInput{{Name: "beta", Dir: b}, {Name: "alpha", Dir: a}}
	if compute(t, in) != first {
		t.Error("skill ordering must not change the fingerprint")
	}
}

// TestTimestampsAreIgnored: v1 uses content hashing, not filesystem
// timestamps. Rewriting identical content must not look like a change.
func TestTimestampsAreIgnored(t *testing.T) {
	in, dir := baseInput(t)
	before := compute(t, in)

	p := filepath.Join(dir, "SKILL.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if compute(t, in) != before {
		t.Error("rewriting identical content must not change the fingerprint")
	}
}

// TestFieldBoundariesAreUnambiguous: adjacent fields must not be re-splittable
// into a different input that hashes the same.
func TestFieldBoundariesAreUnambiguous(t *testing.T) {
	a := Input{SvibeVersion: "1.0.0", Packs: []string{"ab", "c"}}
	b := Input{SvibeVersion: "1.0.0", Packs: []string{"a", "bc"}}

	if compute(t, a) == compute(t, b) {
		t.Error("differently split fields must not collide")
	}
}

func TestMissingSkillDirIsAnError(t *testing.T) {
	in := Input{
		SvibeVersion: "1.0.0",
		Skills:       []SkillInput{{Name: "gone", Dir: filepath.Join(t.TempDir(), "absent")}},
	}
	if _, err := Compute(in); err == nil {
		t.Error("expected an error for a missing skill directory")
	}
}

func TestEmptyInputIsStable(t *testing.T) {
	in := Input{SvibeVersion: "1.0.0"}
	first := compute(t, in)
	second := compute(t, in)
	if first != second {
		t.Error("empty input should be stable")
	}
}
