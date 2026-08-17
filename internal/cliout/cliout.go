// Package cliout implements the CLI output contract.
//
// Rules (architecture 15):
//
//   - stdout is pure JSON when --json is used;
//   - human-readable warnings and errors go to stderr;
//   - warnings also appear as structured JSON data;
//   - "ok" represents command success, not a claim that no advisory state
//     exists;
//   - automation inspects JSON state rather than inferring domain semantics
//     from clever exit-code conventions;
//   - non-zero process exit remains appropriate for actual command failure.
package cliout

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/mwenkdev/structured-vibe/internal/diag"
)

// Envelope is the consistent top-level JSON shape for every command.
type Envelope struct {
	OK       bool              `json:"ok"`
	Warnings []diag.Diagnostic `json:"warnings"`
	Errors   []diag.Diagnostic `json:"errors"`
	Result   any               `json:"result"`
}

// Writer emits command output in either human or JSON form.
type Writer struct {
	Out  io.Writer
	Err  io.Writer
	JSON bool
}

// New builds a Writer.
func New(out, errw io.Writer, asJSON bool) *Writer {
	return &Writer{Out: out, Err: errw, JSON: asJSON}
}

// Emit writes the command outcome and reports whether it succeeded.
//
// ok is the command's own success determination. Errors in d also force
// failure, so a caller cannot accidentally report success alongside errors.
func (w *Writer) Emit(ok bool, d diag.Diagnostics, result any) bool {
	d = d.Sorted()
	warnings := d.Warnings()
	errors := d.Errors()
	success := ok && len(errors) == 0

	if w.JSON {
		env := Envelope{
			OK:       success,
			Warnings: warnings,
			Errors:   errors,
			Result:   result,
		}
		// Warnings and errors are still human-visible on stderr so a person
		// running with --json is not left guessing.
		w.humanDiagnostics(warnings, errors)

		enc := json.NewEncoder(w.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(env); err != nil {
			fmt.Fprintf(w.Err, "svibe: cannot encode JSON output: %v\n", err)
			return false
		}
		return success
	}

	w.humanDiagnostics(warnings, errors)
	if p, isPrintable := result.(Printable); isPrintable && p != nil {
		p.PrintHuman(w.Out)
	}
	return success
}

func (w *Writer) humanDiagnostics(warnings, errors diag.Diagnostics) {
	for _, x := range warnings {
		w.printDiagnostic("warning", x)
	}
	for _, x := range errors {
		w.printDiagnostic("error", x)
	}
}

func (w *Writer) printDiagnostic(label string, x diag.Diagnostic) {
	if x.Path != "" {
		fmt.Fprintf(w.Err, "%s: %s\n  at %s\n  (%s)\n", label, x.Message, x.Path, x.Code)
		return
	}
	fmt.Fprintf(w.Err, "%s: %s\n  (%s)\n", label, x.Message, x.Code)
}

// Printable is implemented by results that render a human form.
type Printable interface {
	PrintHuman(w io.Writer)
}
