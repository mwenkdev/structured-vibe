// Workflow-level regression check for .github/workflows/release.yml.
//
// scripts/rollback-release.sh is covered by rollback_test.go, but the script is
// only reached if the workflow calls it correctly, and the workflow can only be
// exercised for real by cutting releases. The properties below are therefore
// asserted statically so a later edit cannot silently reintroduce `if:
// failure()` alone, drop a timeout, reroute the rollback inputs through a step
// output, make rollback depend on the pipeline checkout, or fetch the script
// from a reference that can move
// (docs/specs/release-rollback-coverage.md, milestone M2).
//
// What this check deliberately does NOT assert is github.workflow_sha over
// github.sha. For a `release: published` event both resolve to the tagged
// commit, so requiring one over the other would encode a distinction with no
// runtime effect (D3, R-9). Either is accepted; a movable reference is not.
package release

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// rollbackScriptPath is the repository path of the script the workflow must
// call rather than embedding rollback logic in YAML (D3).
const rollbackScriptPath = "scripts/rollback-release.sh"

// immutableRefContexts resolve to a commit SHA on a release: published event.
// Both are accepted on purpose; see the package comment above.
var immutableRefContexts = []string{"github.workflow_sha", "github.sha"}

// requiredEventInputs must reach the rollback step straight from the release
// event. A run cancelled before the "Identify release" step completes has no
// step output to read, which is precisely the case rollback exists to cover.
var requiredEventInputs = []string{
	"github.event.release.tag_name",
	"github.event.release.prerelease",
}

type workflowStep struct {
	Name           string            `yaml:"name"`
	If             string            `yaml:"if"`
	TimeoutMinutes *int              `yaml:"timeout-minutes"`
	Env            map[string]string `yaml:"env"`
	Run            string            `yaml:"run"`
}

type workflowJob struct {
	TimeoutMinutes *int           `yaml:"timeout-minutes"`
	Steps          []workflowStep `yaml:"steps"`
}

type releaseWorkflow struct {
	Jobs map[string]workflowJob `yaml:"jobs"`
}

// contentsRefRe extracts the ref the contents-API fetch is pinned to.
var contentsRefRe = regexp.MustCompile(
	`contents/` + regexp.QuoteMeta(rollbackScriptPath) + `\?ref=([^"'\s&]+)`)

// expressionRe extracts the context inside a ${{ ... }} expression.
var expressionRe = regexp.MustCompile(`\$\{\{\s*(.+?)\s*\}\}`)

// workspaceInvocationRe matches invoking the script from the checked-out
// workspace, which is the checkout dependency the bootstrap exists to avoid.
var workspaceInvocationRe = regexp.MustCompile(
	`(^|\s)(\./|\$GITHUB_WORKSPACE/|\$\{GITHUB_WORKSPACE\}/)` +
		regexp.QuoteMeta(rollbackScriptPath))

// checkReleaseWorkflow returns one violation per property the release workflow
// fails to preserve. An empty result means the workflow is intact.
//
// Violations are prefixed with a stable code so the tests below can assert that
// a specific mutation trips the specific check meant to catch it.
func checkReleaseWorkflow(data []byte) []string {
	var wf releaseWorkflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return []string{fmt.Sprintf("parse: release workflow is not valid YAML: %v", err)}
	}

	var violations []string
	var rollback *workflowStep
	var owningJob workflowJob
	found := 0

	for _, job := range wf.Jobs {
		for i, step := range job.Steps {
			if strings.Contains(step.Run, rollbackScriptPath) {
				found++
				rollback = &job.Steps[i]
				owningJob = job
			}
		}
	}

	if found == 0 {
		return []string{
			"script: no step invokes " + rollbackScriptPath +
				"; rollback logic must live in the tested script, not in workflow YAML (D3)",
		}
	}
	if found > 1 {
		violations = append(violations, fmt.Sprintf(
			"script: %d steps invoke %s; expected exactly one rollback step",
			found, rollbackScriptPath))
	}

	violations = append(violations, checkGuard(*rollback)...)
	violations = append(violations, checkTimeouts(owningJob, *rollback)...)
	violations = append(violations, checkInputs(*rollback)...)
	violations = append(violations, checkBootstrap(*rollback)...)

	return violations
}

// checkGuard enforces D1: cancellation and timeout must reach rollback.
func checkGuard(step workflowStep) []string {
	var violations []string
	for _, condition := range []string{"failure()", "cancelled()"} {
		if !strings.Contains(step.If, condition) {
			violations = append(violations, fmt.Sprintf(
				"guard: rollback step guard %q does not cover %s; "+
					"cancellation and job timeout would not reach rollback (D1)",
				step.If, condition))
		}
	}
	return violations
}

// checkTimeouts enforces the explicit job bound and the shorter step bound.
func checkTimeouts(job workflowJob, step workflowStep) []string {
	var violations []string

	if job.TimeoutMinutes == nil {
		violations = append(violations,
			"timeout: the release job declares no timeout-minutes; a hung pipeline "+
				"would run until the six-hour default (D1)")
	}
	if step.TimeoutMinutes == nil {
		violations = append(violations,
			"timeout: the rollback step declares no timeout-minutes; it could hang "+
				"past the cancellation grace period")
		return violations
	}
	if job.TimeoutMinutes != nil && *step.TimeoutMinutes >= *job.TimeoutMinutes {
		violations = append(violations, fmt.Sprintf(
			"timeout: rollback step timeout (%d) is not shorter than the job timeout (%d)",
			*step.TimeoutMinutes, *job.TimeoutMinutes))
	}
	return violations
}

// checkInputs enforces D3: rollback inputs come from the release event
// directly, never through a step output.
func checkInputs(step workflowStep) []string {
	var violations []string
	surface := step.Run
	for _, v := range step.Env {
		surface += "\n" + v
	}

	for _, input := range requiredEventInputs {
		if !strings.Contains(surface, input) {
			violations = append(violations, fmt.Sprintf(
				"inputs: rollback step does not read %s directly from the release event (D3, D4)",
				input))
		}
	}

	if strings.Contains(surface, "steps.") {
		violations = append(violations,
			"inputs: rollback step reads a step output; a run cancelled before that "+
				"step completes would roll back an empty version (D3)")
	}
	return violations
}

// checkBootstrap enforces that the script is obtained independently of the
// pipeline checkout and pinned to an immutable commit SHA.
func checkBootstrap(step workflowStep) []string {
	var violations []string

	if workspaceInvocationRe.MatchString(step.Run) {
		violations = append(violations,
			"bootstrap: rollback invokes the script from the workspace; a run cancelled "+
				"before checkout completes would find no script (D3)")
	}

	if !strings.Contains(step.Run, "RUNNER_TEMP") {
		violations = append(violations,
			"bootstrap: rollback does not run the script from runner temporary storage; "+
				"the bootstrap must not depend on the pipeline checkout (D3)")
	}

	match := contentsRefRe.FindStringSubmatch(step.Run)
	if match == nil {
		violations = append(violations,
			"bootstrap: rollback does not fetch "+rollbackScriptPath+
				" from the repository contents API pinned to a ref (D3)")
		return violations
	}

	ref, ok := resolveRef(match[1], step.Env)
	if !ok {
		violations = append(violations, fmt.Sprintf(
			"bootstrap: script is fetched from %q, which is not a workflow context; "+
				"the bootstrap must pin an immutable commit SHA (D3)", match[1]))
		return violations
	}

	for _, allowed := range immutableRefContexts {
		if ref == allowed {
			return violations
		}
	}

	violations = append(violations, fmt.Sprintf(
		"bootstrap: script is fetched at %q, which is not an immutable commit SHA; "+
			"a movable reference could be repointed between trigger and rollback (D3). "+
			"Use one of %v -- either is equally acceptable.",
		ref, immutableRefContexts))
	return violations
}

// resolveRef reduces the ref used by the contents fetch to the workflow context
// behind it, following one level of shell-variable indirection through env.
func resolveRef(raw string, env map[string]string) (string, bool) {
	if match := expressionRe.FindStringSubmatch(raw); match != nil {
		return match[1], true
	}

	name := strings.TrimPrefix(raw, "$")
	name = strings.TrimPrefix(name, "{")
	name = strings.TrimSuffix(name, "}")

	value, ok := env[name]
	if !ok {
		return "", false
	}
	if match := expressionRe.FindStringSubmatch(value); match != nil {
		return match[1], true
	}
	return value, true
}

func releaseWorkflowPath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatalf("resolving workflow path: %v", err)
	}
	return p
}

func readReleaseWorkflow(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(releaseWorkflowPath(t))
	if err != nil {
		t.Fatalf("reading release workflow: %v", err)
	}
	return string(data)
}

// mustReplace fails the test when old is absent, so a mutation can never
// silently become a no-op and report a false pass.
func mustReplace(t *testing.T, source, old, replacement string) string {
	t.Helper()
	if !strings.Contains(source, old) {
		t.Fatalf("mutation target not found in release.yml: %q", old)
	}
	return strings.Replace(source, old, replacement, 1)
}

// TestReleaseWorkflowPreservesRollbackContract is the check itself: the shipped
// workflow must satisfy every property. Failing it fails CI.
func TestReleaseWorkflowPreservesRollbackContract(t *testing.T) {
	violations := checkReleaseWorkflow([]byte(readReleaseWorkflow(t)))
	for _, v := range violations {
		t.Errorf("release workflow regression: %s", v)
	}
}

// TestReleaseWorkflowCheckDetectsRegressions proves the check has teeth. Each
// case removes one required property and expects the matching violation.
func TestReleaseWorkflowCheckDetectsRegressions(t *testing.T) {
	cases := []struct {
		name     string
		old      string
		new      string
		wantCode string
	}{
		{
			name:     "guard narrowed back to failure only",
			old:      "if: failure() || cancelled()",
			new:      "if: failure()",
			wantCode: "guard:",
		},
		{
			name:     "job timeout removed",
			old:      "    timeout-minutes: 30\n",
			new:      "",
			wantCode: "timeout:",
		},
		{
			name:     "rollback step timeout removed",
			old:      "        timeout-minutes: 3\n",
			new:      "",
			wantCode: "timeout:",
		},
		{
			name:     "rollback step timeout no longer shorter than the job timeout",
			old:      "        timeout-minutes: 3\n",
			new:      "        timeout-minutes: 45\n",
			wantCode: "timeout:",
		},
		{
			name:     "version rerouted through a step output",
			old:      "VERSION: ${{ github.event.release.tag_name }}",
			new:      "VERSION: ${{ steps.release.outputs.version }}",
			wantCode: "inputs:",
		},
		{
			name:     "prerelease flag no longer passed",
			old:      "PRERELEASE: ${{ github.event.release.prerelease }}",
			new:      "PRERELEASE: \"false\"",
			wantCode: "inputs:",
		},
		{
			name:     "bootstrap pinned to the movable release tag",
			old:      "SCRIPT_SHA: ${{ github.workflow_sha }}",
			new:      "SCRIPT_SHA: ${{ github.event.release.tag_name }}",
			wantCode: "bootstrap:",
		},
		{
			name:     "bootstrap pinned inline to a branch",
			old:      "?ref=$SCRIPT_SHA",
			new:      "?ref=main",
			wantCode: "bootstrap:",
		},
		{
			name:     "bootstrap made dependent on the checkout",
			old:      `script="$RUNNER_TEMP/rollback-release.sh"`,
			new:      `script="scripts/rollback-release.sh"`,
			wantCode: "bootstrap:",
		},
	}

	original := readReleaseWorkflow(t)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mutated := mustReplace(t, original, tc.old, tc.new)

			violations := checkReleaseWorkflow([]byte(mutated))
			if len(violations) == 0 {
				t.Fatalf("mutation went undetected; the check would not catch this regression")
			}

			for _, v := range violations {
				if strings.HasPrefix(v, tc.wantCode) {
					return
				}
			}
			t.Fatalf("no %s violation reported; got %v", tc.wantCode, violations)
		})
	}
}

// TestReleaseWorkflowCheckAcceptsEitherImmutableSHA pins R-9: github.sha and
// github.workflow_sha are the same commit for this trigger, so the check must
// not prefer one over the other. Without this test a future edit could quietly
// turn the check into an assertion about which context is used.
func TestReleaseWorkflowCheckAcceptsEitherImmutableSHA(t *testing.T) {
	original := readReleaseWorkflow(t)

	swapped := mustReplace(t, original,
		"SCRIPT_SHA: ${{ github.workflow_sha }}",
		"SCRIPT_SHA: ${{ github.sha }}")

	if violations := checkReleaseWorkflow([]byte(swapped)); len(violations) != 0 {
		t.Fatalf("github.sha rejected, but it is the same commit as github.workflow_sha "+
			"for a release: published event: %v", violations)
	}
}

// TestReleaseWorkflowCheckRequiresTheScriptCall covers the case the mutation
// table cannot express: rollback logic inlined back into YAML.
func TestReleaseWorkflowCheckRequiresTheScriptCall(t *testing.T) {
	inlined := []byte(`
jobs:
  release:
    timeout-minutes: 30
    steps:
      - name: Roll back release and tag
        if: failure() || cancelled()
        timeout-minutes: 3
        run: gh release delete "$VERSION" --yes --cleanup-tag
`)

	violations := checkReleaseWorkflow(inlined)
	if len(violations) != 1 || !strings.HasPrefix(violations[0], "script:") {
		t.Fatalf("inlined rollback logic not reported: %v", violations)
	}
}
