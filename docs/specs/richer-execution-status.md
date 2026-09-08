# Specification: Richer Execution Status

Bead: `sv-08e`
Status: revision 4 — findings R-01..R-23 addressed; design settled, review rounds closed; ready for sv-beads
Roadmap: `docs/ROADMAP.md` "Richer execution status", under "Future orchestration capabilities"

## Purpose

Make the execution state of an epic directly observable instead of inferred.

Today a human who wants to know how an epic is going must run several `bd`
queries and assemble the answer in their head, or ask a model to do it and
accept a non-reproducible narrative. Both consume human attention on
task-state tracking, which `structured-vibe-spec.md:341` names explicitly as
work the system should reduce.

This specification defines a deterministic, read-only projection over the
Beads work graph, surfaced as a CLI command and wrapped by a thin skill.

## Scope

### Included

- A new read-only CLI command, `svibe progress <bead-id>`, reporting for one
  bead's subtree: structural progress, per-member state, verification
  coverage, lifecycle-integrity anomalies, and the structural reason no work
  is available.
- A Beads convention for recording verification outcomes, written by
  `sv-verify` and read by the command.
- A thin core skill, `sv-progress`.

### Explicitly excluded

- **Semantic blocker diagnosis.** Belongs to `sv-gx4`.
- **Autonomous-run stop reasons.** Requires `sv-bpq`.
- **Any change to `svibe status`.** Sync freshness keeps its output verbatim.
- **Any new persistent store owned by Structured Vibe.**
- **Cross-epic dashboards.** One subtree per invocation.
- **Mutation by the command.** `sv-verify` writes verification state; the
  command only reads it.
- **A second readiness graph.** Beads readiness is authoritative and is
  *consumed*, never recomputed. (Reviews R-02, R-10.)

## Decisions

### D1 — Delivery is a CLI primitive plus a thin skill

`svibe` gains a read-only command that derives and emits the projection. A
thin core skill wraps it for conversational use.

**Alternatives considered.** *Skill only* — rejected: model-regenerated
output is non-deterministic and unscriptable (`PRINCIPLES.md:15`, roadmap
`sv-vzf`). *Extend `svibe status`* — rejected on the naming hazard; snapshot
freshness and execution progress are unrelated. *CLI only* — rejected; the
primary consumer is a human in a host conversation.

**Boundary check.** `architecture.md:162` forbids commands that *run the
workflow*; `architecture.md:159` lists "status" as legitimate CLI
infrastructure. A read-only projection is a status query. The guard test at
`internal/cli/cli_test.go:377` stays passing and unmodified.

### D2 — Beads remains the sole source of work state

Every field is derived from `bd` at invocation time. Nothing is cached,
persisted, or reconciled. (`spec.md:279-291`, `PRINCIPLES.md:18-24`,
`architecture.md:54, 1051`)

### D3 — `bd` is a hard requirement for this command only

The command fails with a clear diagnostic when `bd` is absent or unusable.
`init`, `sync`, `resolve`, `status`, `advise`, `validate`, `admin`, and
`version` continue to work with no `bd` installed.

### D4 — Capability contract, not a version floor

All reads go through documented `bd` subcommands with `--json`. Structured
Vibe never opens the Dolt database directly. The implementation enumerates
the *bd read contract* (below) in one place, validates responses structurally
at runtime — a missing required field or undecodable payload produces a
diagnostic naming the subcommand and observed `bd version`, never a silently
wrong answer — and asserts no minimum version. `bd 1.1.2` is the observed
baseline, recorded as fact. (Review R-08.)

### D5 — Verification convention: commit metadata, then state dimension

**Decision.** `sv-verify`, upon completing verification and *before* the bead
is closed, records **in this order** (review R-14):

1. `bd update <id> --set-metadata verification_commit=<sha>` — the verified
   commit. Confirmed to replace the named key while preserving unrelated
   metadata, and returned in bulk by `bd list --json`.
2. `bd set-state <id> verification=pass|fail --reason "<summary>"` — the
   outcome.

**Order matters.** Writing the binding first means an interruption leaves a
commit with no outcome, which reads as *unverified* — the safe, truthful
failure. The reverse order could publish an authoritative-looking `pass` with
no traceable implementation, violating `spec.md:836-846`. A recorded outcome
*without* commit metadata is reported as a `verification-without-commit`
anomaly rather than presented as a normal verified result.

**Read-back channel.** (Review R-21.) `bd set-state` materialises the outcome
as the label `verification:<value>` in each member's `labels[]` array,
overwritten in place on re-record; `--set-metadata` materialises the commit
under `metadata.verification_commit`. Both are returned in bulk by the single
member-list call, which is why the read contract consumes `labels` and
`metadata`. The projection reads only these two fields; it never reads event
beads. A literal hand-applied `verification:` label is indistinguishable from
a `set-state` record and is treated identically — an accepted consequence of
using the Beads-native mechanism rather than inventing a private one.

**Values.** `pass` and `fail` only. ESCALATE is lifecycle state (bead stays
open), not a verification result.

**Lifecycle alignment.** (Review R-02.) PASS is recorded, then the bead may
close. REJECT and ESCALATE leave the bead open, with `verification=fail`
recorded on REJECT. A *closed* bead with `verification:fail` is a
lifecycle-integrity anomaly. It never affects readiness or blockage: a closed
dependency is a satisfied dependency, and Beads owns that judgment.

**Staleness.** `verification_commit` is reported verbatim so a consumer can
compare it against HEAD. `verification:pass` on a non-closed bead is reported
as `stale-pass` (the reopen case). Re-verification overwrites both writes.

**Residual gap, accepted and adjudicated.** A bead reopened and re-closed
without re-verification carries a pass describing an older implementation.
The workflow rule (verify before close) is the defense; detecting it would
require reading event history. Recorded as a limitation, not re-litigated.

### D6 — "Reason for any stop" means structural blockage, reported not re-derived

The command reports blocking structure: each blocked member, its blockers,
and each blocker's status, including blockers outside the subtree. It reports
Beads' readiness semantics and does not compute an alternative. It does not
interpret or advise; `sv-gx4` owns interpretation.

### D7 — Command name

`svibe progress <bead-id>`. `status` is taken by sync freshness. `progress`
is a noun, reads as a query, cannot be mistaken for a workflow verb.

### D8 — Subtree membership: recursive parent-child descent, work types only

(Reviews R-01, R-12, R-17. Adjudicated.)

- **Membership** is defined by `parent-child` edges exclusively. `blocks`
  edges are readiness information, never membership. Matches the repository's
  own usage (`sv-i1j.*`) and the `sv-beads` hierarchy model.
- **Depth** is recursive across all parent-child descendants.
- **Computed in memory, not by `bd` traversal.** (Review R-17; see D12.) The
  parent→children map is built from a single full-issue read and walked
  breadth-first from the root with a visited-set cycle guard. `bd`'s own
  `--parent` filtering is **not** used, because its descent semantics are
  inconsistent between subcommands (D12).
- **Excluded entirely** (not counted, not listed, not in any denominator):
  operational infrastructure beads — `event` (as created by `bd set-state`),
  agent, role, message, and gate beads. Empirically confirmed: a `set-state`
  call creates an `event`-type bead as a *parent-child child* of the work
  bead, so without this exclusion every verified bead would pollute its own
  subtree. `bd types --json` reports only *core* types and is therefore not
  treated as a complete registry; the exclusion list is maintained explicitly
  in one named location and defended by fixtures.
- **Container types** — `epic` and `milestone` — are **listed** as members
  with bucket `container` but **excluded from every denominator**. `bd`'s own
  definitions justify this: `milestone` "contains no work itself" and `epic`
  spans multiple issues. A nested epic contributes through its descendants
  only, so closing it cannot add progress on top of work its children already
  supplied. All other types — `task`, `bug`, `feature`, `chore`, `decision`,
  `spike`, `story`, and unrecognised custom types — count as work.
- **Any bead id is accepted** as root, not only epics. The root is reported in
  `root` and is never a member of its own subtree.

### D9 — Bucket taxonomy: readiness delegated to Beads

**Decision.** (Reviews R-10, R-19, R-22.) Availability and blockage are
**read from `bd`'s global sets and intersected with membership**, never
recomputed from edges:

- `bd ready --limit 0 --json` → global available set; intersect with members.
- `bd blocked --json` → global blocked set; intersect with members. Each entry
  carries `blocked_by[]`, which is **the authoritative source of per-member
  blocker ids**.

**Blocker detail comes from `blocked_by[]`, not from `blocks` edges.**
(Review R-19.) Blockage propagates through parent-child edges: a child of a
blocked parent is itself reported blocked, with `blocked_by` naming the
*parent*, despite having no `blocks` edge of its own. Deriving blocker detail
from raw `blocks` edges would leave such members blocked with an empty
blocker list and make `dependency_blocked` unable to name anything. Raw
`blocks` edges are not consulted for classification or blocker detail.

Every work-type member is assigned to exactly one bucket by the first
matching rule:

| # | Bucket | Predicate |
|---|--------|-----------|
| 1 | `container` | `issue_type` is `epic` or `milestone` (D8) |
| 2 | `completed` | stored status `closed` |
| 3 | `excluded` | stored status `pinned` |
| 4 | `deferred` | stored status `deferred` |
| 5 | `active` | stored status `in_progress` or `hooked` |
| 6 | `available` | member id ∈ global `bd ready` set |
| 7 | `blocked` | member id ∈ global `bd blocked` set, **or** stored status `blocked` |
| 8 | `unclassified` | anything else |

Notes:

- An `in_progress` member with blockers is `active` (rule 5 precedes rules
  6–7): someone is in fact working on it.
- Rule 7's stored-status disjunct is **necessary, not defensive**: a bead with
  stored status `blocked` and no dependency edges appears in *neither* the
  global ready set nor the global blocked set. Without the disjunct it would
  fall through to `unclassified`.
- `unclassified` members emit a warning diagnostic naming the status and the
  member. Reaching rule 8 means Beads knows something this projection does
  not — reported honestly rather than bucketed by guess.
- **Denominator.** `total_countable` = `completed + deferred + active +
  blocked + available + unclassified`. `container` and `excluded` members are
  listed but in no denominator: neither a structural container nor a
  deliberately persistent bead should make a subtree incompletable.
- **Ratio.** `completion_ratio` = `completed / total_countable`, a JSON
  number in `[0,1]` rounded to 4 decimal places for byte-stability. With a
  zero denominator it is `null` — never 0 or 1.
- Deferred work counts against completion: it is planned but incomplete.

### D10 — Structural progress and verification coverage are separate

(Reviews R-06, R-15.)

- **progress** — bucket counts and ratio (D9). Purely structural. A ratio of
  1 means all countable members are closed and makes no verification claim.
- **member verification** — every work member carries its *latest recorded*
  verification state, regardless of bucket: `pass`, `fail`, `stale-pass`
  (recorded pass on a non-closed member), or `unverified`. An open member
  carrying `fail` after a REJECT is normal in-flight state, reported
  informationally and **not** an anomaly.
- **verification_counts** — coverage over `completed` members **only**:
  `completed_total`, `pass`, `fail`, `unverified`. `stale-pass` cannot occur
  here by definition (it requires a non-closed member) and is therefore not a
  coverage key; it surfaces at member level and in anomalies. No compliance
  judgment is computed, because no durable marker distinguishes "verification
  required but missing" from "trivial, self-verification legitimate"
  (`spec.md:775-779`). The projection reports coverage; the human judges
  sufficiency.
- **anomalies** — lifecycle-integrity findings, each naming the member and a
  rule code from this closed vocabulary:
  - `closed-with-failed-verification` — closed member recorded `fail`;
  - `stale-pass` — non-closed member recorded `pass`;
  - `verification-without-commit` — outcome recorded with no
    `verification_commit` (D5).

Beads closed before this convention existed report `unverified`: correct, and
no longer misleading once coverage is separate from progress.

### D11 — Stop taxonomy

**Decision.** (Review R-11.) `stop.stopped` is true when the subtree has
countable non-completed members and **no** member is in `available` or
`active`. Every stopped result carries at least one categorised reason, drawn
from this closed vocabulary:

| Category | Meaning |
|---|---|
| `dependency_blocked` | members in the `blocked` bucket with named blockers |
| `stored_blocked` | members with stored status `blocked` and no named blockers |
| `deferred` | members deliberately deferred |
| `unclassified` | members in the `unclassified` bucket |

Mixed cases list every applicable category with its members. `reasons` is
populated **only** when `stopped` is true; a non-stopped result always has
`reasons: []`. Non-stopped results are explicit rather than absent:

- all countable members completed → `stopped: false`, `reasons: []`,
  `complete: true`;
- zero countable members (empty subtree, or containers/pinned only) →
  `stopped: false`, `reasons: []`, `complete: false`, and
  `completion_ratio` is `null`;
- available or active work exists → `stopped: false`, `reasons: []`.

`stop` is a non-null object in every successful M3 result, so a consumer
never has to distinguish "not stopped" from "not computed".

### D12 — `bd`'s `--parent` filtering is not used

**Decision.** (Review R-17; extended by R-22, found during revision.) The
implementation never passes `--parent` to any `bd` subcommand.

**Why.** The `--parent` flag is documented identically on multiple
subcommands as "filter to descendants of this bead/epic", but its behaviour
is **not** consistent, verified on a four-level fixture under `bd 1.1.2`:

| Invocation | Returns |
|---|---|
| `bd list --parent <root>` | immediate children **only** |
| `bd ready --parent <root>` | full recursive descendants |
| `bd blocked --parent <root>` | immediate children **only** |

A design reading membership from `bd list --parent` silently drops every
member at depth ≥ 2. A design reading blockage from `bd blocked --parent`
reports an empty blocked set for a root whose blocked work sits deeper —
verified: `bd blocked --parent <root>` returned `[]` while the unfiltered
`bd blocked` correctly returned both blocked descendants. Either produces a
confidently wrong answer, which D4 exists to preclude.

**Instead:** one unfiltered full-issue read supplies every member record with
its `parent` field; membership is computed in memory (D8); the unfiltered
global `ready` and `blocked` sets are intersected against it (D9).

**Consequence and tradeoff.** The command issues a **fixed four `bd`
invocations regardless of subtree size**, rather than one per member. The
cost is that reads are O(repository) rather than O(subtree). For a local
embedded database serving a status command this is the right trade: it is
bounded, predictable, avoids N+1 subprocess spawning on large epics, and
depends on no undocumented traversal semantics. Revisit only if repository
scale makes the full read expensive.

## The bd read contract

(Reviews R-04, R-07, R-08, R-10, R-16, R-17, R-18, R-22.)

Exactly four subcommands, all read-only, all invoked once per command run:

| # | Subcommand | Purpose | Fields consumed |
|---|---|---|---|
| 1 | `bd version` | contract diagnostics | version string |
| 2 | `bd list --all --limit 0 --json` | every issue; membership computed in memory | `id`, `title`, `status`, `issue_type`, `priority`, `parent`, `labels`, `metadata` |
| 3 | `bd ready --limit 0 --json` | global available set | `id` |
| 4 | `bd blocked --json` | global blocked set | `id`, `blocked_by[]` |

Flag notes, each empirically verified:

- `bd list` requires `--all` to include closed issues and `--limit 0` to
  avoid the default 50-row truncation. Its `parent` field is what makes
  in-memory membership possible — but **`parent`, `labels`, and `metadata`
  are omitted from the JSON entirely when empty** (review R-23; verified:
  root and orphan issues carry no `parent` key at all). Structural validation
  must treat absence of these three keys as empty, never as a contract
  violation. `id`, `status`, and `issue_type` are always present and remain
  hard-required.
- `bd ready` defaults to `--limit 100`; `--limit 0` is mandatory.
- **`bd blocked` has no `--limit` flag** (review R-18). Passing one is an
  `unknown flag` error and would fail every invocation. Its output is
  observed untruncated; if a future version truncates, D4's structural
  validation is the backstop.
- No `--parent` on any invocation (D12).
- Out-of-subtree blockers need no extra call: call 2 already returns every
  issue, so a blocker's status is resolved from the same map.

Invocation rules:

- **Process.** Executed directly with an argv slice — never through a shell.
- **Identifier handling.** The bead id is passed as a single argv element
  with **no pre-validation**. Beads owns its identifier grammar (hierarchical
  ids contain periods, prefixes vary), argv execution already precludes shell
  injection, and inventing a conservative subset would reject valid ids and
  contradict D8's promise to accept any bead. A root id absent from call 2's
  result set is reported as a diagnostic naming the id. (Review R-16.)
- **Target.** `-C <project-root>` (git top level, per `architecture.md`
  project-context rules); database discovery is left to `bd`.
- **Read-only enforcement.** Every invocation passes `--readonly`. A tested
  allowlist restricts invocations to the four subcommands above.
- **Timeout.** Bounded per invocation; expiry kills the process and produces
  a diagnostic naming the subcommand.
- **Failure semantics.** Non-zero exit, undecodable stdout, or
  contract-violating JSON produces a command failure with a diagnostic
  carrying the subcommand and a bounded stderr excerpt. Partial output is
  never silently used.
- **Consistency.** Four invocations are required and no atomicity is claimed:
  a concurrent `bd` mutation can produce a projection spanning two states.
  Accepted and documented; determinism is conditional on quiescent `bd`
  state. No retry or snapshot machinery is built.

## Output schema

(Reviews R-05, R-13, R-20.) The complete `result` object of the standard
envelope (`architecture.md:791-819`). Primary example — a subtree with work
in flight, so `stopped` is false and `reasons` is consequently empty:

```json
{
  "root": {"id": "sv-08e", "title": "Richer execution status",
           "status": "open", "issue_type": "epic"},
  "bd_version": "1.1.2",
  "counts": {
    "completed": 3, "deferred": 1, "active": 1, "blocked": 2,
    "available": 1, "unclassified": 0,
    "container": 1, "excluded": 0, "total_countable": 8
  },
  "completion_ratio": 0.375,
  "members": [
    {
      "id": "sv-08e.1", "title": "Subtree projection", "status": "closed",
      "issue_type": "task", "priority": 1, "parent": "sv-08e",
      "bucket": "completed",
      "verification": "pass",
      "verification_raw": null,
      "verification_commit": "a1b2c3d4e5f6",
      "blocked_by": []
    },
    {
      "id": "sv-08e.2", "title": "Stop reason", "status": "open",
      "issue_type": "task", "priority": 2, "parent": "sv-08e",
      "bucket": "blocked",
      "verification": "unverified",
      "verification_raw": null,
      "verification_commit": null,
      "blocked_by": [
        {"id": "sv-08e.1", "status": "closed", "in_subtree": true},
        {"id": "sv-zzz", "status": "open", "in_subtree": false}
      ]
    }
  ],
  "verification_counts": {
    "completed_total": 3, "pass": 2, "fail": 0, "unverified": 1
  },
  "anomalies": [
    {"member": "sv-08e.4", "code": "stale-pass",
     "message": "verification pass recorded on a non-closed bead"}
  ],
  "stop": {"stopped": false, "complete": false, "reasons": []}
}
```

Secondary example — the `stop` object when work has actually halted, showing
a mixed-category result (other keys omitted for brevity; they are always
present):

```json
{
  "stop": {
    "stopped": true,
    "complete": false,
    "reasons": [
      {"category": "dependency_blocked", "members": ["sv-08e.2"]},
      {"category": "deferred", "members": ["sv-08e.5"]}
    ]
  }
}
```

Rules:

- **Canonical ordering.** `members` by id ascending; `blocked_by` by id
  ascending; `anomalies` by member id then code; `stop.reasons` by the fixed
  category order of D11, each `members` list by id ascending. Ordering is
  owned by `svibe` serialization, never inherited from `bd` response order.
- **No volatile passthrough.** No `bd` timestamps are emitted. Byte-identical
  output for unchanged `bd` state is a tested property, achievable precisely
  because serialization is owned and volatile fields excluded.
- **Nullability.** All keys are always present. Absence is expressed as
  `null` (scalars) or `[]` (collections), never by omission, so consumers can
  index without existence checks. `completion_ratio` is `null` only when
  `total_countable` is 0.
- **`verification` values:** `"pass" | "fail" | "stale-pass" | "unverified"`.
  An unrecognised recorded value yields `verification: "unverified"`, the raw
  string in `verification_raw`, and a warning diagnostic.
- **`blocked_by` versus `stop`.** `members[].blocked_by` is per-member blocker
  detail sourced from `bd blocked`'s `blocked_by[]` (D9) and populated in M1.
  `stop` is the subtree-level summary added in M3; it introduces no blocker
  data of its own and references members by id.
- **Reserved until later milestones:** `verification`, `verification_raw`,
  `verification_commit`, `verification_counts` are `null` until M2; `stop` is
  `null` until M3. Shapes are fixed now so no consumer breaks when they
  populate.
- Human output renders the same data; presentation only, no extra
  information.
- Pre-1.0 the shape is intentionally stable but not contractually frozen
  (`architecture.md:819`).

## Constraints

1. **Beads owns the work graph and its readiness semantics.** Availability
   and blockage are consumed from the global `bd ready`/`bd blocked` sets,
   never recomputed. (`spec.md:279-291`; reviews R-02, R-10)
2. **The command is strictly read-only.** `--readonly` on every invocation
   plus a tested subcommand allowlist.
3. **The CLI does not gain workflow commands.**
   `internal/cli/cli_test.go:377` stays passing and unmodified.
4. **The `--json` envelope contract holds.**
5. **Existing commands keep working without `bd`.** (D3)
6. **No new persistent state.** (`architecture.md:54, 1051`)
7. **Structured Vibe does not become an agent harness.**
8. **Subprocess safety.** Argv slices, timeouts, bounded stderr capture.
9. **No `--parent` filtering.** (D12)

## Milestones

### M1 — Subtree progress projection

*Depends on: nothing.*

`svibe progress <bead-id>` exists. It computes membership in memory (D8),
classifies members from the global ready/blocked sets (D9), populates
`members[].blocked_by`, and emits the full schema with verification and stop
fields null.

**Testable outcomes.**

- **Depth:** a depth-3 parent-child fixture reports members at every level.
  A guard test fails if any `bd` argv contains `--parent` (D12).
- Membership: beads linked only by `blocks` edges are excluded;
  infrastructure types are excluded — specifically, a bead carrying a
  `set-state` event child does not gain a member from it; a parent-child
  cycle terminates via the visited set rather than hanging; the root is never
  its own member.
- Containers: a nested `epic` and a `milestone` appear with bucket
  `container`, are absent from `total_countable`, and their descendants are
  counted; closing the nested epic does not change `completion_ratio`.
- Readiness delegation: fixtures where naive edge-walking and the global
  ready/blocked sets disagree — deferred blocker, gate, external blocker —
  classify per `bd`. A guard test fails if availability is computed from
  dependency edges.
- **Propagated blockage:** a child of a blocked parent, with no `blocks` edge
  of its own, is bucketed `blocked` and its `blocked_by` names the parent
  (R-19).
- **Stored-blocked:** a bead with stored status `blocked` and no edges — absent
  from both global sets — lands in `blocked` via rule 7's disjunct, not in
  `unclassified`.
- Truncation: a repository with more than 100 issues is fully reported
  (proves `--all --limit 0` on list and `--limit 0` on ready).
- **Flag validity:** a test asserts the `bd blocked` argv carries no `--limit`
  (R-18).
- Bucketing: fixtures covering every D9 rule, including `in_progress` with
  blockers landing in `active`, `pinned`, and a custom status producing
  `unclassified` plus a warning; counts sum to `total_countable` plus
  `container` plus `excluded`.
- Zero countable members: valid result, `completion_ratio` null, no error.
- Unknown id: failure with a diagnostic naming the id; a hierarchical id
  containing periods is accepted.
- Missing `bd`: failure naming `bd`; `svibe status` and `svibe resolve`
  succeed in the same environment.
- Contract violation: well-formed JSON missing a required field produces a
  failure diagnostic naming the subcommand and observed `bd version`.
- Determinism: two runs against unchanged state produce byte-identical
  `--json` stdout, verified against permuted `bd` response order.
- Read-only: a test fails if any `bd` argv lacks `--readonly` or uses a
  subcommand outside the four-entry allowlist.

### M2 — Verification convention and coverage

*Depends on: M1.*

`sv-verify` records outcomes per D5. The command populates member
verification, `verification_counts`, and `anomalies`, reading `labels[]` and
`metadata.verification_commit` from the existing member-list call.

**Testable outcomes.**

- Member-level state is reported for *every* work member regardless of
  bucket: closed `pass`, closed `fail`, non-closed `pass` → `stale-pass`,
  non-closed `fail` (informational, not an anomaly), and absent →
  `unverified`; `verification_commit` emitted verbatim when present.
- The `verification:<value>` label is read from `labels[]` and the commit
  from `metadata`; no event bead is read (guard test).
- `verification_counts` covers `completed` members only; `stale-pass` never
  appears in it.
- Anomalies: `closed-with-failed-verification`, `stale-pass`, and
  `verification-without-commit` each fire on their fixture and nowhere else.
- A closed member with `verification:fail` appears in `anomalies` and does
  **not** appear as a blocker of anything (R-02 guard).
- An unrecognised value yields `unverified` + `verification_raw` + warning.
- An all-closed subtree with unverified members reports ratio 1.0 *and*
  incomplete coverage, visibly distinct in both output forms.
- The `sv-verify` skill text names both writes **in order** (metadata commit
  first, then `set-state`), states they precede closure, and contains no
  `blocked` verification value.

### M3 — Structural stop reason

*Depends on: M1. Independent of M2; reads no verification data (guard test).*

The command populates `stop` per D11.

**Testable outcomes.**

- Each D11 category fires on a dedicated fixture: dependency-blocked,
  stored-blocked without blockers, deferred-only, unclassified-only.
- A mixed subtree lists every applicable category with correct members.
- `stopped: true` **always** carries at least one reason — a guard test
  asserts no result can be stopped with an empty `reasons` list.
- `stopped: false` **always** carries `reasons: []` — the converse guard
  (R-20), asserted against the schema examples used as golden files.
- All countable members completed → `stopped: false`, `complete: true`.
- Zero countable members → `stopped: false`, `complete: false`.
- Blockers outside the subtree are reported with id, status,
  `in_subtree: false`, resolved from the full-issue map with no extra `bd`
  call.

### M4 — Skill and documentation

*Depends on: M2 and M3.*

The `sv-progress` core skill invokes the command and frames the result.
Documentation updated: `README.md` command listing, `architecture.md` section
4.1 command inventory and section 14 boundary note, core skill listings.

**Testable outcomes.**

- Skill passes `svibe validate`; directory name equals frontmatter name;
  description within limits; `minimum_driver_tier` declared.
- `make generate` run; `make generate-check` and `make check` pass.
- The skill instructs invoking the CLI and prohibits re-deriving state the
  command already computes.
- Documentation describes the full command set.

## Open questions

**Q1 — Infrastructure type list.** The exact non-core type names (`event`,
`agent`, `role`, `message`, `gate`) must be confirmed empirically, since
`bd types --json` reports only core types. `event` is confirmed. Kept in one
named location with fixtures. *Resolver: implementation; low risk — an
unrecognised type falls through to `unclassified` with a warning rather than
being silently miscounted.*

**Q2 — Metadata key naming.** `verification_commit` is proposed. If Beads
establishes a competing convention, prefer theirs. *Resolver: implementation;
low risk.*

(Earlier questions are resolved: membership, depth, and non-epic roots into
D8; container counting adjudicated into D8; skill shape into M4; retroactive
verification into D10; version floor into D4; stop taxonomy into D11;
identifier validation and `--parent` avoidance into D12 and the read
contract.)

## Assumptions

1. **`bd`'s `--json` output is a stable interface.** Observed on `bd 1.1.2`.
   D4 runtime validation converts breakage into a named diagnostic rather
   than a wrong answer, but breakage still breaks the command.
2. **`bd list --all --limit 0 --json` returns every issue, with `id`,
   `status`, and `issue_type` always present; `parent`, `labels`, and
   `metadata` appear only when non-empty** (omitted-when-empty; review
   R-23). Verified on `bd 1.1.2`. This single call is load-bearing:
   membership, verification state, and out-of-subtree blocker status all
   derive from it, with key absence read as empty. (Replaces the former
   assumption about `--parent` descent consistency, which R-22 disproved.)
3. **The global `bd blocked` set is untruncated.** It exposes no `--limit`
   flag to control this. D4 structural validation is the backstop.
4. **Two-write verification is acceptable.** Commit-first ordering makes the
   interrupted state read as unverified rather than falsely verified.
5. **Verification is per-bead, single-valued, overwritten on
   re-verification.** History lives in `bd` event beads, unread here.
6. **Reading the full issue set per invocation is acceptable.** Fixed at four
   subprocesses regardless of subtree size; O(repository) rather than
   O(subtree). Revisit only at scale. (D12)
7. **One subtree per invocation is sufficient.**
8. **Shelling out to a sibling CLI is acceptable.** Second such dependency
   after `git` (`internal/paths/paths.go:72`), mitigated by the read
   contract.
9. **Torn reads across the four invocations are acceptable** for a status
   command in a single-operator repository. Revisit if multi-writer use
   emerges.

## Traceability

- Roadmap: `docs/ROADMAP.md:185-190`
- Bead: `sv-08e`; findings R-01..R-22 recorded on the bead
- Related, not depended upon: `sv-gx4`, `sv-bpq`, `sv-vzf`, `sv-dwx`
- Governing: `docs/specs/architecture.md` sections 4.1, 14, 15;
  `docs/specs/structured-vibe-spec.md` sections 5.5, 13; `docs/PRINCIPLES.md`
