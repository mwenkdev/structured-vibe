package models

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `
version: 1
models:
  claude-opus-5:
    tier: A
    aliases:
      - anthropic/claude-opus-5
      - openrouter/anthropic/claude-opus-5
  claude-sonnet-5:
    tier: B
    aliases:
      - anthropic/claude-sonnet-5
  claude-haiku-4-5:
    tier: C
    aliases:
      - anthropic/claude-haiku-4-5
`

func mustParse(t *testing.T) *Registry {
	t.Helper()
	r, d := Parse([]byte(sample), "test")
	if d.HasErrors() {
		t.Fatalf("unexpected errors: %+v", d.Errors())
	}
	return r
}

// TestExactAliasMatching is the core registry invariant: identity is never
// inferred from substring similarity or naming conventions.
func TestExactAliasMatching(t *testing.T) {
	r := mustParse(t)

	tests := []struct {
		external string
		wantID   string
		wantOK   bool
	}{
		{"anthropic/claude-opus-5", "claude-opus-5", true},
		{"openrouter/anthropic/claude-opus-5", "claude-opus-5", true},
		{"anthropic/claude-sonnet-5", "claude-sonnet-5", true},

		// Near misses must NOT match.
		{"claude-opus-5", "", false},                        // missing provider prefix
		{"anthropic/claude-opus-5-fast", "", false},         // superstring
		{"anthropic/claude-opus", "", false},                // substring
		{"ANTHROPIC/CLAUDE-OPUS-5", "", false},              // case differs
		{"anthropic/claude-opus-5 ", "", false},             // trailing space
		{"someprovider/anthropic/claude-opus-5", "", false}, // unknown provider
		{"", "", false},
	}

	for _, tt := range tests {
		m, ok := r.Lookup(tt.external)
		if ok != tt.wantOK {
			t.Errorf("Lookup(%q) ok = %v, want %v", tt.external, ok, tt.wantOK)
			continue
		}
		if ok && m.ID != tt.wantID {
			t.Errorf("Lookup(%q) id = %q, want %q", tt.external, m.ID, tt.wantID)
		}
	}
}

// TestUnknownIsAStateNotATier guards the distinction the architecture draws.
func TestUnknownIsAStateNotATier(t *testing.T) {
	r := mustParse(t)

	got := r.TierOf("some/unregistered-model")
	if got.Valid() {
		t.Errorf("unregistered model produced a valid tier %q", got)
	}
	if got.String() != "unknown" {
		t.Errorf("String() = %q, want unknown", got.String())
	}
	if _, ok := ParseTier("unknown"); ok {
		t.Error("\"unknown\" must not parse as a tier value")
	}
}

func TestTierOrdering(t *testing.T) {
	tests := []struct {
		current, required Tier
		want              bool
	}{
		{TierA, TierA, true},
		{TierA, TierB, true},
		{TierA, TierC, true},
		{TierB, TierA, false},
		{TierB, TierB, true},
		{TierB, TierC, true},
		{TierC, TierA, false},
		{TierC, TierB, false},
		{TierC, TierC, true},

		// Unknown never satisfies a requirement: it cannot be shown to.
		{TierUnknown, TierA, false},
		{TierUnknown, TierC, false},

		// When nothing is required, anything satisfies it.
		{TierUnknown, TierUnknown, true},
		{TierC, TierUnknown, true},
	}

	for _, tt := range tests {
		if got := tt.current.Meets(tt.required); got != tt.want {
			t.Errorf("%s.Meets(%s) = %v, want %v", tt.current, tt.required, got, tt.want)
		}
	}
}

func TestInvalidTierRejected(t *testing.T) {
	_, d := Parse([]byte("models:\n  x:\n    tier: D\n    aliases: [a/b]\n"), "test")
	if !d.HasErrors() {
		t.Fatal("expected an error for tier D")
	}
}

func TestDuplicateAliasRejected(t *testing.T) {
	_, d := Parse([]byte(`
models:
  first:
    tier: A
    aliases: [shared/id]
  second:
    tier: B
    aliases: [shared/id]
`), "test")
	if !d.HasErrors() {
		t.Fatal("an alias mapping to two models must be an error")
	}
}

func TestNilRegistryIsSafe(t *testing.T) {
	var r *Registry
	if _, ok := r.Lookup("anything"); ok {
		t.Error("nil registry should not resolve")
	}
	if r.TierOf("anything").Valid() {
		t.Error("nil registry should yield unknown")
	}
	if r.Len() != 0 {
		t.Error("nil registry length should be 0")
	}
}

// TestShippedRegistryIsValid parses the registry that actually ships.
func TestShippedRegistryIsValid(t *testing.T) {
	path := filepath.Join("..", "..", "config", "models.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("shipped registry not present: %v", err)
	}

	r, d := Parse(raw, path)
	if d.HasErrors() {
		t.Fatalf("shipped registry is invalid: %+v", d.Errors())
	}
	if r.Len() == 0 {
		t.Fatal("shipped registry is empty")
	}

	// Every model must declare at least one alias, or it can never be matched.
	for _, m := range r.Models() {
		if len(m.Aliases) == 0 {
			t.Errorf("model %q has no aliases and can never be matched", m.ID)
		}
		if !m.Tier.Valid() {
			t.Errorf("model %q has an invalid tier", m.ID)
		}
	}
}
