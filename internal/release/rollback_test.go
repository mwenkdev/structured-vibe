// Package release hosts tests for the release automation shell scripts.
//
// scripts/rollback-release.sh is invoked by .github/workflows/release.yml and
// cannot be exercised by cutting real releases, which is exactly what must not
// be used as a test bed (docs/specs/release-rollback-coverage.md, decision D3).
// The script resolves the gh executable through $GH, so these tests inject a
// stub and assert the control flow, the idempotency behavior from D2, and the
// prerelease exemption from D4.
//
// Stub fidelity: the stub matches on the gh subcommand and API path and returns
// canned output. It does not implement --jq, so the jq filters in the script
// itself are not exercised here; the filters are deliberately simple and their
// real behavior is covered by the empirical validation in sv-29u.3. What these
// tests do cover is every branch the script can take in response to a given
// answer, which is where the reported bug lives.
package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// scriptPath locates the script under test relative to this package.
func scriptPath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "scripts", "rollback-release.sh"))
	if err != nil {
		t.Fatalf("resolving script path: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("script not found at %s: %v", p, err)
	}
	return p
}

// stub describes how the fake gh responds.
//
// releaseRows and tagRows hold the raw rows the real gh would emit for the
// script's two existence queries, so the script's own exact-match logic is what
// the tests exercise. Each entry is one answer in a sequence: call n gets entry
// n, and the final entry repeats once the list is exhausted. Use the empty
// string for "the query returned nothing".
type stub struct {
	// releaseRows answers the "list releases" query. Each row is formatted
	// "<tag>\t<id>"; a single answer may carry several rows separated by "|".
	releaseRows []string
	// tagRows answers the matching-refs query with refs like "refs/tags/1.2.3".
	tagRows []string
	// failDeleteRelease makes DELETE on a release exit non-zero.
	failDeleteRelease bool
	// failDeleteTag makes DELETE on a tag ref exit non-zero.
	failDeleteTag bool
	// failLookup makes every existence query exit non-zero, simulating an API
	// failure that must not be confused with absence.
	failLookup bool
	// failReleaseLookup and failTagLookup fail only one of the two queries, so a
	// missing guard on one cannot hide behind the other's error message.
	failReleaseLookup bool
	failTagLookup     bool
}

// writeStub writes a fake gh into dir and returns its path plus the path of the
// log that records every invocation.
func writeStub(t *testing.T, dir string, s stub) (string, string) {
	t.Helper()

	ghPath := filepath.Join(dir, "gh")
	logPath := filepath.Join(dir, "gh.log")
	statePath := filepath.Join(dir, "state")

	// Absence is encoded as the sentinel "NONE" rather than an empty line:
	// command substitution strips trailing newlines, so a trailing empty answer
	// would silently disappear from the sequence.
	seq := func(vals []string) string {
		if len(vals) == 0 {
			return "NONE"
		}
		encoded := make([]string, len(vals))
		for i, v := range vals {
			if v == "" {
				encoded[i] = "NONE"
			} else {
				encoded[i] = v
			}
		}
		return strings.Join(encoded, "\n")
	}

	// The stub keeps a per-query call counter in $statePath so a scenario can
	// answer differently across calls, which is what the end-state re-check in
	// D2 requires.
	body := `#!/usr/bin/env bash
set -uo pipefail

log_file=` + shellQuote(logPath) + `
state_dir=` + shellQuote(statePath) + `
mkdir -p "$state_dir"

printf '%s\n' "$*" >> "$log_file"

args="$*"

fail_release_lookup=` + boolLiteral(s.failLookup || s.failReleaseLookup) + `
fail_tag_lookup=` + boolLiteral(s.failLookup || s.failTagLookup) + `
fail_delete_release=` + boolLiteral(s.failDeleteRelease) + `
fail_delete_tag=` + boolLiteral(s.failDeleteTag) + `

release_answers=$(cat <<'EOF'
` + seq(s.releaseRows) + `
EOF
)
tag_answers=$(cat <<'EOF'
` + seq(s.tagRows) + `
EOF
)

# answer <name> <answers>: prints the nth line of <answers> for the nth call,
# repeating the final line once the list is exhausted. The sentinel NONE prints
# nothing, which is how the real gh reports an absent object.
answer() {
	local name="$1" answers="$2" counter n total value
	counter="$state_dir/$name"
	n=0
	if [[ -f "$counter" ]]; then
		n=$(cat "$counter")
	fi
	printf '%s' "$((n + 1))" > "$counter"

	total=$(printf '%s\n' "$answers" | grep -c '')
	if [[ "$n" -ge "$total" ]]; then
		n=$((total - 1))
	fi
	value=$(printf '%s' "$answers" | sed -n "$((n + 1))p")
	if [[ "$value" != NONE ]]; then
		# A single answer may carry several rows, separated by "|" so the
		# answer sequence itself stays one-per-line.
		printf '%s\n' "${value//|/$'\n'}"
	fi
}

case "$args" in
	*"--method DELETE"*"/releases/"*)
		if [[ "$fail_delete_release" == true ]]; then
			echo "stub: release delete failed" >&2
			exit 1
		fi
		exit 0
		;;
	*"--method DELETE"*"/git/refs/tags/"*)
		if [[ "$fail_delete_tag" == true ]]; then
			echo "stub: tag delete failed" >&2
			exit 1
		fi
		exit 0
		;;
	*"/git/matching-refs/tags/"*)
		if [[ "$fail_tag_lookup" == true ]]; then
			echo "stub: tag lookup failed" >&2
			exit 1
		fi
		answer tag "$tag_answers"
		exit 0
		;;
	*"/releases"*)
		if [[ "$fail_release_lookup" == true ]]; then
			echo "stub: release lookup failed" >&2
			exit 1
		fi
		answer release "$release_answers"
		exit 0
		;;
esac

echo "stub: unexpected gh invocation: $args" >&2
exit 64
`

	if err := os.WriteFile(ghPath, []byte(body), 0o755); err != nil {
		t.Fatalf("writing stub gh: %v", err)
	}
	return ghPath, logPath
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func boolLiteral(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

type result struct {
	exit   int
	stdout string
	stderr string
	calls  []string
}

// run executes the script with the given arguments and stub.
func run(t *testing.T, s stub, args ...string) result {
	t.Helper()

	dir := t.TempDir()
	ghPath, logPath := writeStub(t, dir, s)

	cmd := exec.Command(scriptPath(t), args...)
	cmd.Env = append(os.Environ(),
		"GH="+ghPath,
		"GITHUB_REPOSITORY=mwenkdev/structured-vibe",
	)
	var out, errBuf strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()

	exit := 0
	if err != nil {
		var ee *exec.ExitError
		if ok := asExitError(err, &ee); !ok {
			t.Fatalf("running script: %v (stderr: %s)", err, errBuf.String())
		}
		exit = ee.ExitCode()
	}

	var calls []string
	if raw, readErr := os.ReadFile(logPath); readErr == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			if line != "" {
				calls = append(calls, line)
			}
		}
	}

	return result{exit: exit, stdout: out.String(), stderr: errBuf.String(), calls: calls}
}

func asExitError(err error, target **exec.ExitError) bool {
	ee, ok := err.(*exec.ExitError)
	if ok {
		*target = ee
	}
	return ok
}

func (r result) output() string { return r.stdout + r.stderr }

func (r result) deleteCalls() []string {
	var out []string
	for _, c := range r.calls {
		if strings.Contains(c, "--method DELETE") {
			out = append(out, c)
		}
	}
	return out
}

func TestDeletesReleaseAndTagWhenBothPresent(t *testing.T) {
	r := run(t, stub{
		releaseRows: []string{"1.2.3\t42", ""},
		tagRows:     []string{"refs/tags/1.2.3", ""},
	}, "1.2.3", "false")

	if r.exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", r.exit, r.output())
	}

	deletes := r.deleteCalls()
	if len(deletes) != 2 {
		t.Fatalf("got %d delete calls, want 2: %v", len(deletes), deletes)
	}
	if !strings.Contains(deletes[0], "/releases/42") {
		t.Errorf("first delete = %q, want the release id 42", deletes[0])
	}
	if !strings.Contains(deletes[1], "/git/refs/tags/1.2.3") {
		t.Errorf("second delete = %q, want the tag ref", deletes[1])
	}
}

func TestToleratesReleaseAlreadyDeleted(t *testing.T) {
	r := run(t, stub{
		releaseRows: []string{""},
		tagRows:     []string{"refs/tags/1.2.3", ""},
	}, "1.2.3", "false")

	if r.exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", r.exit, r.output())
	}
	if !strings.Contains(r.output(), "already absent") {
		t.Errorf("output does not report the release as already absent:\n%s", r.output())
	}

	for _, c := range r.deleteCalls() {
		if strings.Contains(c, "/releases/") {
			t.Errorf("deleted a release that was already absent: %q", c)
		}
	}
}

func TestToleratesTagAlreadyDeleted(t *testing.T) {
	r := run(t, stub{
		releaseRows: []string{"1.2.3\t42", ""},
		tagRows:     []string{""},
	}, "1.2.3", "false")

	if r.exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", r.exit, r.output())
	}

	for _, c := range r.deleteCalls() {
		if strings.Contains(c, "/git/refs/tags/") {
			t.Errorf("deleted a tag that was already absent: %q", c)
		}
	}
}

func TestSucceedsWhenBothAlreadyAbsent(t *testing.T) {
	r := run(t, stub{
		releaseRows: []string{""},
		tagRows:     []string{""},
	}, "1.2.3", "false")

	if r.exit != 0 {
		t.Fatalf("exit = %d, want 0 (rollback must be safe to run twice)\n%s", r.exit, r.output())
	}
	if got := r.deleteCalls(); len(got) != 0 {
		t.Errorf("attempted deletes with nothing present: %v", got)
	}
}

func TestFailsWhenReleaseDeletionFails(t *testing.T) {
	r := run(t, stub{
		releaseRows:       []string{"1.2.3\t42"},
		tagRows:           []string{""},
		failDeleteRelease: true,
	}, "1.2.3", "false")

	if r.exit == 0 {
		t.Fatalf("exit = 0, want non-zero when the release deletion fails\n%s", r.output())
	}
	if !strings.Contains(r.output(), "1.2.3") {
		t.Errorf("failure output does not name the release:\n%s", r.output())
	}
}

func TestFailsWhenTagDeletionFails(t *testing.T) {
	r := run(t, stub{
		releaseRows:   []string{"1.2.3\t42", ""},
		tagRows:       []string{"refs/tags/1.2.3"},
		failDeleteTag: true,
	}, "1.2.3", "false")

	if r.exit == 0 {
		t.Fatalf("exit = 0, want non-zero when the tag deletion fails\n%s", r.output())
	}
	if !strings.Contains(r.output(), "tag '1.2.3'") {
		t.Errorf("failure output does not name the surviving tag:\n%s", r.output())
	}
}

// The defect D2 fixes: a delete that reports success while the object survives
// must not be reported as a successful rollback.
func TestEndStateRecheckCatchesSurvivingRelease(t *testing.T) {
	r := run(t, stub{
		releaseRows: []string{"1.2.3\t42", "1.2.3\t42"},
		tagRows:     []string{""},
	}, "1.2.3", "false")

	if r.exit == 0 {
		t.Fatalf("exit = 0, want non-zero when the release survives a reported-successful delete\n%s", r.output())
	}
	out := r.output()
	if !strings.Contains(out, "rollback incomplete") {
		t.Errorf("output does not report an incomplete rollback:\n%s", out)
	}
	if !strings.Contains(out, "release '1.2.3'") {
		t.Errorf("output does not name the surviving release:\n%s", out)
	}
}

func TestEndStateRecheckCatchesSurvivingTag(t *testing.T) {
	r := run(t, stub{
		releaseRows: []string{"1.2.3\t42", ""},
		tagRows:     []string{"refs/tags/1.2.3", "refs/tags/1.2.3"},
	}, "1.2.3", "false")

	if r.exit == 0 {
		t.Fatalf("exit = 0, want non-zero when the tag survives\n%s", r.output())
	}
	if !strings.Contains(r.output(), "tag '1.2.3'") {
		t.Errorf("output does not name the surviving tag:\n%s", r.output())
	}
}

func TestPrereleaseIsACompleteNoOp(t *testing.T) {
	r := run(t, stub{
		releaseRows: []string{"2.0.0-beta.1\t42"},
		tagRows:     []string{"refs/tags/2.0.0-beta.1"},
	}, "2.0.0-beta.1", "true")

	if r.exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", r.exit, r.output())
	}
	if len(r.calls) != 0 {
		t.Errorf("prerelease exemption must not call gh at all, got: %v", r.calls)
	}
	if !strings.Contains(r.output(), "exempt") {
		t.Errorf("output does not report the exemption:\n%s", r.output())
	}
}

func TestRejectsEmptyVersion(t *testing.T) {
	r := run(t, stub{}, "", "false")

	if r.exit == 0 {
		t.Fatalf("exit = 0, want non-zero for an empty version\n%s", r.output())
	}
	if !strings.Contains(r.output(), "empty value") {
		t.Errorf("output does not name the problem:\n%s", r.output())
	}
	if len(r.calls) != 0 {
		t.Errorf("called gh despite an empty version: %v", r.calls)
	}
}

func TestRejectsMissingVersion(t *testing.T) {
	r := run(t, stub{})

	if r.exit == 0 {
		t.Fatalf("exit = 0, want non-zero when no arguments are given\n%s", r.output())
	}
}

func TestRejectsNonBooleanPrereleaseFlag(t *testing.T) {
	r := run(t, stub{}, "1.2.3", "yes")

	if r.exit == 0 {
		t.Fatalf("exit = 0, want non-zero for a non-boolean prerelease flag\n%s", r.output())
	}
	if len(r.calls) != 0 {
		t.Errorf("called gh despite an invalid flag: %v", r.calls)
	}
}

// The GitHub prerelease flag is authoritative and the disagreement is reported
// rather than silently resolved (D4). A flagged 1.2.3 is retained.
func TestFlaggedFullSemVerIsRetainedWithWarning(t *testing.T) {
	r := run(t, stub{
		releaseRows: []string{"1.2.3\t42"},
		tagRows:     []string{"refs/tags/1.2.3"},
	}, "1.2.3", "true")

	if r.exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", r.exit, r.output())
	}
	out := r.output()
	if !strings.Contains(out, "::warning::") {
		t.Errorf("no warning emitted for disagreeing signals:\n%s", out)
	}
	if !strings.Contains(out, "1.2.3") || !strings.Contains(out, "flag") {
		t.Errorf("warning does not name both the version and the flag:\n%s", out)
	}
	if len(r.calls) != 0 {
		t.Errorf("flagged release must be retained, got gh calls: %v", r.calls)
	}
}

// The other direction: an unflagged 2.0.0-beta.1 is rolled back like any other
// full release, with the disagreement reported.
func TestUnflaggedPrereleaseSemVerIsRolledBackWithWarning(t *testing.T) {
	r := run(t, stub{
		releaseRows: []string{"2.0.0-beta.1\t42", ""},
		tagRows:     []string{"refs/tags/2.0.0-beta.1", ""},
	}, "2.0.0-beta.1", "false")

	if r.exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", r.exit, r.output())
	}
	out := r.output()
	if !strings.Contains(out, "::warning::") {
		t.Errorf("no warning emitted for disagreeing signals:\n%s", out)
	}
	if !strings.Contains(out, "2.0.0-beta.1") || !strings.Contains(out, "flag") {
		t.Errorf("warning does not name both the version and the flag:\n%s", out)
	}
	if got := r.deleteCalls(); len(got) != 2 {
		t.Errorf("got %d delete calls, want 2 (release and tag): %v", len(got), got)
	}
}

// Build metadata carries a hyphen without being a prerelease suffix, so it must
// not trigger the disagreement warning.
func TestBuildMetadataIsNotAPrereleaseSuffix(t *testing.T) {
	r := run(t, stub{
		releaseRows: []string{"1.2.3+linux-amd64\t42", ""},
		tagRows:     []string{"refs/tags/1.2.3+linux-amd64", ""},
	}, "1.2.3+linux-amd64", "false")

	if r.exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", r.exit, r.output())
	}
	if strings.Contains(r.output(), "::warning::") {
		t.Errorf("build metadata treated as a prerelease suffix:\n%s", r.output())
	}
}

func TestAgreeingSignalsEmitNoWarning(t *testing.T) {
	r := run(t, stub{
		releaseRows: []string{"1.2.3\t42", ""},
		tagRows:     []string{"refs/tags/1.2.3", ""},
	}, "1.2.3", "false")

	if strings.Contains(r.output(), "::warning::") {
		t.Errorf("unexpected warning when the flag and the suffix agree:\n%s", r.output())
	}
}

// An API failure during an existence query must not be mistaken for absence,
// because that would silently report a successful rollback of a release that
// still exists.
func TestLookupFailureIsNotTreatedAsAbsence(t *testing.T) {
	r := run(t, stub{failLookup: true}, "1.2.3", "false")

	if r.exit == 0 {
		t.Fatalf("exit = 0, want non-zero when the existence query fails\n%s", r.output())
	}
	if !strings.Contains(r.output(), "could not determine") {
		t.Errorf("output does not distinguish a lookup failure from absence:\n%s", r.output())
	}
	if got := r.deleteCalls(); len(got) != 0 {
		t.Errorf("attempted a delete despite an unknown state: %v", got)
	}
}

// Each lookup is guarded separately. Failing only one proves the guard on that
// query exists, rather than letting the other query's error cover for it.
func TestReleaseLookupFailureAloneStopsRollback(t *testing.T) {
	r := run(t, stub{
		failReleaseLookup: true,
		tagRows:           []string{"refs/tags/1.2.3"},
	}, "1.2.3", "false")

	if r.exit == 0 {
		t.Fatalf("exit = 0, want non-zero when the release lookup fails\n%s", r.output())
	}
	if !strings.Contains(r.output(), "whether release '1.2.3' exists") {
		t.Errorf("output does not name the failed release lookup:\n%s", r.output())
	}
	if got := r.deleteCalls(); len(got) != 0 {
		t.Errorf("attempted a delete despite an unknown release state: %v", got)
	}
}

func TestTagLookupFailureAloneStopsRollback(t *testing.T) {
	r := run(t, stub{
		releaseRows:   []string{""},
		failTagLookup: true,
	}, "1.2.3", "false")

	if r.exit == 0 {
		t.Fatalf("exit = 0, want non-zero when the tag lookup fails\n%s", r.output())
	}
	if !strings.Contains(r.output(), "whether tag '1.2.3' exists") {
		t.Errorf("output does not name the failed tag lookup:\n%s", r.output())
	}
}

// Matching refs is a prefix query, so a rollback of 1.2.3 must not touch 1.2.30.
// Deleting a bystander release would be worse than failing to roll back.
func TestDoesNotMatchPrefixCollidingTag(t *testing.T) {
	r := run(t, stub{
		releaseRows: []string{"1.2.30\t99"},
		tagRows:     []string{"refs/tags/1.2.30"},
	}, "1.2.3", "false")

	if r.exit != 0 {
		t.Fatalf("exit = %d, want 0 (nothing matching 1.2.3 exists)\n%s", r.exit, r.output())
	}
	if got := r.deleteCalls(); len(got) != 0 {
		t.Errorf("deleted an object belonging to a different release: %v", got)
	}
}

func TestSelectsExactMatchAmongSeveralReleases(t *testing.T) {
	r := run(t, stub{
		releaseRows: []string{"1.2.30\t99|1.2.3\t42|0.1.0\t7", ""},
		tagRows:     []string{"refs/tags/1.2.30|refs/tags/1.2.3", ""},
	}, "1.2.3", "false")

	if r.exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", r.exit, r.output())
	}
	deletes := r.deleteCalls()
	if len(deletes) != 2 {
		t.Fatalf("got %d delete calls, want 2: %v", len(deletes), deletes)
	}
	if !strings.Contains(deletes[0], "/releases/42") {
		t.Errorf("deleted the wrong release: %q, want id 42", deletes[0])
	}
}

// The version reaches the script unvalidated, because a version-format failure
// is one of the conditions rollback compensates for. A quote in the tag name
// must not break the existence queries.
func TestHandlesVersionContainingAQuote(t *testing.T) {
	version := `1.2.3"weird`
	r := run(t, stub{
		releaseRows: []string{version + "\t42", ""},
		tagRows:     []string{"refs/tags/" + version, ""},
	}, version, "false")

	if r.exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", r.exit, r.output())
	}
	if got := r.deleteCalls(); len(got) != 2 {
		t.Errorf("got %d delete calls, want 2: %v", len(got), got)
	}
}

func TestRequiresRepository(t *testing.T) {
	dir := t.TempDir()
	ghPath, _ := writeStub(t, dir, stub{})

	cmd := exec.Command(scriptPath(t), "1.2.3", "false")
	// Deliberately no GITHUB_REPOSITORY and no third argument.
	cmd.Env = []string{"GH=" + ghPath, "PATH=" + os.Getenv("PATH")}
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()

	if err == nil {
		t.Fatalf("exit = 0, want non-zero without a repository\n%s", out.String())
	}
	if !strings.Contains(out.String(), "owner/name") {
		t.Errorf("output does not explain the missing repository:\n%s", out.String())
	}
}

func TestAcceptsRepositoryAsThirdArgument(t *testing.T) {
	r := run(t, stub{
		releaseRows: []string{""},
		tagRows:     []string{""},
	}, "1.2.3", "false", "someone/elsewhere")

	if r.exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", r.exit, r.output())
	}
	for _, c := range r.calls {
		if !strings.Contains(c, "someone/elsewhere") {
			t.Errorf("call did not target the given repository: %q", c)
		}
	}
}
