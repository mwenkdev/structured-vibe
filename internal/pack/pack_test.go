package pack

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/scope"
)

// build writes a pack with the given manifest and skills (name -> body).
func build(t *testing.T, manifest string, skills map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if manifest != "" {
		if err := os.WriteFile(filepath.Join(root, ManifestName), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range skills {
		dir := filepath.Join(root, SkillsDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func skillBody(name string) string {
	return "---\nname: " + name + "\ndescription: A valid description for testing.\n---\nbody\n"
}

func TestLoadValidPack(t *testing.T) {
	root := build(t, "name: mike-general\nversion: 0.1.0\ndescription: test\n", map[string]string{
		"java-backend": skillBody("java-backend"),
		"testing":      skillBody("testing"),
	})

	p, d := Load(root, scope.User)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if p.Manifest.Name != "mike-general" {
		t.Errorf("name = %q", p.Manifest.Name)
	}
	if len(p.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(p.Skills))
	}
	// Skills are sorted so output is stable across filesystem order.
	if p.Skills[0].Name != "java-backend" || p.Skills[1].Name != "testing" {
		t.Errorf("skills not sorted: %v", p.Skills)
	}
}

func TestManifestValidation(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		wantCode string
	}{
		{"missing manifest", "", "pack.manifest.missing"},
		{"malformed yaml", "name: [unclosed\n", "pack.manifest.invalid"},
		{"missing name", "version: 1.0.0\n", "pack.name.missing"},
		{"invalid name", "name: Mike_General\nversion: 1.0.0\n", "pack.name.invalid"},
		{"missing version", "name: ok\n", "pack.version.missing"},
		{"invalid semver", "name: ok\nversion: 1.0\n", "pack.version.invalid"},
		{"invalid semver text", "name: ok\nversion: v1.0.0\n", "pack.version.invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := build(t, tt.manifest, nil)
			p, d := Load(root, scope.User)
			if p != nil {
				t.Error("expected no pack")
			}
			if !hasCode(d.Errors(), tt.wantCode) {
				t.Errorf("want %q, got %+v", tt.wantCode, d.Errors())
			}
		})
	}
}

func TestValidSemverAccepted(t *testing.T) {
	for _, v := range []string{"0.1.0", "1.0.0", "2.0.0-beta.1", "1.2.3+build.5"} {
		root := build(t, "name: ok\nversion: "+v+"\n", nil)
		if _, d := Load(root, scope.User); d.HasErrors() {
			t.Errorf("version %q rejected: %+v", v, d.Errors())
		}
	}
}

// TestOnlyImmediateChildrenAreSkills confirms svibe does not search the rest
// of a pack for SKILL.md (architecture 7.2).
func TestOnlyImmediateChildrenAreSkills(t *testing.T) {
	root := build(t, "name: ok\nversion: 1.0.0\n", map[string]string{
		"real-skill": skillBody("real-skill"),
	})

	// A SKILL.md nested two levels below skills/ is not a skill.
	deep := filepath.Join(root, SkillsDir, "group", "nested-skill")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, "SKILL.md"), []byte(skillBody("nested-skill")), 0o644); err != nil {
		t.Fatal(err)
	}
	// A SKILL.md outside skills/ entirely is not a skill either.
	other := filepath.Join(root, "docs", "example")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "SKILL.md"), []byte(skillBody("example")), 0o644); err != nil {
		t.Fatal(err)
	}

	p, d := Load(root, scope.User)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if len(p.Skills) != 1 || p.Skills[0].Name != "real-skill" {
		t.Errorf("expected only real-skill, got %v", p.Skills)
	}
}

func TestPackWithNoSkillsIsValid(t *testing.T) {
	root := build(t, "name: empty\nversion: 0.1.0\n", nil)
	p, d := Load(root, scope.User)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if len(p.Skills) != 0 {
		t.Errorf("expected no skills")
	}
}

func TestDiscoverUserPacks(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"alpha", "beta"} {
		root := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Join(root, SkillsDir), 0o755); err != nil {
			t.Fatal(err)
		}
		manifest := "name: " + name + "\nversion: 0.1.0\n"
		if err := os.WriteFile(filepath.Join(root, ManifestName), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A directory without a manifest is quietly ignored; it may be anything.
	if err := os.MkdirAll(filepath.Join(dir, "not-a-pack"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Discovery is not recursive.
	nested := filepath.Join(dir, "alpha", "nested-pack")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, ManifestName), []byte("name: nested\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	packs, d := DiscoverUserPacks(dir)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if len(packs) != 2 {
		t.Fatalf("expected 2 packs, got %d: %v", len(packs), packs)
	}
	if packs[0].Manifest.Name != "alpha" || packs[1].Manifest.Name != "beta" {
		t.Errorf("packs not sorted: %v", packs)
	}
}

func TestDiscoverUserPacksMissingDirIsNotAnError(t *testing.T) {
	packs, d := DiscoverUserPacks(filepath.Join(t.TempDir(), "absent"))
	if d.HasErrors() {
		t.Errorf("unexpected errors: %+v", d.Errors())
	}
	if len(packs) != 0 {
		t.Errorf("expected no packs")
	}
}

func TestDeriveName(t *testing.T) {
	tests := map[string]string{
		"structured-vibe": "structured-vibe",
		"MyProject":       "myproject",
		"my_project":      "my-project",
		"my project 2":    "my-project-2",
		"--weird--":       "weird",
		"...":             "project",
		"":                "project",
	}
	for in, want := range tests {
		if got := DeriveName(in); got != want {
			t.Errorf("DeriveName(%q) = %q, want %q", in, got, want)
		}
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
