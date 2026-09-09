package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/mwenkdev/structured-vibe/internal/bd"
	"github.com/mwenkdev/structured-vibe/internal/managed"
	"github.com/mwenkdev/structured-vibe/internal/paths"
)

// --- fixture work graph ---------------------------------------------------

// fixtureIssue mirrors one bd list record. parent is omitempty so the fixture
// reproduces bd's real behaviour of omitting the key entirely when empty.
type fixtureIssue struct {
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	Status    string `json:"status"`
	IssueType string `json:"issue_type"`
	Priority  int    `json:"priority,omitempty"`
	Parent    string `json:"parent,omitempty"`
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

	// Reserved until later milestones.
	if res["verification_counts"] != nil {
		t.Errorf("verification_counts = %v, want null in M1", res["verification_counts"])
	}
	if res["stop"] != nil {
		t.Errorf("stop = %v, want null in M1", res["stop"])
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
	for _, k := range []string{"verification", "verification_raw", "verification_commit"} {
		if m[k] != nil {
			t.Errorf("member %s = %v, want null in M1", k, m[k])
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
