package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/mwenkdev/structured-vibe/internal/buildinfo"
	"github.com/mwenkdev/structured-vibe/internal/cliout"
	"github.com/mwenkdev/structured-vibe/internal/hostint"
	"github.com/mwenkdev/structured-vibe/internal/paths"
	"github.com/mwenkdev/structured-vibe/internal/sync"
)

// Snapshot states reported by status.
const (
	// stateCurrent means the published snapshot matches current inputs.
	stateCurrent = "current"
	// stateStale means inputs changed since the last sync. Staleness is
	// informational state, not command failure (architecture 14).
	stateStale = "stale"
	// stateMissing means nothing has been published yet.
	stateMissing = "missing"
	// stateInvalid means a snapshot exists but cannot be interpreted.
	stateInvalid = "invalid"
)

type statusResult struct {
	ProjectRoot  string `json:"project_root,omitempty"`
	ConfigHome   string `json:"config_home"`
	SnapshotRoot string `json:"snapshot_root,omitempty"`

	State string `json:"state"`
	// SkillCount is the number of skills the current resolution would publish.
	SkillCount int `json:"skill_count"`

	Fingerprint     string `json:"fingerprint"`
	SyncedAt        string `json:"synced_at,omitempty"`
	SyncedFrom      string `json:"synced_fingerprint,omitempty"`
	SvibeVersion    string `json:"svibe_version"`
	SnapshotVersion string `json:"snapshot_svibe_version,omitempty"`
	// VersionDrift means the snapshot was produced by a different svibe.
	// The CLI version is also the resolver/rules version.
	VersionDrift bool `json:"version_drift"`

	// SnapshotRelPath is the repo-relative path registered in skills.paths.
	SnapshotRelPath string `json:"snapshot_rel_path"`

	IntegrationInstalled bool   `json:"integration_installed"`
	IntegrationPath      string `json:"integration_path,omitempty"`
	Registered           bool   `json:"snapshot_registered"`
}

func (r *statusResult) PrintHuman(w io.Writer) {
	if r.ProjectRoot != "" {
		fmt.Fprintf(w, "project:     %s\n", r.ProjectRoot)
	} else {
		fmt.Fprintln(w, "project:     (none: not inside a git repository)")
	}
	fmt.Fprintf(w, "config:      %s\n", r.ConfigHome)
	fmt.Fprintf(w, "svibe:       %s\n", r.SvibeVersion)

	fmt.Fprintf(w, "\nsnapshot:    %s\n", r.State)
	if r.SnapshotRoot != "" {
		fmt.Fprintf(w, "  path:      %s\n", r.SnapshotRoot)
	}
	fmt.Fprintf(w, "  skills:    %d resolved\n", r.SkillCount)
	fmt.Fprintf(w, "  inputs:    %s\n", short(r.Fingerprint))
	if r.SyncedFrom != "" {
		fmt.Fprintf(w, "  published: %s at %s\n", short(r.SyncedFrom), r.SyncedAt)
	}
	if r.VersionDrift {
		fmt.Fprintf(w, "  drift:     published by svibe %s\n", r.SnapshotVersion)
	}

	fmt.Fprintf(w, "\nintegration: ")
	if r.IntegrationInstalled {
		fmt.Fprintf(w, "installed (%s)\n", r.IntegrationPath)
	} else {
		fmt.Fprintln(w, "not installed - run \"svibe admin setup opencode\"")
	}
	fmt.Fprintf(w, "registered:  %v\n", r.Registered)

	if r.State == stateStale {
		fmt.Fprintln(w, "\nRun \"svibe sync\" to republish.")
	}
	if r.State == stateMissing {
		fmt.Fprintln(w, "\nRun \"svibe sync\" to publish the snapshot.")
	}
}

func runStatus(e *Env, args []string) error {
	fs := newFlagSet("status", e.Stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON on stdout")

	rest, err := parseMixed(fs, args)
	if err != nil {
		return &ExitError{Code: 2}
	}
	if len(rest) > 0 {
		fmt.Fprintln(e.Stderr, "svibe: status takes no positional arguments")
		return &ExitError{Code: 2}
	}

	out := cliout.New(e.Stdout, e.Stderr, *asJSON)

	p, d := buildPlan(e)
	if p == nil || d.HasErrors() {
		out.Emit(false, d, nil)
		return Failure
	}

	res := &statusResult{
		ProjectRoot:     p.env.ProjectRoot,
		ConfigHome:      p.env.ConfigHome,
		SnapshotRoot:    p.snapshotRoot,
		SkillCount:      len(p.resolution.Skills),
		Fingerprint:     p.fingerprint,
		SvibeVersion:    buildinfo.Version,
		SnapshotRelPath: paths.OpenCodeSkillsRelPath,
	}

	res.State = evaluateSnapshot(p.snapshotRoot, p.fingerprint, res)

	// Integration presence is reported, not enforced. status is read-only and
	// a missing integration is only fatal when sync tries to publish.
	if p.env.Home != "" {
		path := hostint.InstalledPluginPath(p.env.Home)
		res.IntegrationPath = path
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Size() > 0 {
			res.IntegrationInstalled = true
		} else {
			d.Warnf("status.integration-missing", path,
				"the OpenCode integration is not installed, so capability warnings will "+
					"not appear in the host; run \"svibe admin setup opencode\"")
		}
	}

	if p.env.ProjectRoot != "" {
		res.Registered = snapshotRegistered(p.env.ProjectRoot)
		if !res.Registered {
			d.Warnf("status.not-registered", p.env.ProjectRoot,
				"the project OpenCode config does not list %q in skills.paths, so the "+
					"host will not load the synced snapshot; run \"svibe init\"",
				paths.OpenCodeSkillsRelPath)
		}
		d.Extend(hostint.CheckGlobalSkillsPathsShadowed(p.env.Home, p.env.ProjectRoot))
	}

	// Staleness is informational state, not command failure.
	if !out.Emit(!d.HasErrors(), d, res) {
		return Failure
	}
	return nil
}

// evaluateSnapshot classifies the published snapshot against current inputs.
func evaluateSnapshot(root, want string, res *statusResult) string {
	if root == "" {
		return stateMissing
	}

	skillsDir := sync.SkillsDir(root)
	if _, err := os.Stat(skillsDir); err != nil {
		return stateMissing
	}

	state, err := sync.ReadState(root)
	if err != nil {
		if os.IsNotExist(err) {
			// Skills are published but no state file describes them, so
			// freshness cannot be established.
			return stateInvalid
		}
		return stateInvalid
	}

	res.SyncedAt = state.SyncedAt
	res.SyncedFrom = state.Fingerprint
	res.SnapshotVersion = state.SvibeVersion
	res.VersionDrift = state.SvibeVersion != buildinfo.Version

	if state.Fingerprint == "" {
		return stateInvalid
	}
	if state.Fingerprint != want {
		return stateStale
	}
	// The CLI version is the rules version: identical inputs can resolve
	// differently under a different svibe, so drift means stale.
	if res.VersionDrift {
		return stateStale
	}
	return stateCurrent
}

// snapshotRegistered reports whether the project config lists the snapshot.
func snapshotRegistered(projectRoot string) bool {
	path, exists := hostint.FindProjectConfig(projectRoot)
	if !exists {
		return false
	}
	registered, err := hostint.HasSnapshotPath(path)
	if err != nil {
		return false
	}
	return registered
}
