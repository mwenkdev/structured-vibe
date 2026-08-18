// Package env assembles the active Structured Vibe environment: the core
// pack, discovered user packs, and the project pack when one exists.
//
// Outside a Git repository, project scope does not exist but resolve and sync
// still work with core and user scopes (architecture 6).
package env

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/managed"
	"github.com/mwenkdev/structured-vibe/internal/models"
	"github.com/mwenkdev/structured-vibe/internal/pack"
	"github.com/mwenkdev/structured-vibe/internal/paths"
	"github.com/mwenkdev/structured-vibe/internal/scope"
)

// Environment is the resolved set of inputs for a command invocation.
type Environment struct {
	ConfigHome string
	// ProjectRoot is empty when not inside a Git repository.
	ProjectRoot string
	Home        string
	Packs       []*pack.Pack
	// Registry is the managed model registry. It may be nil when the registry
	// could not be loaded; callers treat every model as unknown in that case.
	Registry *models.Registry
}

// Options controls environment assembly.
type Options struct {
	// Manifest is the managed payload defining closed core membership.
	// When nil, the embedded release manifest is used.
	Manifest managed.Manifest
}

// Load discovers the active environment from the given working directory.
func Load(cwd string) (*Environment, diag.Diagnostics) {
	return LoadWithOptions(cwd, Options{})
}

// LoadWithOptions discovers the environment with explicit options.
func LoadWithOptions(cwd string, opts Options) (*Environment, diag.Diagnostics) {
	var d diag.Diagnostics

	manifest := opts.Manifest
	if manifest == nil {
		manifest = managed.Embedded()
	}

	configHome, err := paths.ConfigHome()
	if err != nil {
		d.Errorf("env.config-home", "", "cannot determine configuration root: %v", err)
		return nil, d
	}

	e := &Environment{ConfigHome: configHome}
	if home, herr := os.UserHomeDir(); herr == nil {
		e.Home = home
	}

	// Core scope. Membership is closed: it comes from the shipped manifest
	// rather than whatever happens to exist on disk.
	coreSkills := managed.CoreSkills(manifest)
	coreDir := paths.CoreDir(configHome)
	if _, statErr := os.Stat(coreDir); statErr == nil {
		p, pd := pack.LoadWithOptions(coreDir, scope.Core, pack.Options{
			AllowSkill: func(name string) bool { return coreSkills[name] },
		})
		d.Extend(pd)
		if p != nil {
			e.Packs = append(e.Packs, p)
		}
	}

	// Managed model registry.
	registryPath := filepath.Join(configHome, filepath.FromSlash(managed.ModelRegistryPath))
	if _, statErr := os.Stat(registryPath); statErr == nil {
		r, rd := models.Load(registryPath)
		d.Extend(rd)
		e.Registry = r
	}

	// User scope.
	userPacks, ud := pack.DiscoverUserPacks(paths.UserPacksDir(configHome))
	d.Extend(ud)
	e.Packs = append(e.Packs, userPacks...)

	// Project scope.
	root, perr := paths.ProjectRoot(cwd)
	switch {
	case perr == nil:
		e.ProjectRoot = root
		packDir := paths.ProjectPackDir(root)
		if _, statErr := os.Stat(packDir); statErr == nil {
			p, pd := pack.Load(packDir, scope.Project)
			d.Extend(pd)
			if p != nil {
				e.Packs = append(e.Packs, p)
			}
		}
	case errors.Is(perr, paths.ErrNoProject):
		// Project scope simply does not exist. Not an error.
	default:
		d.Warnf("env.project-root", cwd, "cannot determine project root: %v", perr)
	}

	return e, d
}
