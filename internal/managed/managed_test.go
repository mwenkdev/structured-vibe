package managed

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/mwenkdev/structured-vibe/internal/diag"
)

// install writes files beneath root and returns a manifest describing them.
func install(t *testing.T, root string, files map[string]string) Manifest {
	t.Helper()
	m := Manifest{}
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(content))
		m[rel] = hex.EncodeToString(sum[:])
	}
	return m
}

func TestIntactInstallationPasses(t *testing.T) {
	root := t.TempDir()
	m := install(t, root, map[string]string{
		"config/models.yaml":           "models: {}\n",
		"core/skills/sv-plan/SKILL.md": "plan\n",
		"core/structured-vibe.yaml":    "name: core\n",
	})

	rep, d := Check(root, m)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if len(d.Warnings()) != 0 {
		t.Errorf("unexpected warnings: %+v", d.Warnings())
	}
	if !rep.OK() {
		t.Error("report should be OK")
	}
	if rep.Checked != 3 {
		t.Errorf("checked = %d, want 3", rep.Checked)
	}
}

// TestModifiedFileWarnsAndContinues: modification is unsupported but allowed.
func TestModifiedFileWarnsAndContinues(t *testing.T) {
	root := t.TempDir()
	m := install(t, root, map[string]string{
		"config/models.yaml": "models: {}\n",
	})

	// Edit the file after the manifest was computed.
	if err := os.WriteFile(filepath.Join(root, "config", "models.yaml"), []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, d := Check(root, m)
	if d.HasErrors() {
		t.Fatalf("modification must not be an error: %+v", d.Errors())
	}
	if len(d.Warnings()) != 1 {
		t.Fatalf("expected 1 warning, got %+v", d.Warnings())
	}
	if d.Warnings()[0].Code != "managed.modified" {
		t.Errorf("code = %q", d.Warnings()[0].Code)
	}
	if !rep.OK() {
		t.Error("a modified file must not make the installation incomplete")
	}
	if len(rep.Modified) != 1 {
		t.Errorf("modified = %v", rep.Modified)
	}
}

// TestMissingFileIsFatal: deletion is treated differently from modification
// because the installed release is incomplete.
func TestMissingFileIsFatal(t *testing.T) {
	root := t.TempDir()
	m := install(t, root, map[string]string{
		"config/models.yaml":           "models: {}\n",
		"core/skills/sv-plan/SKILL.md": "plan\n",
	})

	if err := os.Remove(filepath.Join(root, "config", "models.yaml")); err != nil {
		t.Fatal(err)
	}

	rep, d := Check(root, m)
	if !d.HasErrors() {
		t.Fatal("a missing managed file must be fatal")
	}
	if !hasCode(d, "managed.missing") {
		t.Errorf("want managed.missing, got %+v", d.Errors())
	}
	if rep.OK() {
		t.Error("report must not be OK")
	}
}

// TestExtraFilesIgnored: unexpected files beneath managed directories are
// ignored, with no warning and no deletion.
func TestExtraFilesIgnored(t *testing.T) {
	root := t.TempDir()
	m := install(t, root, map[string]string{
		"core/skills/sv-plan/SKILL.md": "plan\n",
	})

	extra := filepath.Join(root, "core", "skills", "sv-plan", "notes.md")
	if err := os.WriteFile(extra, []byte("scratch"), 0o644); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(root, "core", "unexpected.txt")
	if err := os.WriteFile(stray, []byte("stray"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, d := Check(root, m)
	if d.HasErrors() || len(d.Warnings()) != 0 {
		t.Errorf("extra files must be ignored, got %+v", d)
	}
	if _, err := os.Stat(stray); err != nil {
		t.Error("extra files must not be deleted")
	}
}

// TestCoreMembershipIsClosed: core skills come from the shipped manifest, so
// dropping a SKILL.md directory into the installed tree creates nothing.
func TestCoreMembershipIsClosed(t *testing.T) {
	m := Manifest{
		"core/structured-vibe.yaml":                 "x",
		"core/skills/sv-plan/SKILL.md":              "x",
		"core/skills/sv-review/SKILL.md":            "x",
		"core/skills/sv-review/references/notes.md": "x",
		"config/models.yaml":                        "x",
	}

	got := CoreSkills(m)
	want := map[string]bool{"sv-plan": true, "sv-review": true}

	if len(got) != len(want) {
		t.Fatalf("CoreSkills = %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing core skill %q", k)
		}
	}
	// A skill someone adds locally is not a core skill.
	if got["locally-added"] {
		t.Error("unlisted skill must not be core")
	}
}

// TestCoreMembershipIgnoresSupportingFiles confirms only <name>/SKILL.md
// declares a skill.
func TestCoreMembershipIgnoresSupportingFiles(t *testing.T) {
	m := Manifest{
		"core/skills/sv-plan/templates/SKILL.md": "x",
	}
	if got := CoreSkills(m); len(got) != 0 {
		t.Errorf("nested SKILL.md must not declare a core skill, got %v", got)
	}
}

func TestEmptyManifestPasses(t *testing.T) {
	_, d := Check(t.TempDir(), Manifest{})
	if d.HasErrors() || len(d.Warnings()) != 0 {
		t.Errorf("empty manifest should pass cleanly, got %+v", d)
	}
}

// TestEmbeddedManifestDescribesRealPayload guards the generated manifest.
func TestEmbeddedManifestDescribesRealPayload(t *testing.T) {
	m := Embedded()
	if len(m) == 0 {
		t.Fatal("embedded manifest is empty; run make generate")
	}
	if _, ok := m[ModelRegistryPath]; !ok {
		t.Errorf("embedded manifest is missing %s", ModelRegistryPath)
	}
	if len(CoreSkills(m)) == 0 {
		t.Error("embedded manifest declares no core skills")
	}
	for _, hash := range m {
		if len(hash) != 64 {
			t.Errorf("hash %q is not a SHA-256 hex digest", hash)
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
