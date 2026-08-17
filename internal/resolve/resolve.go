// Package resolve selects the winning definition of each skill across scopes.
//
// Rules (architecture 9):
//
//   - Higher-precedence scopes replace lower-precedence definitions of the
//     same skill ID. The replacement is complete: there is no inheritance and
//     no merging.
//   - Two definitions of the same skill ID in the same scope is a hard error.
//     Structured Vibe never resolves same-scope ambiguity by filesystem order,
//     alphabetical order, last-write-wins, or host-specific behavior.
//   - Resolution tracks provenance, including shadowed definitions.
//
// Replacing a core workflow skill is normal and does not warrant a warning.
package resolve

import (
	"sort"

	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/hostskills"
	"github.com/mwenkdev/structured-vibe/internal/pack"
	"github.com/mwenkdev/structured-vibe/internal/scope"
	"github.com/mwenkdev/structured-vibe/internal/skill"
)

// Origin identifies where a skill definition came from.
type Origin struct {
	Scope string `json:"scope"`
	Pack  string `json:"pack"`
	Path  string `json:"path"`
}

// Resolved is one winning skill and the definitions it shadowed.
type Resolved struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Recommends        []string `json:"recommends,omitempty"`
	MinimumDriverTier string   `json:"minimum_driver_tier,omitempty"`

	Origin   Origin   `json:"origin"`
	Shadowed []Origin `json:"shadowed,omitempty"`

	// Dir is the winning skill directory, used by sync to materialize it.
	Dir string `json:"-"`
}

// Resolution is the outcome of resolving an environment.
type Resolution struct {
	Skills []Resolved `json:"skills"`
	Packs  []PackRef  `json:"packs"`
}

// PackRef is a loaded pack recorded for provenance.
type PackRef struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Scope   string `json:"scope"`
	Root    string `json:"root"`
}

// Input is the set of packs to resolve, grouped by scope.
type Input struct {
	Packs []*pack.Pack

	// ProjectRoot and Home enable host-collision detection. Both optional.
	ProjectRoot string
	Home        string
	// ExcludeRoots are directories to ignore during collision detection,
	// normally the generated snapshot.
	ExcludeRoots []string
}

type candidate struct {
	sk    skill.Skill
	pk    *pack.Pack
	scope scope.Scope
}

// Resolve computes the winning skill set.
func Resolve(in Input) (*Resolution, diag.Diagnostics) {
	var d diag.Diagnostics

	// Loaded pack names must be unique within the active environment.
	byPackName := map[string]*pack.Pack{}
	for _, p := range in.Packs {
		if prev, dup := byPackName[p.Manifest.Name]; dup {
			d.Errorf("resolve.pack.duplicate", p.Root,
				"pack name %q is loaded twice (also at %s); loaded pack names must be unique",
				p.Manifest.Name, prev.Root)
			continue
		}
		byPackName[p.Manifest.Name] = p
	}
	if d.HasErrors() {
		return nil, d
	}

	// Group candidates by skill name, then by scope.
	grouped := map[string]map[int][]candidate{}
	for _, p := range in.Packs {
		for _, s := range p.Skills {
			if grouped[s.Name] == nil {
				grouped[s.Name] = map[int][]candidate{}
			}
			grouped[s.Name][p.Scope.Rank] = append(grouped[s.Name][p.Scope.Rank],
				candidate{sk: s, pk: p, scope: p.Scope})
		}
	}

	names := make([]string, 0, len(grouped))
	for n := range grouped {
		names = append(names, n)
	}
	sort.Strings(names)

	var out []Resolved
	for _, name := range names {
		byRank := grouped[name]

		// Same-scope ambiguity is a hard error, checked in every scope rather
		// than only the winning one. An ambiguous lower scope is still a
		// broken environment even if a higher scope would have won.
		ambiguous := false
		ranks := make([]int, 0, len(byRank))
		for r := range byRank {
			ranks = append(ranks, r)
		}
		sort.Ints(ranks)

		for _, r := range ranks {
			cands := byRank[r]
			if len(cands) < 2 {
				continue
			}
			ambiguous = true
			sort.Slice(cands, func(i, j int) bool {
				return cands[i].pk.Manifest.Name < cands[j].pk.Manifest.Name
			})
			packNames := make([]string, 0, len(cands))
			for _, c := range cands {
				packNames = append(packNames, c.scope.Name+":"+c.pk.Manifest.Name+"/"+name)
			}
			d.Errorf("resolve.ambiguous", cands[0].sk.Path,
				"skill %q is defined by multiple packs in the same scope (%s); "+
					"resolve this by removing or renaming one definition",
				name, joinWith(packNames, ", "))
		}
		if ambiguous {
			continue
		}

		// Highest rank wins; everything below it is shadowed.
		winnerRank := ranks[len(ranks)-1]
		win := byRank[winnerRank][0]

		var shadowed []Origin
		for i := len(ranks) - 2; i >= 0; i-- {
			c := byRank[ranks[i]][0]
			shadowed = append(shadowed, Origin{
				Scope: c.scope.Name,
				Pack:  c.pk.Manifest.Name,
				Path:  c.sk.Path,
			})
		}

		out = append(out, Resolved{
			Name:              win.sk.Name,
			Description:       win.sk.Description,
			Recommends:        win.sk.Recommends,
			MinimumDriverTier: win.sk.MinimumDriverTier,
			Origin: Origin{
				Scope: win.scope.Name,
				Pack:  win.pk.Manifest.Name,
				Path:  win.sk.Path,
			},
			Shadowed: shadowed,
			Dir:      win.sk.Dir,
		})
	}

	if d.HasErrors() {
		return nil, d
	}

	d.Extend(detectHostCollisions(out, in))

	packs := make([]PackRef, 0, len(in.Packs))
	for _, p := range in.Packs {
		packs = append(packs, PackRef{
			Name:    p.Manifest.Name,
			Version: p.Manifest.Version,
			Scope:   p.Scope.Name,
			Root:    p.Root,
		})
	}
	sort.Slice(packs, func(i, j int) bool {
		if packs[i].Scope != packs[j].Scope {
			return packs[i].Scope < packs[j].Scope
		}
		return packs[i].Name < packs[j].Name
	})

	return &Resolution{Skills: out, Packs: packs}, d
}

// detectHostCollisions warns when a resolved skill name is also defined in a
// location the host discovers independently.
//
// This is a warning, not an error: the environment is structurally usable,
// and the user may legitimately want both. But which one the host actually
// loads is not deterministic, so the human needs to know.
func detectHostCollisions(resolved []Resolved, in Input) diag.Diagnostics {
	var d diag.Diagnostics
	if in.ProjectRoot == "" && in.Home == "" {
		return d
	}

	host := hostskills.Scan(in.ProjectRoot, in.Home, in.ExcludeRoots)
	if len(host) == 0 {
		return d
	}

	for _, r := range resolved {
		found, ok := host[r.Name]
		if !ok {
			continue
		}
		for _, f := range found {
			d.Warnf("resolve.host-collision", f.Path,
				"skill %q also exists in %s; the host resolves duplicate skill names "+
					"nondeterministically, so it may load that definition instead of the "+
					"resolved one at %s",
				r.Name, f.Origin, r.Origin.Path)
		}
	}
	return d
}

func joinWith(items []string, sep string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}
