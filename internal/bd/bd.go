// Package bd reads the Beads work graph through the bd command line.
//
// Structured Vibe does not own work state; Beads does (spec 5.5). This
// package is the only place svibe talks to bd, and it is strictly read-only:
// it executes a fixed four-call contract, validates the responses
// structurally, and returns an in-memory snapshot. It never invokes a
// mutating subcommand, and it never opens the Dolt database directly, because
// bd's CLI JSON is a public interface and its storage schema is not.
//
// The contract's flags are load-bearing, each verified against bd 1.1.2:
//
//   - bd blocked has no --limit flag. Passing one is an unknown-flag error
//     that would fail every invocation.
//   - bd list needs --all to include closed issues, and --limit 0.
//   - bd ready defaults to --limit 100, so --limit 0 is mandatory.
//   - --parent is never passed. Its descent semantics are inconsistent
//     between subcommands: bd list --parent is immediate-children-only,
//     bd ready --parent is recursive, and bd blocked --parent is
//     immediate-children-only, despite identical flag documentation.
//     Callers compute membership in memory from the parent field instead.
//
// See docs/specs/richer-execution-status.md, "The bd read contract" (D4, D12).
package bd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/mwenkdev/structured-vibe/internal/diag"
)

// Binary is the bd executable name, resolved through PATH.
const Binary = "bd"

// DefaultTimeout bounds a single bd invocation.
const DefaultTimeout = 30 * time.Second

// maxStderr bounds the stderr excerpt carried in a diagnostic, so a runaway
// bd cannot flood the envelope.
const maxStderr = 2000

// unknownVersion is reported when bd's version cannot be determined. The
// version is diagnostic metadata, so an unparseable one must not fail a read.
const unknownVersion = "unknown"

// Diagnostic codes. They follow the repository convention of
// <package>.<subject>.
const (
	// CodeMissing reports that the bd executable is not on PATH.
	CodeMissing = "bd.missing"
	// CodeExec reports a bd invocation that could not be run at all.
	CodeExec = "bd.exec"
	// CodeTimeout reports a bd invocation killed at its deadline.
	CodeTimeout = "bd.timeout"
	// CodeExit reports a bd invocation that exited non-zero.
	CodeExit = "bd.exit"
	// CodeDecode reports bd output that is not decodable JSON.
	CodeDecode = "bd.decode"
	// CodeContract reports decodable bd output missing a required field.
	CodeContract = "bd.contract"
)

// The four subcommands of the read contract.
const (
	subVersion = "version"
	subList    = "list"
	subReady   = "ready"
	subBlocked = "blocked"
)

// allowed is the closed set of subcommands this package may invoke. It is a
// second line of defence behind --readonly: a mutating subcommand cannot be
// reached even by a construction defect.
var allowed = map[string]bool{
	subVersion: true,
	subList:    true,
	subReady:   true,
	subBlocked: true,
}

// Issue is one bd issue, limited to the fields the read contract consumes.
//
// Only ID, Status and IssueType are required. bd omits parent, labels and
// metadata from its JSON when they are empty, so their absence is normal and
// is read as empty rather than as a contract violation.
type Issue struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Status    string         `json:"status"`
	IssueType string         `json:"issue_type"`
	Priority  int            `json:"priority"`
	Parent    string         `json:"parent"`
	Labels    []string       `json:"labels"`
	Metadata  map[string]any `json:"metadata"`
}

// MetadataString returns a string-valued metadata entry.
//
// bd metadata values are arbitrary JSON, so a non-string value reports absent
// rather than failing the whole read.
func (i Issue) MetadataString(key string) (string, bool) {
	v, ok := i.Metadata[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// Blocked is one entry of the global blocked set.
//
// BlockedBy is authoritative for per-member blocker identity. Blockage
// propagates through parent-child edges, so a child of a blocked parent is
// itself reported blocked with its parent named here despite having no
// blocks edge of its own.
type Blocked struct {
	ID        string   `json:"id"`
	BlockedBy []string `json:"blocked_by"`
}

// Snapshot is one read of the work graph.
//
// It is not atomic. The four invocations can straddle a concurrent bd
// mutation, which is accepted for a status projection; determinism is
// conditional on quiescent bd state.
type Snapshot struct {
	// Version is the observed bd version, or "unknown".
	Version string
	// Issues is every issue in the repository, keyed by id.
	Issues map[string]Issue
	// Ready is the set of ids bd reports as available.
	Ready map[string]bool
	// Blocked is the set of blocked entries, keyed by id.
	Blocked map[string]Blocked
}

// Runner executes one bd invocation and returns its captured streams.
//
// It exists so tests can supply a fake bd and assert on the constructed argv.
// A nil Runner uses the real executable.
type Runner func(ctx context.Context, args []string) (stdout, stderr []byte, err error)

// Client reads the work graph through bd.
type Client struct {
	// Root is the project root passed to bd via -C. Callers resolve it;
	// this package does not rediscover it.
	Root string
	// Timeout bounds each invocation. Zero means DefaultTimeout.
	Timeout time.Duration
	// Runner overrides process execution. Nil means the real bd binary.
	Runner Runner
}

// Read executes the four-call contract and returns a snapshot.
//
// Any failure returns a nil snapshot and at least one error diagnostic naming
// the failing subcommand and the observed bd version. Partial output is never
// returned.
func (c *Client) Read() (*Snapshot, diag.Diagnostics) {
	var d diag.Diagnostics

	version, vd := c.readVersion()
	d.Extend(vd)
	if d.HasErrors() {
		return nil, d
	}

	issues, isd := c.readIssues(version)
	d.Extend(isd)
	if d.HasErrors() {
		return nil, d
	}

	ready, rd := c.readReady(version)
	d.Extend(rd)
	if d.HasErrors() {
		return nil, d
	}

	blocked, bld := c.readBlocked(version)
	d.Extend(bld)
	if d.HasErrors() {
		return nil, d
	}

	return &Snapshot{Version: version, Issues: issues, Ready: ready, Blocked: blocked}, d
}

// argv builds one invocation.
//
// Every invocation carries --readonly, so a defect in argument construction
// cannot mutate the graph, and --json so the response is machine-readable.
// -C targets the project root and leaves database discovery to bd. No
// invocation carries --parent.
func (c *Client) argv(sub string, extra ...string) []string {
	args := make([]string, 0, len(extra)+6)
	args = append(args, "-C", c.Root, sub)
	args = append(args, extra...)
	return append(args, "--readonly", "--json")
}

// run executes one subcommand and returns its stdout.
//
// version is the already-observed bd version, or empty while determining it.
func (c *Client) run(version, sub string, extra ...string) ([]byte, diag.Diagnostics) {
	var d diag.Diagnostics

	if !allowed[sub] {
		d.Errorf(CodeExec, "", "bd %s is outside the read contract", sub)
		return nil, d
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	runner := c.Runner
	if runner == nil {
		runner = execRunner
	}

	stdout, stderr, err := runner(ctx, c.argv(sub, extra...))

	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		d.Errorf(CodeTimeout, "", "bd %s timed out after %s and was killed%s",
			sub, timeout, versionSuffix(version))
		return nil, d

	case errors.Is(err, exec.ErrNotFound):
		d.Errorf(CodeMissing, "",
			"bd executable not found on PATH: this command requires Beads")
		return nil, d

	case err != nil:
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			d.Errorf(CodeExit, "", "bd %s exited %d%s%s",
				sub, exitErr.ExitCode(), versionSuffix(version), stderrExcerpt(stderr))
			return nil, d
		}
		d.Errorf(CodeExec, "", "bd %s could not be run: %v%s%s",
			sub, err, versionSuffix(version), stderrExcerpt(stderr))
		return nil, d
	}

	return stdout, d
}

// readVersion resolves the observed bd version.
func (c *Client) readVersion() (string, diag.Diagnostics) {
	out, d := c.run("", subVersion)
	if d.HasErrors() {
		return "", d
	}

	var v struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		d.Errorf(CodeDecode, "", "bd version returned undecodable JSON: %v", err)
		return "", d
	}
	if v.Version == "" {
		return unknownVersion, d
	}
	return v.Version, d
}

// readIssues fetches every issue. --all includes closed issues and --limit 0
// defeats the documented default row limit.
func (c *Client) readIssues(version string) (map[string]Issue, diag.Diagnostics) {
	out, d := c.run(version, subList, "--all", "--limit", "0")
	if d.HasErrors() {
		return nil, d
	}

	var raw []Issue
	if err := json.Unmarshal(out, &raw); err != nil {
		d.Errorf(CodeDecode, "", "bd list returned undecodable JSON: %v%s",
			err, versionSuffix(version))
		return nil, d
	}

	issues := make(map[string]Issue, len(raw))
	for i, is := range raw {
		if field := missingRequired(is); field != "" {
			d.Errorf(CodeContract, "", "bd list record %d is missing required field %q%s",
				i, field, versionSuffix(version))
			return nil, d
		}
		issues[is.ID] = is
	}
	return issues, d
}

// readReady fetches the global available set. bd ready defaults to a 100-row
// limit, so --limit 0 is mandatory.
func (c *Client) readReady(version string) (map[string]bool, diag.Diagnostics) {
	out, d := c.run(version, subReady, "--limit", "0")
	if d.HasErrors() {
		return nil, d
	}

	var raw []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		d.Errorf(CodeDecode, "", "bd ready returned undecodable JSON: %v%s",
			err, versionSuffix(version))
		return nil, d
	}

	set := make(map[string]bool, len(raw))
	for i, r := range raw {
		if r.ID == "" {
			d.Errorf(CodeContract, "", "bd ready record %d is missing required field %q%s",
				i, "id", versionSuffix(version))
			return nil, d
		}
		set[r.ID] = true
	}
	return set, d
}

// readBlocked fetches the global blocked set.
//
// bd blocked accepts no --limit flag; passing one is an unknown-flag error
// that would fail every invocation. Do not add one.
func (c *Client) readBlocked(version string) (map[string]Blocked, diag.Diagnostics) {
	out, d := c.run(version, subBlocked)
	if d.HasErrors() {
		return nil, d
	}

	var raw []Blocked
	if err := json.Unmarshal(out, &raw); err != nil {
		d.Errorf(CodeDecode, "", "bd blocked returned undecodable JSON: %v%s",
			err, versionSuffix(version))
		return nil, d
	}

	set := make(map[string]Blocked, len(raw))
	for i, b := range raw {
		if b.ID == "" {
			d.Errorf(CodeContract, "", "bd blocked record %d is missing required field %q%s",
				i, "id", versionSuffix(version))
			return nil, d
		}
		set[b.ID] = b
	}
	return set, d
}

// missingRequired names the first absent required field.
//
// Only id, status and issue_type are required. parent, labels and metadata
// are omitted by bd when empty, so their absence is never a violation.
func missingRequired(i Issue) string {
	switch {
	case i.ID == "":
		return "id"
	case i.Status == "":
		return "status"
	case i.IssueType == "":
		return "issue_type"
	}
	return ""
}

// versionSuffix renders the observed bd version for a diagnostic.
func versionSuffix(version string) string {
	if version == "" {
		version = unknownVersion
	}
	return fmt.Sprintf(" (bd version %s)", version)
}

// stderrExcerpt renders a bounded excerpt of bd's stderr.
func stderrExcerpt(stderr []byte) string {
	trimmed := bytes.TrimSpace(stderr)
	if len(trimmed) == 0 {
		return ""
	}
	if len(trimmed) > maxStderr {
		trimmed = append(trimmed[:maxStderr:maxStderr], []byte("...")...)
	}
	return fmt.Sprintf(": %s", trimmed)
}

// execRunner runs the real bd binary. CommandContext kills the process when
// the deadline expires.
func execRunner(ctx context.Context, args []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, Binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
