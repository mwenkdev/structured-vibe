package hostint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mwenkdev/structured-vibe/internal/paths"
)

func readConfig(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(stripJSONComments(raw), &cfg); err != nil {
		t.Fatalf("config is not valid JSON: %v\n%s", err, raw)
	}
	return cfg
}

func skillsPaths(t *testing.T, cfg map[string]any) []string {
	t.Helper()
	skills, ok := cfg["skills"].(map[string]any)
	if !ok {
		return nil
	}
	list, ok := skills["paths"].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, v := range list {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func TestRegisterCreatesConfig(t *testing.T) {
	root := t.TempDir()

	changed, d := RegisterSnapshotPath(root)
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	if !changed {
		t.Error("expected the config to be created")
	}

	cfg := readConfig(t, filepath.Join(root, "opencode.json"))
	got := skillsPaths(t, cfg)
	if len(got) != 1 || got[0] != paths.OpenCodeSkillsRelPath {
		t.Errorf("skills.paths = %v", got)
	}
	if cfg["$schema"] != "https://opencode.ai/config.json" {
		t.Errorf("schema not set: %v", cfg["$schema"])
	}
}

// TestRegisterPreservesExistingPaths is the consequence of the spike finding:
// the host REPLACES the skills.paths array at the higher-precedence scope
// rather than merging, so overwriting it would silently destroy user entries.
func TestRegisterPreservesExistingPaths(t *testing.T) {
	root := t.TempDir()
	existing := `{
  "$schema": "https://opencode.ai/config.json",
  "skills": { "paths": ["my-team-skills", "/abs/other/skills"] }
}`
	if err := os.WriteFile(filepath.Join(root, "opencode.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, d := RegisterSnapshotPath(root); d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}

	got := skillsPaths(t, readConfig(t, filepath.Join(root, "opencode.json")))
	want := map[string]bool{
		"my-team-skills":            true,
		"/abs/other/skills":         true,
		paths.OpenCodeSkillsRelPath: true,
	}
	if len(got) != len(want) {
		t.Fatalf("skills.paths = %v, want all of %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected entry %q", p)
		}
	}
}

// TestRegisterPreservesUnrelatedSettings: init must not damage the user's config.
func TestRegisterPreservesUnrelatedSettings(t *testing.T) {
	root := t.TempDir()
	existing := `{
  "$schema": "https://opencode.ai/config.json",
  "model": "anthropic/claude-opus-5",
  "lsp": true,
  "permission": { "edit": "ask" }
}`
	if err := os.WriteFile(filepath.Join(root, "opencode.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, d := RegisterSnapshotPath(root); d.HasErrors() {
		t.Fatal(d.Errors())
	}

	cfg := readConfig(t, filepath.Join(root, "opencode.json"))
	if cfg["model"] != "anthropic/claude-opus-5" {
		t.Errorf("model lost: %v", cfg["model"])
	}
	if cfg["lsp"] != true {
		t.Errorf("lsp lost: %v", cfg["lsp"])
	}
	if _, ok := cfg["permission"]; !ok {
		t.Error("permission lost")
	}
}

func TestRegisterIsIdempotent(t *testing.T) {
	root := t.TempDir()

	if _, d := RegisterSnapshotPath(root); d.HasErrors() {
		t.Fatal(d.Errors())
	}
	changed, d := RegisterSnapshotPath(root)
	if d.HasErrors() {
		t.Fatal(d.Errors())
	}
	if changed {
		t.Error("second registration should be a no-op")
	}

	got := skillsPaths(t, readConfig(t, filepath.Join(root, "opencode.json")))
	if len(got) != 1 {
		t.Errorf("entry duplicated: %v", got)
	}
}

// TestRegisterUsesExistingJsonc honours the host's alternate filename.
func TestRegisterUsesExistingJsonc(t *testing.T) {
	root := t.TempDir()
	existing := `{
  // team defaults
  "skills": { "paths": ["team"] }
}`
	if err := os.WriteFile(filepath.Join(root, "opencode.jsonc"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, d := RegisterSnapshotPath(root); d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}

	// The jsonc file is updated; a stray opencode.json must not appear.
	if _, err := os.Stat(filepath.Join(root, "opencode.json")); err == nil {
		t.Error("a second config file was created")
	}
	got := skillsPaths(t, readConfig(t, filepath.Join(root, "opencode.jsonc")))
	if len(got) != 2 {
		t.Errorf("skills.paths = %v", got)
	}
}

// TestRegisterRefusesUnparseableConfig: never clobber what svibe cannot read.
func TestRegisterRefusesUnparseableConfig(t *testing.T) {
	root := t.TempDir()
	broken := `{ "skills": { "paths": [ this is not json`
	path := filepath.Join(root, "opencode.json")
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, d := RegisterSnapshotPath(root)
	if changed {
		t.Error("must not modify an unparseable config")
	}
	if !d.HasErrors() {
		t.Error("expected an error")
	}

	raw, _ := os.ReadFile(path)
	if string(raw) != broken {
		t.Error("the unparseable config was overwritten")
	}
}

func TestHasSnapshotPath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "opencode.json")

	if err := os.WriteFile(path, []byte(`{"skills":{"paths":["other"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := HasSnapshotPath(path); got {
		t.Error("should not report registered")
	}

	if _, d := RegisterSnapshotPath(root); d.HasErrors() {
		t.Fatal(d.Errors())
	}
	if got, _ := HasSnapshotPath(path); !got {
		t.Error("should report registered")
	}
}

// TestGlobalSkillsPathsShadowWarning surfaces the array-replace behavior to
// the human, since svibe cannot change it.
func TestGlobalSkillsPathsShadowWarning(t *testing.T) {
	home := t.TempDir()
	projectRoot := t.TempDir()

	globalDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "opencode.json"),
		[]byte(`{"skills":{"paths":["~/my-global-skills"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// No project config yet: nothing shadows the global one.
	if d := CheckGlobalSkillsPathsShadowed(home, projectRoot); len(d) != 0 {
		t.Errorf("unexpected warning before a project config exists: %+v", d)
	}

	if _, d := RegisterSnapshotPath(projectRoot); d.HasErrors() {
		t.Fatal(d.Errors())
	}

	d := CheckGlobalSkillsPathsShadowed(home, projectRoot)
	if len(d.Warnings()) != 1 {
		t.Fatalf("expected a shadowing warning, got %+v", d)
	}
	if d.Warnings()[0].Code != "hostint.global-skills-shadowed" {
		t.Errorf("code = %q", d.Warnings()[0].Code)
	}
	if d.HasErrors() {
		t.Error("shadowing is advisory, not an error")
	}
}

func TestNoGlobalSkillsPathsNoWarning(t *testing.T) {
	home := t.TempDir()
	projectRoot := t.TempDir()

	globalDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "opencode.json"),
		[]byte(`{"lsp":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, d := RegisterSnapshotPath(projectRoot); d.HasErrors() {
		t.Fatal(d.Errors())
	}

	if d := CheckGlobalSkillsPathsShadowed(home, projectRoot); len(d) != 0 {
		t.Errorf("unexpected warning: %+v", d)
	}
}

func TestVerifyIntegration(t *testing.T) {
	home := t.TempDir()

	// Not installed.
	if d := VerifyIntegration(home); !d.HasErrors() {
		t.Error("a missing integration must be an error")
	}

	dir := UserPluginDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, PluginFileName)

	// Empty file is not a usable integration.
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if d := VerifyIntegration(home); !d.HasErrors() {
		t.Error("an empty integration must be an error")
	}

	// Installed.
	if err := os.WriteFile(path, []byte("export default () => ({})\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if d := VerifyIntegration(home); d.HasErrors() {
		t.Errorf("installed integration should verify: %+v", d.Errors())
	}
}

func TestStripJSONComments(t *testing.T) {
	tests := []struct {
		name, in string
		wantKey  string
		wantVal  string
	}{
		{"line comment", "{\n // note\n \"a\": \"b\"\n}", "a", "b"},
		{"block comment", "{ /* note */ \"a\": \"b\" }", "a", "b"},
		{"url not mangled", `{"a": "https://example.com/x"}`, "a", "https://example.com/x"},
		{"comment marker inside string", `{"a": "// not a comment"}`, "a", "// not a comment"},
		{"escaped quote", `{"a": "he said \"hi\" //x"}`, "a", `he said "hi" //x`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg map[string]any
			if err := json.Unmarshal(stripJSONComments([]byte(tt.in)), &cfg); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if cfg[tt.wantKey] != tt.wantVal {
				t.Errorf("%s = %v, want %q", tt.wantKey, cfg[tt.wantKey], tt.wantVal)
			}
		})
	}
}

func TestInstallCopiesShippedIntegration(t *testing.T) {
	configHome := t.TempDir()
	home := t.TempDir()

	src := filepath.Join(configHome, filepath.FromSlash(ManagedPluginPath))
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "export default async () => ({})\n"
	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	dst, d := Install(configHome, home)
	if d.HasErrors() {
		t.Fatalf("install failed: %+v", d.Errors())
	}
	if dst != InstalledPluginPath(home) {
		t.Errorf("installed to %q", dst)
	}

	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != body {
		t.Errorf("content = %q", raw)
	}

	// The freshly installed integration must satisfy verification.
	if vd := VerifyIntegration(home); vd.HasErrors() {
		t.Errorf("installed integration does not verify: %+v", vd.Errors())
	}
}

// TestInstallLeavesNoTemporaryFiles guards the staged write.
func TestInstallLeavesNoTemporaryFiles(t *testing.T) {
	configHome := t.TempDir()
	home := t.TempDir()

	src := filepath.Join(configHome, filepath.FromSlash(ManagedPluginPath))
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, d := Install(configHome, home); d.HasErrors() {
		t.Fatal(d.Errors())
	}

	entries, err := os.ReadDir(UserPluginDir(home))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestInstallFailsWithoutShippedPayload(t *testing.T) {
	_, d := Install(t.TempDir(), t.TempDir())
	if !d.HasErrors() {
		t.Error("expected an error when the release ships no integration")
	}
}

func TestInstalledMatchesRelease(t *testing.T) {
	configHome := t.TempDir()
	home := t.TempDir()

	src := filepath.Join(configHome, filepath.FromSlash(ManagedPluginPath))
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("current\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Nothing installed yet.
	if matches, installed := InstalledMatchesRelease(configHome, home); matches || installed {
		t.Error("expected not installed")
	}

	if _, d := Install(configHome, home); d.HasErrors() {
		t.Fatal(d.Errors())
	}
	if matches, installed := InstalledMatchesRelease(configHome, home); !matches || !installed {
		t.Error("expected a match after install")
	}

	// An integration from an older release.
	if err := os.WriteFile(InstalledPluginPath(home), []byte("older\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if matches, installed := InstalledMatchesRelease(configHome, home); matches || !installed {
		t.Error("expected installed but not matching")
	}
}
