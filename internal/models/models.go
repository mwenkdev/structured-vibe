// Package models implements the canonical model registry and capability
// tiers.
//
// Structured Vibe owns the canonical model-to-tier mapping (architecture 11).
// Alias matching is exact: identity is never inferred from substring
// similarity, regular expressions, or guessed provider naming conventions.
// An identifier that matches no alias is unknown, and unknown is a state
// rather than a tier.
package models

import (
	"os"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/mwenkdev/structured-vibe/internal/diag"
)

// Tier is a capability tier. The empty Tier means unknown.
type Tier string

// The v1 tiers.
const (
	TierA Tier = "A"
	TierB Tier = "B"
	TierC Tier = "C"
	// TierUnknown is the state produced when the registry cannot map a model.
	// It is deliberately not a tier value in the registry file.
	TierUnknown Tier = ""
)

// rank orders tiers by capability. Higher is more capable.
func (t Tier) rank() int {
	switch t {
	case TierA:
		return 3
	case TierB:
		return 2
	case TierC:
		return 1
	default:
		return 0
	}
}

// Valid reports whether t is a real tier rather than the unknown state.
func (t Tier) Valid() bool { return t.rank() > 0 }

// String renders the tier, or "unknown".
func (t Tier) String() string {
	if !t.Valid() {
		return "unknown"
	}
	return string(t)
}

// Meets reports whether t satisfies the required tier.
//
// An unknown tier never satisfies a requirement: the caller cannot show that
// it does. The result is advisory and produces a warning, not a block.
func (t Tier) Meets(required Tier) bool {
	if !required.Valid() {
		return true // nothing was required
	}
	if !t.Valid() {
		return false
	}
	return t.rank() >= required.rank()
}

// ParseTier converts a registry or frontmatter value into a Tier.
func ParseTier(s string) (Tier, bool) {
	switch Tier(s) {
	case TierA:
		return TierA, true
	case TierB:
		return TierB, true
	case TierC:
		return TierC, true
	default:
		return TierUnknown, false
	}
}

// Model is one canonical model identity.
type Model struct {
	// ID is the canonical Structured Vibe identity, not a provider string.
	ID      string   `json:"id"`
	Tier    Tier     `json:"tier"`
	Aliases []string `json:"aliases,omitempty"`
}

// Registry maps external model identifiers to canonical identities.
type Registry struct {
	models  map[string]Model
	byAlias map[string]string
}

// file mirrors the on-disk registry shape.
type file struct {
	Version int `yaml:"version"`
	Models  map[string]struct {
		Tier    string   `yaml:"tier"`
		Aliases []string `yaml:"aliases"`
	} `yaml:"models"`
}

// Load reads the registry from path.
func Load(path string) (*Registry, diag.Diagnostics) {
	var d diag.Diagnostics

	raw, err := os.ReadFile(path)
	if err != nil {
		d.Errorf("models.unreadable", path, "cannot read model registry: %v", err)
		return nil, d
	}
	return Parse(raw, path)
}

// Parse builds a registry from raw YAML.
func Parse(raw []byte, path string) (*Registry, diag.Diagnostics) {
	var d diag.Diagnostics

	var f file
	if err := yaml.Unmarshal(raw, &f); err != nil {
		d.Errorf("models.invalid", path, "model registry is not valid YAML: %v", err)
		return nil, d
	}

	r := &Registry{
		models:  map[string]Model{},
		byAlias: map[string]string{},
	}

	ids := make([]string, 0, len(f.Models))
	for id := range f.Models {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		entry := f.Models[id]

		tier, ok := ParseTier(entry.Tier)
		if !ok {
			d.Errorf("models.tier.invalid", path,
				"model %q declares tier %q; valid tiers are A, B, C "+
					"(unknown is a state, not a registry value)", id, entry.Tier)
			continue
		}

		m := Model{ID: id, Tier: tier, Aliases: entry.Aliases}
		r.models[id] = m

		for _, alias := range entry.Aliases {
			if prev, dup := r.byAlias[alias]; dup && prev != id {
				d.Errorf("models.alias.duplicate", path,
					"alias %q maps to both %q and %q; an alias must identify exactly one model",
					alias, prev, id)
				continue
			}
			r.byAlias[alias] = id
		}
	}

	if d.HasErrors() {
		return nil, d
	}
	return r, d
}

// Lookup resolves an external model identifier to its canonical model.
//
// Matching is exact. A miss returns ok=false, which the caller renders as
// unknown.
func (r *Registry) Lookup(external string) (Model, bool) {
	if r == nil {
		return Model{}, false
	}
	id, ok := r.byAlias[external]
	if !ok {
		return Model{}, false
	}
	m, ok := r.models[id]
	return m, ok
}

// TierOf returns the tier for an external identifier, or TierUnknown.
func (r *Registry) TierOf(external string) Tier {
	m, ok := r.Lookup(external)
	if !ok {
		return TierUnknown
	}
	return m.Tier
}

// Len reports how many canonical models are registered.
func (r *Registry) Len() int {
	if r == nil {
		return 0
	}
	return len(r.models)
}

// Models returns all canonical models sorted by ID.
func (r *Registry) Models() []Model {
	if r == nil {
		return nil
	}
	out := make([]Model, 0, len(r.models))
	for _, m := range r.models {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
