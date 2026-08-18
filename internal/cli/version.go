package cli

import (
	"fmt"
	"io"

	"github.com/mwenkdev/structured-vibe/internal/buildinfo"
	"github.com/mwenkdev/structured-vibe/internal/cliout"
	"github.com/mwenkdev/structured-vibe/internal/scope"
)

type versionResult struct {
	Version string `json:"version"`
}

func (r *versionResult) PrintHuman(w io.Writer) {
	fmt.Fprintln(w, r.Version)
}

func runVersion(e *Env, args []string) error {
	fs := newFlagSet("version", e.Stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON on stdout")
	if err := fs.Parse(args); err != nil {
		return &ExitError{Code: 2}
	}

	out := cliout.New(e.Stdout, e.Stderr, *asJSON)
	out.Emit(true, e.baseDiags(), &versionResult{Version: buildinfo.Version})
	return nil
}

// scopeNames lists the active scopes from lowest to highest precedence.
func scopeNames() []string {
	ordered := scope.Ordered()
	names := make([]string, 0, len(ordered))
	for _, s := range ordered {
		names = append(names, s.Name)
	}
	return names
}
