// Package diag carries validation and resolution diagnostics.
//
// Structured Vibe distinguishes errors from warnings (architecture 10):
// errors cause command failure; warnings identify advisory or unsupported
// conditions that do not make the environment structurally unusable.
// Warnings do not become errors merely to make CI strict.
package diag

import (
	"fmt"
	"sort"
)

func sprintf(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// Severity classifies a diagnostic.
type Severity string

const (
	// SeverityError makes the command fail.
	SeverityError Severity = "error"
	// SeverityWarning is advisory and does not fail the command.
	SeverityWarning Severity = "warning"
)

// Diagnostic is a single validation or resolution finding.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	// Path is the file or directory the finding relates to, when known.
	Path string `json:"path,omitempty"`
}

// Diagnostics is an ordered collection of findings.
type Diagnostics []Diagnostic

// Errorf appends an error diagnostic.
func (d *Diagnostics) Errorf(code, path, format string, args ...any) {
	*d = append(*d, Diagnostic{
		Severity: SeverityError,
		Code:     code,
		Message:  sprintf(format, args...),
		Path:     path,
	})
}

// Warnf appends a warning diagnostic.
func (d *Diagnostics) Warnf(code, path, format string, args ...any) {
	*d = append(*d, Diagnostic{
		Severity: SeverityWarning,
		Code:     code,
		Message:  sprintf(format, args...),
		Path:     path,
	})
}

// Extend appends all diagnostics from other.
func (d *Diagnostics) Extend(other Diagnostics) {
	*d = append(*d, other...)
}

// HasErrors reports whether any diagnostic is an error.
func (d Diagnostics) HasErrors() bool {
	for _, x := range d {
		if x.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Errors returns only the error diagnostics.
func (d Diagnostics) Errors() Diagnostics { return d.filter(SeverityError) }

// Warnings returns only the warning diagnostics.
func (d Diagnostics) Warnings() Diagnostics { return d.filter(SeverityWarning) }

func (d Diagnostics) filter(s Severity) Diagnostics {
	out := Diagnostics{}
	for _, x := range d {
		if x.Severity == s {
			out = append(out, x)
		}
	}
	return out
}

// Sorted returns the diagnostics ordered by path then code, so output is
// stable across filesystem iteration order.
func (d Diagnostics) Sorted() Diagnostics {
	out := make(Diagnostics, len(d))
	copy(out, d)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Code < out[j].Code
	})
	return out
}
