package cli

import (
	"path/filepath"

	"github.com/mwenkdev/structured-vibe/internal/buildinfo"
	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/env"
	"github.com/mwenkdev/structured-vibe/internal/fingerprint"
	"github.com/mwenkdev/structured-vibe/internal/paths"
	"github.com/mwenkdev/structured-vibe/internal/resolve"
)

// plan is the shared preparation for sync and status: load the environment,
// resolve, and compute the fingerprint of the inputs.
type plan struct {
	env          *env.Environment
	resolution   *resolve.Resolution
	fingerprint  string
	snapshotRoot string
}

// buildPlan resolves the environment and computes the input fingerprint.
func buildPlan(e *Env) (*plan, diag.Diagnostics) {
	d := e.baseDiags()

	environment, ed := env.LoadWithOptions(e.cwd(), env.Options{Manifest: e.manifest()})
	d.Extend(ed)
	if environment == nil || d.HasErrors() {
		return nil, d
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
		return nil, d
	}

	fp, err := fingerprint.Compute(fingerprintInput(resolution))
	if err != nil {
		d.Errorf("plan.fingerprint", "", "cannot compute input fingerprint: %v", err)
		return nil, d
	}

	return &plan{
		env:          environment,
		resolution:   resolution,
		fingerprint:  fp,
		snapshotRoot: snapshotRoot(environment),
	}, d
}

// fingerprintInput assembles everything that could change the snapshot.
func fingerprintInput(r *resolve.Resolution) fingerprint.Input {
	in := fingerprint.Input{
		SvibeVersion: buildinfo.Version,
		Scopes:       scopeNames(),
	}
	for _, p := range r.Packs {
		in.Packs = append(in.Packs, p.Scope+":"+p.Name+"@"+p.Version+":"+p.Root)
	}
	for _, s := range r.Skills {
		in.Skills = append(in.Skills, fingerprint.SkillInput{
			Name:  s.Name,
			Scope: s.Origin.Scope,
			Pack:  s.Origin.Pack,
			Dir:   s.Dir,
		})
	}
	return in
}

// snapshotRoot is the OpenCode snapshot location for the environment.
//
// Inside a Git repository the snapshot is repo-local. Outside one, it falls
// back to the user config root (architecture 13.2).
func snapshotRoot(environment *env.Environment) string {
	if environment.ProjectRoot != "" {
		return paths.OpenCodeSnapshotDir(environment.ProjectRoot)
	}
	if environment.ConfigHome == "" {
		return ""
	}
	return filepath.Join(paths.FallbackGeneratedDir(environment.ConfigHome), "opencode")
}
