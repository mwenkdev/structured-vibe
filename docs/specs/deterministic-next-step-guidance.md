# Specification: Deterministic Next-Step Guidance

Bead: `sv-vzf`
Status: revision 5 — findings R-01..R-28 addressed; review rounds closed;
blocked on `sv-e85`
Roadmap: `docs/ROADMAP.md` "Deterministic next-step guidance"

## Purpose

Make every host workflow stop tell the human what action resumes or advances
the workflow. The recommendation must come from Structured Vibe policy, not
from whether the model running a workflow skill remembers the correct handoff.

This improves the current manual workflow without turning `svibe` into an
agent harness. Models still make the judgments assigned to each workflow
phase. Structured Vibe maps a bounded phase outcome, plus its phase-specific
target data and durable Beads state, to a next action, and the host presents it.

## Prerequisite

This epic depends on `sv-e85`, "Host workflow command definitions".

The `/svibe:*` commands do not exist. Verified 2026-09-09: there are no command
definitions in this repository and none in the user OpenCode config, which
contains only `plugins/svibe.js` and `skills/`. The slash forms currently in
use are typed by the human and passed through as ordinary prompt text.

The design below needs a real command invocation as the trigger, as the carrier
of the target argument, and as the boundary that marks a workflow phase. It
cannot be implemented against commands that do not exist. Human decision on
2026-09-09: define those commands in a separate epic and block this one on it,
rather than expanding this epic to cover them.

## Scope

### Included

- The host workflow phases backed by `sv-plan`, `sv-review`, `sv-beads`,
  `sv-finalize`, `sv-execute`, and `sv-verify`.
- A closed outcome vocabulary for each phase.
- A transport-independent policy that maps a valid phase outcome, combined
  with current Beads readiness where applicable, to one immediate recommended
  next action.
- A CLI surface exposing that policy through the standard human and `--json`
  output paths.
- OpenCode integration that recognizes Structured Vibe workflow invocations,
  obtains their bounded outcome, requests the recommendation from `svibe`, and
  presents it durably through a non-model host channel.
- Explicit withholding of guidance when a workflow invocation does not produce
  a valid outcome.
- Human-decision guidance as well as command guidance.

### Explicitly excluded

- Defining the `/svibe:*` host commands. That is `sv-e85`.
- Remediation for infrastructure commands such as `svibe status`, `sync`,
  `init`, and `admin`. Those commands may continue their existing guidance;
  standardizing it is separate work.
- Automatic execution of the recommended action.
- Autonomous epic execution, retry policy, task prioritization, parallelism,
  model routing, fresh-context creation, or host capability detection. Those
  remain separate roadmap items.
- Semantic blocker diagnosis, including explaining *why* nothing is ready.
  That is `sv-gx4`.
- A new persistent workflow-state store or a graph parallel to Beads.
- Parsing unrestricted model prose to infer an outcome.
- Workflow subcommands such as `svibe plan`, `svibe review`, or
  `svibe execute`.
- Host integrations other than OpenCode in the first delivery.

## Decisions

### D1 — Separate outcome judgment from next-action policy

Each workflow phase owns its existing judgment and reports one value from a
closed outcome vocabulary. A transport-independent Structured Vibe policy owns
the mapping from `(phase, outcome, target, root?)` plus current Beads state to
one next action.

The model may decide that review found material problems or that verification
must reject, but it does not author the recommended command. The policy result
is the only authoritative recommendation presented by the integration.

**What this does and does not eliminate.** It removes the dependency on a model
remembering *which* command comes next, which is the failure named in the
bead's acceptance criteria. It does not make a model's own phase judgment
correct: a model that wrongly reports `pass` produces deterministic guidance
built on a wrong judgment. That residual is accepted; independent verification,
not next-step guidance, is the control for wrong judgments.

**Alternatives considered.** Adding stronger prose to each skill was rejected:
it still relies on model compliance and produces inconsistent wording. Having
the OpenCode plugin own a lookup table was rejected because architecture 12.4
requires host adapters to delegate Structured Vibe decisions to `svibe`.
Inferring the entire phase outcome from repository artifacts was rejected
because several workflow judgments are not durably observable today.

### D2 — One immediate recommendation

The policy returns exactly one immediate recommended action. An action is one
of:

- `command`: invoke a named host workflow command with explicit arguments;
- `bookkeeping_command`: run one explicitly named Beads bookkeeping command;
- `human_decision`: provide the requested judgment or resolve the reported
  blocker; where the outcome does not determine a safe next phase, the action
  does not invent one;
- `complete`: the target's workflow is finished and no further action applies;
- `unavailable`: no safe recommendation can be determined.

The action is computed from state that exists when the query runs. The policy
does not pair a state-changing prerequisite with a recommendation that assumes
the mutation has already happened. In particular, passing verification
recommends closing the verified bead; it does not predict what Beads will make
ready after closure.

`bookkeeping_command` is advisory like every other action. It is rendered as
an argv-safe command from a validated Beads identifier and is never executed by
the query or host integration. Its first use is `bd close <id> --suggest-next`,
which lets Beads report its own post-mutation readiness when the human
explicitly runs the command.

**Alternatives considered.** An ordered list of actions was rejected: lists
require prerequisite resolution, deduplication, and partial-completion
semantics that belong to orchestration. Returning a closure prerequisite plus
an already-resolved following action was rejected because a read-only query
cannot observe readiness that exists only after closure, and predicting that
readiness would violate D4.

### D3 — Outcomes are phase-specific and closed

The vocabulary is:

| Phase | Outcomes |
| --- | --- |
| `plan` | `needs-input`, `escalate`, `spec-ready` |
| `review` | `findings`, `no-material-findings` |
| `beads` | `needs-input`, `blocked`, `graph-ready` |
| `finalize` | `blocked`, `packet-ready` |
| `execute` | `blocked`, `verification-required`, `self-verified` |
| `verify` | `reject`, `escalate`, `pass` |

Every phase that can stop for a human has a way to say so. `plan` carries
`escalate` because `sv-plan` has explicit escalation conditions distinct from
merely needing an answer. `review` distinguishes material findings from a
calibrated stop, because `sv-review` is explicitly allowed to pass and has its
own stopping rule.

The vocabulary describes handoff state, not every internal result. Adding an
outcome requires an explicit policy mapping and tests. Unknown phase/outcome
pairs are rejected rather than treated as synonyms.

The policy mappings are:

| Phase outcome | Recommended action |
| --- | --- |
| `plan.needs-input` | `human_decision`: answer the planner's questions, then rerun `/svibe:plan <target>` |
| `plan.escalate` | `human_decision`: resolve the reported conflict or scope change, then rerun `/svibe:plan <target>` |
| `plan.spec-ready` | `command`: `/svibe:review <target>` |
| `review.findings` | `human_decision`: adjudicate the findings, then rerun `/svibe:plan <target>` to revise |
| `review.no-material-findings` | `human_decision`: approve the specification, then run `/svibe:beads <target>` |
| `beads.needs-input` | `human_decision`: answer the decomposition question, then rerun `/svibe:beads <target>` |
| `beads.blocked` | `human_decision`: resolve the reported blocker, then rerun `/svibe:beads <target>` |
| `beads.graph-ready` | resolved against Beads readiness; see D4 |
| `finalize.packet-ready` | `command`: `/svibe:execute <target>` |
| `finalize.blocked` | `human_decision`: adjudicate the reported blocker; no next phase is prescribed |
| `execute.verification-required` | `command`: `/svibe:verify <target>` |
| `execute.self-verified` | `bookkeeping_command`: `bd close <target> --suggest-next` |
| `execute.blocked` | `human_decision`: adjudicate the reported blocker; no next phase is prescribed |
| `verify.reject` | `command`: `/svibe:execute <target>`; the durable Beads failure summary and current session output carry the findings |
| `verify.escalate` | `human_decision`: adjudicate the reported specification or scope conflict |
| `verify.pass` | `bookkeeping_command`: `bd close <target> --suggest-next` |

Human adjudication and approval are explicit actions rather than implicit
preconditions. `sv-beads` requires an adjudicated specification, so the policy
interrupts for that judgment instead of assuming it happened. Generic blocked
outcomes intentionally stop at adjudication: the underlying report may require
a retry, a stronger executor, re-finalization, decomposition, or planning, and
the bounded outcome does not contain enough information to choose safely.

The finalized execution packet gains one explicit closed field,
`verification_mode: independent | self`. `self` is permitted only for the
narrow trivial-work exception from the product specification. An executor that
completes the authorized self-verification emits `self-verified`; otherwise a
successful execution emits `verification-required`. Free-form verification
expectations remain in the packet but do not substitute for this field.

Before emitting `verify.pass` or `verify.reject`, `sv-verify` must successfully
perform the ordered pair required by `richer-execution-status.md` D5: record
`verification_commit`, then record `verification=pass|fail`. The fail write
includes a bounded actionable reason. A failed persistence write produces no
valid terminal outcome and is handled by D8. The durable failure reason is the
minimum cross-context carrier; the full finding report remains in the current
session output.

### D4 — Readiness comes from Beads, never from the model

For `beads.graph-ready`, the outcome record carries the identifier of the root
bead created or populated by decomposition. The policy determines the next
thing by querying Beads for that root's ready descendants. It does not ask the
model to enumerate candidates. The root is validated by exact lookup and must
contain at least one countable descendant; it need not equal the invocation
target, which may be a specification path or other planning artifact. Passing
verification does not use this resolution because its target is a leaf and the
query runs before closure.

Resolution rules:

- exactly one ready descendant → `command`: `/svibe:finalize <id>`;
- more than one → `human_decision` listing the ready identifiers, because
  choosing among them is task prioritization and belongs to `sv-zas`;
- none ready and open work remains → `human_decision` stating that no work is
  ready, without diagnosing why, which belongs to `sv-gx4`;
- none ready and no incomplete countable work remains → `complete`.

Membership, issue-type exclusions, containers, and work buckets are exactly
those in `richer-execution-status.md` D8 and D9. "Incomplete countable work"
means members in the `deferred`, `active`, `blocked`, `available`, or
`unclassified` buckets. Pinned and operational members do not prevent
completion. A zero-countable-work graph is not complete; it is an invalid
`graph-ready` result and yields a failed envelope with an `unavailable` action.
When there are no ready members, active or unclassified work prevents
completion and is included in the no-work-ready human decision; it does not
override the exactly-one-ready rule.

**Why.** `richer-execution-status.md` D2 establishes that Beads readiness is
authoritative and is consumed, never recomputed. Letting a model report which
beads are ready would create a second, unverified readiness source and produce
confidently wrong guidance when the model miscounts.

**Consequence.** `bd` becomes a hard requirement for this query only, exactly
as it already is for `svibe progress` under `richer-execution-status.md` D3.
Existing commands continue to work with no `bd` installed. The query reuses the
established `bd` read contract and its runtime structural validation rather
than inventing new reads; contract violations produce a diagnostic and an
`unavailable` action, never a guessed one.

### D5 — The CLI is a policy transport, not a workflow runner

Add a read-only CLI query that accepts phase, outcome, target, and the optional
phase-specific root, and returns the policy result. `root` is required only for
`beads.graph-ready` and rejected for every other phase/outcome. The exact
command name and flag spelling may be settled during implementation, but its
behavior and JSON shape are part of this specification.

The result is a discriminated schema. Every key shown is always present; absent
scalar values are `null` and absent collections are `[]`.

Conceptual JSON result:

```json
{
  "ok": true,
  "warnings": [],
  "errors": [],
  "result": {
    "phase": "verify",
    "outcome": "pass",
    "target": "sv-abc.1",
    "root": null,
    "action": {
      "kind": "bookkeeping_command",
      "command": "bd",
      "arguments": ["close", "sv-abc.1", "--suggest-next"],
      "candidates": [],
      "message": "Close the verified bead; Beads will report work made ready by closure."
    }
  }
}
```

`action.kind` is the discriminator:

- `command`: `command` is the host command name, `arguments` contains its
  argument vector, and `candidates` is empty;
- `bookkeeping_command`: `command` is the executable name, `arguments` contains
  its argv, and `candidates` is empty;
- `human_decision`: `command` is null, `arguments` is empty, and `candidates`
  contains canonical identifier strings only when several ready beads require
  prioritization;
- `complete`: `command` is null and both collections are empty;
- `unavailable`: `command` is null and both collections are empty.

Every action has a non-empty `message`. Candidates are sorted by identifier.
Invalid phase/outcome pairs, missing or unexpected phase-specific root data,
invalid phase-specific target/root state, a zero-countable `graph-ready`
result, missing `bd`, and violated `bd` contracts are command failures: `ok` is
false, `errors` is non-empty, the process exits non-zero, and `result.action`
is `unavailable` so a consumer that parses the envelope can present the safe
domain result.
Recognized no-ready and many-ready states are successful advisory results with
`ok: true`. The OpenCode integration parses a valid bounded envelope even on a
non-zero exit; malformed or absent output falls back to its own unavailable
notice.

The application policy returns typed data and has no stdout dependency. The
CLI adapter serializes it using `cliout`. No workflow execution is added to the
Go CLI.

Architecture 15 governs **this query only**. The guidance the OpenCode
integration displays is host UI, not envelope output, and is not bound by the
stdout/stderr contract.

**Alternative considered.** Putting `next_action` inside every existing CLI
result was rejected because this epic covers host workflow outcomes, not
infrastructure-command remediation, and because failure outcomes often have no
command result object.

### D6 — OpenCode obtains an explicit outcome; it does not parse prose

The OpenCode integration records the invoked Structured Vibe workflow phase and
target. The workflow prompt contract requires the phase to emit a small,
machine-readable terminal outcome record in addition to its normal human
report. The integration validates the record against the invoked phase and
target, invokes the CLI policy query, and presents the returned recommendation
after the workflow response finishes.

The record contains the phase, the closed outcome, the invocation target, and
an optional `root`. `root` is required only for `beads.graph-ready`; it is the
identifier of the root bead actually created or populated by decomposition.
Explanations remain ordinary human output and are never parsed to choose an
action. The record is session-local and is not persisted as project state.

The invocation target is an opaque bounded string whose exact bytes must equal
the target recorded when the command was invoked. The optional root is likewise
bounded. The integration does not invent a Beads identifier grammar.
`finalize`, `execute`, and `verify` validate the target by exact lookup in the
Beads snapshot. `beads.graph-ready` instead validates its root by exact lookup;
other `beads` outcomes must not include a root, and an unexpected root fails
the presence check.
`plan`, `review`, and decomposition inputs may target ideas or artifact paths
and do not require an existing bead. Commands and the CLI pass targets and
roots as individual argv values and never through a shell. A record failing the
length, equality, presence, or phase-specific lookup checks is treated under
D8.

**Alternatives considered.** Prompting the model to print the next command was
rejected because it is the failure mode this epic exists to remove. Parsing
headings such as `PASS` or `BLOCKED` from unrestricted prose was rejected as
ambiguous and brittle. Persisting every phase outcome in Beads was rejected as
unnecessary state and as overlap with durable execution-ready epic state.

### D7 — The presentation channel must be durable

The recommendation must be presented in a channel that persists for the rest of
the session, so a human who looks away does not lose it.

An ephemeral-only channel does not satisfy this specification. The existing
plugin's sole human channel is a TUI toast, which disappears and is explicitly
tolerated as best-effort; if a toast is the only mechanism the pinned host API
offers, that fails the acceptance criterion and the gate in M1 must report it.
A toast may accompany a durable presentation but may not replace it.

### D8 — Missing outcomes withhold guidance rather than guess

If the workflow response has no valid terminal record, has more than one, uses
an unknown value, or conflicts with the invoked phase or target, the
integration presents an `unavailable` action stating that no safe next step
could be determined and pointing at the workflow output.

It must not guess from prose, must not silently omit the notice, and must not
prescribe a rerun. Prescribing a rerun is unsafe: rerunning `/svibe:execute`
after an implementation has already landed acts against completed work. Failing
closed means withholding a recommendation, not inventing a generic one.

### D9 — Behavior under version and snapshot drift is defined

The outcome contract lives in core skills and reaches the host only through
`svibe sync` and a host restart. A newer integration therefore can run against
a snapshot whose skills predate the contract, in which case every workflow
command would otherwise produce the D8 notice with no explanation.

Generated snapshot metadata gains a small versioned capability marker for the
workflow-outcome contract. At plugin initialization, the integration records
the snapshot fingerprint and capability marker present on disk. Before
presenting guidance it compares those captured values with current disk state.

- no captured outcome capability means the loaded snapshot predates the
  contract, so guidance names `svibe sync` and host restart remediation;
- a changed fingerprint or capability means the disk snapshot changed after
  host initialization, so guidance names host restart remediation;
- a loaded snapshot declaring the capability but producing no valid record is
  handled by D8, including when a project or user skill override omits it.

This marker extends the minimal sync state because it is necessary to identify
a host-facing contract; it is not a second resolution manifest. Existing
staleness, version-drift, and registration facts remain supporting diagnostics
but are not treated as proof of what the running host loaded.

Guidance is best-effort: a stale snapshot degrades guidance, never the session.

### D10 — Guidance is advisory and never auto-executes

The integration displays the recommendation but does not invoke the command,
answer a question, close a bead, change models, create a context, or mutate
files. This preserves human control and the architecture rule that the host
owns execution mechanics.

The recommendation is required to be explicit; acting on it remains a human
choice until a separate orchestration feature owns that transition.

### D11 — Core skills remain ordinary skills

The six skills gain a shared outcome-reporting contract and phase-specific
allowed values. They do not gain privileged loading, execution, or persistence
semantics. Project or user overrides may replace these skills as today; an
override that does not honor the contract receives the visible D8 notice.

## Constraints

- Preserve architecture 4.1: the Go CLI provides policy infrastructure and does
  not execute planning, review, decomposition, or implementation.
- Preserve architecture 4.2: policy logic is transport-independent and is not
  coupled to stdout or OpenCode APIs.
- Preserve architecture 12.4: the OpenCode adapter observes host state and
  invokes `svibe`; it does not reimplement next-action policy.
- Preserve architecture 15 for the CLI query: `--json` stdout is pure JSON,
  diagnostics are structured in the envelope, and human-readable diagnostics go
  to stderr.
- Beads remains the durable work graph. This feature creates no parallel
  workflow database and recomputes no readiness.
- Guidance must never execute an action or mutate project state.
- Guidance must not claim a unique next bead when more than one is ready.
- A malformed or spoofed outcome can produce at most advisory UI; it must not
  authorize or trigger execution.
- `bd` remains optional for every command except this query and `progress`.
- Generated host-facing output is regenerated through existing tooling and is
  not hand-edited.
- Existing workflows remain usable without the OpenCode integration; they
  merely lack the deterministic host presentation in that environment.

## Milestones

### M1 — Prove the host mechanism, or stop

**Dependencies:** `sv-e85` must have defined the workflow commands.

This milestone exists first because M2 through M4 are worthless if the host
cannot support the mechanism.

**Testable outcomes:**

- Demonstrated observation of a workflow command invocation, its target, its
  terminal outcome record, and its completion, without starting another model
  turn.
- Demonstrated durable presentation channel per D7, or an explicit report that
  the pinned API offers only ephemeral channels.
- The findings are recorded against this specification.
- If either gate fails, work stops and escalates for a design decision rather
  than substituting prompt injection, prose matching, or a toast-only channel.

### M2 — Define and expose the next-action policy

**Dependencies:** M1.

**Testable outcomes:**

- Every phase/outcome pair in D3 maps to exactly one typed action.
- Readiness-dependent outcomes resolve through Beads per D4, including the
  one-ready, many-ready, none-ready, and nothing-open cases.
- `beads.graph-ready` requires a bounded root identifier, validates that root
  and its countable descendants, and does not require it to equal an artifact
  invocation target.
- `verify.pass` and `execute.self-verified` produce only the immediate closing
  bookkeeping command and never predict post-closure readiness.
- The target is treated as opaque for `plan`, `review`, and `beads`; executable
  phases validate it by exact Beads lookup rather than a local identifier
  grammar.
- Every action kind emits the complete D5 shape, and failure envelopes retain
  an `unavailable` action that the integration can present.
- Unknown phases, unknown outcomes, malformed targets, and `bd` contract
  violations produce stable diagnostics and an `unavailable` action, never a
  guessed one.
- The policy has no dependency on CLI writers or OpenCode types.
- The CLI query emits the standard envelope in JSON mode and readable guidance
  in human mode, with stdout pure in JSON mode.
- Existing commands keep working with no `bd` installed.
- The test forbidding workflow-execution subcommands still passes unmodified.

### M3 — Add the bounded workflow outcome contract

**Dependencies:** M1.

**Testable outcomes:**

- Each of the six core workflow skills names its allowed outcomes and requires
  exactly one terminal outcome record on every completed or blocked return. A
  run terminated by a failed required Beads write is exempt: it produces no
  record and is intentionally surfaced through D8.
- Each installed skill remains self-contained: it includes the complete small
  record syntax and its phase-specific values. A single canonical test fixture
  asserts that the repeated schema is identical rather than relying on an
  external shared file.
- `sv-beads` includes the actual created or populated root in `graph-ready` and
  emits no `graph-ready` record when it cannot identify a valid root.
- `sv-finalize` adds the closed `verification_mode: independent | self` field
  to every packet, with `self` restricted to the narrow trivial-work exception.
- `sv-verify` emits `pass` or `reject` only after both ordered D3 Beads writes
  succeed, and `sv-execute` may emit `self-verified` only when the finalized
  packet explicitly permits it.
- Tests detect removal of the contract from a core workflow skill.
- User and project overrides remain valid when they omit the contract;
  omission is handled at runtime by D8, not by pack validation.
- Managed-file manifests are regenerated through repository tooling.

### M4 — Present deterministic guidance in OpenCode

**Dependencies:** M2 and M3.

**Testable outcomes:**

- For each valid phase outcome, the integration invokes `svibe` and durably
  presents exactly the returned recommendation.
- The integration contains no duplicate phase/outcome action table.
- Missing, duplicate, malformed, mismatched, and unknown outcome records all
  produce the D8 notice, derive no action from prose, and prescribe no rerun.
- Missing capability, snapshot changes after plugin initialization, and other
  version drift produce the D9 explanation rather than an unexplained failure.
- Concurrent sessions and consecutive workflow commands cannot consume each
  other's outcomes.
- Internal host agents and unrelated commands produce no guidance.
- CLI absence, CLI failure, malformed JSON, missing `bd`, and a detached TUI
  remain non-fatal to the host session and trigger no action.
- Tests demonstrate that no command or mutation runs automatically.

### M5 — Align architecture and user documentation

**Dependencies:** M2 through M4.

**Testable outcomes:**

- Architecture documents the policy query as infrastructure, the host adapter
  boundary, the `bd` dependency for this query, the workflow outcome contract,
  and the section 14 sync-state capability marker.
- The product specification's execution-packet contract documents the closed
  verification mode field.
- User documentation shows the guidance behavior and distinguishes it from
  autonomous execution.
- Documentation does not promise deterministic guidance for unsupported hosts
  or for infrastructure-command remediation.
- Repository validation, Go tests, integration type-checking, and managed
  payload checks pass.

## Open Questions

None requiring product adjudication before decomposition.

The CLI command name, terminal-record encoding, and the specific OpenCode
presentation primitive are implementation details within the constraints
above. M1 must settle the latter two as tested host contracts, including
session and command correlation, before implementation continues. It carries
the two gates that can invalidate the design.

## Assumptions

- `sv-e85` will define phase-specific target handling, preserve the target as
  one opaque argument, and produce commands observable by the integration; if
  it does not, M1's gate fails.
- A phase can reliably choose one value from its closed vocabulary even though
  free-form explanations remain model-authored.
- A durable host-presented recommendation satisfies "reported explicitly"; the
  guidance need not be authored into the assistant's prose.
- The first version can be OpenCode-specific because OpenCode is the only
  supported host integration in the current architecture.
- The existing `bd` read contract is sufficient for readiness resolution
  without new subcommands.

## Review record

Reviewed by `sv-review` across four rounds: revision 1 produced findings R-01
through R-11 (same-model caveat recorded at the time); revision 2 produced
R-12 through R-20; revision 3 produced R-21 through R-26; revision 4 produced
no material findings and two minor wording fixes, R-27 and R-28. Review
process, human decision 2026-09-10: rounds after revision 1 ran on different
models within one continuous session, and shared session context is accepted
as satisfying review independence. Review rounds are closed.

Resolutions:

- **R-01** (blocking) accepted: `sv-e85` created, `sv-vzf` blocked on it, and
  command definition excluded from scope. Human decision 2026-09-09.
- **R-02** (blocking) accepted: `verify.pass` no longer maps to completion.
  Revision 2 introduced a mechanical prerequisite; R-12 subsequently replaced
  it with the immediate bookkeeping action in revision 3.
- **R-03** (blocking) accepted: D4 sources readiness from Beads, and the model
  no longer reports candidates.
- **R-04** accepted: the feasibility gate became M1.
- **R-05** accepted: D9 defines drift behavior.
- **R-06** accepted: `review` outcomes split, and adjudication and approval are
  explicit `human_decision` actions.
- **R-07** accepted: D8 withholds rather than prescribing a rerun.
- **R-08** accepted: D7 requires a durable channel and makes toast-only a
  gate failure.
- **R-09** accepted: `plan.escalate` added.
- **R-10** accepted: D6 bounds the model-supplied target and requires exact
  equality with the invocation target before it can reach rendered guidance.
- **R-11** accepted: D5 scopes architecture 15 to the CLI query.
- **R-12** (blocking) accepted: `verify.pass` no longer predicts readiness
  after an unperformed close. It recommends only `bd close --suggest-next`,
  and D4 is limited to `beads.graph-ready` with an explicit validated root.
- **R-13** (blocking) accepted: D6 treats invocation targets as opaque and
  requires exact Beads lookup only for phases that consume Beads state.
- **R-14** accepted: D3 makes successful verification-state persistence a
  precondition for `pass` and `reject` terminal outcomes.
- **R-15** accepted: generic finalization and execution blockers no longer
  prescribe a same-phase rerun without enough policy information.
- **R-16** accepted: execution distinguishes independent verification from the
  explicitly authorized trivial self-verification path.
- **R-17** accepted: the Beads failure reason is the durable minimum findings
  carrier; full findings remain in current-session output.
- **R-18** accepted: D4 adopts the richer-status membership and bucket taxonomy
  and defines zero-work and completion behavior.
- **R-19** accepted: D5 defines every action variant, nullability, candidates,
  diagnostics, and process failure semantics.
- **R-20** accepted: D9 adds an explicit snapshot capability marker and
  distinguishes loaded state from current disk state.
- **R-21** accepted: `verify.reject` now requires the same commit-first ordered
  write pair as `verify.pass` before emitting a terminal outcome.
- **R-22** accepted: `beads.graph-ready` carries the actual created or populated
  root separately from the invocation target, and M3 assigns that obligation to
  `sv-beads`.
- **R-23** accepted: the finalized packet gains a closed verification-mode
  field, and M3 assigns its production and consumption explicitly.
- **R-24** accepted: D4 clarifies that active and unclassified work affect the
  no-ready case and do not override an exactly-one-ready result.
- **R-25** accepted: a zero-countable `graph-ready` result is explicitly a
  failed envelope carrying an `unavailable` action.
- **R-26** accepted: M5 now includes architecture section 14 and the product
  execution-packet contract among required documentation updates.
- **R-27** accepted: M3 exempts write-failure terminations from the
  exactly-one-record requirement; they surface through D8.
- **R-28** accepted: D6 states that non-`graph-ready` outcomes must not
  include a root, making unexpected presence an explicit failure.

## Next Step

Review rounds are closed. Once the human has approved this specification and
`sv-e85` is complete, `sv-beads` compiles it into a dependency graph.
