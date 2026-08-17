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
)

// Env carries process context so commands stay testable.
type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	Cwd    string
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
		if c.Name == args[0] {
			return c.Run(e, args[1:])
		}
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
