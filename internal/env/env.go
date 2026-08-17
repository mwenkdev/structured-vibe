// Package env assembles the active Structured Vibe environment: the core
// pack, discovered user packs, and the project pack when one exists.
//
// Outside a Git repository, project scope does not exist but resolve and sync
// still work with core and user scopes (architecture 6).
package env

import (
	"errors"
	"os"

	"github.com/mwenkdev/structured-vibe/internal/diag"
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
}

// Load discovers the active environment from the given working directory.
func Load(cwd string) (*Environment, diag.Diagnostics) {
	var d diag.Diagnostics

	configHome, err := paths.ConfigHome()
	if err != nil {
		d.Errorf("env.config-home", "", "cannot determine configuration root: %v", err)
		return nil, d
	}

	e := &Environment{ConfigHome: configHome}
	if home, herr := os.UserHomeDir(); herr == nil {
		e.Home = home
	}

	// Core scope. A missing core pack is not fatal here; managed-installation
	// integrity is the layer that decides whether a missing shipped payload is
	// a hard failure.
	coreDir := paths.CoreDir(configHome)
	if _, statErr := os.Stat(coreDir); statErr == nil {
		p, pd := pack.Load(coreDir, scope.Core)
		d.Extend(pd)
		if p != nil {
			e.Packs = append(e.Packs, p)
		}
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
