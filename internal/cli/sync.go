package cli

import (
	"fmt"
	"io"

	"github.com/mwenkdev/structured-vibe/internal/buildinfo"
	"github.com/mwenkdev/structured-vibe/internal/cliout"
	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/hostint"
	"github.com/mwenkdev/structured-vibe/internal/sync"
)

type syncResult struct {
	SnapshotRoot string `json:"snapshot_root"`
	SkillsDir    string `json:"skills_dir"`
	SkillCount   int    `json:"skill_count"`
	Fingerprint  string `json:"fingerprint"`
	SyncedAt     string `json:"synced_at"`
	// RestartRequired is always true: the host memoizes skill discovery at
	// startup and never hot-reloads it.
	RestartRequired bool `json:"restart_required"`
}

func (r *syncResult) PrintHuman(w io.Writer) {
	fmt.Fprintf(w, "synced %d skill(s)\n", r.SkillCount)
	fmt.Fprintf(w, "  %s\n", r.SkillsDir)
	fmt.Fprintf(w, "  fingerprint %s\n", short(r.Fingerprint))
	fmt.Fprintln(w, "\nRestart OpenCode to pick up these skills.")
	fmt.Fprintln(w, "OpenCode reads skills once at startup and does not reload them.")
}

func runSync(e *Env, args []string) error {
	fs := newFlagSet("sync", e.Stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON on stdout")

	rest, err := parseMixed(fs, args)
	if err != nil {
		return &ExitError{Code: 2}
	}
	if len(rest) > 0 {
		fmt.Fprintln(e.Stderr, "svibe: sync takes no positional arguments")
		return &ExitError{Code: 2}
	}

	out := cliout.New(e.Stdout, e.Stderr, *asJSON)

	p, d := buildPlan(e)
	if p == nil || d.HasErrors() {
		out.Emit(false, d, nil)
		return Failure
	}

	if p.snapshotRoot == "" {
		d.Errorf("sync.no-target", "", "cannot determine where to publish the snapshot")
		out.Emit(false, d, nil)
		return Failure
	}

	// A project-scope skills.paths replaces rather than merges with a global
	// one, so tell the user before their global skills quietly disappear.
	d.Extend(hostint.CheckGlobalSkillsPathsShadowed(p.env.Home, p.env.ProjectRoot))

	home := p.env.Home
	res, sd := sync.Run(sync.Request{
		SnapshotRoot:      p.snapshotRoot,
		Resolution:        p.resolution,
		Fingerprint:       p.fingerprint,
		SvibeVersion:      buildinfo.Version,
		VerifyIntegration: integrationVerifier(e, home),
	})
	d.Extend(sd)
	if res == nil {
		out.Emit(false, d, nil)
		return Failure
	}

	result := &syncResult{
		SnapshotRoot:    res.SnapshotRoot,
		SkillsDir:       res.SkillsDir,
		SkillCount:      res.SkillCount,
		Fingerprint:     res.Fingerprint,
		SyncedAt:        res.SyncedAt,
		RestartRequired: true,
	}

	if !out.Emit(!d.HasErrors(), d, result) {
		return Failure
	}
	return nil
}

// integrationVerifier returns the host-integration precondition.
//
// A missing or unusable OpenCode integration is a hard sync failure
// (architecture 13.4). Tests override this to describe an installed host.
func integrationVerifier(e *Env, home string) func() diag.Diagnostics {
	if e.VerifyIntegration != nil {
		return e.VerifyIntegration
	}
	return func() diag.Diagnostics { return hostint.VerifyIntegration(home) }
}

func short(fp string) string {
	if len(fp) > 12 {
		return fp[:12]
	}
	return fp
}
