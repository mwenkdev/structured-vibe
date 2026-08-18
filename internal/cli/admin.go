package cli

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/mwenkdev/structured-vibe/internal/buildinfo"
	"github.com/mwenkdev/structured-vibe/internal/cliout"
	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/hostint"
	"github.com/mwenkdev/structured-vibe/internal/paths"
)

// hostTargets are the host integrations svibe can install.
//
// OpenCode is the only v1 host. The list exists so "admin update" with no
// target can iterate rather than special-casing one name.
var hostTargets = []string{"opencode"}

type adminResult struct {
	Action  string            `json:"action"`
	Targets []adminTargetInfo `json:"targets"`
	Version string            `json:"svibe_version"`
}

type adminTargetInfo struct {
	Host string `json:"host"`
	// Installed reports whether the integration is present after the command.
	Installed bool   `json:"installed"`
	Path      string `json:"path,omitempty"`
	// Changed reports whether this invocation modified the installation.
	Changed bool `json:"changed"`
	// Skipped reports that the target was not installed, so update left it
	// alone rather than installing it implicitly.
	Skipped bool `json:"skipped"`
}

func (r *adminResult) PrintHuman(w io.Writer) {
	for _, t := range r.Targets {
		switch {
		case t.Skipped:
			fmt.Fprintf(w, "%s: not installed, skipped\n", t.Host)
			fmt.Fprintf(w, "  run \"svibe admin setup %s\" to install it\n", t.Host)
		case t.Changed:
			fmt.Fprintf(w, "%s: installed %s\n", t.Host, t.Path)
		default:
			fmt.Fprintf(w, "%s: already current (%s)\n", t.Host, t.Path)
		}
	}
	if r.Action == "setup" {
		fmt.Fprintln(w, "\nRestart OpenCode to load the integration.")
	}
}

func runAdmin(e *Env, args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(e.Stderr, "svibe: admin requires a subcommand: setup or update")
		return &ExitError{Code: 2}
	}

	switch args[0] {
	case "setup":
		return runAdminSetup(e, args[1:])
	case "update":
		return runAdminUpdate(e, args[1:])
	default:
		fmt.Fprintf(e.Stderr, "svibe: unknown admin subcommand %q\n", args[0])
		return &ExitError{Code: 2}
	}
}

// runAdminSetup installs a host integration. A target is required: setup is a
// deliberate act, not something to spray across every known host.
func runAdminSetup(e *Env, args []string) error {
	fs := newFlagSet("admin setup", e.Stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON on stdout")

	rest, err := parseMixed(fs, args)
	if err != nil {
		return &ExitError{Code: 2}
	}

	out := cliout.New(e.Stdout, e.Stderr, *asJSON)
	d := e.baseDiags()

	targets, terr := resolveTargets(rest, true)
	if terr != nil {
		fmt.Fprintln(e.Stderr, "svibe:", terr)
		return &ExitError{Code: 2}
	}

	configHome, cerr := paths.ConfigHome()
	if cerr != nil {
		d.Errorf("admin.config-home", "", "cannot determine configuration root: %v", cerr)
		out.Emit(false, d, nil)
		return Failure
	}

	res := &adminResult{Action: "setup", Version: buildinfo.Version}
	home := homeDir()

	for _, host := range targets {
		info, hd := installHost(host, configHome, home)
		d.Extend(hd)
		res.Targets = append(res.Targets, info)
	}

	if !out.Emit(!d.HasErrors(), d, res) {
		return Failure
	}
	return nil
}

// runAdminUpdate refreshes installed host integrations.
//
// With no target it updates every installed integration. It does not install
// a host that was never set up: update means "bring what I have in line with
// this release", not "adopt every host svibe knows about".
func runAdminUpdate(e *Env, args []string) error {
	fs := newFlagSet("admin update", e.Stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON on stdout")

	rest, err := parseMixed(fs, args)
	if err != nil {
		return &ExitError{Code: 2}
	}

	out := cliout.New(e.Stdout, e.Stderr, *asJSON)
	d := e.baseDiags()

	targets, terr := resolveTargets(rest, false)
	if terr != nil {
		fmt.Fprintln(e.Stderr, "svibe:", terr)
		return &ExitError{Code: 2}
	}

	configHome, cerr := paths.ConfigHome()
	if cerr != nil {
		d.Errorf("admin.config-home", "", "cannot determine configuration root: %v", cerr)
		out.Emit(false, d, nil)
		return Failure
	}

	res := &adminResult{Action: "update", Version: buildinfo.Version}
	home := homeDir()
	explicit := len(rest) > 0

	for _, host := range targets {
		if !explicit && !hostInstalled(host, home) {
			res.Targets = append(res.Targets, adminTargetInfo{Host: host, Skipped: true})
			continue
		}
		info, hd := installHost(host, configHome, home)
		d.Extend(hd)
		res.Targets = append(res.Targets, info)
	}

	if !out.Emit(!d.HasErrors(), d, res) {
		return Failure
	}
	return nil
}

// installHost installs or refreshes one host integration.
func installHost(host, configHome, home string) (adminTargetInfo, diag.Diagnostics) {
	var d diag.Diagnostics
	info := adminTargetInfo{Host: host}

	switch host {
	case "opencode":
		matched, present := hostint.InstalledMatchesRelease(configHome, home)
		if present && matched {
			info.Installed = true
			info.Path = hostint.InstalledPluginPath(home)
			return info, d
		}

		path, id := hostint.Install(configHome, home)
		d.Extend(id)
		if id.HasErrors() {
			return info, d
		}
		info.Installed = true
		info.Changed = true
		info.Path = path
	default:
		d.Errorf("admin.unknown-host", "", "unknown host integration %q", host)
	}
	return info, d
}

func hostInstalled(host, home string) bool {
	if host != "opencode" || home == "" {
		return false
	}
	info, err := os.Stat(hostint.InstalledPluginPath(home))
	return err == nil && !info.IsDir() && info.Size() > 0
}

// resolveTargets validates requested hosts, defaulting to all known hosts.
func resolveTargets(requested []string, requireTarget bool) ([]string, error) {
	if len(requested) == 0 {
		if requireTarget {
			return nil, fmt.Errorf("admin setup requires a target host: %v", hostTargets)
		}
		out := append([]string(nil), hostTargets...)
		sort.Strings(out)
		return out, nil
	}

	for _, r := range requested {
		if !knownHost(r) {
			return nil, fmt.Errorf("unknown host integration %q; known hosts: %v", r, hostTargets)
		}
	}
	return requested, nil
}

func knownHost(name string) bool {
	for _, h := range hostTargets {
		if h == name {
			return true
		}
	}
	return false
}
