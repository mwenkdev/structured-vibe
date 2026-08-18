package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mwenkdev/structured-vibe/internal/managed"
	"github.com/mwenkdev/structured-vibe/internal/pack"
)

const testRegistry = `
version: 1
models:
  big-model:
    tier: A
    aliases:
      - provider/big
  mid-model:
    tier: B
    aliases:
      - provider/mid
  small-model:
    tier: C
    aliases:
      - provider/small
`

// newInstallation builds a config root with a core pack whose skills declare
// the given tiers, plus a model registry, and returns a matching manifest.
func newInstallation(t *testing.T, configHome string, tiers map[string]string) managed.Manifest {
	t.Helper()

	core := filepath.Join(configHome, "core")
	if err := os.MkdirAll(core, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(core, pack.ManifestName),
		[]byte("name: structured-vibe-core\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rel := []string{"core/" + pack.ManifestName}
	for name, tier := range tiers {
		dir := filepath.Join(core, pack.SkillsDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + name + "\ndescription: Test skill " + name + ".\n"
		if tier != "" {
			body += "minimum_driver_tier: " + tier + "\n"
		}
		body += "---\nbody\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		rel = append(rel, "core/"+pack.SkillsDir+"/"+name+"/SKILL.md")
	}

	cfg := filepath.Join(configHome, "config")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "models.yaml"), []byte(testRegistry), 0o644); err != nil {
		t.Fatal(err)
	}
	rel = append(rel, managed.ModelRegistryPath)

	return manifestFor(t, configHome, rel...)
}

func advise(t *testing.T, configHome string, m managed.Manifest, skill, model string) adviseResult {
	t.Helper()
	got := runCLIWithManifest(t, configHome, t.TempDir(), m,
		"advise", "--skill", skill, "--model", model, "--json")
	if got.err != nil {
		t.Fatalf("advise failed: %v\n%s", got.err, got.stderr)
	}

	var env struct {
		OK     bool         `json:"ok"`
		Result adviseResult `json:"result"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &env); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, got.stdout)
	}
	if !env.OK {
		t.Errorf("advise should succeed even when it warns")
	}
	return env.Result
}

func TestAdviseModelMeetsRecommendation(t *testing.T) {
	configHome := t.TempDir()
	m := newInstallation(t, configHome, map[string]string{"sv-finalize": "B"})

	// An A-tier model exceeds a B-tier recommendation.
	got := advise(t, configHome, m, "sv-finalize", "provider/big")
	if !got.Meets {
		t.Error("A should satisfy B")
	}
	if got.Warn {
		t.Errorf("no warning expected, got %q", got.Message)
	}
	if got.ModelID != "big-model" {
		t.Errorf("canonical id = %q", got.ModelID)
	}
}

func TestAdviseExactTierMatchIsSilent(t *testing.T) {
	configHome := t.TempDir()
	m := newInstallation(t, configHome, map[string]string{"sv-finalize": "B"})

	got := advise(t, configHome, m, "sv-finalize", "provider/mid")
	if !got.Meets || got.Warn {
		t.Errorf("B should satisfy B silently, got %+v", got)
	}
}

func TestAdviseModelBelowRecommendationWarns(t *testing.T) {
	configHome := t.TempDir()
	m := newInstallation(t, configHome, map[string]string{"sv-finalize": "B"})

	got := advise(t, configHome, m, "sv-finalize", "provider/small")
	if got.Meets {
		t.Error("C must not satisfy B")
	}
	if !got.Warn {
		t.Fatal("expected a warning")
	}
	if !strings.Contains(got.Message, "tier B") || !strings.Contains(got.Message, "tier C") {
		t.Errorf("message should name both tiers: %q", got.Message)
	}
}

// TestAdviseUnknownModelWarns: an unmappable model warns and continues.
func TestAdviseUnknownModelWarns(t *testing.T) {
	configHome := t.TempDir()
	m := newInstallation(t, configHome, map[string]string{"sv-finalize": "B"})

	got := advise(t, configHome, m, "sv-finalize", "someone/unlisted-model")
	if got.ModelTier != "unknown" {
		t.Errorf("tier = %q, want unknown", got.ModelTier)
	}
	if got.Meets {
		t.Error("unknown must not satisfy a requirement")
	}
	if !got.Warn {
		t.Fatal("expected a warning for an unknown model")
	}
	if !strings.Contains(got.Message, "registry") {
		t.Errorf("message should explain the model is unregistered: %q", got.Message)
	}
}

// TestAdviseSkillWithoutRecommendationNeverWarns: a skill that declares no
// minimum is silent whatever the model, including an unknown one.
func TestAdviseSkillWithoutRecommendationNeverWarns(t *testing.T) {
	configHome := t.TempDir()
	m := newInstallation(t, configHome, map[string]string{"plain-skill": ""})

	for _, model := range []string{"provider/small", "someone/unlisted-model", ""} {
		got := advise(t, configHome, m, "plain-skill", model)
		if got.Warn {
			t.Errorf("model %q: unexpected warning %q", model, got.Message)
		}
		if !got.Meets {
			t.Errorf("model %q: should trivially meet", model)
		}
		if got.RequiredTier != "" {
			t.Errorf("model %q: required tier should be empty", model)
		}
	}
}

func TestAdviseUnknownSkillIsNotAnError(t *testing.T) {
	configHome := t.TempDir()
	m := newInstallation(t, configHome, map[string]string{"sv-finalize": "B"})

	got := advise(t, configHome, m, "not-a-skill", "provider/small")
	if got.SkillKnown {
		t.Error("skill should be reported as unknown")
	}
	if got.Warn {
		t.Error("an unknown skill has no recommendation to violate")
	}
}

func TestAdviseRequiresSkillFlag(t *testing.T) {
	got := runCLI(t, t.TempDir(), t.TempDir(), "advise", "--model", "provider/big")
	if got.err == nil {
		t.Fatal("expected failure without --skill")
	}
	if !strings.Contains(got.stderr, "requires --skill") {
		t.Errorf("stderr = %q", got.stderr)
	}
}

// TestAdviseResolvesAliases confirms the plugin can pass any host spelling.
func TestAdviseResolvesAliases(t *testing.T) {
	configHome := t.TempDir()
	m := newInstallation(t, configHome, map[string]string{"sv-plan": "A"})

	got := advise(t, configHome, m, "sv-plan", "provider/big")
	if got.ModelID != "big-model" {
		t.Errorf("alias not canonicalized: %+v", got)
	}
	if got.Warn {
		t.Errorf("unexpected warning: %q", got.Message)
	}
}
