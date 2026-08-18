// Package cli implements command dispatch for the svibe binary.
//
// The CLI provides infrastructure only: discovery, validation, resolution,
// materialization, integrity checks, status, and administrative integration
// work. It deliberately exposes no workflow commands such as plan, review or
// execute; the development workflow runs in the host through skills
// (architecture 4.1).
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/managed"
	"github.com/mwenkdev/structured-vibe/internal/paths"
)

// Env carries process context so commands stay testable.
type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	Cwd    string

	// Manifest overrides the embedded managed payload. Tests set this to
	// describe a constructed installation; production leaves it nil.
	Manifest managed.Manifest

	// VerifyIntegration overrides the host-integration precondition. Tests
	// set this to describe an installed host; production leaves it nil.
	VerifyIntegration func() diag.Diagnostics

	// pre holds diagnostics produced before dispatch, such as managed-file
	// modification warnings, so commands can surface them in their envelope.
	pre diag.Diagnostics
}

// manifest returns the effective managed manifest.
func (e *Env) manifest() managed.Manifest {
	if e.Manifest != nil {
		return e.Manifest
	}
	return managed.Embedded()
}

// baseDiags returns a fresh copy of the pre-dispatch diagnostics.
func (e *Env) baseDiags() diag.Diagnostics {
	out := make(diag.Diagnostics, len(e.pre))
	copy(out, e.pre)
	return out
}

// Command is one svibe subcommand.
type Command struct {
	Name    string
	Summary string
	Run     func(e *Env, args []string) error
}

var commands = []Command{
	{Name: "init", Summary: "create the project pack in the current repository", Run: runInit},
	{Name: "validate", Summary: "validate the active environment, or one pack", Run: runValidate},
	{Name: "resolve", Summary: "show the resolved skill set and its provenance", Run: runResolve},
	{Name: "advise", Summary: "compare a skill's capability recommendation to a model", Run: runAdvise},
	{Name: "sync", Summary: "publish the resolved skill snapshot for the host", Run: runSync},
	{Name: "status", Summary: "report whether generated output is current", Run: runStatus},
	{Name: "version", Summary: "print the svibe version", Run: runVersion},
}

// ExitError carries a process exit code.
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

// Failure is the standard non-zero outcome for a command that reported
// errors through the output envelope. The diagnostics are already printed.
var Failure = &ExitError{Code: 1}

// Run dispatches args to a subcommand.
func Run(e *Env, args []string) error {
	if len(args) == 0 {
		usage(e.Stderr)
		return &ExitError{Code: 2}
	}

	switch args[0] {
	case "-h", "--help", "help":
		usage(e.Stdout)
		return nil
	case "-v", "--version":
		return runVersion(e, nil)
	}

	for _, c := range commands {
		if c.Name != args[0] {
			continue
		}
		// Integrity is checked before executing any requested command,
		// deliberately including innocuous ones (architecture 16.2).
		if err := e.checkIntegrity(); err != nil {
			return err
		}
		return c.Run(e, args[1:])
	}

	fmt.Fprintf(e.Stderr, "svibe: unknown command %q\n\n", args[0])
	usage(e.Stderr)
	return &ExitError{Code: 2}
}

func usage(w io.Writer) {
	fmt.Fprint(w, "svibe - Structured Vibe infrastructure\n\nUsage:\n  svibe <command> [flags]\n\nCommands:\n")
	for _, c := range commands {
		fmt.Fprintf(w, "  %-10s %s\n", c.Name, c.Summary)
	}
	fmt.Fprint(w, "\nFlags:\n  --json    emit machine-readable JSON on stdout\n")
}

// newFlagSet builds a flag set that reports errors through our own usage.
func newFlagSet(name string, out io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(out)
	return fs
}

// parseMixed parses flags that may appear before, after, or between
// positional arguments.
//
// The standard flag package stops at the first non-flag argument, which would
// make "svibe validate ./pack --json" silently treat --json as a positional.
func parseMixed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			return positional, nil
		}
		positional = append(positional, rest[0])
		rest = rest[1:]
	}
}

// checkIntegrity verifies the managed runtime payload before dispatch.
//
// A missing managed file fails here, before the command runs, because the
// installed release is incomplete. A modified managed file only warns; the
// warning is carried into the command's own output so it appears in the JSON
// envelope alongside everything else.
func (e *Env) checkIntegrity() error {
	configHome, err := paths.ConfigHome()
	if err != nil {
		fmt.Fprintf(e.Stderr, "svibe: cannot determine configuration root: %v\n", err)
		return Failure
	}

	_, d := managed.Check(configHome, e.manifest())

	if d.HasErrors() {
		for _, x := range d.Sorted() {
			if x.Path != "" {
				fmt.Fprintf(e.Stderr, "%s: %s\n  at %s\n  (%s)\n", x.Severity, x.Message, x.Path, x.Code)
				continue
			}
			fmt.Fprintf(e.Stderr, "%s: %s\n  (%s)\n", x.Severity, x.Message, x.Code)
		}
		return Failure
	}

	e.pre = d
	return nil
}

// Cwd returns the working directory, defaulting to the process one.
func (e *Env) cwd() string {
	if e.Cwd != "" {
		return e.Cwd
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
