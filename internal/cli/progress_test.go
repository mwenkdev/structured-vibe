package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mwenkdev/structured-vibe/internal/bd"
	"github.com/mwenkdev/structured-vibe/internal/managed"
	"github.com/mwenkdev/structured-vibe/internal/paths"
)

// --- fixture work graph ---------------------------------------------------

// fixtureIssue mirrors one bd list record. parent, labels and metadata are
// omitempty so the fixture reproduces bd's real behaviour of omitting those
// keys entirely when empty.
type fixtureIssue struct {
	ID        string         `json:"id"`
	Title     string         `json:"title,omitempty"`
	Status    string         `json:"status"`
	IssueType string         `json:"issue_type"`
	Priority  int            `json:"priority,omitempty"`
	Parent    string         `json:"parent,omitempty"`
	Labels    []string       `json:"labels,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// fixture is a whole fake work graph: the three data calls of the read
// contract, served without a real bd binary.
type fixture struct {
	issues  []fixtureIssue
	ready   []string
	blocked map[string][]string
}

// reversed returns the same graph with every response order inverted, so
// determinism can be tested against permuted bd response order.
func (fx fixture) reversed() fixture {
	out := fixture{blocked: fx.blocked}
	for i := len(fx.issues) - 1; i >= 0; i-- {
		out.issues = append(out.issues, fx.issues[i])
	}
	for i := len(fx.ready) - 1; i >= 0; i-- {
		out.ready = append(out.ready, fx.ready[i])
	}
	return out
}

// subcommandOf finds the contract subcommand in a constructed argv.
func subcommandOf(args []string) string {
	for _, a := range args {
		switch a {
		case "version", "list", "ready", "blocked":
			return a
		}
	}
	return ""
}

// runner serves the fixture as a fake bd, optionally recording every argv.
func (fx fixture) runner(t *testing.T, rec *[][]string) bd.Runner {
	t.Helper()
	return func(_ context.Context, args []string) ([]byte, []byte, error) {
		if rec != nil {
			*rec = append(*rec, append([]string(nil), args...))
		}

		marshal := func(v any) ([]byte, []byte, error) {
			raw, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("fixture marshal: %v", err)
			}
			return raw, nil, nil
		}

		switch sub := subcommandOf(args); sub {
		case "version":
			return []byte(`{"version":"1.1.2"}`), nil, nil

		case "list":
			issues := fx.issues
			if issues == nil {
				issues = []fixtureIssue{}
			}
			return marshal(issues)

		case "ready":
			rows := []map[string]string{}
			for _, id := range fx.ready {
				rows = append(rows, map[string]string{"id": id})
			}
			return marshal(rows)

		case "blocked":
			ids := make([]string, 0, len(fx.blocked))
			for id := range fx.blocked {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			rows := []map[string]any{}
			for _, id := range ids {
				rows = append(rows, map[string]any{"id": id, "blocked_by": fx.blocked[id]})
			}
			return marshal(rows)

		default:
			t.Fatalf("unexpected bd argv %v", args)
			return nil, nil, nil
		}
	}
}

// --- harness --------------------------------------------------------------

// runProgress executes the progress command against a fixture work graph.
func runProgressFixture(t *testing.T, fx fixture, args ...string) runOutput {
	t.Helper()
	return runProgressRecording(t, fx, nil, args...)
}

func runProgressRecording(t *testing.T, fx fixture, rec *[][]string, args ...string) runOutput {
	t.Helper()
	t.Setenv(paths.ConfigHomeEnv, t.TempDir())

	var out, errw bytes.Buffer
	e := &Env{
		Stdout:   &out,
		Stderr:   &errw,
		Cwd:      newRepo(t),
		Manifest: managed.Manifest{},
		BDRunner: fx.runner(t, rec),
	}
	err := Run(e, args)
	return runOutput{stdout: out.String(), stderr: errw.String(), err: err}
}

// decodeResult parses the envelope and returns the progress result.
func decodeResult(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var env struct {
		OK     bool           `json:"ok"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout)
	}
	if !env.OK {
		t.Fatalf("envelope reports failure: %s", stdout)
	}
	if env.Result == nil {
		t.Fatalf("envelope carries no result: %s", stdout)
	}
	return env.Result
}

// memberBuckets maps member id to bucket.
func memberBuckets(t *testing.T, res map[string]any) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, raw := range res["members"].([]any) {
		m := raw.(map[string]any)
		out[m["id"].(string)] = m["bucket"].(string)
	}
	return out
}

func memberIDs(t *testing.T, res map[string]any) []string {
	t.Helper()
	out := []string{}
	for _, raw := range res["members"].([]any) {
		out = append(out, raw.(map[string]any)["id"].(string))
	}
	return out
}

// task builds an ordinary work bead.
func task(id, parent, status string) fixtureIssue {
	return fixtureIssue{ID: id, Title: "bead " + id, Status: status, IssueType: "task", Parent: parent}
}

// --- membership (D8) ------------------------------------------------------

func TestProgressMembershipIsRecursive(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		task("sv-1.1", "sv-1", "closed"),
		task("sv-1.1.1", "sv-1.1", "closed"),
		task("sv-1.1.1.1", "sv-1.1.1", "open"),
	}}

	got := runProgressFixture(t, fx, "progress", "sv-1", "--json")
	if got.err != nil {
		t.Fatalf("progress failed: %v\n%s", got.err, got.stderr)
	}

	res := decodeResult(t, got.stdout)
	want := []string{"sv-1.1", "sv-1.1.1", "sv-1.1.1.1"}
	if ids := memberIDs(t, res); !equalStrings(ids, want) {
		t.Errorf("members = %v, want %v (depth-3 descent)", ids, want)
	}
}

// TestProgressBlocksEdgesNeverConferMembership guards D8: blocks edges are
// readiness information, never membership.
func TestProgressBlocksEdgesNeverConferMembership(t *testing.T) {
	fx := fixture{
		issues: []fixtureIssue{
			{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
			task("sv-1.1", "sv-1", "open"),
			// sv-zzz is linked only by a blocks edge, never by parent.
			{ID: "sv-zzz", Title: "outsider", Status: "open", IssueType: "task"},
		},
		blocked: map[string][]string{"sv-1.1": {"sv-zzz"}},
	}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))
	if _, found := memberBuckets(t, res)["sv-zzz"]; found {
		t.Error("a bead linked only by a blocks edge became a member")
	}
}

// TestProgressExcludesSetStateEvent is the load-bearing exclusion: bd
// set-state creates an event bead as a parent-child CHILD of the work bead,
// so without the exclusion every verified bead would pollute its own subtree.
func TestProgressExcludesSetStateEvent(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		task("sv-1.1", "sv-1", "closed"),
		{ID: "sv-ev", Title: "state change", Status: "open", IssueType: "event", Parent: "sv-1.1"},
	}}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))
	if ids := memberIDs(t, res); !equalStrings(ids, []string{"sv-1.1"}) {
		t.Errorf("members = %v, want only sv-1.1: the set-state event must add no member", ids)
	}
}

func TestProgressExcludesEveryInfrastructureType(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		task("sv-1.work", "sv-1", "open"),
	}}
	for _, kind := range []string{"event", "agent", "role", "message", "gate"} {
		fx.issues = append(fx.issues, fixtureIssue{
			ID: "sv-1." + kind, Title: kind, Status: "open", IssueType: kind, Parent: "sv-1",
		})
	}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))
	if ids := memberIDs(t, res); !equalStrings(ids, []string{"sv-1.work"}) {
		t.Errorf("members = %v, want only sv-1.work: infrastructure types are excluded entirely", ids)
	}
}

// TestProgressCycleTerminates drives a cycle that is actually reachable from
// the root. Because a bead has one parent, the only cycle a descent can enter
// is one the root itself sits in, so that is the case worth testing: without
// the visited set this walk re-queues the root forever and the test hangs.
func TestProgressCycleTerminates(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic", Parent: "sv-1.1"},
		task("sv-1.1", "sv-1", "open"),
		task("sv-1.2", "sv-1", "open"),
	}}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))
	if ids := memberIDs(t, res); !equalStrings(ids, []string{"sv-1.1", "sv-1.2"}) {
		t.Errorf("members = %v, want sv-1.1 and sv-1.2 with the cycle broken", ids)
	}
	if _, found := memberBuckets(t, res)["sv-1"]; found {
		t.Error("the cycle walked back into the root and listed it as its own member")
	}
}

func TestProgressRootIsNeverItsOwnMember(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		task("sv-1.1", "sv-1", "open"),
		// A self-parenting root must not list itself.
		task("sv-1.2", "sv-1", "open"),
	}}
	fx.issues[0].Parent = "sv-1"

	res := decodeResult(t, mustRun(t, fx, "sv-1"))
	if _, found := memberBuckets(t, res)["sv-1"]; found {
		t.Error("the root listed itself as a member of its own subtree")
	}
	if res["root"].(map[string]any)["id"] != "sv-1" {
		t.Errorf("root = %v", res["root"])
	}
}

func TestProgressAcceptsHierarchicalRootID(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-08e", Title: "epic", Status: "open", IssueType: "epic"},
		task("sv-08e.2", "sv-08e", "open"),
		task("sv-08e.2.1", "sv-08e.2", "open"),
	}}

	res := decodeResult(t, mustRun(t, fx, "sv-08e.2"))
	if ids := memberIDs(t, res); !equalStrings(ids, []string{"sv-08e.2.1"}) {
		t.Errorf("members = %v; a hierarchical id must be accepted as root", ids)
	}
}

// --- containers -----------------------------------------------------------

func TestProgressContainersAreListedButNotCounted(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		{ID: "sv-1.e", Title: "nested epic", Status: "open", IssueType: "epic", Parent: "sv-1"},
		{ID: "sv-1.m", Title: "milestone", Status: "open", IssueType: "milestone", Parent: "sv-1"},
		task("sv-1.e.1", "sv-1.e", "closed"),
		task("sv-1.m.1", "sv-1.m", "open"),
	}}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))
	buckets := memberBuckets(t, res)
	for _, id := range []string{"sv-1.e", "sv-1.m"} {
		if buckets[id] != bucketContainer {
			t.Errorf("%s bucket = %q, want container", id, buckets[id])
		}
	}
	// Descendants of containers are counted.
	for _, id := range []string{"sv-1.e.1", "sv-1.m.1"} {
		if _, found := buckets[id]; !found {
			t.Errorf("descendant %s of a container was not counted", id)
		}
	}

	counts := res["counts"].(map[string]any)
	if counts["container"].(float64) != 2 {
		t.Errorf("container count = %v, want 2", counts["container"])
	}
	if counts["total_countable"].(float64) != 2 {
		t.Errorf("total_countable = %v, want 2: containers are in no denominator",
			counts["total_countable"])
	}
}

// TestProgressClosingNestedEpicDoesNotChangeRatio is the point of the
// container rule: a container must not add progress on top of the work its
// children already supplied.
func TestProgressClosingNestedEpicDoesNotChangeRatio(t *testing.T) {
	open := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		{ID: "sv-1.e", Title: "nested epic", Status: "open", IssueType: "epic", Parent: "sv-1"},
		task("sv-1.e.1", "sv-1.e", "closed"),
		task("sv-1.e.2", "sv-1.e", "open"),
	}}
	closed := fixture{issues: append([]fixtureIssue{}, open.issues...)}
	closed.issues[1].Status = "closed"

	before := decodeResult(t, mustRun(t, open, "sv-1"))["completion_ratio"]
	after := decodeResult(t, mustRun(t, closed, "sv-1"))["completion_ratio"]

	if before != after {
		t.Errorf("completion_ratio changed from %v to %v when a nested epic closed", before, after)
	}
}

// --- buckets (D9) ---------------------------------------------------------

func TestProgressBucketRules(t *testing.T) {
	cases := []struct {
		name       string
		issue      fixtureIssue
		ready      []string
		blocked    map[string][]string
		wantBucket string
	}{
		{
			name:       "rule 1 container",
			issue:      fixtureIssue{ID: "m", Status: "open", IssueType: "epic", Parent: "sv-1"},
			wantBucket: bucketContainer,
		},
		{
			name:       "rule 2 completed",
			issue:      task("m", "sv-1", "closed"),
			wantBucket: bucketCompleted,
		},
		{
			name:       "rule 3 pinned is excluded",
			issue:      task("m", "sv-1", "pinned"),
			wantBucket: bucketExcluded,
		},
		{
			name:       "rule 4 deferred",
			issue:      task("m", "sv-1", "deferred"),
			wantBucket: bucketDeferred,
		},
		{
			name:       "rule 5 in_progress",
			issue:      task("m", "sv-1", "in_progress"),
			wantBucket: bucketActive,
		},
		{
			name:       "rule 5 hooked",
			issue:      task("m", "sv-1", "hooked"),
			wantBucket: bucketActive,
		},
		{
			name:       "rule 5 precedes 6: in_progress in the ready set is still active",
			issue:      task("m", "sv-1", "in_progress"),
			ready:      []string{"m"},
			wantBucket: bucketActive,
		},
		{
			name:       "rule 5 precedes 7: in_progress with blockers is active",
			issue:      task("m", "sv-1", "in_progress"),
			blocked:    map[string][]string{"m": {"sv-x"}},
			wantBucket: bucketActive,
		},
		{
			name:       "rule 6 available comes from the global ready set",
			issue:      task("m", "sv-1", "open"),
			ready:      []string{"m"},
			wantBucket: bucketAvailable,
		},
		{
			name:       "rule 7 blocked via the global blocked set",
			issue:      task("m", "sv-1", "open"),
			blocked:    map[string][]string{"m": {"sv-x"}},
			wantBucket: bucketBlocked,
		},
		{
			name:       "rule 7 stored-status disjunct: blocked with no edges",
			issue:      task("m", "sv-1", "blocked"),
			wantBucket: bucketBlocked,
		},
		{
			name:       "rule 8 unclassified",
			issue:      task("m", "sv-1", "wibble"),
			wantBucket: bucketUnclassified,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := fixture{
				issues: []fixtureIssue{
					{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
					tc.issue,
				},
				ready:   tc.ready,
				blocked: tc.blocked,
			}

			res := decodeResult(t, mustRun(t, fx, "sv-1"))
			if got := memberBuckets(t, res)["m"]; got != tc.wantBucket {
				t.Errorf("bucket = %q, want %q", got, tc.wantBucket)
			}
		})
	}
}

func TestProgressUnclassifiedWarns(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		task("sv-1.1", "sv-1", "wibble"),
	}}

	got := runProgressFixture(t, fx, "progress", "sv-1", "--json")
	if got.err != nil {
		t.Fatalf("an unclassified member must not fail the command: %v", got.err)
	}

	var env struct {
		Warnings []struct{ Code, Message string } `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Warnings) != 1 || env.Warnings[0].Code != codeUnclassified {
		t.Fatalf("warnings = %+v, want one %s", env.Warnings, codeUnclassified)
	}
	for _, want := range []string{"sv-1.1", "wibble"} {
		if !strings.Contains(env.Warnings[0].Message, want) {
			t.Errorf("warning %q does not name %q", env.Warnings[0].Message, want)
		}
	}
}

func TestProgressCountsSumToMembers(t *testing.T) {
	fx := fixture{
		issues: []fixtureIssue{
			{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
			{ID: "sv-1.c", Status: "open", IssueType: "epic", Parent: "sv-1"},
			task("sv-1.1", "sv-1", "closed"),
			task("sv-1.2", "sv-1", "deferred"),
			task("sv-1.3", "sv-1", "in_progress"),
			task("sv-1.4", "sv-1", "open"),
			task("sv-1.5", "sv-1", "open"),
			task("sv-1.6", "sv-1", "wibble"),
			task("sv-1.7", "sv-1", "pinned"),
		},
		ready:   []string{"sv-1.4"},
		blocked: map[string][]string{"sv-1.5": {"sv-1.1"}},
	}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))
	c := res["counts"].(map[string]any)
	n := func(k string) int { return int(c[k].(float64)) }

	countable := n("completed") + n("deferred") + n("active") + n("blocked") +
		n("available") + n("unclassified")
	if countable != n("total_countable") {
		t.Errorf("bucket sum %d != total_countable %d", countable, n("total_countable"))
	}
	if total := n("total_countable") + n("container") + n("excluded"); total != len(res["members"].([]any)) {
		t.Errorf("total_countable+container+excluded = %d, members = %d",
			total, len(res["members"].([]any)))
	}
	if n("total_countable") != 6 {
		t.Errorf("total_countable = %d, want 6 (container and pinned excluded)", n("total_countable"))
	}
}

func TestProgressCompletionRatio(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		task("sv-1.1", "sv-1", "closed"),
		task("sv-1.2", "sv-1", "open"),
		task("sv-1.3", "sv-1", "open"),
	}}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))
	if got := res["completion_ratio"].(float64); got != 0.3333 {
		t.Errorf("completion_ratio = %v, want 0.3333 (rounded to 4 places)", got)
	}
}

// TestProgressZeroCountableSucceeds covers a subtree of containers and pinned
// beads only: a valid result with a null ratio, never 0 and never 1.
func TestProgressZeroCountableSucceeds(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		{ID: "sv-1.c", Status: "open", IssueType: "milestone", Parent: "sv-1"},
		task("sv-1.p", "sv-1", "pinned"),
	}}

	got := runProgressFixture(t, fx, "progress", "sv-1", "--json")
	if got.err != nil {
		t.Fatalf("a zero-countable subtree must succeed: %v\n%s", got.err, got.stderr)
	}

	res := decodeResult(t, got.stdout)
	if res["completion_ratio"] != nil {
		t.Errorf("completion_ratio = %v, want null", res["completion_ratio"])
	}
	if _, present := res["completion_ratio"]; !present {
		t.Error("completion_ratio key is absent; it must be present and null")
	}
}

// --- propagated blockage --------------------------------------------------

// TestProgressPropagatedBlockage covers R-19: blockage propagates through
// parent-child edges, so a child of a blocked parent is itself blocked with
// blocked_by naming the parent despite having no blocks edge of its own.
func TestProgressPropagatedBlockage(t *testing.T) {
	fx := fixture{
		issues: []fixtureIssue{
			{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
			task("sv-1.1", "sv-1", "open"),
			task("sv-1.1.1", "sv-1.1", "open"),
		},
		blocked: map[string][]string{
			"sv-1.1":   {"sv-zzz"},
			"sv-1.1.1": {"sv-1.1"},
		},
	}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))
	if got := memberBuckets(t, res)["sv-1.1.1"]; got != bucketBlocked {
		t.Errorf("child bucket = %q, want blocked", got)
	}

	for _, raw := range res["members"].([]any) {
		m := raw.(map[string]any)
		if m["id"] != "sv-1.1.1" {
			continue
		}
		by := m["blocked_by"].([]any)
		if len(by) != 1 {
			t.Fatalf("blocked_by = %v, want the parent named", by)
		}
		entry := by[0].(map[string]any)
		if entry["id"] != "sv-1.1" {
			t.Errorf("blocked_by[0].id = %v, want sv-1.1", entry["id"])
		}
		if entry["in_subtree"] != true {
			t.Errorf("blocked_by[0].in_subtree = %v, want true", entry["in_subtree"])
		}
	}
}

func TestProgressOutOfSubtreeBlockerResolves(t *testing.T) {
	fx := fixture{
		issues: []fixtureIssue{
			{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
			task("sv-1.1", "sv-1", "open"),
			{ID: "sv-zzz", Title: "outsider", Status: "closed", IssueType: "task"},
		},
		blocked: map[string][]string{"sv-1.1": {"sv-zzz"}},
	}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))
	entry := res["members"].([]any)[0].(map[string]any)["blocked_by"].([]any)[0].(map[string]any)
	if entry["in_subtree"] != false {
		t.Errorf("in_subtree = %v, want false", entry["in_subtree"])
	}
	if entry["status"] != "closed" {
		t.Errorf("status = %v, want closed: resolved from the full issue map", entry["status"])
	}
}

// --- stop (D11) -----------------------------------------------------------

// decodeStop returns the stop object, failing if it is absent or null.
func decodeStop(t *testing.T, res map[string]any) map[string]any {
	t.Helper()
	stop, ok := res["stop"].(map[string]any)
	if !ok {
		t.Fatalf("stop = %v, want a non-null object", res["stop"])
	}
	return stop
}

// stopReasons flattens stop.reasons into ordered category/member pairs.
func stopReasons(t *testing.T, stop map[string]any) []struct {
	category string
	members  []string
} {
	t.Helper()
	var out []struct {
		category string
		members  []string
	}
	for _, raw := range stop["reasons"].([]any) {
		r := raw.(map[string]any)
		entry := struct {
			category string
			members  []string
		}{category: r["category"].(string)}
		for _, m := range r["members"].([]any) {
			entry.members = append(entry.members, m.(string))
		}
		out = append(out, entry)
	}
	return out
}

// TestProgressStopCategories drives each D11 category on a dedicated fixture.
// Every case is stopped: no member is available or active.
func TestProgressStopCategories(t *testing.T) {
	cases := []struct {
		name        string
		issues      []fixtureIssue
		blocked     map[string][]string
		wantReasons []struct {
			category string
			members  []string
		}
	}{
		{
			name: "dependency_blocked: a blocked member with a named blocker",
			issues: []fixtureIssue{
				{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
				task("sv-1.1", "sv-1", "open"),
			},
			blocked: map[string][]string{"sv-1.1": {"sv-zzz"}},
			wantReasons: []struct {
				category string
				members  []string
			}{{stopDependencyBlocked, []string{"sv-1.1"}}},
		},
		{
			// Stored-blocked with no dependency edge has no blocker to name,
			// so reporting it as dependency_blocked would promise detail that
			// does not exist.
			name: "stored_blocked: stored status blocked with no named blockers",
			issues: []fixtureIssue{
				{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
				task("sv-1.1", "sv-1", "blocked"),
			},
			wantReasons: []struct {
				category string
				members  []string
			}{{stopStoredBlocked, []string{"sv-1.1"}}},
		},
		{
			name: "deferred: deliberately deferred work",
			issues: []fixtureIssue{
				{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
				task("sv-1.1", "sv-1", "deferred"),
			},
			wantReasons: []struct {
				category string
				members  []string
			}{{stopDeferred, []string{"sv-1.1"}}},
		},
		{
			name: "unclassified: a status no bucket rule claims",
			issues: []fixtureIssue{
				{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
				task("sv-1.1", "sv-1", "wibble"),
			},
			wantReasons: []struct {
				category string
				members  []string
			}{{stopUnclassified, []string{"sv-1.1"}}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := fixture{issues: tc.issues, blocked: tc.blocked}
			stop := decodeStop(t, decodeResult(t, mustRun(t, fx, "sv-1")))

			if stop["stopped"] != true {
				t.Fatalf("stopped = %v, want true", stop["stopped"])
			}
			if stop["complete"] != false {
				t.Errorf("complete = %v, want false", stop["complete"])
			}

			got := stopReasons(t, stop)
			if len(got) != len(tc.wantReasons) {
				t.Fatalf("reasons = %+v, want %+v", got, tc.wantReasons)
			}
			for i, want := range tc.wantReasons {
				if got[i].category != want.category {
					t.Errorf("reasons[%d].category = %q, want %q", i, got[i].category, want.category)
				}
				if !equalStrings(got[i].members, want.members) {
					t.Errorf("reasons[%d].members = %v, want %v", i, got[i].members, want.members)
				}
			}
		})
	}
}

// TestProgressStopMixedCategories covers a subtree stopped for several
// distinct reasons at once: every applicable category is listed, in the fixed
// D11 order, with the correct members under each.
func TestProgressStopMixedCategories(t *testing.T) {
	fx := fixture{
		issues: []fixtureIssue{
			{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
			task("sv-1.1", "sv-1", "closed"),
			task("sv-1.2", "sv-1", "open"),     // dependency_blocked
			task("sv-1.3", "sv-1", "blocked"),  // stored_blocked
			task("sv-1.4", "sv-1", "deferred"), // deferred
			task("sv-1.5", "sv-1", "wibble"),   // unclassified
			task("sv-1.6", "sv-1", "open"),     // dependency_blocked
		},
		blocked: map[string][]string{
			"sv-1.2": {"sv-zzz"},
			"sv-1.6": {"sv-1.2"},
		},
	}

	stop := decodeStop(t, decodeResult(t, mustRun(t, fx, "sv-1")))
	if stop["stopped"] != true {
		t.Fatalf("stopped = %v, want true", stop["stopped"])
	}

	want := []struct {
		category string
		members  []string
	}{
		{stopDependencyBlocked, []string{"sv-1.2", "sv-1.6"}},
		{stopStoredBlocked, []string{"sv-1.3"}},
		{stopDeferred, []string{"sv-1.4"}},
		{stopUnclassified, []string{"sv-1.5"}},
	}

	got := stopReasons(t, stop)
	if len(got) != len(want) {
		t.Fatalf("reasons = %+v, want %d categories in D11 order", got, len(want))
	}
	for i := range want {
		if got[i].category != want[i].category {
			t.Errorf("reasons[%d].category = %q, want %q (fixed D11 order)",
				i, got[i].category, want[i].category)
		}
		if !equalStrings(got[i].members, want[i].members) {
			t.Errorf("reasons[%d].members = %v, want %v (sorted by id ascending)",
				i, got[i].members, want[i].members)
		}
	}

	// A populated reasons list is the part of the schema most exposed to
	// iteration order, so byte-stability is asserted here rather than only on
	// the non-stopped fixture the general determinism test uses.
	if first, permuted := mustRun(t, fx, "sv-1"), mustRun(t, fx.reversed(), "sv-1"); first != permuted {
		t.Errorf("permuted bd response order changed stdout:\n%s\n---\n%s", first, permuted)
	}
}

// TestProgressStoppedImpliesReasons is the guard in one direction: a stopped
// result can never carry an empty reasons list. It is structural, not
// defensive - a stopped subtree has no available or active members, so every
// outstanding member falls into some category.
func TestProgressStoppedImpliesReasons(t *testing.T) {
	fixtures := []fixture{
		{issues: []fixtureIssue{
			{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
			task("sv-1.1", "sv-1", "deferred"),
		}},
		{issues: []fixtureIssue{
			{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
			task("sv-1.1", "sv-1", "blocked"),
			task("sv-1.2", "sv-1", "wibble"),
		}},
		{
			issues: []fixtureIssue{
				{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
				task("sv-1.1", "sv-1", "closed"),
				task("sv-1.2", "sv-1", "open"),
			},
			blocked: map[string][]string{"sv-1.2": {"sv-1.1"}},
		},
	}

	for i, fx := range fixtures {
		stop := decodeStop(t, decodeResult(t, mustRun(t, fx, "sv-1")))
		if stop["stopped"] != true {
			t.Fatalf("fixture %d: stopped = %v, want true", i, stop["stopped"])
		}
		if reasons := stop["reasons"].([]any); len(reasons) == 0 {
			t.Errorf("fixture %d: stopped with an empty reasons list", i)
		}
	}
}

// TestProgressNotStoppedHasEmptyReasons is the converse guard (R-20):
// reasons is populated only when stopped is true.
func TestProgressNotStoppedHasEmptyReasons(t *testing.T) {
	cases := []struct {
		name         string
		fx           fixture
		wantComplete bool
	}{
		{
			// Available work exists, so the subtree has not stopped even
			// though other members are blocked and deferred.
			name: "available work exists",
			fx: fixture{
				issues: []fixtureIssue{
					{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
					task("sv-1.1", "sv-1", "open"),
					task("sv-1.2", "sv-1", "deferred"),
					task("sv-1.3", "sv-1", "blocked"),
				},
				ready: []string{"sv-1.1"},
			},
		},
		{
			name: "active work exists",
			fx: fixture{issues: []fixtureIssue{
				{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
				task("sv-1.1", "sv-1", "in_progress"),
				task("sv-1.2", "sv-1", "deferred"),
			}},
		},
		{
			name: "all countable members completed",
			fx: fixture{issues: []fixtureIssue{
				{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
				task("sv-1.1", "sv-1", "closed"),
				task("sv-1.2", "sv-1", "closed"),
			}},
			wantComplete: true,
		},
		{
			// Containers and pinned beads only: nothing to finish, so the
			// subtree is neither stopped nor complete.
			name: "zero countable members",
			fx: fixture{issues: []fixtureIssue{
				{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
				{ID: "sv-1.c", Status: "open", IssueType: "milestone", Parent: "sv-1"},
				task("sv-1.p", "sv-1", "pinned"),
			}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stop := decodeStop(t, decodeResult(t, mustRun(t, tc.fx, "sv-1")))

			if stop["stopped"] != false {
				t.Errorf("stopped = %v, want false", stop["stopped"])
			}
			if reasons, ok := stop["reasons"].([]any); !ok || len(reasons) != 0 {
				t.Errorf("reasons = %v, want [] when not stopped", stop["reasons"])
			}
			if stop["complete"] != tc.wantComplete {
				t.Errorf("complete = %v, want %v", stop["complete"], tc.wantComplete)
			}
		})
	}
}

// TestProgressStopReadsNoVerificationData guards M3's independence from M2:
// the stop path must classify identically whether or not verification labels
// and metadata are present, so the two milestones can land in either order.
func TestProgressStopReadsNoVerificationData(t *testing.T) {
	plain := fixture{
		issues: []fixtureIssue{
			{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
			task("sv-1.1", "sv-1", "closed"),
			task("sv-1.2", "sv-1", "open"),
			task("sv-1.3", "sv-1", "deferred"),
		},
		blocked: map[string][]string{"sv-1.2": {"sv-1.1"}},
	}

	// The same graph, with verification state recorded on every member.
	verified := fixture{issues: []fixtureIssue{}, blocked: plain.blocked}
	for _, is := range plain.issues {
		is.Labels = []string{"verification:pass"}
		is.Metadata = map[string]any{"verification_commit": "a1b2c3d4"}
		verified.issues = append(verified.issues, is)
	}

	before := decodeStop(t, decodeResult(t, mustRun(t, plain, "sv-1")))
	after := decodeStop(t, decodeResult(t, mustRun(t, verified, "sv-1")))

	if !reflect.DeepEqual(before, after) {
		t.Errorf("stop changed when verification data was present:\n%+v\n%+v", before, after)
	}
}

// TestProgressStopOutOfSubtreeBlocker pins that a blocker outside the subtree
// still produces dependency_blocked, with its status resolved from the
// full-issue map rather than an extra bd call.
func TestProgressStopOutOfSubtreeBlocker(t *testing.T) {
	fx := fixture{
		issues: []fixtureIssue{
			{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
			task("sv-1.1", "sv-1", "open"),
			{ID: "sv-zzz", Title: "outsider", Status: "open", IssueType: "task"},
		},
		blocked: map[string][]string{"sv-1.1": {"sv-zzz"}},
	}

	var argv [][]string
	got := runProgressRecording(t, fx, &argv, "progress", "sv-1", "--json")
	if got.err != nil {
		t.Fatalf("progress failed: %v\n%s", got.err, got.stderr)
	}
	if len(argv) != 4 {
		t.Errorf("bd invocations = %d, want exactly 4: stop introduces no new call", len(argv))
	}

	res := decodeResult(t, got.stdout)
	reasons := stopReasons(t, decodeStop(t, res))
	if len(reasons) != 1 || reasons[0].category != stopDependencyBlocked {
		t.Fatalf("reasons = %+v, want one dependency_blocked", reasons)
	}

	entry := res["members"].([]any)[0].(map[string]any)["blocked_by"].([]any)[0].(map[string]any)
	if entry["id"] != "sv-zzz" || entry["in_subtree"] != false || entry["status"] != "open" {
		t.Errorf("blocked_by[0] = %v, want sv-zzz resolved with in_subtree false", entry)
	}
}

func TestProgressStopHumanOutput(t *testing.T) {
	fx := fixture{
		issues: []fixtureIssue{
			{ID: "sv-1", Title: "root epic", Status: "open", IssueType: "epic"},
			task("sv-1.1", "sv-1", "closed"),
			task("sv-1.2", "sv-1", "open"),
			task("sv-1.3", "sv-1", "deferred"),
		},
		blocked: map[string][]string{"sv-1.2": {"sv-1.1"}},
	}

	got := runProgressFixture(t, fx, "progress", "sv-1")
	if got.err != nil {
		t.Fatalf("progress failed: %v\n%s", got.err, got.stderr)
	}
	for _, want := range []string{"stopped", stopDependencyBlocked, "sv-1.2", stopDeferred, "sv-1.3"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("human output omits %q:\n%s", want, got.stdout)
		}
	}
}

// --- verification (D5, D10) -----------------------------------------------

// recorded stamps a bead with the two writes of the D5 convention, as bd
// materialises them: the outcome as a label, the commit as metadata. An empty
// commit reproduces the interrupted case, where only the outcome exists.
func recorded(is fixtureIssue, value, commit string) fixtureIssue {
	is.Labels = []string{verificationLabelPrefix + value}
	if commit != "" {
		is.Metadata = map[string]any{verificationCommitKey: commit}
	}
	return is
}

func memberByID(t *testing.T, res map[string]any, id string) map[string]any {
	t.Helper()
	for _, raw := range res["members"].([]any) {
		if m := raw.(map[string]any); m["id"] == id {
			return m
		}
	}
	t.Fatalf("member %s is absent from %v", id, memberIDs(t, res))
	return nil
}

// anomalyPairs flattens anomalies into "member/code" strings in emitted order.
func anomalyPairs(t *testing.T, res map[string]any) []string {
	t.Helper()
	out := []string{}
	for _, raw := range res["anomalies"].([]any) {
		a := raw.(map[string]any)
		if a["message"] == "" || a["message"] == nil {
			t.Errorf("anomaly %v carries no message", a)
		}
		out = append(out, a["member"].(string)+"/"+a["code"].(string))
	}
	return out
}

func verificationCountsOf(t *testing.T, res map[string]any) map[string]int {
	t.Helper()
	out := map[string]int{}
	for k, v := range res["verification_counts"].(map[string]any) {
		out[k] = int(v.(float64))
	}
	return out
}

// TestProgressMemberVerificationEveryBucket pins D10's "every work member
// regardless of bucket": verification is member state, not a property of
// completed work, so an open member carries it too.
func TestProgressMemberVerificationEveryBucket(t *testing.T) {
	cases := []struct {
		name   string
		issue  fixtureIssue
		want   string
		commit any
	}{
		{
			name:   "closed with a recorded pass",
			issue:  recorded(task("m", "sv-1", "closed"), "pass", "a1b2c3"),
			want:   verificationPass,
			commit: "a1b2c3",
		},
		{
			name:   "closed with a recorded fail",
			issue:  recorded(task("m", "sv-1", "closed"), "fail", "a1b2c3"),
			want:   verificationFail,
			commit: "a1b2c3",
		},
		{
			name:   "non-closed with a recorded pass is stale",
			issue:  recorded(task("m", "sv-1", "open"), "pass", "a1b2c3"),
			want:   verificationStalePass,
			commit: "a1b2c3",
		},
		{
			// Ordinary in-flight state after a REJECT: reported, not faulted.
			name:   "non-closed with a recorded fail",
			issue:  recorded(task("m", "sv-1", "open"), "fail", "a1b2c3"),
			want:   verificationFail,
			commit: "a1b2c3",
		},
		{
			name:   "nothing recorded",
			issue:  task("m", "sv-1", "closed"),
			want:   verificationUnverified,
			commit: nil,
		},
		{
			// The interruption the commit-first order is designed to survive.
			name:   "commit metadata with no outcome reads as unverified",
			issue:  fixtureIssue{ID: "m", Status: "closed", IssueType: "task", Parent: "sv-1", Metadata: map[string]any{verificationCommitKey: "a1b2c3"}},
			want:   verificationUnverified,
			commit: "a1b2c3",
		},
		{
			name:   "an in-flight member carries verification too",
			issue:  recorded(task("m", "sv-1", "in_progress"), "fail", "a1b2c3"),
			want:   verificationFail,
			commit: "a1b2c3",
		},
		{
			name:   "a container carries verification too",
			issue:  recorded(fixtureIssue{ID: "m", Status: "closed", IssueType: "epic", Parent: "sv-1"}, "pass", "a1b2c3"),
			want:   verificationPass,
			commit: "a1b2c3",
		},
		{
			name:   "a pinned member carries verification too",
			issue:  recorded(task("m", "sv-1", "pinned"), "pass", "a1b2c3"),
			want:   verificationStalePass,
			commit: "a1b2c3",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := fixture{issues: []fixtureIssue{
				{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
				tc.issue,
			}}

			m := memberByID(t, decodeResult(t, mustRun(t, fx, "sv-1")), "m")
			if m["verification"] != tc.want {
				t.Errorf("verification = %v, want %q", m["verification"], tc.want)
			}
			if m["verification_commit"] != tc.commit {
				t.Errorf("verification_commit = %v, want %v (emitted verbatim)",
					m["verification_commit"], tc.commit)
			}
			if m["verification_raw"] != nil {
				t.Errorf("verification_raw = %v, want null for a recognised value", m["verification_raw"])
			}
		})
	}
}

// TestProgressVerificationCountsCoverCompletedOnly pins the denominator: an
// open member is not expected to be verified, so counting it as missing
// coverage would manufacture a fault out of ordinary in-flight state.
func TestProgressVerificationCountsCoverCompletedOnly(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		recorded(task("sv-1.1", "sv-1", "closed"), "pass", "c1"),
		recorded(task("sv-1.2", "sv-1", "closed"), "pass", "c2"),
		recorded(task("sv-1.3", "sv-1", "closed"), "fail", "c3"),
		task("sv-1.4", "sv-1", "closed"),
		// Non-closed members are in no coverage key, whatever they recorded.
		recorded(task("sv-1.5", "sv-1", "open"), "pass", "c5"),
		recorded(task("sv-1.6", "sv-1", "open"), "fail", "c6"),
		task("sv-1.7", "sv-1", "open"),
		// Container and pinned members are not completed members either.
		recorded(fixtureIssue{ID: "sv-1.c", Status: "closed", IssueType: "epic", Parent: "sv-1"}, "pass", "c8"),
		recorded(task("sv-1.p", "sv-1", "pinned"), "pass", "c9"),
	}}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))
	got := verificationCountsOf(t, res)
	want := map[string]int{"completed_total": 4, "pass": 2, "fail": 1, "unverified": 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("verification_counts = %v, want %v", got, want)
	}
	if got["pass"]+got["fail"]+got["unverified"] != got["completed_total"] {
		t.Errorf("coverage keys do not sum to completed_total: %v", got)
	}

	// stale-pass is member state and an anomaly, never a coverage key.
	if memberByID(t, res, "sv-1.5")["verification"] != verificationStalePass {
		t.Error("a non-closed recorded pass was not reported as stale-pass")
	}
}

// TestProgressAnomaliesFireOnTheirOwnFixtureAndNowhereElse is the whole
// anomaly contract in one table: each fixture yields exactly the anomalies
// listed and no others.
func TestProgressAnomaliesFireOnTheirOwnFixtureAndNowhereElse(t *testing.T) {
	cases := []struct {
		name  string
		issue fixtureIssue
		want  []string
	}{
		{
			name:  "closed with a recorded fail",
			issue: recorded(task("m", "sv-1", "closed"), "fail", "a1b2c3"),
			want:  []string{"m/" + anomalyClosedFailed},
		},
		{
			name:  "recorded pass on a non-closed bead",
			issue: recorded(task("m", "sv-1", "open"), "pass", "a1b2c3"),
			want:  []string{"m/" + anomalyStalePass},
		},
		{
			name:  "outcome recorded with no commit",
			issue: recorded(task("m", "sv-1", "closed"), "pass", ""),
			want:  []string{"m/" + anomalyNoCommit},
		},
		{
			name:  "a clean verified closure is not an anomaly",
			issue: recorded(task("m", "sv-1", "closed"), "pass", "a1b2c3"),
			want:  []string{},
		},
		{
			name:  "a recorded fail on an open bead is in-flight state, not an anomaly",
			issue: recorded(task("m", "sv-1", "open"), "fail", "a1b2c3"),
			want:  []string{},
		},
		{
			name:  "an unverified bead is not an anomaly",
			issue: task("m", "sv-1", "closed"),
			want:  []string{},
		},
		{
			name:  "an unrecognised value is not an outcome missing a commit",
			issue: recorded(task("m", "sv-1", "closed"), "wibble", ""),
			want:  []string{},
		},
		{
			// Two findings on one member, sorted by code.
			name:  "a stale pass with no commit fires both",
			issue: recorded(task("m", "sv-1", "open"), "pass", ""),
			want:  []string{"m/" + anomalyStalePass, "m/" + anomalyNoCommit},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := fixture{issues: []fixtureIssue{
				{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
				tc.issue,
			}}

			got := anomalyPairs(t, decodeResult(t, mustRun(t, fx, "sv-1")))
			if !equalStrings(got, tc.want) {
				t.Errorf("anomalies = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestProgressClosedFailIsAnomalyNeverBlocker resolves review finding R-02: a
// closed dependency is a satisfied dependency however it was verified, and
// readiness is Beads' judgment alone.
func TestProgressClosedFailIsAnomalyNeverBlocker(t *testing.T) {
	fx := fixture{
		issues: []fixtureIssue{
			{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
			recorded(task("sv-1.1", "sv-1", "closed"), "fail", "a1b2c3"),
			task("sv-1.2", "sv-1", "open"),
		},
		ready: []string{"sv-1.2"},
	}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))
	if got := anomalyPairs(t, res); !equalStrings(got, []string{"sv-1.1/" + anomalyClosedFailed}) {
		t.Errorf("anomalies = %v, want the closed failure reported", got)
	}

	for _, raw := range res["members"].([]any) {
		m := raw.(map[string]any)
		for _, b := range m["blocked_by"].([]any) {
			if b.(map[string]any)["id"] == "sv-1.1" {
				t.Errorf("%v is blocked by a closed member because its verification failed", m["id"])
			}
		}
	}
	if got := memberBuckets(t, res)["sv-1.2"]; got != bucketAvailable {
		t.Errorf("sv-1.2 bucket = %q, want available: a failed verification must not withdraw readiness", got)
	}
	if stop := decodeStop(t, res); stop["stopped"] != false {
		t.Errorf("stopped = %v, want false: verification never affects the stop summary", stop["stopped"])
	}
}

// TestProgressUnrecognisedVerificationValue covers the escape hatch: an
// unknown recorded value is neither trusted nor silently dropped.
func TestProgressUnrecognisedVerificationValue(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		// "blocked" is specifically not a verification value: ESCALATE is
		// lifecycle state, not a verification result.
		recorded(task("sv-1.1", "sv-1", "closed"), "blocked", "a1b2c3"),
	}}

	got := runProgressFixture(t, fx, "progress", "sv-1", "--json")
	if got.err != nil {
		t.Fatalf("an unrecognised value must not fail the command: %v\n%s", got.err, got.stderr)
	}

	res := decodeResult(t, got.stdout)
	m := memberByID(t, res, "sv-1.1")
	if m["verification"] != verificationUnverified {
		t.Errorf("verification = %v, want unverified", m["verification"])
	}
	if m["verification_raw"] != "blocked" {
		t.Errorf("verification_raw = %v, want the raw string preserved", m["verification_raw"])
	}
	if c := verificationCountsOf(t, res); c["unverified"] != 1 {
		t.Errorf("verification_counts = %v, want the member counted unverified", c)
	}

	var env struct {
		Warnings []struct{ Code, Message string } `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Warnings) != 1 || env.Warnings[0].Code != codeUnknownVerification {
		t.Fatalf("warnings = %+v, want one %s", env.Warnings, codeUnknownVerification)
	}
	for _, want := range []string{"sv-1.1", "blocked"} {
		if !strings.Contains(env.Warnings[0].Message, want) {
			t.Errorf("warning %q does not name %q", env.Warnings[0].Message, want)
		}
	}
}

// TestProgressCompleteSubtreeCanBeUnverified is the point of D10: structural
// progress makes no verification claim, so both facts must be visible at once
// and in both output forms.
func TestProgressCompleteSubtreeCanBeUnverified(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root epic", Status: "open", IssueType: "epic"},
		recorded(task("sv-1.1", "sv-1", "closed"), "pass", "a1b2c3"),
		task("sv-1.2", "sv-1", "closed"),
	}}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))
	if res["completion_ratio"] != 1.0 {
		t.Errorf("completion_ratio = %v, want 1.0: every countable member is closed", res["completion_ratio"])
	}
	if stop := decodeStop(t, res); stop["complete"] != true {
		t.Errorf("complete = %v, want true", stop["complete"])
	}
	want := map[string]int{"completed_total": 2, "pass": 1, "fail": 0, "unverified": 1}
	if got := verificationCountsOf(t, res); !reflect.DeepEqual(got, want) {
		t.Errorf("verification_counts = %v, want %v: coverage is incomplete", got, want)
	}

	human := runProgressFixture(t, fx, "progress", "sv-1")
	if human.err != nil {
		t.Fatalf("progress failed: %v\n%s", human.err, human.stderr)
	}
	for _, w := range []string{"100.0%", "verification", verificationUnverified, "incomplete"} {
		if !strings.Contains(human.stdout, w) {
			t.Errorf("human output omits %q, so complete-but-unverified is not visible:\n%s",
				w, human.stdout)
		}
	}
}

func TestProgressVerificationHumanOutput(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root epic", Status: "open", IssueType: "epic"},
		recorded(task("sv-1.1", "sv-1", "closed"), "pass", "a1b2c3"),
		recorded(task("sv-1.2", "sv-1", "open"), "pass", "d4e5f6"),
	}}

	got := runProgressFixture(t, fx, "progress", "sv-1")
	if got.err != nil {
		t.Fatalf("progress failed: %v\n%s", got.err, got.stderr)
	}
	for _, want := range []string{
		verificationPass, verificationStalePass, anomalyStalePass, "a1b2c3", "d4e5f6",
	} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("human output omits %q:\n%s", want, got.stdout)
		}
	}
}

// TestProgressVerificationReadsNoEventBead is the guard: verification state
// comes from labels[] and metadata only. bd set-state creates an event bead
// as a parent-child CHILD of the work bead, and its own labels must be
// invisible here.
func TestProgressVerificationReadsNoEventBead(t *testing.T) {
	plain := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		recorded(task("sv-1.1", "sv-1", "closed"), "pass", "a1b2c3"),
	}}

	// The same graph after a set-state call: an event bead carrying a
	// contradictory outcome hangs beneath the verified member.
	withEvent := fixture{issues: append([]fixtureIssue{}, plain.issues...)}
	withEvent.issues = append(withEvent.issues, recorded(fixtureIssue{
		ID: "sv-ev", Title: "state change", Status: "closed",
		IssueType: "event", Parent: "sv-1.1",
	}, "fail", ""))

	var argv [][]string
	got := runProgressRecording(t, withEvent, &argv, "progress", "sv-1", "--json")
	if got.err != nil {
		t.Fatalf("progress failed: %v\n%s", got.err, got.stderr)
	}
	if len(argv) != 4 {
		t.Errorf("bd invocations = %d, want exactly 4: verification introduces no new call", len(argv))
	}

	res := decodeResult(t, got.stdout)
	if ids := memberIDs(t, res); !equalStrings(ids, []string{"sv-1.1"}) {
		t.Fatalf("members = %v, want only sv-1.1: the event bead is not a member", ids)
	}
	if m := memberByID(t, res, "sv-1.1"); m["verification"] != verificationPass {
		t.Errorf("verification = %v, want pass from the member's own label", m["verification"])
	}
	if a := anomalyPairs(t, res); len(a) != 0 {
		t.Errorf("anomalies = %v, want none: the event bead's own state was read", a)
	}

	// The projection is byte-identical with and without the event bead.
	if before, after := mustRun(t, plain, "sv-1"), got.stdout; before != after {
		t.Errorf("the set-state event bead changed the projection:\n%s\n---\n%s", before, after)
	}
}

// --- output schema --------------------------------------------------------

func TestProgressSchemaKeysAlwaysPresent(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		task("sv-1.1", "sv-1", "closed"),
	}}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))

	for _, k := range []string{
		"root", "bd_version", "counts", "completion_ratio", "members",
		"verification_counts", "anomalies", "stop",
	} {
		if _, present := res[k]; !present {
			t.Errorf("result key %q is absent; every key must always be present", k)
		}
	}

	// Populated from M2 on, and a non-null object in every successful result
	// for the same reason stop is: a consumer never distinguishes "no
	// coverage" from "coverage not computed".
	vc, ok := res["verification_counts"].(map[string]any)
	if !ok {
		t.Fatalf("verification_counts = %v, want a non-null object", res["verification_counts"])
	}
	for _, k := range []string{"completed_total", "pass", "fail", "unverified"} {
		if _, present := vc[k]; !present {
			t.Errorf("verification_counts key %q is absent", k)
		}
	}
	if _, present := vc["stale-pass"]; present {
		t.Error("verification_counts carries a stale-pass key; it cannot occur over completed members")
	}

	// stop is populated from M3 on and is a non-null object in every
	// successful result, so a consumer never distinguishes "not stopped"
	// from "not computed".
	stop, ok := res["stop"].(map[string]any)
	if !ok {
		t.Fatalf("stop = %v, want a non-null object", res["stop"])
	}
	for _, k := range []string{"stopped", "complete", "reasons"} {
		if _, present := stop[k]; !present {
			t.Errorf("stop key %q is absent", k)
		}
	}
	if a, ok := res["anomalies"].([]any); !ok || len(a) != 0 {
		t.Errorf("anomalies = %v, want []", res["anomalies"])
	}

	m := res["members"].([]any)[0].(map[string]any)
	for _, k := range []string{
		"id", "title", "status", "issue_type", "priority", "parent", "bucket",
		"verification", "verification_raw", "verification_commit", "blocked_by",
	} {
		if _, present := m[k]; !present {
			t.Errorf("member key %q is absent", k)
		}
	}
	// A member with nothing recorded reports the closed value unverified,
	// with the two nullable fields null.
	if m["verification"] != verificationUnverified {
		t.Errorf("verification = %v, want %q", m["verification"], verificationUnverified)
	}
	for _, k := range []string{"verification_raw", "verification_commit"} {
		if m[k] != nil {
			t.Errorf("member %s = %v, want null when nothing is recorded", k, m[k])
		}
	}
	if b, ok := m["blocked_by"].([]any); !ok || len(b) != 0 {
		t.Errorf("blocked_by = %v, want [] for an unblocked member", m["blocked_by"])
	}
	if res["bd_version"] != "1.1.2" {
		t.Errorf("bd_version = %v, want 1.1.2", res["bd_version"])
	}
}

func TestProgressCanonicalOrdering(t *testing.T) {
	fx := fixture{
		issues: []fixtureIssue{
			{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
			task("sv-1.3", "sv-1", "open"),
			task("sv-1.1", "sv-1", "open"),
			task("sv-1.2", "sv-1", "open"),
		},
		blocked: map[string][]string{"sv-1.1": {"sv-1.3", "sv-1.2"}},
	}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))

	ids := memberIDs(t, res)
	if !sort.StringsAreSorted(ids) {
		t.Errorf("members = %v, want sorted by id ascending", ids)
	}

	var by []string
	for _, raw := range res["members"].([]any)[0].(map[string]any)["blocked_by"].([]any) {
		by = append(by, raw.(map[string]any)["id"].(string))
	}
	if !sort.StringsAreSorted(by) {
		t.Errorf("blocked_by = %v, want sorted by id ascending", by)
	}
}

// TestProgressDeterminismUnderPermutedOrder is meaningful precisely because
// ordering is owned by svibe serialization rather than inherited from bd.
func TestProgressDeterminismUnderPermutedOrder(t *testing.T) {
	fx := fixture{
		issues: []fixtureIssue{
			{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
			task("sv-1.1", "sv-1", "closed"),
			task("sv-1.2", "sv-1", "open"),
			task("sv-1.3", "sv-1", "in_progress"),
			task("sv-1.4", "sv-1", "deferred"),
		},
		ready:   []string{"sv-1.2"},
		blocked: map[string][]string{"sv-1.2": {"sv-1.1"}},
	}

	first := mustRun(t, fx, "sv-1")
	again := mustRun(t, fx, "sv-1")
	permuted := mustRun(t, fx.reversed(), "sv-1")

	if first != again {
		t.Error("two runs against unchanged state produced different stdout")
	}
	if first != permuted {
		t.Errorf("permuted bd response order changed stdout:\n%s\n---\n%s", first, permuted)
	}
}

func TestProgressEmitsNoTimestamps(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		task("sv-1.1", "sv-1", "closed"),
	}}

	out := mustRun(t, fx, "sv-1")
	for _, forbidden := range []string{"created_at", "updated_at", "closed_at", "_at\""} {
		if strings.Contains(out, forbidden) {
			t.Errorf("output carries a bd timestamp %q, which breaks byte-stability", forbidden)
		}
	}
}

func TestProgressHumanOutput(t *testing.T) {
	fx := fixture{
		issues: []fixtureIssue{
			{ID: "sv-1", Title: "root epic", Status: "open", IssueType: "epic"},
			task("sv-1.1", "sv-1", "closed"),
			task("sv-1.2", "sv-1", "open"),
		},
		blocked: map[string][]string{"sv-1.2": {"sv-1.1"}},
	}

	got := runProgressFixture(t, fx, "progress", "sv-1")
	if got.err != nil {
		t.Fatalf("progress failed: %v\n%s", got.err, got.stderr)
	}
	if strings.HasPrefix(strings.TrimSpace(got.stdout), "{") {
		t.Error("human output emitted JSON")
	}
	for _, want := range []string{"sv-1", "root epic", "1.1.2", "50.0%", "sv-1.2", "blocked by sv-1.1"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("human output omits %q:\n%s", want, got.stdout)
		}
	}
}

// --- errors ---------------------------------------------------------------

func TestProgressUnknownRootFails(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
	}}

	got := runProgressFixture(t, fx, "progress", "sv-nope", "--json")
	if got.err == nil {
		t.Fatal("an unknown root must fail the command")
	}
	if !strings.Contains(got.stderr, "sv-nope") {
		t.Errorf("stderr does not name the unknown id: %q", got.stderr)
	}
	if !strings.Contains(got.stderr, codeUnknownRoot) {
		t.Errorf("stderr does not carry %s: %q", codeUnknownRoot, got.stderr)
	}
}

func TestProgressRequiresExactlyOneID(t *testing.T) {
	fx := fixture{}
	for _, args := range [][]string{{"progress"}, {"progress", "a", "b"}} {
		got := runProgressFixture(t, fx, args...)
		if got.err == nil {
			t.Errorf("%v: expected a usage failure", args)
		}
	}
}

// TestProgressFailsWhenBDAbsent also pins that svibe's own commands stay
// usable in the same environment: only progress depends on bd.
func TestProgressFailsWhenBDAbsent(t *testing.T) {
	configHome := t.TempDir()
	repo := newRepo(t)
	t.Setenv("PATH", "")

	var out, errw bytes.Buffer
	t.Setenv(paths.ConfigHomeEnv, configHome)
	e := &Env{Stdout: &out, Stderr: &errw, Cwd: repo, Manifest: managed.Manifest{}}
	if err := Run(e, []string{"progress", "sv-1", "--json"}); err == nil {
		t.Fatal("progress must fail when bd is absent")
	}
	if !strings.Contains(errw.String(), "bd") {
		t.Errorf("stderr does not name bd: %q", errw.String())
	}

	for _, cmd := range []string{"status", "resolve"} {
		got := runCLI(t, configHome, repo, cmd)
		if got.err != nil {
			t.Errorf("svibe %s must still succeed with bd absent: %v\n%s",
				cmd, got.err, got.stderr)
		}
	}
}

// --- guards ---------------------------------------------------------------

// TestProgressArgvNeverCarriesParent guards D12 end to end, at the CLI level
// rather than only inside the read layer.
func TestProgressArgvNeverCarriesParent(t *testing.T) {
	fx := fixture{issues: []fixtureIssue{
		{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
		task("sv-1.1", "sv-1", "open"),
	}}

	var argv [][]string
	got := runProgressRecording(t, fx, &argv, "progress", "sv-1", "--json")
	if got.err != nil {
		t.Fatalf("progress failed: %v\n%s", got.err, got.stderr)
	}
	if len(argv) != 4 {
		t.Fatalf("bd invocations = %d, want exactly 4 regardless of subtree size", len(argv))
	}

	for _, args := range argv {
		for _, a := range args {
			if a == "--parent" {
				t.Errorf("bd argv carries --parent, which D12 forbids: %v", args)
			}
		}
		if subcommandOf(args) == "blocked" {
			for _, a := range args {
				if a == "--limit" {
					t.Errorf("bd blocked argv carries --limit, which bd rejects: %v", args)
				}
			}
		}
	}
}

// TestProgressAvailabilityComesFromTheReadySet guards the defect that failed
// review round 2: readiness must be consumed from bd's global set, never
// re-derived. The member below looks unblocked in every structural sense and
// must still not be available, because bd did not say it was.
func TestProgressAvailabilityComesFromTheReadySet(t *testing.T) {
	fx := fixture{
		issues: []fixtureIssue{
			{ID: "sv-1", Title: "root", Status: "open", IssueType: "epic"},
			task("sv-1.1", "sv-1", "open"),
		},
		ready: nil, // bd reports nothing available
	}

	res := decodeResult(t, mustRun(t, fx, "sv-1"))
	if got := memberBuckets(t, res)["sv-1.1"]; got == bucketAvailable {
		t.Error("an open member absent from the global ready set was bucketed available, " +
			"so availability was re-derived instead of consumed")
	}

	// The converse: bd alone can make it available.
	fx.ready = []string{"sv-1.1"}
	res = decodeResult(t, mustRun(t, fx, "sv-1"))
	if got := memberBuckets(t, res)["sv-1.1"]; got != bucketAvailable {
		t.Errorf("bucket = %q, want available once bd reports it ready", got)
	}
}

// --- helpers --------------------------------------------------------------

// mustRun executes progress against a fixture and returns raw JSON stdout.
func mustRun(t *testing.T, fx fixture, rootID string) string {
	t.Helper()
	got := runProgressFixture(t, fx, "progress", rootID, "--json")
	if got.err != nil {
		t.Fatalf("progress %s failed: %v\n%s", rootID, got.err, got.stderr)
	}
	return got.stdout
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
