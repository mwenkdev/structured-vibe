# Specification: Release Rollback Coverage

Bead: `sv-29u`
Status: revision 4 — third-round review finding adjudicated, ready for
decomposition
Authoritative for release mechanics: `docs/specs/releasing.md`. This
specification amends it; where the two disagree, `releasing.md` must be updated
rather than reinterpreted.

Revision 3 preserved the revision 1 adjudications and resolved the second-round
`sv-review` findings: R-5 rollback-script availability, R-6 timeout-probe
procedure, R-7 disagreement-warning ownership and coverage, and R-8 queued-run
cancellation.

Revision 4 resolves third-round finding R-9. Revision 3 justified the rollback
script bootstrap with a privilege boundary between `github.workflow_sha` and
`github.sha`. That boundary does not exist: for a `release: published` event
both resolve to the tagged commit. The checkout-independent bootstrap is
retained because it is functionally necessary, but its rationale and the M2
check that encoded the false distinction are corrected below. R-10 (naming the
log evidence for M3's bootstrap validation) was not adopted in this revision.

## Purpose

`docs/specs/releasing.md` states the release invariant:

> A release/tag should exist only if the entire release pipeline completes
> successfully.

`.github/workflows/release.yml` enforces it with a single rollback step guarded
by `if: failure()`. GitHub Actions does not run `failure()` steps when a run is
cancelled, and a job that exceeds its timeout is cancelled rather than failed.
A manually cancelled release, a timed-out release, or an interrupted runner
therefore leaves the GitHub release and its tag published with no successful
pipeline behind them — exactly the state the invariant forbids.

This specification closes the cancellation and timeout cases inside the
workflow, makes the rollback itself deterministic and testable, proves the
widened guard actually fires, and records both the failure modes that remain
outside a workflow's reach and the human duty that covers them.

## Scope

### Included

- Rollback coverage for completed non-success conclusions the workflow can
  observe: step failure, manual cancellation, and job timeout.
- A deterministic, idempotent rollback that tolerates partial state and reports
  exactly what it could not remove.
- Automated tests for the rollback behavior, runnable under `make check`.
- Empirical validation that the widened guard fires under real cancellation and
  real timeout (D7).
- A prerelease exemption (D4) and its documentation.
- A documented human post-release verification duty (D8).
- Amendments to `docs/specs/releasing.md`, `docs/specs/architecture.md`,
  `AGENTS.md`, and the `release.yml` header comment covering the above and the
  accepted residual risks.

### Explicitly excluded

- A scheduled reconciliation workflow. Human decision 2026-09-12 selected the
  in-workflow fix; D5 records the policy a reconciler must follow **if** one is
  ever built, so the decision does not have to be relitigated.
- Draft-until-verified release initiation. It was considered and rejected
  (D1 alternatives) because it changes the documented human release ritual.
- Any change to the exact-commit contract, the prerequisite CI check, the
  artifact set, or the checksum contract.
- Retry tags, moving an existing tag, per-asset cleanup, and recursive repair
  of a failed rollback. `releasing.md` forbids all four and this work does not
  revisit them.
- Marking or annotating a retained prerelease. Human decision 2026-09-12
  (D4): retention is reported in the workflow log only.

## Decisions

### D1 — Keep compensating rollback; widen the guard to every observable non-success

The rollback step guard becomes `failure() || cancelled()`, and the job gains
an explicit `timeout-minutes` so a hung pipeline reaches a decision point long
before the six-hour default.

This keeps the existing publish-then-compensate model. The human creates a real
release, the workflow validates and packages it, and a non-success conclusion
removes it.

**Alternative considered: draft-until-verified.** The human would create the
release as a draft, which creates no git tag, and the workflow would publish it
only on success. That makes the invariant structural rather than compensating:
a cancelled run, a dead runner, or a workflow that never triggers would leave
an invisible draft and no tag, so there would be nothing to roll back. It was
rejected because it rewrites the release-initiation ritual in `releasing.md`
and moves the build reference from the tag to `target_commitish`, which is a
larger change than this bug warrants. It remains the strongest available design
if the residual risks in D6 ever become real incidents.

**Alternative considered: scheduled reconciler.** Rejected for now as
disproportionate; see D5.

### D2 — Rollback is idempotent and verifies the end state

`gh release delete --cleanup-tag` is a single call that assumes both the
release and the tag exist and that deleting them is all-or-nothing. A rerun, a
partially completed earlier rollback, or a tag deleted by hand turns that
assumption into a spurious failure, and a partial success is currently reported
as success.

Rollback therefore:

1. deletes the release if it exists, tolerating "already absent";
2. deletes the tag if it exists, tolerating "already absent";
3. re-checks both and succeeds only when neither remains;
4. on any remaining object, fails loudly naming the release, the tag, and which
   one survived.

The goal is an operation that is safe to run twice and honest about what it
left behind. It remains simple: no per-asset cleanup, no retries against a
failing API, no repair of a failed rollback.

### D3 — Rollback logic lives in a tested script, not in workflow YAML

The rollback becomes a shell script in the repository that the workflow calls.
It resolves the `gh` executable through an overridable variable so tests can
supply a stub.

Workflow YAML is unreachable by `go test` and can only be exercised by cutting
real releases, which is precisely what must not be used as a test bed. A script
with an injectable `gh` makes the failure matrix — release present, release
absent, tag absent, both absent, delete failure, prerelease — assertable in
ordinary CI.

The workflow keeps only the guard, a minimal bootstrap that obtains the script,
and the call.

#### Script availability before checkout

The rollback must not depend on the pipeline checkout. Cancellation can happen
before checkout completes, and version validation can fail before checkout even
starts. In either case a repository script expected in the workspace is absent,
which would turn rollback into a file-not-found failure and leave the release
and tag behind.

The rollback step therefore fetches the script through the GitHub API into
runner temporary storage before invoking it, and runs it from there.

**Which commit to fetch from is arbitrary.** The bootstrap uses
`github.workflow_sha`, but `github.sha` would fetch identical bytes: for a
`release: published` event GitHub executes the workflow definition from the
commit the release tag points at, so both contexts resolve to that same tagged
commit. There is no second, more-trusted revision available to this trigger.
`github.workflow_sha` is chosen only because its name states the intent — the
revision containing the workflow and the script it calls — and because if a
future trigger ever separates the two, the script that matches the running YAML
is the correct one to execute. Nothing about the choice is load-bearing.

**Trust model.** Release creation requires write access to the repository, and
the workflow inherently executes the tagged commit's own definition. The
rollback script is therefore no more privileged than the workflow that calls
it: whoever can trigger this pipeline can already supply the YAML it runs. The
requirement that matters is not which of two identical SHAs is used, but that
the bootstrap is **checkout-independent** and pins an **immutable commit SHA**
rather than a movable reference such as the release tag name or a branch. A
movable ref could be repointed between trigger and rollback; a SHA cannot.

Failure to obtain or execute the script is a rollback failure and is reported
as such. The workflow-level check in M2 protects those two bootstrap properties;
ordinary rollback behavior remains tested at the script boundary in M1.

#### Script inputs

The script takes the release version and the prerelease flag as explicit
arguments. It does not infer either from workflow context, so tests can drive
every combination.

The workflow passes `github.event.release.tag_name` **directly** rather than
`steps.release.outputs.version`. The step output does not exist if the run is
cancelled before the "Identify release" step completes — which is precisely the
scenario D1 exists to cover — and an empty version would otherwise reach the
rollback in the most important case. The script rejects an empty or missing
version argument with a loud failure rather than proceeding.

### D4 — Prereleases are exempt from automatic rollback

Human decision 2026-09-12. A failed or cancelled pipeline for a prerelease
leaves the prerelease and its tag in place; the workflow reports the condition
instead of deleting.

The reasoning is that a prerelease is explicitly provisional, `gh release list`
and the installer's "latest" selection both exclude prereleases by default, and
a human experimenting with `2.0.0-beta.1` should not have their artifact
removed by automation.

#### The authoritative prerelease signal

The exemption keys on the **GitHub `prerelease` flag**
(`github.event.release.prerelease`), not on the SemVer prerelease suffix.

The flag is the correct signal because D4's entire justification — exclusion
from `gh release list` and from the installer's "latest" selection — is a
property of the flag, not of the version string. The two can disagree: a
`2.0.0-beta.1` release created without the flag is treated by GitHub as a full
release, and a `1.2.3` release can carry the flag.

When the flag and the SemVer suffix disagree, **the flag decides** and the
rollback script emits a warning naming both values. A flagged `1.2.3` is
retained; an unflagged `2.0.0-beta.1` is rolled back like any other full
release. Silently preferring either signal would surprise someone, so the
disagreement is made visible instead. Keeping this decision in the script makes
both disagreement directions testable rather than embedding semantic logic in
workflow YAML.

#### Retained assets

Human decision 2026-09-12: a retained prerelease keeps whatever assets were
uploaded before the non-success conclusion. The rollback is a complete no-op
for prereleases — no release deletion, no tag deletion, and no per-asset
cleanup. A prerelease may therefore carry a partial or missing asset set, and
verifying it is the installing human's responsibility.

#### No marking

Human decision 2026-09-12: a retained prerelease is **not** annotated. The
workflow reports the retention in its own log and does not mutate release
notes, titles, or labels. Keeping the workflow out of release content is worth
more than distinguishing an abandoned prerelease from a verified one on the
releases page, because the Actions run already records which is which.

#### The invariant narrows

**This narrows the release invariant, and the narrowing must be stated, not
implied.** `releasing.md` currently asserts the invariant for releases without
qualification, and so do `architecture.md`, `AGENTS.md`, and the `release.yml`
header comment. All four must be amended to say that the transactional guarantee
applies to full releases, or defer to `releasing.md` rather than restating the
rule. A prerelease may exist without a successful pipeline and therefore carries
no completeness guarantee — including the possibility of missing or partial
assets. A consumer who installs a prerelease by explicit version is responsible
for verifying it.

The exemption applies to the automatic deletion only. Prerelease runs still
execute the full pipeline, still verify the exact commit and prerequisite CI,
and still upload assets on success.

### D5 — Recorded policy for a future reconciler, which is not built now

Human decision 2026-09-12: if a reconciliation path is ever added, it deletes
the offending release and tag automatically rather than alerting a human or
quarantining the release.

Nothing in this specification builds that path. The decision is recorded so the
question is already settled if D6's residual risks justify the work later. A
reconciler would need its own specification covering at minimum: the
authoritative "this release was verified" signal, the grace window that
distinguishes an in-flight release from an abandoned one, and its interaction
with the D4 prerelease exemption.

### D6 — Residual risks are accepted and documented, not silently carried

The in-workflow fix cannot guarantee cleanup when no workflow step runs or the
runner disappears before cleanup completes:

- the runner is lost or the infrastructure fails mid-run;
- the cancellation grace period expires before the rollback step completes;
- the run is cancelled while queued, before any step can execute;
- the workflow never triggers, because Actions are disabled for the repository
  or `release.yml` is absent or invalid on the triggering ref.

In each case a full release and tag can survive without a successful pipeline.
`releasing.md` must name these cases and state the manual remedy: delete the
release and tag, then create the release again. They are accepted because each
requires an infrastructure failure, cancellation before work begins, or a
deliberate repository misconfiguration, and because D1's alternative already
exists if that judgment proves wrong.

D8 assigns the human duty that detects these cases.

### D7 — Prove the widened guard actually fires

Resolves review finding R-1.

D1's correctness depends on a runtime assumption: that a `cancelled()`-guarded
step runs both when a run is cancelled by hand and when a job exceeds
`timeout-minutes`. GitHub's workflow-cancellation reference documents the
manual-cancellation path — the server re-evaluates `if` conditions for
unfinished steps, and steps whose condition is true continue to run within a
five-minute termination window — but it does **not** state that job-level
timeout follows the same path. A static check that the guard is present in YAML
cannot detect a guard that never fires, so without this decision the timeout
half of the bug could remain unfixed while every acceptance criterion appears
satisfied.

The assumption is therefore validated empirically, once, against real GitHub
infrastructure, and the observed conclusions are recorded in `releasing.md`.

D4 makes this cheap and safe. A prerelease is exempt from deletion, so a
throwaway prerelease can be used as the test subject without risking any
published state: the rollback step runs, finds the prerelease flag, and reports
the exemption rather than deleting anything. The observable evidence is that
**the step executed at all**.

Two runs are required:

1. a prerelease run cancelled by hand before the normal checkout completes;
2. a prerelease run that exceeds the production job `timeout-minutes`.

In both, the rollback step must appear as executed and must report the
prerelease exemption.

The timeout probe must not temporarily edit `release.yml` on `main` or weaken
the production timeout. It uses a throwaway branch based on the reviewed
implementation commit. A probe-only commit makes one checked-out build command
block only when `GITHUB_EVENT_NAME=release`, while behaving normally in CI. The
CI workflow is dispatched explicitly for that branch and must succeed for the
probe commit's exact SHA before a throwaway prerelease is created targeting it.
The release then blocks in the probe until the production job timeout is
reached. The probe commit is never merged.

**If the timeout run does not reach the step**, the job-level timeout does not
support `cancelled()` and D1's mechanism is insufficient on its own. The
fallback is to add step-level `timeout-minutes` to the long-running steps —
build, validation, packaging, upload — so that exceeding a bound produces an
ordinary step failure, which `failure()` already covers. The fallback is
specified now so the discovery does not require re-planning.

The throwaway prerelease, tag, and probe branch are deleted by hand afterwards.
That deletion is expected, not a rollback failure.

### D8 — The human verifies the release landed clean

Human decision 2026-09-12. Automation cannot close D6's residual risks, and the
D4 exemption means a retained prerelease is indistinguishable on the releases
page from a verified one. The remaining gap is covered by an explicit, written
human duty rather than left implicit.

After creating a release, the human who created it is responsible for
confirming that the release transaction actually settled:

1. the release workflow run reached a conclusion, rather than still running or
   disappearing without one;
2. on success, the release and tag exist and carry the expected asset set and
   checksums;
3. on any non-success conclusion, the release and tag are **gone** — and if
   they are not, for any of D6's reasons, the human deletes them by hand and
   then recreates the release after fixing the cause;
4. for a prerelease, that retention was intended and that the asset set is
   whatever the human expects, since D4 rolls nothing back and permits a
   partial set.

This duty is the documented compensating control for D6, and it belongs in
`releasing.md` beside the invariant it protects — not in tribal memory. A
release is not "done" when the workflow is dispatched; it is done when the
human has confirmed the end state.

## Constraints

- `docs/specs/releasing.md` stays authoritative and must not contradict shipped
  behavior. Every decision above that changes stated behavior lands as an
  amendment in the same change.
- The rollback stays simple: no per-asset cleanup, no alternate retry tags, no
  moving an existing release tag, no release branches to repair transaction
  state, and no recursive repair of a failed rollback.
- Exact-commit behavior is unchanged: the workflow continues to build the
  commit the tag points at and to require a successful CI run for that exact
  SHA.
- Release immutability stays disabled, because it would prevent rollback.
- The rollback must never delete a release it was not asked to roll back. It
  operates on the triggering release's version only.
- The workflow must never mutate release content: no editing notes, titles, or
  labels (D4).
- Tests must not call the real GitHub API or require network access.
- `make check` remains the validation contract and must pass.
- D7's empirical validation uses a throwaway prerelease only. It must never be
  run against a real release version, modify the production timeout temporarily,
  or merge the timeout-probe commit.

## Milestones

### M1 — Testable rollback

**Dependencies:** none.

**Testable outcomes:**

- A rollback script exists in the repository, takes the release version and the
  prerelease flag as explicit arguments, and resolves `gh` through an
  overridable variable.
- It rejects an empty or missing version argument with a non-zero exit and a
  message naming the problem (D3).
- It implements D2: deletes what exists, tolerates what is already absent,
  re-checks the end state, and fails naming any surviving object.
- It is a complete no-op when the prerelease flag is set — no release deletion,
  no tag deletion, no asset cleanup — and reports that it was exempt (D4).
- It emits a warning naming the version and flag when their prerelease signals
  disagree; tests cover both a flagged full SemVer (`1.2.3`) and an unflagged
  prerelease SemVer (`2.0.0-beta.1`), and verify that the flag wins (D4).
- Automated tests drive it with a stub `gh` and cover: release and tag both
  present; release already deleted; tag already deleted; both already absent;
  release deletion failing; tag deletion failing; the end-state re-check
  catching a surviving object; the prerelease exemption leaving both the
  release and its assets untouched; and an empty version argument.
- The tests run under `make check` and require no network.

### M2 — Workflow coverage

**Dependencies:** M1.

**Testable outcomes:**

- The rollback step guard is `failure() || cancelled()`, so cancellation and
  timeout reach it.
- The job declares an explicit `timeout-minutes`, and the rollback step
  declares its own shorter timeout so it cannot hang past the cancellation
  grace period.
- The workflow calls the M1 script rather than embedding rollback logic, and
  passes `github.event.release.tag_name` and `github.event.release.prerelease`
  directly rather than through step outputs (D3, D4).
- The rollback step obtains the script from the repository contents API
  independently of the pipeline checkout, pinned to an immutable commit SHA
  (`github.workflow_sha`), then invokes it from runner temporary storage (D3).
- A workflow-level check asserts the guard, the timeouts, and the direct event
  inputs are present, and asserts that the script bootstrap is
  checkout-independent and pins an immutable commit SHA rather than a movable
  reference such as the release tag name or a branch. It does **not** assert
  `github.workflow_sha` over `github.sha`: the two are the same commit for this
  trigger, so requiring one over the other would encode a distinction with no
  runtime effect (D3, R-9). The check exists so a later edit cannot silently
  reintroduce `if: failure()` alone, reroute the version through a step output,
  make rollback depend on checkout, or fetch the script from a ref that can
  move. Failing that check fails CI.
- The prerelease exemption is observable in the workflow's own reporting: a
  cancelled prerelease run states that the prerelease was retained.
- The `release.yml` header comment no longer states the invariant without the
  D4 qualification (R-4).

### M3 — Empirical guard validation

**Dependencies:** M2.

**Testable outcomes:**

- A throwaway prerelease run cancelled by hand before checkout completes shows
  the rollback step as executed and reporting the prerelease exemption. This
  validates R-5's independent script bootstrap, not merely the guard.
- A throwaway prerelease run using D7's unmerged, release-only blocking probe
  exceeds the production job `timeout-minutes` and shows the same.
- If the timeout run does not reach the step, step-level `timeout-minutes` are
  added to the long-running steps per D7's fallback, and the timeout case is
  re-validated until a non-success conclusion demonstrably reaches rollback.
- The observed conclusions for both runs — including which guard fired — are
  recorded in `releasing.md`, so the next person does not have to rediscover
  GitHub's behavior.
- The throwaway prerelease, tag, and probe branch are deleted afterwards,
  leaving no residue and no probe commit on `main`.

### M4 — Documentation alignment

**Dependencies:** M3.

**Testable outcomes:**

- `docs/specs/releasing.md` states that rollback covers failure, cancellation,
  and timeout, and records the M3 observations that prove it.
- It states the D4 prerelease exemption explicitly, including that the GitHub
  prerelease flag is the authoritative signal, that a prerelease may exist
  without a successful pipeline, that it may carry incomplete assets, and that
  it is not marked in any way.
- It records the D6 residual risks and the manual remedy.
- It contains the D8 human post-release verification duty as a short, ordered
  checklist a releaser can follow, positioned with the release invariant.
- It notes that rollback is idempotent and verifies its end state.
- `AGENTS.md`'s statement of the release invariant carries the D4
  qualification or defers to `releasing.md` instead of restating it (R-4).
- `docs/specs/architecture.md`'s transactional-release statements carry the D4
  qualification or defer to `releasing.md` rather than restating it.
- No statement in `releasing.md`, `architecture.md`, `AGENTS.md`, or
  `release.yml` contradicts shipped behavior after the change.

## Open questions

None. Both questions carried by revision 1 were resolved by human decision on
2026-09-12 and are recorded in D4: a retained prerelease keeps whatever assets
it has, and it is not marked.

## Assumptions

- GitHub runs steps guarded by `cancelled()` during the cancellation grace
  period, and that period is long enough for the script bootstrap and rollback
  API calls. M2's step timeout bounds the attempt, M3 validates the assumption
  against real infrastructure rather than trusting it, and D6 accepts the case
  where the grace period still expires first.
- A job that exceeds `timeout-minutes` is reported as cancelled rather than
  failed, which is why D1 needs both conditions rather than just `failure()`.
  This is the assumption M3 exists to test; D7 specifies the fallback if it is
  false.
- `gh release delete` and `gh api` distinguish "not found" from other errors
  clearly enough for D2's tolerate-absent behavior to be implemented without
  parsing human-readable message text. M1 verifies this against the stub
  contract it defines.
- `github.event.release.prerelease` is present and accurate on the
  `release: published` payload for both flagged and unflagged releases.
- `github.workflow_sha` and `github.sha` both resolve to the tagged commit on a
  `release: published` event, so the bootstrap ref choice is arbitrary and
  neither is more trusted than the other (D3). Both are immutable commit SHAs,
  which is the property the bootstrap actually relies on. If the release tag is
  later deleted — by rollback or by hand — the commit either SHA names may
  become unreachable, so the bootstrap is not assumed to work *after* a
  completed rollback; it only needs to work during one.
- The repository contents API serves a file at a commit SHA without a prior
  checkout, using the job's `github.token`. The M3 early-cancellation run
  validates this against real infrastructure.
- The one existing release (`0.1.0`) is a full release, so D4's exemption has
  no retroactive effect on published state.
