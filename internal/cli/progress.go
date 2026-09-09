package cli

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/mwenkdev/structured-vibe/internal/bd"
	"github.com/mwenkdev/structured-vibe/internal/cliout"
	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/paths"
)

// Diagnostic codes emitted by progress.
const (
	// codeNoProject reports that the working directory is not in a repository.
	codeNoProject = "progress.no-project"
	// codeUnknownRoot reports a root id absent from the work graph.
	codeUnknownRoot = "progress.unknown-root"
	// codeUnclassified reports a member no bucket rule could claim.
	codeUnclassified = "progress.unclassified"
)

// Buckets of the D9 taxonomy. Every listed member carries exactly one.
const (
	bucketContainer    = "container"
	bucketCompleted    = "completed"
	bucketExcluded     = "excluded"
	bucketDeferred     = "deferred"
	bucketActive       = "active"
	bucketAvailable    = "available"
	bucketBlocked      = "blocked"
	bucketUnclassified = "unclassified"
)

// Stored bd statuses the bucket rules test by name.
const (
	statusClosed     = "closed"
	statusPinned     = "pinned"
	statusDeferred   = "deferred"
	statusInProgress = "in_progress"
	statusHooked     = "hooked"
	statusBlocked    = "blocked"
)

// infrastructureTypes are operational bead types excluded from the projection
// entirely: not listed, not counted, in no denominator (D8).
//
// This is the single named location the specification requires. The event
// exclusion is load-bearing rather than tidy: bd set-state creates an
// event-type bead as a parent-child CHILD of the work bead, so without this
// exclusion every verified bead would pollute its own subtree.
//
// bd types --json reports only core types, so it is not a complete registry
// and this list is maintained explicitly and defended by fixtures.
var infrastructureTypes = map[string]bool{
	"event":   true,
	"agent":   true,
	"role":    true,
	"message": true,
	"gate":    true,
}

// containerTypes hold no work of their own (D8). They are listed with bucket
// container but excluded from every denominator, so closing a nested epic
// cannot add progress on top of the work its children already supplied.
var containerTypes = map[string]bool{
	"epic":      true,
	"milestone": true,
}

// progressResult is the complete result object of the output schema.
//
// Every key is always present. Absence is expressed as null for scalars and
// [] for collections, never by omission, so consumers can index without
// existence checks.
type progressResult struct {
	Root      rootRef `json:"root"`
	BDVersion string  `json:"bd_version"`

	Counts          progressCounts `json:"counts"`
	CompletionRatio *float64       `json:"completion_ratio"`

	Members []member `json:"members"`

	// VerificationCounts, Anomalies are reserved for M2. Stop is populated in M3.
	VerificationCounts any         `json:"verification_counts"`
	Anomalies          []any       `json:"anomalies"`
	Stop               *stopResult `json:"stop"`
}

// rootRef identifies the subtree root, which is never its own member.
type rootRef struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	IssueType string `json:"issue_type"`
}

// progressCounts is the bucket census.
//
// TotalCountable excludes container and excluded members: neither a
// structural container nor a deliberately persistent bead should make a
// subtree incompletable.
type progressCounts struct {
	Completed      int `json:"completed"`
	Deferred       int `json:"deferred"`
	Active         int `json:"active"`
	Blocked        int `json:"blocked"`
	Available      int `json:"available"`
	Unclassified   int `json:"unclassified"`
	Container      int `json:"container"`
	Excluded       int `json:"excluded"`
	TotalCountable int `json:"total_countable"`
}

// member is one bead in the subtree.
type member struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	IssueType string `json:"issue_type"`
	Priority  int    `json:"priority"`
	Parent    string `json:"parent"`
	Bucket    string `json:"bucket"`

	// Verification fields are reserved for M2 and are null here.
	Verification       *string `json:"verification"`
	VerificationRaw    *string `json:"verification_raw"`
	VerificationCommit *string `json:"verification_commit"`

	BlockedBy []blocker `json:"blocked_by"`
}

// blocker is one named blocker of a member.
//
// Blocker identity comes from bd blocked's blocked_by[], never from raw
// blocks edges: blockage propagates through parent-child edges, so a child of
// a blocked parent is reported blocked with its parent named here despite
// having no blocks edge of its own.
type blocker struct {
	ID string `json:"id"`
	// Status is null when the blocker is absent from the work graph.
	Status    *string `json:"status"`
	InSubtree bool    `json:"in_subtree"`
}

// Stop reason categories: the closed vocabulary of D11.
//
// stopCategoryOrder fixes serialization order, so reasons never inherit map
// or bucket iteration order and output stays byte-stable.
const (
	stopDependencyBlocked = "dependency_blocked"
	stopStoredBlocked     = "stored_blocked"
	stopDeferred          = "deferred"
	stopUnclassified      = "unclassified"
)

var stopCategoryOrder = []string{
	stopDependencyBlocked,
	stopStoredBlocked,
	stopDeferred,
	stopUnclassified,
}

// stopReason is one categorised reason, naming the members that caused it.
//
// It introduces no blocker data of its own: members are referenced by id and
// their blocker detail already lives on members[].blocked_by (D11).
type stopReason struct {
	Category string   `json:"category"`
	Members  []string `json:"members"`
}

// stopResult is the subtree-level stop summary (D6, D11).
//
// It is a non-null object in every successful result, so a consumer never has
// to distinguish "not stopped" from "not computed". Reasons is populated only
// when Stopped is true and is otherwise an empty list.
type stopResult struct {
	Stopped  bool         `json:"stopped"`
	Complete bool         `json:"complete"`
	Reasons  []stopReason `json:"reasons"`
}

func (r *progressResult) PrintHuman(w io.Writer) {
	fmt.Fprintf(w, "root:     %s  %s\n", r.Root.ID, r.Root.Title)
	fmt.Fprintf(w, "type:     %s (%s)\n", r.Root.IssueType, r.Root.Status)
	fmt.Fprintf(w, "bd:       %s\n", r.BDVersion)

	fmt.Fprintf(w, "\nprogress: ")
	if r.CompletionRatio == nil {
		fmt.Fprintln(w, "n/a (no countable members)")
	} else {
		fmt.Fprintf(w, "%.1f%% (%d/%d complete)\n",
			*r.CompletionRatio*100, r.Counts.Completed, r.Counts.TotalCountable)
	}

	// Presentation only: the same structural data the JSON carries, with no
	// interpretation and nothing the machine-readable form does not have.
	if s := r.Stop; s != nil {
		fmt.Fprintf(w, "state:    ")
		switch {
		case s.Complete:
			fmt.Fprintln(w, "complete (all countable members closed)")
		case s.Stopped:
			fmt.Fprintln(w, "stopped (no available or active work)")
		case r.Counts.TotalCountable == 0:
			fmt.Fprintln(w, "no countable work in this subtree")
		default:
			fmt.Fprintln(w, "in progress")
		}
		for _, reason := range s.Reasons {
			fmt.Fprintf(w, "  %-20s %s\n", reason.Category, strings.Join(reason.Members, ", "))
		}
	}

	c := r.Counts
	fmt.Fprintf(w, "\ncounts (%d countable):\n", c.TotalCountable)
	for _, row := range []struct {
		name string
		n    int
	}{
		{bucketCompleted, c.Completed},
		{bucketActive, c.Active},
		{bucketAvailable, c.Available},
		{bucketBlocked, c.Blocked},
		{bucketDeferred, c.Deferred},
		{bucketUnclassified, c.Unclassified},
		{bucketContainer, c.Container},
		{bucketExcluded, c.Excluded},
	} {
		fmt.Fprintf(w, "  %-14s %d\n", row.name, row.n)
	}

	fmt.Fprintf(w, "\nmembers (%d):\n", len(r.Members))
	if len(r.Members) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	for _, m := range r.Members {
		fmt.Fprintf(w, "  %-14s %-24s %s\n", m.Bucket, m.ID, m.Title)
		for _, b := range m.BlockedBy {
			scope := "outside subtree"
			if b.InSubtree {
				scope = "in subtree"
			}
			fmt.Fprintf(w, "      blocked by %s (%s)\n", b.ID, scope)
		}
	}
}

func runProgress(e *Env, args []string) error {
	fs := newFlagSet("progress", e.Stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON on stdout")

	rest, err := parseMixed(fs, args)
	if err != nil {
		return &ExitError{Code: 2}
	}
	if len(rest) != 1 {
		fmt.Fprintln(e.Stderr, "svibe: progress takes exactly one bead id")
		return &ExitError{Code: 2}
	}
	// The bead id is passed through unvalidated. Beads owns its identifier
	// grammar, hierarchical ids contain periods, and argv execution already
	// precludes shell injection, so a conservative pattern would reject
	// valid ids.
	rootID := rest[0]

	out := cliout.New(e.Stdout, e.Stderr, *asJSON)
	d := e.baseDiags()

	root, err := paths.ProjectRoot(e.cwd())
	if err != nil {
		d.Errorf(codeNoProject, e.cwd(),
			"progress needs a Git repository so bd can locate the work graph")
		out.Emit(false, d, nil)
		return Failure
	}

	client := &bd.Client{Root: root, Runner: e.BDRunner}
	snap, rd := client.Read()
	d.Extend(rd)
	if snap == nil || d.HasErrors() {
		out.Emit(false, d, nil)
		return Failure
	}

	rootIssue, ok := snap.Issues[rootID]
	if !ok {
		d.Errorf(codeUnknownRoot, "", "bead %q was not found in the work graph", rootID)
		out.Emit(false, d, nil)
		return Failure
	}

	res, pd := project(snap, rootIssue)
	d.Extend(pd)

	if !out.Emit(!d.HasErrors(), d, res) {
		return Failure
	}
	return nil
}

// project computes the subtree projection for one root.
func project(snap *bd.Snapshot, root bd.Issue) (*progressResult, diag.Diagnostics) {
	var d diag.Diagnostics

	ids := membership(snap, root.ID)
	inSubtree := make(map[string]bool, len(ids))
	for _, id := range ids {
		inSubtree[id] = true
	}

	res := &progressResult{
		Root: rootRef{
			ID:        root.ID,
			Title:     root.Title,
			Status:    root.Status,
			IssueType: root.IssueType,
		},
		BDVersion: snap.Version,
		Members:   []member{},
		Anomalies: []any{},
	}

	for _, id := range ids {
		is := snap.Issues[id]
		bucket := classify(is, snap.Ready, snap.Blocked)
		if bucket == bucketUnclassified {
			d.Warnf(codeUnclassified, "",
				"bead %s has status %q, which no bucket rule claims; Beads knows "+
					"something this projection does not", is.ID, is.Status)
		}

		res.Members = append(res.Members, member{
			ID:        is.ID,
			Title:     is.Title,
			Status:    is.Status,
			IssueType: is.IssueType,
			Priority:  is.Priority,
			Parent:    is.Parent,
			Bucket:    bucket,
			BlockedBy: blockersOf(snap, is.ID, inSubtree),
		})
		res.Counts.add(bucket)
	}

	// Ordering is owned by svibe serialization, never inherited from bd
	// response order. This is what makes the determinism test meaningful.
	sort.Slice(res.Members, func(i, j int) bool { return res.Members[i].ID < res.Members[j].ID })

	res.Counts.TotalCountable = res.Counts.countable()
	res.CompletionRatio = ratio(res.Counts.Completed, res.Counts.TotalCountable)

	res.Stop = computeStop(res)
	return res, d
}

// computeStop derives the stop summary from data the projection already holds
// (D11). It reads bucket assignments and blocked_by detail only: no new bd
// call, no dependency-edge walk, and deliberately no verification data.
//
// Structural only. It reports that work has halted and which members caused
// it; it never interprets why a blocker is hard or advises how to clear it.
func computeStop(res *progressResult) *stopResult {
	c := res.Counts
	outstanding := c.TotalCountable - c.Completed

	out := &stopResult{
		// Complete means every countable member is closed. A zero-countable
		// subtree is not complete: there was no work to finish.
		Complete: c.TotalCountable > 0 && outstanding == 0,
		// Stopped means work remains but nothing can be picked up and nothing
		// is under way.
		Stopped: outstanding > 0 && c.Available == 0 && c.Active == 0,
		Reasons: []stopReason{},
	}
	if !out.Stopped {
		return out
	}

	// Members are already sorted by id ascending, so appending in iteration
	// order keeps every reason's member list sorted without re-sorting.
	byCategory := map[string][]string{}
	for _, m := range res.Members {
		if cat := stopCategoryOf(m); cat != "" {
			byCategory[cat] = append(byCategory[cat], m.ID)
		}
	}

	for _, cat := range stopCategoryOrder {
		if members := byCategory[cat]; len(members) > 0 {
			out.Reasons = append(out.Reasons, stopReason{Category: cat, Members: members})
		}
	}
	return out
}

// stopCategoryOf maps one member to its D11 stop category, or "" when the
// member is not a reason for the subtree being stopped.
//
// A stopped subtree has no available or active members, so every outstanding
// member lands in exactly one category here. That is what makes the
// "stopped implies at least one reason" guarantee structural rather than
// defensive.
func stopCategoryOf(m member) string {
	switch m.Bucket {
	case bucketBlocked:
		// The split is whether Beads named a blocker. A stored-blocked bead
		// with no dependency edge has nothing to name, and reporting it as
		// dependency_blocked would promise blocker detail that does not exist.
		if len(m.BlockedBy) > 0 {
			return stopDependencyBlocked
		}
		return stopStoredBlocked
	case bucketDeferred:
		return stopDeferred
	case bucketUnclassified:
		return stopUnclassified
	}
	return ""
}

// membership walks parent-child edges breadth-first from the root (D8).
//
// blocks edges never confer membership. The root is never a member of its own
// subtree. Infrastructure beads are not listed, but the walk still descends
// through them so that work parented beneath one is not silently dropped.
func membership(snap *bd.Snapshot, rootID string) []string {
	children := make(map[string][]string, len(snap.Issues))
	for id, is := range snap.Issues {
		if is.Parent == "" {
			continue
		}
		children[is.Parent] = append(children[is.Parent], id)
	}
	for parent := range children {
		sort.Strings(children[parent])
	}

	// The visited set is the cycle guard, and seeding it with the root also
	// keeps a cycle back through the root from listing the root as its own
	// member.
	visited := map[string]bool{rootID: true}
	queue := []string{rootID}
	out := []string{}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, id := range children[cur] {
			if visited[id] {
				continue
			}
			visited[id] = true
			queue = append(queue, id)

			if infrastructureTypes[snap.Issues[id].IssueType] {
				continue
			}
			out = append(out, id)
		}
	}
	return out
}

// classify assigns exactly one bucket by the first matching D9 rule.
//
// Availability and blockage are read from bd's global sets intersected with
// membership, never recomputed from dependency edges: Beads owns readiness.
func classify(is bd.Issue, ready map[string]bool, blocked map[string]bd.Blocked) string {
	switch {
	case containerTypes[is.IssueType]:
		return bucketContainer
	case is.Status == statusClosed:
		return bucketCompleted
	case is.Status == statusPinned:
		return bucketExcluded
	case is.Status == statusDeferred:
		return bucketDeferred
	case is.Status == statusInProgress, is.Status == statusHooked:
		// Rule 5 precedes 6-7 deliberately: an in_progress member with
		// blockers is active, because someone is in fact working on it.
		return bucketActive
	case ready[is.ID]:
		return bucketAvailable
	}

	if _, isBlocked := blocked[is.ID]; isBlocked {
		return bucketBlocked
	}
	// The stored-status disjunct is necessary, not defensive: a bead with
	// stored status blocked and no dependency edges appears in neither the
	// global ready set nor the global blocked set.
	if is.Status == statusBlocked {
		return bucketBlocked
	}
	return bucketUnclassified
}

// blockersOf resolves one member's named blockers, sorted by id ascending.
func blockersOf(snap *bd.Snapshot, id string, inSubtree map[string]bool) []blocker {
	out := []blocker{}
	entry, ok := snap.Blocked[id]
	if !ok {
		return out
	}

	for _, bid := range entry.BlockedBy {
		b := blocker{ID: bid, InSubtree: inSubtree[bid]}
		// Call 2 returns every issue, so an out-of-subtree blocker's status
		// resolves from the same map with no extra bd invocation.
		if is, found := snap.Issues[bid]; found {
			status := is.Status
			b.Status = &status
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// add records one member in the census.
func (c *progressCounts) add(bucket string) {
	switch bucket {
	case bucketCompleted:
		c.Completed++
	case bucketDeferred:
		c.Deferred++
	case bucketActive:
		c.Active++
	case bucketBlocked:
		c.Blocked++
	case bucketAvailable:
		c.Available++
	case bucketUnclassified:
		c.Unclassified++
	case bucketContainer:
		c.Container++
	case bucketExcluded:
		c.Excluded++
	}
}

// countable is the denominator: container and excluded members are listed but
// counted in no denominator.
func (c progressCounts) countable() int {
	return c.Completed + c.Deferred + c.Active + c.Blocked + c.Available + c.Unclassified
}

// ratio is completed/total rounded to 4 decimal places for byte stability,
// and null with a zero denominator: never 0 and never 1.
func ratio(completed, total int) *float64 {
	if total == 0 {
		return nil
	}
	r := math.Round(float64(completed)/float64(total)*10000) / 10000
	return &r
}
