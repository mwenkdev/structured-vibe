package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mwenkdev/structured-vibe/internal/diag"
)

// writeSkill creates a skill directory named dir containing body.
func writeSkill(t *testing.T, root, dir, body string) string {
	t.Helper()
	sd := filepath.Join(root, dir)
	if err := os.MkdirAll(sd, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sd, FileName), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return sd
}

const validBody = `---
name: java-backend
description: Java backend implementation conventions and practices.
---

# Java backend
`

func TestLoadValid(t *testing.T) {
	root := t.TempDir()
	sd := writeSkill(t, root, "java-backend", validBody)

	s, d := Load(sd)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if s == nil {
		t.Fatal("expected a skill")
	}
	if s.Name != "java-backend" {
		t.Errorf("name = %q", s.Name)
	}
	if s.Description == "" {
		t.Error("description not captured")
	}
}

func TestLoadOptionalMetadata(t *testing.T) {
	root := t.TempDir()
	sd := writeSkill(t, root, "kubernetes-change", `---
name: kubernetes-change
description: Production Kubernetes change procedure.
minimum_driver_tier: B
---
body
`)

	s, d := Load(sd)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if s.MinimumDriverTier != "B" {
		t.Errorf("tier = %q, want B", s.MinimumDriverTier)
	}
}

func TestLoadValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		body    string
		wantErr string
	}{
		{
			name:    "missing frontmatter",
			dir:     "thing",
			body:    "# no frontmatter\n",
			wantErr: "skill.frontmatter.missing",
		},
		{
			name:    "unterminated frontmatter",
			dir:     "thing",
			body:    "---\nname: thing\n",
			wantErr: "skill.frontmatter.unterminated",
		},
		{
			name:    "invalid yaml",
			dir:     "thing",
			body:    "---\nname: [unclosed\n---\nbody\n",
			wantErr: "skill.frontmatter.invalid",
		},
		{
			name:    "missing name",
			dir:     "thing",
			body:    "---\ndescription: has no name.\n---\nbody\n",
			wantErr: "skill.name.missing",
		},
		{
			name:    "invalid name",
			dir:     "thing",
			body:    "---\nname: Thing_One\ndescription: bad id.\n---\nbody\n",
			wantErr: "skill.name.invalid",
		},
		{
			name:    "name does not match directory",
			dir:     "thing",
			body:    "---\nname: other-thing\ndescription: mismatched.\n---\nbody\n",
			wantErr: "skill.name.mismatch",
		},
		{
			name:    "missing description",
			dir:     "thing",
			body:    "---\nname: thing\n---\nbody\n",
			wantErr: "skill.description.missing",
		},
		{
			name:    "blank description",
			dir:     "thing",
			body:    "---\nname: thing\ndescription: \"   \"\n---\nbody\n",
			wantErr: "skill.description.missing",
		},
		{
			name: "description too long",
			dir:  "thing",
			body: "---\nname: thing\ndescription: " +
				strings.Repeat("x", MaxDescriptionLen+1) + "\n---\nbody\n",
			wantErr: "skill.description.too-long",
		},
		{
			name:    "invalid tier",
			dir:     "thing",
			body:    "---\nname: thing\ndescription: ok.\nminimum_driver_tier: D\n---\nbody\n",
			wantErr: "skill.tier.invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			sd := writeSkill(t, root, tt.dir, tt.body)

			s, d := Load(sd)
			if s != nil {
				t.Error("expected no skill to be returned")
			}
			if !d.HasErrors() {
				t.Fatal("expected errors")
			}
			if !hasCode(d.Errors(), tt.wantErr) {
				t.Errorf("want code %q, got %+v", tt.wantErr, d.Errors())
			}
		})
	}
}

// TestNestedSkillMdRejected guards the phantom-skill leak. The host scans
// configured skill paths recursively for **/SKILL.md, so a SKILL.md bundled
// beneath a skill's supporting files would register as a separate skill.
func TestNestedSkillMdRejected(t *testing.T) {
	root := t.TempDir()
	sd := writeSkill(t, root, "thing", validBodyNamed("thing"))

	nested := filepath.Join(sd, "references")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, FileName), []byte("---\nname: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, d := Load(sd)
	if s != nil {
		t.Error("expected the skill to be rejected")
	}
	if !hasCode(d.Errors(), "skill.nested-skill") {
		t.Errorf("want skill.nested-skill, got %+v", d.Errors())
	}
}

// TestSupportingFilesAllowed confirms containment does not reject ordinary
// bundled material.
func TestSupportingFilesAllowed(t *testing.T) {
	root := t.TempDir()
	sd := writeSkill(t, root, "thing", validBodyNamed("thing"))

	for _, sub := range []string{"references", "templates", "scripts"} {
		dir := filepath.Join(sd, sub)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s, d := Load(sd)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if s == nil {
		t.Fatal("expected the skill to load")
	}
}

// TestSymlinkEscapeRejected enforces skill self-containment.
func TestSymlinkEscapeRejected(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(outside, "shared.md")
	if err := os.WriteFile(target, []byte("shared"), 0o644); err != nil {
		t.Fatal(err)
	}

	sd := writeSkill(t, root, "thing", validBodyNamed("thing"))
	link := filepath.Join(sd, "shared.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	s, d := Load(sd)
	if s != nil {
		t.Error("expected the skill to be rejected")
	}
	if !hasCode(d.Errors(), "skill.path.escape") {
		t.Errorf("want skill.path.escape, got %+v", d.Errors())
	}
}

// TestInternalSymlinkAllowed confirms a link that stays inside is fine.
func TestInternalSymlinkAllowed(t *testing.T) {
	root := t.TempDir()
	sd := writeSkill(t, root, "thing", validBodyNamed("thing"))

	target := filepath.Join(sd, "real.md")
	if err := os.WriteFile(target, []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(sd, "alias.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, d := Load(sd); d.HasErrors() {
		t.Errorf("unexpected errors: %+v", d.Errors())
	}
}

func TestCRLFAndBOMTolerated(t *testing.T) {
	root := t.TempDir()
	body := "\xef\xbb\xbf---\r\nname: thing\r\ndescription: windows authored.\r\n---\r\nbody\r\n"
	sd := writeSkill(t, root, "thing", body)

	s, d := Load(sd)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if s.Name != "thing" {
		t.Errorf("name = %q", s.Name)
	}
}

func TestValidID(t *testing.T) {
	tests := map[string]bool{
		"java-backend":   true,
		"a":              true,
		"a1":             true,
		"api-design-v2":  true,
		"Java-Backend":   false,
		"java_backend":   false,
		"-leading":       false,
		"trailing-":      false,
		"double--hyphen": false,
		"":               false,
	}
	for in, want := range tests {
		if got := ValidID(in); got != want {
			t.Errorf("ValidID(%q) = %v, want %v", in, got, want)
		}
	}
}

func validBodyNamed(name string) string {
	return "---\nname: " + name + "\ndescription: A valid test skill description.\n---\nbody\n"
}

func hasCode(d diag.Diagnostics, code string) bool {
	for _, x := range d {
		if x.Code == code {
			return true
		}
	}
	return false
}
