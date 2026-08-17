package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/mwenkdev/structured-vibe/internal/cliout"
	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/env"
	"github.com/mwenkdev/structured-vibe/internal/pack"
	"github.com/mwenkdev/structured-vibe/internal/paths"
	"github.com/mwenkdev/structured-vibe/internal/resolve"
	"github.com/mwenkdev/structured-vibe/internal/scope"
)

// validateResult is the machine-readable outcome of validation.
type validateResult struct {
	Target string             `json:"target"`
	Packs  []resolve.PackRef  `json:"packs"`
	Skills []validatedSkill   `json:"skills"`
	Counts validateCountsJSON `json:"counts"`
}

type validatedSkill struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
	Pack  string `json:"pack"`
	Path  string `json:"path"`
}

type validateCountsJSON struct {
	Packs    int `json:"packs"`
	Skills   int `json:"skills"`
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

func (r *validateResult) PrintHuman(w io.Writer) {
	fmt.Fprintf(w, "target: %s\n", r.Target)
	if len(r.Packs) > 0 {
		fmt.Fprintf(w, "\npacks (%d):\n", len(r.Packs))
		for _, p := range r.Packs {
			fmt.Fprintf(w, "  %-10s %s %s\n", p.Scope, p.Name, p.Version)
		}
	}
	if len(r.Skills) > 0 {
		fmt.Fprintf(w, "\nskills (%d):\n", len(r.Skills))
		for _, s := range r.Skills {
			fmt.Fprintf(w, "  %-24s %s:%s\n", s.Name, s.Scope, s.Pack)
		}
	}
	if r.Counts.Errors == 0 {
		fmt.Fprintf(w, "\nvalid")
		if r.Counts.Warnings > 0 {
			fmt.Fprintf(w, " (%d warning(s))", r.Counts.Warnings)
		}
		fmt.Fprintln(w)
	}
}

func runValidate(e *Env, args []string) error {
	fs := newFlagSet("validate", e.Stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON on stdout")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return &ExitError{Code: 2}
	}

	out := cliout.New(e.Stdout, e.Stderr, *asJSON)

	if len(rest) > 1 {
		fmt.Fprintln(e.Stderr, "svibe: validate accepts at most one pack path")
		return &ExitError{Code: 2}
	}

	if len(rest) == 1 {
		return validateSinglePack(out, rest[0])
	}
	return validateEnvironment(out, e.cwd())
}

// validateSinglePack validates one pack in isolation.
func validateSinglePack(out *cliout.Writer, target string) error {
	var d diag.Diagnostics

	abs, err := filepath.Abs(target)
	if err != nil {
		d.Errorf("validate.path", target, "cannot resolve pack path: %v", err)
		out.Emit(false, d, nil)
		return Failure
	}

	// An isolated pack has no scope of its own. Project is used only so the
	// pack loader has a value; precedence is meaningless for a single pack.
	p, pd := pack.Load(abs, scope.Project)
	d.Extend(pd)

	res := &validateResult{Target: abs}
	if p != nil {
		res.Packs = []resolve.PackRef{{
			Name: p.Manifest.Name, Version: p.Manifest.Version, Root: p.Root, Scope: "isolated",
		}}
		for _, s := range p.Skills {
			res.Skills = append(res.Skills, validatedSkill{
				Name: s.Name, Scope: "isolated", Pack: p.Manifest.Name, Path: s.Path,
			})
		}
	}
	res.Counts = validateCountsJSON{
		Packs:    len(res.Packs),
		Skills:   len(res.Skills),
		Errors:   len(d.Errors()),
		Warnings: len(d.Warnings()),
	}

	if !out.Emit(!d.HasErrors(), d, res) {
		return Failure
	}
	return nil
}

// validateEnvironment validates the active core/user/project environment.
func validateEnvironment(out *cliout.Writer, cwd string) error {
	environment, d := env.Load(cwd)
	if environment == nil {
		out.Emit(false, d, nil)
		return Failure
	}

	target := "active environment"
	if environment.ProjectRoot != "" {
		target = environment.ProjectRoot
	}

	res := &validateResult{Target: target}

	if !d.HasErrors() {
		resolution, rd := resolve.Resolve(resolve.Input{
			Packs:        environment.Packs,
			ProjectRoot:  environment.ProjectRoot,
			Home:         environment.Home,
			ExcludeRoots: excludeRoots(environment.ProjectRoot),
		})
		d.Extend(rd)
		if resolution != nil {
			res.Packs = resolution.Packs
			for _, s := range resolution.Skills {
				res.Skills = append(res.Skills, validatedSkill{
					Name: s.Name, Scope: s.Origin.Scope, Pack: s.Origin.Pack, Path: s.Origin.Path,
				})
			}
		}
	}

	res.Counts = validateCountsJSON{
		Packs:    len(res.Packs),
		Skills:   len(res.Skills),
		Errors:   len(d.Errors()),
		Warnings: len(d.Warnings()),
	}

	if !out.Emit(!d.HasErrors(), d, res) {
		return Failure
	}
	return nil
}

// excludeRoots lists directories that host-collision detection must ignore.
func excludeRoots(projectRoot string) []string {
	if projectRoot == "" {
		return nil
	}
	return []string{paths.GeneratedDir(projectRoot)}
}
