package bd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mwenkdev/structured-vibe/internal/diag"
)

// --- fixtures -------------------------------------------------------------

const (
	versionJSON = `{"branch":"HEAD","build":"abc","version":"1.1.2","schema_version":1}`

	// listJSON deliberately mixes shapes: sv-1 is a root with no parent,
	// labels or metadata keys at all, which is exactly what bd emits for an
	// issue with none of them.
	listJSON = `[
	  {"id":"sv-1","title":"root epic","status":"open","issue_type":"epic","priority":2},
	  {"id":"sv-1.1","title":"child","status":"closed","issue_type":"task","priority":1,
	   "parent":"sv-1","labels":["verification:pass"],
	   "metadata":{"minimum_executor_tier":"B","verification_commit":"abc123"}},
	  {"id":"sv-1.2","title":"blocked child","status":"open","issue_type":"task",
	   "priority":2,"parent":"sv-1"}
	]`

	readyJSON   = `[{"id":"sv-1"}]`
	blockedJSON = `[{"id":"sv-1.2","blocked_by":["sv-1.1"]}]`
)

func responses() map[string]string {
	return map[string]string{
		subVersion: versionJSON,
		subList:    listJSON,
		subReady:   readyJSON,
		subBlocked: blockedJSON,
	}
}

// subcommandOf finds the contract subcommand in a constructed argv.
func subcommandOf(t *testing.T, args []string) string {
	t.Helper()
	for _, a := range args {
		if allowed[a] {
			return a
		}
	}
	t.Fatalf("no known subcommand in argv %v", args)
	return ""
}

// fakeRunner serves canned responses and optionally records every argv.
func fakeRunner(t *testing.T, resp map[string]string, rec *[][]string) Runner {
	t.Helper()
	return func(_ context.Context, args []string) ([]byte, []byte, error) {
		if rec != nil {
			*rec = append(*rec, append([]string(nil), args...))
		}
		sub := subcommandOf(t, args)
		body, ok := resp[sub]
		if !ok {
			t.Fatalf("unexpected subcommand %q", sub)
		}
		return []byte(body), nil, nil
	}
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func findCode(d diag.Diagnostics, code string) (diag.Diagnostic, bool) {
	for _, x := range d {
		if x.Code == code {
			return x, true
		}
	}
	return diag.Diagnostic{}, false
}

// requireCode asserts exactly one error diagnostic with the given code.
func requireCode(t *testing.T, d diag.Diagnostics, code string) diag.Diagnostic {
	t.Helper()
	got, ok := findCode(d, code)
	if !ok {
		t.Fatalf("expected diagnostic %s, got %+v", code, d)
	}
	if got.Severity != diag.SeverityError {
		t.Errorf("diagnostic %s severity = %q, want error", code, got.Severity)
	}
	return got
}

// --- happy path -----------------------------------------------------------

func TestReadReturnsSnapshot(t *testing.T) {
	c := &Client{Root: "/repo", Runner: fakeRunner(t, responses(), nil)}

	snap, d := c.Read()
	if d.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", d)
	}
	if snap.Version != "1.1.2" {
		t.Errorf("version = %q, want 1.1.2", snap.Version)
	}
	if len(snap.Issues) != 3 {
		t.Fatalf("issues = %d, want 3", len(snap.Issues))
	}

	child := snap.Issues["sv-1.1"]
	if child.Parent != "sv-1" || child.Status != "closed" || child.IssueType != "task" {
		t.Errorf("child decoded wrong: %+v", child)
	}
	if got, ok := child.MetadataString("verification_commit"); !ok || got != "abc123" {
		t.Errorf("verification_commit = %q,%v want abc123,true", got, ok)
	}
	if len(child.Labels) != 1 || child.Labels[0] != "verification:pass" {
		t.Errorf("labels = %v", child.Labels)
	}

	if !snap.Ready["sv-1"] || len(snap.Ready) != 1 {
		t.Errorf("ready = %v", snap.Ready)
	}
	blk, ok := snap.Blocked["sv-1.2"]
	if !ok || len(blk.BlockedBy) != 1 || blk.BlockedBy[0] != "sv-1.1" {
		t.Errorf("blocked = %+v", snap.Blocked)
	}
}

// TestOmittedOptionalFieldsAreEmpty guards the R-23 correction: bd omits
// parent, labels and metadata when empty, and their absence must never be
// reported as a contract violation.
func TestOmittedOptionalFieldsAreEmpty(t *testing.T) {
	c := &Client{Root: "/repo", Runner: fakeRunner(t, responses(), nil)}

	snap, d := c.Read()
	if d.HasErrors() {
		t.Fatalf("root issue without parent/labels/metadata produced diagnostics: %+v", d)
	}

	root := snap.Issues["sv-1"]
	if root.Parent != "" {
		t.Errorf("parent = %q, want empty", root.Parent)
	}
	if root.Labels != nil {
		t.Errorf("labels = %v, want nil", root.Labels)
	}
	if root.Metadata != nil {
		t.Errorf("metadata = %v, want nil", root.Metadata)
	}
	if _, ok := root.MetadataString("anything"); ok {
		t.Error("MetadataString on absent metadata reported present")
	}
}

func TestMetadataStringIgnoresNonStringValues(t *testing.T) {
	resp := responses()
	resp[subList] = `[{"id":"sv-1","status":"open","issue_type":"task","metadata":{"n":7}}]`
	c := &Client{Root: "/repo", Runner: fakeRunner(t, resp, nil)}

	snap, d := c.Read()
	if d.HasErrors() {
		t.Fatalf("non-string metadata must not fail the read: %+v", d)
	}
	if _, ok := snap.Issues["sv-1"].MetadataString("n"); ok {
		t.Error("non-string metadata value reported as a string")
	}
}

// --- argv guards ----------------------------------------------------------

// TestArgvContract is the guard test for the read contract. It fails if any
// invocation omits --readonly, carries --parent, passes --limit to
// bd blocked, or names a subcommand outside the four-entry allowlist.
func TestArgvContract(t *testing.T) {
	var got [][]string
	c := &Client{Root: "/repo", Runner: fakeRunner(t, responses(), &got)}

	if _, d := c.Read(); d.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", d)
	}
	if len(got) != 4 {
		t.Fatalf("invocations = %d, want exactly 4", len(got))
	}

	seen := map[string]bool{}
	for _, args := range got {
		sub := subcommandOf(t, args)
		seen[sub] = true

		if !allowed[sub] {
			t.Errorf("subcommand %q outside the allowlist: %v", sub, args)
		}
		if !hasArg(args, "--readonly") {
			t.Errorf("bd %s argv omits --readonly: %v", sub, args)
		}
		if !hasArg(args, "--json") {
			t.Errorf("bd %s argv omits --json: %v", sub, args)
		}
		if hasArg(args, "--parent") {
			t.Errorf("bd %s argv carries --parent, which D12 forbids: %v", sub, args)
		}
		if len(args) < 2 || args[0] != "-C" || args[1] != "/repo" {
			t.Errorf("bd %s argv does not target the project root: %v", sub, args)
		}
	}

	for sub := range allowed {
		if !seen[sub] {
			t.Errorf("contract subcommand %q was never invoked", sub)
		}
	}
}

// TestBlockedCarriesNoLimit pins the flag fact that bd blocked has no --limit
// flag: passing one is an unknown-flag error that fails every invocation.
func TestBlockedCarriesNoLimit(t *testing.T) {
	var got [][]string
	c := &Client{Root: "/repo", Runner: fakeRunner(t, responses(), &got)}
	if _, d := c.Read(); d.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", d)
	}

	for _, args := range got {
		if subcommandOf(t, args) != subBlocked {
			continue
		}
		if hasArg(args, "--limit") {
			t.Errorf("bd blocked argv carries --limit, which bd rejects: %v", args)
		}
		return
	}
	t.Fatal("bd blocked was never invoked")
}

// TestPaginationFlags pins the two limits that must be defeated explicitly.
func TestPaginationFlags(t *testing.T) {
	var got [][]string
	c := &Client{Root: "/repo", Runner: fakeRunner(t, responses(), &got)}
	if _, d := c.Read(); d.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", d)
	}

	for _, args := range got {
		switch subcommandOf(t, args) {
		case subList:
			if !hasArg(args, "--all") {
				t.Errorf("bd list omits --all, so closed issues would be dropped: %v", args)
			}
			if !hasArg(args, "--limit") || !hasArg(args, "0") {
				t.Errorf("bd list omits --limit 0: %v", args)
			}
		case subReady:
			if !hasArg(args, "--limit") || !hasArg(args, "0") {
				t.Errorf("bd ready omits --limit 0, so it would truncate at 100: %v", args)
			}
		}
	}
}

func TestDisallowedSubcommandIsRefused(t *testing.T) {
	c := &Client{Root: "/repo", Runner: func(context.Context, []string) ([]byte, []byte, error) {
		t.Fatal("a disallowed subcommand reached the runner")
		return nil, nil, nil
	}}

	_, d := c.run("1.1.2", "close")
	got := requireCode(t, d, CodeExec)
	if !strings.Contains(got.Message, "close") {
		t.Errorf("message does not name the refused subcommand: %q", got.Message)
	}
}

// --- required-field validation -------------------------------------------

func TestMissingRequiredFieldIsContractViolation(t *testing.T) {
	cases := []struct {
		name, record, field string
	}{
		{"id", `{"status":"open","issue_type":"task"}`, "id"},
		{"status", `{"id":"sv-1","issue_type":"task"}`, "status"},
		{"issue_type", `{"id":"sv-1","status":"open"}`, "issue_type"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := responses()
			resp[subList] = "[" + tc.record + "]"
			c := &Client{Root: "/repo", Runner: fakeRunner(t, resp, nil)}

			snap, d := c.Read()
			if snap != nil {
				t.Error("a contract violation must not return a partial snapshot")
			}
			got := requireCode(t, d, CodeContract)
			if !strings.Contains(got.Message, tc.field) {
				t.Errorf("message does not name the missing field %q: %q", tc.field, got.Message)
			}
			if !strings.Contains(got.Message, "1.1.2") {
				t.Errorf("message does not name the observed bd version: %q", got.Message)
			}
			if !strings.Contains(got.Message, subList) {
				t.Errorf("message does not name the failing subcommand: %q", got.Message)
			}
		})
	}
}

func TestMissingIDInReadyAndBlockedIsContractViolation(t *testing.T) {
	for _, sub := range []string{subReady, subBlocked} {
		t.Run(sub, func(t *testing.T) {
			resp := responses()
			resp[sub] = `[{"blocked_by":["x"]}]`
			c := &Client{Root: "/repo", Runner: fakeRunner(t, resp, nil)}

			snap, d := c.Read()
			if snap != nil {
				t.Error("a contract violation must not return a partial snapshot")
			}
			got := requireCode(t, d, CodeContract)
			if !strings.Contains(got.Message, sub) {
				t.Errorf("message does not name the failing subcommand: %q", got.Message)
			}
		})
	}
}

// --- failure modes --------------------------------------------------------

func TestUndecodableStdout(t *testing.T) {
	for _, sub := range []string{subVersion, subList, subReady, subBlocked} {
		t.Run(sub, func(t *testing.T) {
			resp := responses()
			resp[sub] = "not json at all"
			c := &Client{Root: "/repo", Runner: fakeRunner(t, resp, nil)}

			snap, d := c.Read()
			if snap != nil {
				t.Error("undecodable output must not return a partial snapshot")
			}
			got := requireCode(t, d, CodeDecode)
			if !strings.Contains(got.Message, sub) {
				t.Errorf("message does not name the failing subcommand: %q", got.Message)
			}
		})
	}
}

func TestMissingBinary(t *testing.T) {
	c := &Client{Root: "/repo", Runner: func(context.Context, []string) ([]byte, []byte, error) {
		return nil, nil, &exec.Error{Name: Binary, Err: exec.ErrNotFound}
	}}

	snap, d := c.Read()
	if snap != nil {
		t.Error("a missing binary must not return a partial snapshot")
	}
	got := requireCode(t, d, CodeMissing)
	if !strings.Contains(got.Message, "bd") {
		t.Errorf("message does not name bd: %q", got.Message)
	}
}

func TestTimeoutKillsProcessAndReports(t *testing.T) {
	c := &Client{
		Root:    "/repo",
		Timeout: 20 * time.Millisecond,
		Runner: func(ctx context.Context, _ []string) ([]byte, []byte, error) {
			<-ctx.Done() // a real bd would be killed here by CommandContext
			return nil, nil, ctx.Err()
		},
	}

	snap, d := c.Read()
	if snap != nil {
		t.Error("a timeout must not return a partial snapshot")
	}
	got := requireCode(t, d, CodeTimeout)
	if !strings.Contains(got.Message, subVersion) {
		t.Errorf("message does not name the failing subcommand: %q", got.Message)
	}
}

func TestGenericRunFailureIsReported(t *testing.T) {
	c := &Client{Root: "/repo", Runner: func(context.Context, []string) ([]byte, []byte, error) {
		return nil, []byte("permission denied"), errors.New("fork/exec: boom")
	}}

	_, d := c.Read()
	got := requireCode(t, d, CodeExec)
	if !strings.Contains(got.Message, "permission denied") {
		t.Errorf("message omits the stderr excerpt: %q", got.Message)
	}
}

// TestNonZeroExit drives a real child process so the *exec.ExitError branch
// is exercised rather than simulated. The child is this test binary, so no
// real bd is required.
func TestNonZeroExit(t *testing.T) {
	c := &Client{Root: "/repo", Runner: helperRunner("", "bd: unknown flag: --limit", 2)}

	snap, d := c.Read()
	if snap != nil {
		t.Error("a non-zero exit must not return a partial snapshot")
	}
	got := requireCode(t, d, CodeExit)
	for _, want := range []string{subVersion, "2", "unknown flag"} {
		if !strings.Contains(got.Message, want) {
			t.Errorf("message %q omits %q", got.Message, want)
		}
	}
}

func TestStderrExcerptIsBounded(t *testing.T) {
	long := bytes.Repeat([]byte("x"), maxStderr*2)
	got := stderrExcerpt(long)
	if len(got) > maxStderr+16 {
		t.Errorf("excerpt length = %d, want bounded near %d", len(got), maxStderr)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncated excerpt should be marked: %q", got[max(0, len(got)-8):])
	}
	if stderrExcerpt([]byte("  \n ")) != "" {
		t.Error("blank stderr should produce no excerpt")
	}
}

func TestVersionFallsBackWhenAbsent(t *testing.T) {
	resp := responses()
	resp[subVersion] = `{"branch":"HEAD"}`
	c := &Client{Root: "/repo", Runner: fakeRunner(t, resp, nil)}

	snap, d := c.Read()
	if d.HasErrors() {
		t.Fatalf("a missing version key must not fail the read: %+v", d)
	}
	if snap.Version != unknownVersion {
		t.Errorf("version = %q, want %q", snap.Version, unknownVersion)
	}
}

// --- helper process -------------------------------------------------------

const (
	helperEnv       = "SVIBE_BD_TEST_HELPER"
	helperEnvStdout = "SVIBE_BD_TEST_HELPER_STDOUT"
	helperEnvStderr = "SVIBE_BD_TEST_HELPER_STDERR"
	helperEnvCode   = "SVIBE_BD_TEST_HELPER_CODE"
)

// helperRunner returns a Runner that executes this test binary as a stand-in
// for bd, producing a genuine *exec.ExitError.
func helperRunner(stdout, stderr string, code int) Runner {
	return func(ctx context.Context, _ []string) ([]byte, []byte, error) {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHelperProcess$")
		cmd.Env = append(os.Environ(),
			helperEnv+"=1",
			helperEnvStdout+"="+stdout,
			helperEnvStderr+"="+stderr,
			helperEnvCode+"="+strconv.Itoa(code),
		)
		var o, e bytes.Buffer
		cmd.Stdout = &o
		cmd.Stderr = &e
		err := cmd.Run()
		return o.Bytes(), e.Bytes(), err
	}
}

// TestHelperProcess is not a real test. It is the child process spawned by
// helperRunner, and exits immediately so the test framework prints nothing.
func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "1" {
		t.Skip("helper process; not run directly")
	}
	fmt.Fprint(os.Stdout, os.Getenv(helperEnvStdout))
	fmt.Fprint(os.Stderr, os.Getenv(helperEnvStderr))
	code, _ := strconv.Atoi(os.Getenv(helperEnvCode))
	os.Exit(code)
}
