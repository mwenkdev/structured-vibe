// Package scope defines the ordered precedence scopes Structured Vibe
// resolves skills across.
//
// V1 exposes core < user < project. Resolver internals operate on an ordered
// set rather than hard-coding three special cases, so another scope (for
// example an organization scope) can be inserted later without changing the
// pack or skill contracts (architecture 9.3).
//
// This is architectural looseness, not a dormant feature.
package scope

// Scope is one precedence level. Higher Rank wins.
type Scope struct {
	Name string
	Rank int
}

// The v1 scopes.
var (
	Core    = Scope{Name: "core", Rank: 0}
	User    = Scope{Name: "user", Rank: 1}
	Project = Scope{Name: "project", Rank: 2}
)

// Ordered returns the active scopes from lowest to highest precedence.
func Ordered() []Scope {
	return []Scope{Core, User, Project}
}

// Beats reports whether s has strictly higher precedence than other.
func (s Scope) Beats(other Scope) bool { return s.Rank > other.Rank }

func (s Scope) String() string { return s.Name }
