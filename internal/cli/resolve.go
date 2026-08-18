package cli

import (
	"fmt"
	"io"

	"github.com/mwenkdev/structured-vibe/internal/buildinfo"
	"github.com/mwenkdev/structured-vibe/internal/cliout"
	"github.com/mwenkdev/structured-vibe/internal/env"
	"github.com/mwenkdev/structured-vibe/internal/resolve"
)

type resolveResult struct {
	ProjectRoot  string             `json:"project_root,omitempty"`
	ConfigHome   string             `json:"config_home"`
	Packs        []resolve.PackRef  `json:"packs"`
	Skills       []resolve.Resolved `json:"skills"`
	Scopes       []string           `json:"scopes"`
	SvibeVersion string             `json:"svibe_version"`
}

func (r *resolveResult) PrintHuman(w io.Writer) {
	if r.ProjectRoot != "" {
		fmt.Fprintf(w, "project: %s\n", r.ProjectRoot)
	} else {
		fmt.Fprintln(w, "project: (none: not inside a git repository)")
	}
	fmt.Fprintf(w, "config:  %s\n", r.ConfigHome)
	fmt.Fprintf(w, "scopes:  %s\n", joinStrings(r.Scopes, " < "))

	fmt.Fprintf(w, "\npacks (%d):\n", len(r.Packs))
	if len(r.Packs) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	for _, p := range r.Packs {
		fmt.Fprintf(w, "  %-8s %-24s %-8s %s\n", p.Scope, p.Name, p.Version, p.Root)
	}

	fmt.Fprintf(w, "\nskills (%d):\n", len(r.Skills))
	if len(r.Skills) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	for _, s := range r.Skills {
		tier := s.MinimumDriverTier
		if tier == "" {
			tier = "-"
		}
		fmt.Fprintf(w, "  %-24s %-8s %-24s tier=%s\n",
			s.Name, s.Origin.Scope, s.Origin.Pack, tier)
		for _, sh := range s.Shadowed {
			fmt.Fprintf(w, "      shadowed: %s:%s (%s)\n", sh.Scope, sh.Pack, sh.Path)
		}
	}
}

func runResolve(e *Env, args []string) error {
	fs := newFlagSet("resolve", e.Stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON on stdout")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return &ExitError{Code: 2}
	}
	if len(rest) > 0 {
		fmt.Fprintln(e.Stderr, "svibe: resolve takes no positional arguments")
		return &ExitError{Code: 2}
	}

	out := cliout.New(e.Stdout, e.Stderr, *asJSON)
	d := e.baseDiags()

	environment, ed := env.LoadWithOptions(e.cwd(), env.Options{Manifest: e.manifest()})
	d.Extend(ed)
	if environment == nil || d.HasErrors() {
		out.Emit(false, d, nil)
		return Failure
	}

	resolution, rd := resolve.Resolve(resolve.Input{
		Packs:          environment.Packs,
		ProjectRoot:    environment.ProjectRoot,
		Home:           environment.Home,
		ExcludeRoots:   excludeRoots(environment.ProjectRoot),
		ExtraSkillDirs: extraSkillDirs(environment.ProjectRoot, environment.Home),
	})
	d.Extend(rd)
	if resolution == nil {
		out.Emit(false, d, nil)
		return Failure
	}

	res := &resolveResult{
		ProjectRoot:  environment.ProjectRoot,
		ConfigHome:   environment.ConfigHome,
		Packs:        resolution.Packs,
		Skills:       resolution.Skills,
		Scopes:       scopeNames(),
		SvibeVersion: buildinfo.Version,
	}

	if !out.Emit(!d.HasErrors(), d, res) {
		return Failure
	}
	return nil
}

func joinStrings(items []string, sep string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}
