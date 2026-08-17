package cliout

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/mwenkdev/structured-vibe/internal/diag"
)

type result struct {
	Value string `json:"value"`
}

func (r *result) PrintHuman(w io.Writer) { fmt.Fprintf(w, "value: %s\n", r.Value) }

// TestJSONStdoutIsPure is the machine-integration contract: stdout must parse
// as JSON with nothing else mixed in.
func TestJSONStdoutIsPure(t *testing.T) {
	var out, errw bytes.Buffer
	w := New(&out, &errw, true)

	var d diag.Diagnostics
	d.Warnf("test.warn", "/some/path", "a warning")

	w.Emit(true, d, &result{Value: "x"})

	var env Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not pure JSON: %v\n%s", err, out.String())
	}
	if !env.OK {
		t.Error("ok should be true")
	}
	if len(env.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(env.Warnings))
	}
	if env.Warnings[0].Code != "test.warn" {
		t.Errorf("warning code = %q", env.Warnings[0].Code)
	}
	// Human-readable warnings still go to stderr so a person is not left guessing.
	if !strings.Contains(errw.String(), "a warning") {
		t.Errorf("stderr missing human warning: %q", errw.String())
	}
	if strings.Contains(out.String(), "value: x") {
		t.Error("human output leaked onto stdout")
	}
}

// TestOKIsNotAClaimOfNoAdvisoryState: warnings do not make ok false.
func TestWarningsDoNotFailCommand(t *testing.T) {
	var out, errw bytes.Buffer
	w := New(&out, &errw, true)

	var d diag.Diagnostics
	d.Warnf("advisory", "", "something advisory")

	if ok := w.Emit(true, d, nil); !ok {
		t.Error("warnings must not fail the command")
	}
}

// TestErrorsForceFailure: a caller cannot report success alongside errors.
func TestErrorsForceFailure(t *testing.T) {
	var out, errw bytes.Buffer
	w := New(&out, &errw, true)

	var d diag.Diagnostics
	d.Errorf("bad", "", "something broke")

	if ok := w.Emit(true, d, nil); ok {
		t.Error("errors must force failure even when the caller claims success")
	}

	var env Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.OK {
		t.Error("envelope ok should be false")
	}
	if len(env.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(env.Errors))
	}
}

func TestHumanModeWritesResultToStdout(t *testing.T) {
	var out, errw bytes.Buffer
	w := New(&out, &errw, false)

	w.Emit(true, diag.Diagnostics{}, &result{Value: "hello"})

	if !strings.Contains(out.String(), "value: hello") {
		t.Errorf("stdout = %q", out.String())
	}
	if errw.Len() != 0 {
		t.Errorf("stderr should be empty, got %q", errw.String())
	}
}

func TestHumanModeDiagnosticsGoToStderr(t *testing.T) {
	var out, errw bytes.Buffer
	w := New(&out, &errw, false)

	var d diag.Diagnostics
	d.Errorf("some.code", "/a/path", "it broke")

	w.Emit(false, d, nil)

	if !strings.Contains(errw.String(), "it broke") {
		t.Errorf("stderr = %q", errw.String())
	}
	if !strings.Contains(errw.String(), "/a/path") {
		t.Errorf("stderr missing path: %q", errw.String())
	}
	if out.Len() != 0 {
		t.Errorf("stdout should be empty, got %q", out.String())
	}
}

// TestEnvelopeAlwaysHasAllKeys keeps the JSON shape stable for automation.
func TestEnvelopeAlwaysHasAllKeys(t *testing.T) {
	var out, errw bytes.Buffer
	New(&out, &errw, true).Emit(true, diag.Diagnostics{}, nil)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"ok", "warnings", "errors", "result"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("envelope missing key %q", k)
		}
	}
	// Empty collections serialize as [] rather than null.
	if string(raw["warnings"]) != "[]" {
		t.Errorf("warnings = %s, want []", raw["warnings"])
	}
	if string(raw["errors"]) != "[]" {
		t.Errorf("errors = %s, want []", raw["errors"])
	}
}

func TestDiagnosticsAreSortedForStableOutput(t *testing.T) {
	var out, errw bytes.Buffer
	w := New(&out, &errw, true)

	var d diag.Diagnostics
	d.Warnf("z.code", "/z", "z")
	d.Warnf("a.code", "/a", "a")

	w.Emit(true, d, nil)

	var env Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Warnings[0].Path != "/a" {
		t.Errorf("diagnostics not sorted: %+v", env.Warnings)
	}
}
