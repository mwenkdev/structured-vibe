# Specification: Host Workflow Command Definitions

Bead: `sv-e85`
Status: revision 17 — **withheld on OpenCode 1.18.30**. Findings R-01..R-18 and
R-20..R-65 addressed (R-19 was never assigned). M1b is complete and its answer
is negative: the host provides no inert pre-interpretation transport for a
command target, so D9's withholding rule applies and no delivery ships. This is
a host capability limitation, not a failed implementation.
Roadmap: not yet listed; consumed by `docs/ROADMAP.md` "Deterministic next-step
guidance" and "Command and identifier completion"

## Purpose

Give each core workflow skill a real OpenCode command that observably carries a
target argument and requests loading of the corresponding winning skill.

Today the slash forms in use (`/svibe:plan sv-xyz`) are ordinary prompt text
typed by the human. Nothing observes them, nothing carries the target as a
distinct value, and nothing marks a workflow phase boundary. Two planned
capabilities need real invocations: deterministic next-step guidance (`sv-vzf`)
requires an observable trigger and target carrier, and command completion
(`sv-7sq`) requires commands that exist to complete.

Commands are thin prompt wrappers. The workflow behavior stays in the skills;
a command names the skill, carries the target, and nothing else. OpenCode
submits that prompt to the model; it does not mechanically execute the skill
tool, so this feature does not claim that every invocation guarantees a load.

Carrying a target turned out to be the hard part. The host interprets command
template text, and F-18 shows it interprets text introduced through the target
as well. A command that carries a target must therefore deliver it without
handing it to any interpretation mechanism represented by the release-managed
probe contract (D5, D9). Unknown interpretation mechanisms introduced by a
future host remain an explicitly accepted residual risk.

**Outcome (human decision 2026-09-12, D10).** M1b answered that question for
OpenCode 1.18.30 and the answer is no. F-20 through F-25 establish that the
host interprets the target after substitution on every reachable path, and that
the only inert arrangement depends on a positional-index guard that moves the
failure threshold rather than removing it. Delivery is therefore withheld
entirely on this host: no command definitions are generated, no delivery is
published, and no production target-channel, probe-runner, or capability-cache
primitive is built. The epic remains specified and can be re-enabled when a
host offers an actually inert pre-interpretation transport and probes establish
that it satisfies the invariants below.

## Scope

### Included

- Seven commands, one per core skill: plan, review, beads, finalize, execute,
  verify, and progress (human decision 2026-09-10: include progress).
- Command definition content shipped as managed release payload and generated
  into the project snapshot through `svibe sync`.
- Delivery to OpenCode by plugin-side injection from the snapshot, with a
  generated-file fallback (human decision 2026-09-10).
- Sync fingerprint, status, and transactionality coverage for the new payload.
- Structural validation of command definitions.
- A target safety contract: delivering the canonical target to the model
  without the host applying any interpretation mechanism represented by the
  shipped probe contract (D5, D9).
- A versioned, isolated behavioral capability probe and machine-local result
  cache so previously unseen OpenCode artifacts can qualify without a new svibe
  release.
- Collision policy against user- and project-defined commands.
- Documentation, including the architecture amendment the plugin capability
  requires.

### Explicitly excluded

- Terminal outcome records, next-action policy, and guidance presentation.
  That is `sv-vzf`, which consumes this epic.
- Command and identifier completion. That is `sv-7sq`.
- `/svibe:do` and any autonomous execution. That is `sv-bpq`.
- Per-command `agent`, `model`, or `subtask` settings. Model selection and
  fresh-context creation remain human- and host-owned; routing and context
  isolation are separate roadmap capabilities. The command schema keeps these
  fields absent, not defaulted.
- Interpreting, resolving, or completing the *meaning* of the target. Commands
  carry it opaquely and phase semantics belong to the skills. This exclusion
  covers semantics only. How the target is delivered without being interpreted
  by the host is in scope (D5, D9).
- Host integrations other than OpenCode.
- Any change to skill resolution, precedence, or the ordinary-skill model.

## Decisions

### D1 — Commands are pointers to skills, not a second behavior definition

Each command's template requests that the model load the named `sv-*` skill via
the host's normal skill tool and apply it to the delivered target. How the
target reaches the model is settled by D5 and is not necessarily template
interpolation. The template contains no workflow instructions, no outcome
vocabulary, and no procedural content. One sentence of intent, the skill
request, and D5's one-sentence empty-target stop instruction are the ceiling.

Because the snapshot already contains the *winning* skill for each name —
project and user overrides included — a command that loads `sv-plan` by name
gets whatever skill resolution selected. Commands therefore privilege nothing:
when the model follows the request, it uses the same loading path as a human
typing "use sv-plan", and override semantics are untouched (architecture 9.1).

The deterministic contract is command registration, prompt delivery, and
target observability. Whether a model follows the prompt and calls the skill
tool remains model behavior. Actual skill loads continue to be observed by the
existing integration and are the only events that trigger capability advice.

**Alternative considered.** Embedding an abbreviated workflow contract in each
template was rejected: it creates a second place workflow behavior lives, which
drifts, and it is exactly what the bead's acceptance criteria forbid.

### D2 — Definitions ship in core and are generated into the snapshot

Command definition sources live at `core/commands/` as managed release payload.
The existing managed-payload generator and packaging pick up `core/**`
automatically, and the integrity check covers presence and hashes on every
invocation.

`svibe sync` materializes command definitions into the project snapshot beneath
`.structured-vibe/generated/opencode/`, beside `skills/`. The snapshot remains
the authoritative desired state (architecture 12.2, 13.2). Under primary
delivery it is the sole generated root; fallback mirrors its definitions into
the host convention directory under D3's explicit ownership and recovery
contract. The exact on-disk format inside the snapshot is an implementation
detail bounded by M2's outcomes; it must be machine-readable by the plugin and
contain, per command: name, description, and template.

The definitions participate in the sync fingerprint as content, not only via
the `svibe` version input, so `svibe status` reports staleness when a release
changes templates without any skill changing.

**Alternative considered.** Skipping the snapshot and having the plugin read
`core/commands/` from the svibe installation directly was rejected: the plugin
would need svibe-installation discovery it does not have, would bypass the
sync/status freshness model, and would break the rule that project-facing
state flows through the snapshot.

### D3 — Delivery is capability-selected injection or recoverable files

**Primary mechanism.** The user-level svibe plugin reads the snapshot's command
definitions at load time and injects them into OpenCode through a supported
command-registration capability. The first candidate is the command-transform
surface (`CommandHooks` in the pinned package's v2 API); M1 also evaluates the
legacy `config` hook because it may provide the same in-memory registration
through the plugin context already used for worktree discovery and warnings.
No host configuration file is written, no second generated root exists, and
project behavior continues to come from the snapshot per architecture 12.2.

M1 proves the complete installed integration, not command upsert in isolation:
worktree-correct snapshot discovery, coexistence with existing hooks, command
registration, collision handling, warnings, and observability. The plugin
registers only snapshot definitions, never executes commands, and never
modifies or removes a command visible before its own registration step.
Primary snapshot discovery covers both the repository snapshot and
architecture 13.2's user-level snapshot outside Git.

**Fallback mechanism** (human decision 2026-09-10: retain a recoverable file
fallback). When primary injection is ineligible but the observed runtime meets
fallback's own registration and target-safety contracts, `svibe sync` publishes
one markdown file per command into the documented OpenCode command directory:
`.opencode/commands/` inside Git and the user command directory outside Git.
Fallback uses hyphenated names on every platform (D4), because the file name
becomes the command name and colons are invalid in Windows filenames.

The release carries a versioned behavioral capability-probe contract derived
from M1 and M1b. It tests each delivery mode's registration capability, selected
target channel, and target-safety properties against a concrete OpenCode
artifact. Compatibility is empirical and local rather than a release-authored
version range.

A cached result is keyed by host kind, exact reported runtime version,
host artifact-set digest, plugin package/API contract, and probe-contract
version. The artifact set contains the launcher and every load-bearing host
code artifact that the identified packaging form uses for command parsing and
expansion. Only that exact key is proven; no untested version or artifact set is
included in a proven interval (human decision 2026-09-12, revision 10; R-42;
revision 12, R-52). Changing any key component requires a new result.

The capability cache is machine-local state beneath the svibe user configuration
root, outside project and user snapshots. A shared svibe probe runner owns it.
On first encounter with an uncached runtime, sync or the plugin invokes that
runner **synchronously** and waits under the caller's bounded timeout. There is
no detached continuation and no lifecycle in which a result produced after the
caller's decision activates commands at a later host restart; a run terminated
at the bound caches nothing, and the next encounter repeats the bounded attempt
(human decision 2026-09-12, revision 11; R-45). The runner serializes probes per
cache key: a concurrent caller waits on the in-flight probe within its own bound
or receives an unavailable outcome, and duplicate probing of one key is
prevented rather than raced, with atomic persistence deciding any residual race
(R-47). The runner starts an immutable private probe copy or equivalent stable
handle derived from the identified artifact set in an isolated temporary
environment with redirected host configuration, data, and cache roots, a
verified private endpoint, and the positive controls and side-effect checks
defined by M1b. The probe fixture prevents model inference after collecting the
pre-model evidence and has no access to user providers, credentials, tools, or
external network. It never loads the production plugin recursively or probes by
exercising the human's live host session. The fixture is production-faithful by
construction: M1b creates the minimal production target-channel implementation,
and both the production plugin and fixture consume that one implementation before
M1b records a passing result. M1b does not register or publish the seven commands;
it establishes the shared primitive and attestation path ahead of M2 and M3. A
passing result therefore attests the release-managed path production uses rather
than a lookalike (human decisions 2026-09-12, revisions 11 and 14; R-46,
R-56). A change to that shared implementation is a probe-contract change: it
bumps the probe-contract
version and invalidates cached results like any other key component. Locally
modified managed plugin bytes remain unsupported under architecture 16.3: they
produce the normal integrity warning but do not change the cache key or block
command delivery, and no target-safety guarantee is made for their behavior
(human decision 2026-09-12, revision 12; R-50).

A complete pass against every interpretation mechanism represented by the probe
contract, or a deterministic behavioral failure, is persisted atomically with
the key, outcome, probe-contract version, and evidence summary.
Launch failures, timeouts, missing identity, and other environmental failures
are *unavailable* outcomes: they withhold the affected commands, are not cached
as capability failures, and are retried on a later encounter. Cache corruption
likewise fails closed and is recoverable by re-probing. The cache may be pruned
as derived state; it is not a source of workflow truth.

Sync derives and hashes the OpenCode artifact set it can reach, using the same
shared derivation implementation the plugin and runner use, applied statically
to artifacts it can identify without a running host (R-53). A statically
derived digest is a valid cache-lookup key only for a packaging form where M1b
proved that static derivation and running-process-bound derivation of the same
bytes yield the same digest. Where that equivalence is unproven, or the
derivations disagree, the sync-time observation is unknown, with exactly the
behavior defined below for an unreachable executable; a lookup miss is never
resolved by version or path proximity. The derivation is one shared
implementation: changing it changes digests and therefore invalidates cache
keys naturally, without a separate versioning mechanism. Sync prefers
primary injection when the sync-visible prerequisites permit a primary record,
uses fallback only when the exact observed artifact set has a passing fallback
result, and otherwise publishes no fallback files. An unreachable executable is
an unknown sync-time observation. That does not prevent a primary record because
the plugin has a later enforcement point, but it always withholds fallback.
Changing runtime artifacts, plugin contracts, or probe-contract versions may
switch modes or withhold delivery on the next sync.

The sync-time probe can disagree with the runtime that actually loads the
plugin — multiple installs, GUI-launched hosts, or a PATH sync never saw. The
plugin is the only component that observes the true runtime, so it queries the
current host server's `/global/health` endpoint relative to the `serverUrl`
OpenCode supplied and uses that response's version as part of the authoritative
observation. Version alone is insufficient to reuse an artifact-keyed result.
M1b must also prove a trustworthy way to derive the artifact set that loaded the
host from the running host rather than from `PATH`. A same-version executable
found elsewhere is not treated as the same artifact merely because its version
string matches. Identity must bind the running process, the artifact-set digest,
and bytes the runner can launch without a path-replacement race. The runner
launches an immutable private copy or equivalent stable handle of those same
bytes. A packaging form whose complete load-bearing set cannot be identified,
hashed, and launched under that invariant is unavailable, never substituted by
a version-matched executable found elsewhere (R-48, R-52).

**The check completes before the registration decision, or nothing registers**
(human decision 2026-09-12, revision 9; R-39). Registration happens inside the
`config` hook during plugin initialization (F-01), and F-08 records that a
fetch to a non-health endpoint on the plugin's own server deadlocks during
init. These two facts previously made "refuses to inject" unimplementable as
written. The resolution is strict: the plugin races the health and artifact
identity observation, cache lookup, and any first-encounter isolated probe
against a generous hard liveness timeout inside the registration hook. A check
or probe that
times out, fails environmentally, or cannot complete before the registration
decision counts as unavailable and produces no injection. The bound is strict
but is not a performance qualification threshold:
the probe runs synchronously inside it, is terminated at it, caches nothing
from a terminated run, and leaves no background runner alive past the decision.
The affected runtime is reported as unverified, and the next plugin load repeats
the bounded attempt. Absent the hard liveness timeout, a probe runs synchronously
to completion even when it makes startup noticeably slower; duration alone never
makes a runtime incompatible. Where startup UI cannot show progress, the runner
emits periodic progress to the normal log (human decisions 2026-09-12,
revisions 11 and 12; R-45, R-51). The
enforcement point stays at registration; svibe never exposes a command it has
not yet verified can carry its target. F-07 suggests
`/global/health` may be exempt from the F-08 init deadlock, but that was not
what F-08 measured; M1b proves whether the health observation reliably
completes and whether the isolated runner can be invoked before the registration
decision. If either cannot, primary injection has no implementable enforcement
point for an uncached actual host and the result escalates back to planning; it
does not silently degrade to registering first and checking later.

For a primary record, an unavailable or unparseable health response, an
unidentifiable artifact, or anything other than a passing exact-key capability
result produces no injection. A deterministic failed result names the failed
capability; an unavailable result names the retry or environment remediation.
When a TUI is attached, the plugin surfaces the condition once per plugin load
using the F-09 fire-and-forget pattern. This check is authoritative for primary
activation: the sync-time record proposes, and the host-local check disposes.

Fallback receives no weaker target guarantee. M1b must prove a fail-closed
lifecycle with two structural properties: the published file contains no target
reference on any expansion path M1b did not clear, and the target is added only
after the actual host has satisfied the recorded target-safety contract. The
condition is structural rather than a race — F-18 places interpretation during
template expansion, strictly before `command.execute.before`, so either the file
references the target and interpretation has already happened, or it does not
and there is nothing to intervene ahead of. If M1b cannot prove that lifecycle,
fallback has no eligible target channel and is withheld. A PATH observation
or version match alone is never enough to clear this gate; the exact actual-host
artifact needs a passing cached result.

**A published fallback command never silently loses its target** (human decision
2026-09-12, revision 8; R-32). Two distinct cases follow.

Where the obstacle is observable at sync time — no plugin installed, a plugin
too old to carry a target, or no passing exact-artifact fallback result —
publication is conditioned on the same guard that would carry the target. A
missing or incompatible required integration retains architecture 13.4's hard
sync failure: sync changes no live snapshot or delivery state, and status reports
the integration remediation. Of the three obstacles above, a missing plugin and
a plugin too old to carry a target are the missing/incompatible-integration
cases and fail sync hard; an absent passing exact-artifact fallback result is
command-specific ineligibility, not an integration failure (R-64). Other
command-specific ineligibility publishes no
host command set and records the corresponding non-delivery state while obeying
the ordinary sync contract. A mode that
cannot carry a target is not delivered in a degraded form, because a command
that appears, runs, and quietly discards the argument the human typed is harder
to diagnose than an absent command.

Where the obstacle arises after publication — the plugin is removed, downgraded,
or fails to load, or a different actual-host artifact loads — the files are
already on disk and no svibe code runs to withdraw them. That residual case
degrades to the D5 floor: the static file text carries one sentence instructing
the model to stop and report that Structured Vibe command delivery is not active
when no target block is present. This is the same class as the empty-target
floor — a model-behavior concern, not a claimed control over host interpretation
(D5 consequence 2) — and it is a diagnostic, not a mitigation. The durable
diagnostic remains `svibe status`, which reports sync-visible mismatches on the
next run; the plugin reports its actual-host decision in the affected session.

Fallback publication spans the snapshot and a host command directory, so it
cannot provide the single-rename atomicity of architecture 13.5. Human decision
2026-09-10: keep fallback and make this a narrow, explicit architecture
exception. Sync plans and stages both domains, holds the same OS lock through
publication and cleanup, writes a durable transaction journal before the first
live change, and uses before-images for compensating rollback. Ordinary failure
returns the prior state in both domains. Process interruption may leave a
journaled partial state; the next `sync` recovers it before planning new work,
and `status` reports it as `delivery_pending` rather than current. Success is
reported only after snapshot state, external files, and the delivery record all
describe the same generation.

If compensating rollback itself fails, sync reports the exact rollback failure
and leaves the journal for deterministic recovery; it never writes a successful
delivery record for that generation.

The versioned delivery record contains at least: schema version, delivery mode
or withheld state, naming scheme, svibe release, probe-contract version,
required plugin/API contract, selected target channel, any sync-time capability
result key, the sync-time observation and whether it was unknown or unavailable,
generated paths, and each published content hash. It is a
cross-component contract, not an
implementation detail: sync writes it, status validates it, and the plugin
reads it to decide whether and how to carry a target. It lives at the snapshot
root beside the sync state file, outside any directory replaced during snapshot
publication, so it and the recovery journal survive the swap.

The plugin injects only when the record exists, is readable, and names a
compatible primary mode. Fallback mode, an absent record, or an unreadable
record all mean the plugin registers nothing — failing toward not injecting,
because fallback files may already provide the commands and a double
registration would expose both name forms. Migration between injection and
fallback, naming changes, command removal, and pre-command snapshots are
ordinary sync plans. Overwrite or deletion of an external file requires both
recorded ownership and the expected content hash; a modified file is left
untouched, ownership is relinquished, and delivery is reported conflicted.

**User-level fallback authority.** The outside-Git fallback files occupy
OpenCode's user command directory and are therefore visible in every
repository. Human decision 2026-09-10: while a valid user-snapshot fallback
record and its hash-matching files exist, that one hyphenated set is
authoritative in every context. A repository sync records that it inherits the
user fallback generation and publishes no project fallback files; the plugin
checks the user record before its context record and abstains from injection.
The repository record references the user-owned generation but does not claim
ownership of it.

A repository sync never creates, refreshes, or retires user-owned fallback
files. If a user fallback record is stale, modified, unreadable, or disagrees
with its files, or if any reserved user fallback path exists without recorded
ownership, repository delivery is conflicted: the plugin abstains, sync does
not publish another command set, and status names the affected paths and the
remediation to run `svibe sync` outside a Git repository. An outside-Git sync
owns recovery: it may refresh fallback, or, when its observed runtime supports
primary injection, remove the hash-matching owned files and publish a primary
user record. A later repository sync can then select its own mode normally.

**Alternatives considered.** Allowing repository colon commands to coexist
with global hyphen commands was rejected because it breaks the one-set
invariant and makes completion and invocation guidance context-dependent.
Removing file fallback outside Git was rejected because it weakens the human
decision to retain fallback precisely where no repository command directory is
available. Cross-scope mutation by repository sync was rejected because it
would turn independent project syncs into multi-lock user-state transactions.

**Alternative considered.** Registering a `command` record in project
`opencode.json` was rejected (human decision 2026-09-10): it embeds
release-owned template content in a user-owned config file, goes stale between
releases, and turns every template change into a config rewrite that sync is
forbidden to perform routinely (architecture 13.4).

### D4 — Naming: `/svibe:plan` if the host accepts it, `/svibe-plan` if not

The preferred names are `svibe:plan`, `svibe:review`, `svibe:beads`,
`svibe:finalize`, `svibe:execute`, `svibe:verify`, and `svibe:progress`,
matching the roadmap's written forms. Colon acceptance in command names is
undocumented and only reachable through injection or config registration; it
is empirically tested in M1.

If the colon form fails, the names are `svibe-plan` through `svibe-progress`
(human decision 2026-09-10). Fallback-mechanism delivery (D3) forces the
hyphen form regardless, since file names become command names.

The delivery record fixes the name form for one published generation. Injection
uses the colon form only when M1 and the applicable capability result permit it;
fallback uses hyphens. A valid user fallback forces every context to inherit
the hyphen form. A successful mode migration removes the old owned form.
`delivery_current` means exactly one effective Structured Vibe command set is
visible in the selected form at the sync-visible publication boundaries, and no
observation available to sync shows that set unable to carry its target.
`delivery_current` is deliberately a desired-delivery and filesystem state; it
does not claim that a separate running OpenCode process activated the commands
(human decision 2026-09-12, revision 10; R-43). An unknown actual-host
observation is named as activation unconfirmed, and the plugin reports its own
registration decision in-session. An observable unowned definition in either
form likewise prevents currency and is reported as conflicted. A journaled interruption may expose
both owned forms temporarily and is reported as `delivery_pending` until
recovered.

### D5 — One opaque target, delivered outside probe-represented interpretation

Each command carries exactly one canonical target and performs no semantic
parsing, validation, or completion of it. The canonical target is the exact
`arguments` string OpenCode exposes to `command.execute.before` after its own
command-line parsing. A bead identifier, a specification path, or a free-text
planning idea are all legal; meaning is phase-specific and owned by the skill
and its consumers (`sv-vzf` revision 5, D6). The contract makes no claim to
preserve quoting syntax that OpenCode removes before producing the canonical
string.

This preserves the consumer contract `sv-vzf` was specified against: one opaque
canonical target per invocation, delivered as a distinct value rather than
recovered from prose.

**Opaque is not inert.** F-18 establishes that on runtime 1.18.30, shell-output
syntax introduced *only* through `$ARGUMENTS` is reprocessed during template
expansion and executes, under both injection and file delivery. F-04 remains
correct about what it measured — the hook `arguments` value and the
`$ARGUMENTS` substitution agree — but agreement is not inertness.

Three consequences bound every later milestone:

1. **A template must not place the canonical target into text the host
   expands** unless M1b proves that path inert against every interpretation
   mechanism represented by the current probe contract for the delivery mode in
   question. The target reaches the model through a channel that the tested
   mechanisms do not rescan.
2. **Prompt text cannot mitigate this.** Host interpretation completes before
   the model sees anything. `command.execute.before` observes already-executed
   output and already-created side effects, so a hook that rewrites parts
   scrubs evidence rather than preventing the effect. Contrast the empty-target
   floor below, which is legitimately a model-behavior concern.
3. **Documenting a narrower target grammar is not a control.** The host
   interprets whatever the human types, so a declared grammar changes
   expectations and not behavior. Target restrictions may be adopted for
   usability, never claimed as safety.

M1b determines, per delivery mode, whether a probe-cleared channel exists. Until
it records one, no command definition ships (M2) and no delivery is published
(M3) for that mode. In this specification, a channel described historically as
"inert" means inert against the interpretation mechanisms represented by the
applicable probe contract, subject to D9's unknown-mechanism residual.

Empty-target handling is best-effort with a defined floor. F-05 records that
the host does not reject an empty target, so each template carries one sentence
instructing the model to stop and ask for a target when the target is empty —
within D1's ceiling — and consumers receive the empty canonical string and
apply their own validation (`sv-vzf` D6 already treats it as an invalid
record). This floor is unaffected by the inertness question.

A fallback template carries a second sentence in the same class: when the
delivery channel is expected to supply a target block and none is present, stop
and report that Structured Vibe command delivery is not active. Both sentences
are model-behavior floors that make an absent value visible rather than silent.
Neither is a control over host interpretation, and neither may be cited as one.

### D6 — Collision behavior follows observable host precedence

The integration promises not to overwrite definitions it can identify as
user-owned; it does not promise provenance or precedence the host API cannot
observe.

Both the colon and hyphen name for each phase are reserved alternatives for
delivery validation. M1's host precedence probe produces the effective-source
rules used by each component: sync and status inspect observable user/project
config and command-file sources, while the plugin inspects commands visible in
the registration draft. Before any primary, local fallback, or inherited
fallback delivery is called current, the selected Structured Vibe definition
must be effective at those observable boundaries and the alternate form must
be absent. An unowned definition that shadows the selected form or exposes the
alternate form produces `delivery_conflicted`, naming the command and
observable source; it is preserved rather than overwritten or removed. A
later plugin remains outside these boundaries under the limitation below.

- Injection skips any same-name command visible when svibe's registration hook
  runs and also skips a phase when its alternate name is visible; both cases
  warn and leave delivery conflicted. A plugin loaded later may replace the
  svibe definition under normal host plugin ordering; OpenCode exposes no
  provenance with which svibe could prevent or diagnose that later replacement.
- Fallback never overwrites an unowned same-path project or user command file.
  A generated project file otherwise participates in normal OpenCode
  precedence, but fallback is current only when the effective-source map proves
  that owned file wins and no alternate form is visible.
- Reserved user fallback paths without valid recorded ownership are global
  collisions under D3: repository delivery abstains rather than exposing a
  second colon-named set alongside them.
- Inherited user fallback is current in a repository only when no observable
  project or user config, command file, or earlier plugin definition shadows a
  selected hyphen command or exposes its colon alternative.
- Files previously generated by svibe are overwritten or removed only when the
  delivery record's path and hash both match. Modified files are preserved and
  become user-owned collisions.

These are source-specific guarantees, not a claim that every user or project
source always wins. Collision and modified-file states appear in status and
identify the affected command.

### D7 — Host sessions degrade safely; status reports delivery state

No snapshot, a snapshot without command definitions, a pre-command svibe
version, or a detached TUI never breaks a host session. The plugin may be unable
to register commands or display warnings in those states. That runtime
degradation is silent only inside the affected host session. `svibe status`
remains the durable diagnostic for sync-visible readiness and does not pretend
to observe a separate running host.

Input fingerprinting reports desired-state staleness but cannot prove delivery.
Status therefore also validates snapshot payload shape, plugin/release byte
compatibility, cached capability results, the delivery record, and
every expected fallback file and hash. It distinguishes at least:
`delivery_current`, `delivery_pending`, `delivery_missing`,
`delivery_modified`, `delivery_conflicted`, `delivery_incompatible`, and
`delivery_unsafe`, plus `delivery_probe_unavailable` when the exact runtime
cannot currently be tested. `delivery_incompatible` means every tested mode
lacks the required registration capability. `delivery_unsafe` means at least
one tested registration mechanism works, but no otherwise eligible mode has a
passing target-safety result. An unsafe fallback does not suppress a passing
primary mode, or vice versa. None of these three non-delivery states publishes
fallback files or causes primary injection.

An unknown sync-time runtime does not withhold a primary record, and it always
withholds fallback. Such a primary record may be `delivery_current` as
sync-visible desired state, but status names actual-host activation as
unconfirmed. The plugin automatically invokes the isolated probe on first
encounter with an identifiable uncached artifact and either activates from a
passing result or withholds in that session. That host-local outcome is not
persisted as delivery status; only the reusable capability result is cached.
`delivery_current` identifies whether fallback is locally delivered or
inherited from the user generation; a broken user authority is conflicted, not
current. Observable effective-source or alternate-form collisions are likewise
conflicted even when every owned file and hash is intact.

### D8 — The observability contract this epic must deliver

`sv-vzf` consumes these commands as its trigger and target carrier. The pinned
API provides `command.execute.before` (command name, session ID, arguments)
and the `command.executed` event (name, session ID, arguments, message ID).
M1 verifies both fire for commands delivered by either mechanism with correct
payloads. This epic delivers observable invocations; it does not consume them.

If commands delivered by either mechanism do not emit these signals, that
mechanism fails its M1 gate even if invocation itself works, because the
consuming epic's stated assumptions would be broken.

### D9 — Target text is untrusted input, and unsafe delivery is withheld

A target is not always a deliberate keystroke. `sv-vzf` D4 emits literal
command strings such as `/svibe:finalize <id>` for a human or a model to reuse,
and targets are plausibly pasted from generated guidance, documentation, a
model response, an issue tracker, or a teammate's message. Structured Vibe
therefore treats target text as untrusted input to the host.

The boundary this epic must hold is narrow and versioned: **invoking a
Structured Vibe command must not, by itself, trigger any host-side execution,
file resolution, or substitution mechanism represented by the shipped probe
contract.** A command is a workflow convenience. It may fail to appear, and it
may decline to run, but it must not expose a target channel that fails any
represented safety control. This is consistent with Structured Vibe not being
an agent harness and not owning tool execution (architecture 2.2, 20).

Human decision 2026-09-12: when a delivery mode cannot meet that boundary, it
delivers nothing. Withholding is preferred when a represented mechanism is
known to remain active, because that hazard would be borne during ordinary use
by a human who has no way to see it. This rule does not eliminate the accepted
unknown-mechanism residual below. Concretely, if M1b finds no probe-cleared
channel for file fallback, fallback
publishes no command files. Status reports `delivery_unsafe` only if no other
eligible mode has a safe channel; otherwise the passing mode remains available.
The existing constraint that `svibe` remains usable without the OpenCode
integration makes total withholding an acceptable degradation rather than a
loss of the workflow.

An unseen runtime is not rejected merely because it postdates the svibe release.
It is tested automatically under D3. The target is not carried until the exact
runtime artifact set has produced a passing result under the current probe
contract. A deterministic failure or a probe that cannot complete leaves
commands unavailable without making the rest of svibe unusable.

This automatic qualification deliberately makes a bounded claim (human decision
2026-09-12, revision 12; R-49). A finite release-managed probe cannot discover a
new expansion syntax or processing stage that a future host introduces but the
probe contract does not represent. A passing result therefore attests only the
enumerated interpretation mechanisms and insertion-stage observations in that
contract, not the absence of every possible host interpretation. User
documentation states this residual risk. Each release reviews current OpenCode
command-processing documentation and observed behavior and updates the probe
contract when a new mechanism is known; changing that contract invalidates old
results. The human accepted this residual in order to retain automatic
qualification of unseen artifacts rather than a release-authored allowlist.

**What this boundary does not cover** (R-35, R-49). The first residual is unknown
host interpretation outside the finite probe contract, as stated above. The
second is that the target reaches a model which does hold tool access, so target
text pasted from guidance, an issue tracker, or a teammate can attempt to
influence model behavior. Structured Vibe does not defend that half and must not
imply that it does: it is not an agent harness and does not own tool execution,
permissions, or sessions (architecture 2.2, 20), so model-side interpretation of
prompt content is owned by the host's permission model.

The distinction that makes host-side interpretation this epic's problem is that
it converts a routine invocation into an execution primitive with no model, no
tool call, and no approval anywhere in the path. Model-side influence is the
ordinary risk of putting any text in front of a model, and it is identical when
the human pastes the same string without a command.

Two consequences. M4's documentation states this residual plainly rather than
leaving a reader of D9 to conclude that a target is safe to paste. And where the
channel M1b proves makes it free, the target is delivered as a delimited,
labelled block distinct from the instruction sentence — adopted as hygiene that
keeps the data/instruction split legible, never claimed as a control, exactly as
D5's third consequence requires of anything in this class.

### D10 — Delivery is withheld on OpenCode 1.18.30

Human decision 2026-09-12, adjudicating M1b's empirical result (F-20..F-25).

**The host has no inert pre-interpretation transport for a command target.**
F-20 establishes the pipeline order, and every arrangement that lets a target
reach the model puts the target's own text through shell execution and `@file`
resolution first. Omitting the target from the template does not help, because
the host appends the raw arguments automatically before interpretation (F-22).

**A positional-index guard is not a security control.** Suppressing the
automatic append requires a positional placeholder, and a placeholder
substitutes the target's tokens once the target supplies enough of them (F-24).
`$999999` does not make a target inert; it relocates the threshold at which
interpretation resumes. Neither that guard nor any declared maximum target
length, token count, or character set may be described, documented, or relied
on as a control. This is D5's third consequence applied to the specific
mechanism M1b found.

**Fallback does not satisfy D3.** A published markdown file must carry no
target reference on an uncleared expansion path. The guard template necessarily
carries one, so file fallback has no eligible channel and publishes nothing.

**Consequences.** For OpenCode 1.18.30, and for any host that has not produced
a passing probe result under a future probe contract:

- no command definitions are generated into `core/commands/` or the snapshot;
- no primary injection and no fallback publication occur;
- no probe runner, capability cache, delivery record, artifact-derivation, or
  shared target-channel primitive is implemented;
- `svibe` continues to work exactly as it does today, because commands were
  always a host convenience rather than a workflow dependency.

**Re-enablement conditions.** This decision is scoped to an observed host
capability, not to the design. The epic may resume when a host provides a
transport that delivers the canonical target to the model *before* or *outside*
the interpretation stages — for example a first-class argument value the host
never re-scans, or a plugin insertion point that is structurally upstream of
expansion — and M1b's probe matrix, rerun unchanged, shows that target-only
shell, `@file`, and positional syntax produce no side effect under working
positive controls, with no dependence on an index, a length bound, or a
grammar restriction.

- The seven templates contain no workflow behavior; the skills remain the only
  behavior definition (bead acceptance criteria; AGENTS.md).
- Core workflow skills remain ordinary skills: no privileged loading,
  execution, or persistence semantics.
- The CLI gains no workflow subcommands; the guard test
  (`internal/cli/cli_test.go:377`) passes unmodified.
- Generated output is disposable and never hand-edited; command definitions
  are regenerated through `svibe sync` and shipped payload through releases.
- The installed plugin must match the running svibe release before command
  payload is published; status reports plugin version drift with
  `svibe admin update opencode` remediation.
- `svibe sync` continues to never rewrite OpenCode global configuration
  (architecture 13.4). Snapshot publication remains all-or-nothing; fallback
  external publication uses the journaled, recoverable exception in D3 and the
  corresponding architecture 13.5 amendment.
- The plugin injects only snapshot-derived definitions, never executes a user
  command or probes through the live host, and never mutates commands it did not
  create. The shared runner may invoke isolated probe commands only in its owned
  temporary host.
- `svibe` remains usable without the OpenCode integration; commands are a host
  convenience, not a workflow dependency. A withheld command set is an
  acceptable outcome; an unsafe one is not (D9). A silently targetless one is
  not either: no mode publishes a command it already knows cannot carry a
  target (D3, R-32).
- No delivered command may trigger a host-side interpretation mechanism
  represented by the shipped probe contract from target text. Prompt or
  template instructions never count as mitigation for host-side interpretation
  (D5). Unknown mechanisms in future hosts remain the explicit D9 residual.
- The automatic probe never enters model inference, sends provider traffic,
  exposes user credentials or tools, loads the production plugin recursively,
  or mutates a live host session. Failure to prove those properties fails M1b.
- The probe fixture and the production plugin share the target-channel
  implementation created during M1b. A fixture exercising a different insertion
  path than production has no attestation authority, and changing the shared
  implementation without a probe-contract version bump is a release-gate
  failure (R-46).
- First-encounter probing is synchronous and strictly bounded at its caller's
  decision point. No detached runner, background completion, or
  second-restart activation lifecycle exists (R-45).
- Every activation is backed by a passing capability result for the exact host
  version, host artifact-set digest, plugin/API contract, and probe-contract
  version. Version proximity is never evidence (D3).
- New persistent state is limited to the versioned delivery record, the derived
  machine-local capability cache, and a short-lived recovery journal required
  for safe fallback publication. None stores workflow state.

## Milestones

### M1 — Prove the delivery mechanism, or fall back, or stop

**Dependencies:** none.

**Testable outcomes:**

- Using explicit prototype fixtures rather than production M2 generation,
  injection is demonstrated end to end on the supported runtime: a
  snapshot-defined command appears in the host's command list, is invocable,
  and delivers its template with `$ARGUMENTS` substituted.
- The v2 command transform and legacy `config` hook are evaluated; the selected
  primary mechanism proves worktree-correct snapshot discovery from repository
  root and subdirectories and user-snapshot discovery outside Git, coexistence
  with the existing legacy plugin hooks, and human-visible warning delivery
  with safe detached-TUI degradation.
- `command.execute.before` and `command.executed` demonstrated firing for an
  injected command with correct name, session ID, arguments, and (for the
  event) message ID.
- Canonical target handling is demonstrated for a bead id, path, whitespace,
  quoting, special characters, empty input, and consecutive commands. The hook
  `arguments` string and `$ARGUMENTS` substitution agree. Whether the host can
  reject an empty target before submission is determined and recorded; if it
  cannot, the D5 template floor applies. Neither outcome fails the mechanism.
  This criterion tests agreement only. Inertness of the delivered target is
  M1b's gate and is not satisfied by any result here.
- Colon naming (`svibe:plan`) empirically accepted or rejected; the naming
  decision (D4) is recorded.
- The exact OpenCode version and host artifact set tested, plugin API/package
  version, and required injection capabilities are recorded as empirical inputs
  to the behavioral probe contract. No surrounding version range is inferred.
- The plugin's host-local runtime observation is demonstrated against the
  current server's `/global/health` through `serverUrl`, including unavailable
  and malformed responses; the observed version is proven to identify the
  server that loaded the plugin.
- If injection fails any gate above, the markdown fallback is demonstrated
  instead in Git and outside Git: a prototype command file produces an
  invocable hyphen-named command with the same observability properties.
- Fallback probes record precedence among user/project config and command-file
  sources and produce the effective-source map consumed by sync and status.
  Injection probes cover existing commands in both reserved name forms and
  earlier/later plugin transforms, documenting the host-order limitation in
  D6. These precedence probes run regardless of which delivery mechanism M1
  selects: D6 delivery validation consumes the map in every mode, including
  primary injection and inherited user fallback.
- If both mechanisms fail their gates, work stops and escalates.
- Findings are recorded against this specification, including the exact
  runtime and package versions tested.

### M1b — Prove a probe-cleared target channel, per delivery mode

**Dependencies:** M1.

This milestone exists because M1's gates measured registration, observability,
and string agreement, and none of them measured whether the delivered target
is interpreted. F-18 shows it is.

M1b is no longer prototype-only (human decision 2026-09-12, revision 14;
R-56). It may add the minimal production target-channel, artifact-derivation,
isolated runner, result-cache, and plugin-to-runner primitives required for a
production-faithful probe, plus their isolated fixture adapters. Those paths
remain dormant outside the owned probe fixture: M1b does not add the seven
command definitions, register them in a user host, publish fallback files, or
activate command delivery. M2 and M3 remain the only milestones that generate
and deliver the command set. Dormancy is attested by the regression guards in
the outcomes below, not asserted.

A release cut after M1b may ship these dormant, attestation-covered primitives
before the sv-e85.5 architecture amendment lands. That window is an explicit,
documented risk acceptance (human decision 2026-09-12, revision 15; R-62):
dormant code proven inactive by the guards does not count as shipped behavior
the architecture must describe, and the amendment must still land before any
M3 delivery behavior ships, per M3's closing outcome.

**Testable outcomes:**

- Probes run in an explicitly owned process namespace or container with a
  verified private endpoint. They never inspect, signal, or clean up
  host-managed OpenCode processes, and they never read or write real user
  configuration or command directories.
- The production-shaped probe fixture demonstrates that it stops before model
  inference, performs no provider or external-network request, exposes no user
  credentials or tools, and cannot recursively load the production plugin.
- Before testing, the finding records a closed candidate-channel inventory for
  the supported runtime from its documented command surface, package API/types,
  and observed lifecycle. The initial inventory includes direct target
  interpolation, host behavior when a static template omits a target reference,
  and plugin construction or replacement of a target-bearing message part from
  the raw hook argument. Discovery of another supported insertion stage expands
  the inventory and probe contract before a no-channel result may be recorded.
  "No channel" means none in this recorded finite inventory passed; it is not a
  claim that an undocumented host extension could never provide one (R-57).
- For every inventoried candidate channel and both delivery modes, each
  interpretation mechanism enumerated by the probe contract is exercised through target-only
  syntax and produces no side effect. The initial contract includes shell-output
  syntax creating no marker, `@file` references resolving nothing, and
  positional placeholders not substituting. Evidence is side-effect state plus
  raw hook and persisted-message parts, never string comparison and never model
  prose. A pass makes no claim about mechanisms the contract does not represent.
- Positive controls prove the harness detects each documented behavior when
  the same syntax appears directly in the template. A control that fails to
  demonstrate the behavior fails the gate rather than producing an
  inconclusive pass.
- The behavior of a template that omits any target reference is determined:
  whether the host appends the arguments anyway, and whether an appended or
  plugin-constructed message part is itself rescanned.
- For each successful channel, the finding records the exact runtime version and
  host artifact set tested, plugin package/API requirements, insertion stage,
  and the evidence that target text bypasses host expansion. These checks become
  the versioned behavioral probe contract; success proves only the exact cache
  key that was exercised.
- A successful fallback channel additionally proves the structural fail-closed
  lifecycle of D3: the published file carries no target reference on any
  uncleared expansion path, and the target appears only after the actual-host
  check. The cases proved are a plugin that is absent, one too old to carry a
  target, an unavailable or malformed health response, and an actual-host
  artifact without a passing exact-key result. In each case, raw pre-model
  message evidence must contain the D5 delivery-inactive instruction and no
  target. M1b does not enter model inference and therefore does not claim that a
  model visibly obeyed the instruction; visible output is usability behavior,
  not part of the security attestation (R-59).
- The `@file` question left open by F-19 is settled with a working positive
  control.
- Whether the plugin's `/global/health` observation reliably completes before
  the registration decision inside the `config` hook is determined on the
  tested runtime, including the timeout-raced path, since F-08 measured only a
  non-health endpoint. If it cannot complete there, primary injection has no
  implementable pre-registration enforcement point and the result escalates to
  planning rather than degrading to register-then-check (D3, R-39).
- A trustworthy actual-host artifact identity is established and matched to the
  immutable bytes the isolated runner probes. Version-string equality alone
  fails this gate. For the tested packaging form, the identity derives from the
  running process, covers the launcher and every load-bearing host code artifact
  used for command parsing and expansion, and yields an artifact-set digest plus
  an immutable private copy or equivalent stable handle the runner can launch.
  Tests replace the original path between identity and launch and prove the
  probe still runs the identified bytes or fails unavailable (R-48, R-52). The
  plugin-to-runner path is exercised during the registration hook and must
  complete before registration without touching the live host session.
- For each supported packaging form, M1b records a conservative, reproducible
  artifact-set derivation rule and the evidence that it covers command parsing
  and expansion. The rule may over-include immutable distribution files. For a
  package-based installation, the default is the launcher plus the complete
  resolved installed package or distribution root rather than an inferred
  subset of modules; for a self-contained executable, it is the executable and
  any separately loaded distribution assets. Process-bound origin evidence,
  package metadata, and the probe's loaded-artifact observations must agree. A
  form for which no defensible conservative boundary can be established is
  unavailable rather than declared complete (R-58).
- The sync-time static derivation of the artifact set is proved
  digest-equivalent to the running-process-bound derivation for the tested
  packaging form, using the shared derivation implementation. If equivalence
  cannot be proved for a form, the finding records that sync-time observations
  of that form are permanently unknown — primary records remain possible,
  fallback is withheld — rather than leaving the disagreement to be discovered
  as a silent cache miss (R-53).
- The end-to-end first-encounter probe duration — isolated host launch through
  atomic result persistence — is measured on the proven runtime. Duration alone
  never fails the compatibility gate. M1b records a generous hard liveness
  timeout derived from the observed completion behavior; its only purpose is to
  stop hangs or deadlocks. A progressing probe runs synchronously to completion,
  and when startup UI cannot surface progress the runner emits periodic normal
  log messages. Timeout remains unavailable and retriable, never a deterministic
  capability failure or a reason to adopt a background or multi-restart
  lifecycle (R-45, R-51).
- The tested host's behavior when the registration hook blocks for durations
  approaching the hard liveness timeout is determined and recorded beside the
  duration measurement: whether the host waits, aborts the hook, or skips the
  plugin, and what each means for injection. Revision 12 made host tolerance
  of a long synchronous hook load-bearing; a host that abandons a slow hook
  still yields the fail-closed no-injection result, but the behavior must be
  measured rather than assumed (R-54).
- F-18 is independently reproduced, since its original fixture tree no longer
  exists. Non-reproduction is a legitimate outcome rather than a broken
  harness, subject to the same positive-control requirement: if the plain
  `$ARGUMENTS` path proves inert under working controls, it is recorded as a
  cleared candidate channel and D5's first consequence relaxes accordingly.
  A non-reproduction with failing controls proves nothing and fails the gate.
- Dormancy of the M1b production primitives is attested rather than asserted
  (R-62). The production plugin built from the M1b tree, loaded by an isolated
  host with no delivery record — and separately with a fallback-mode record and
  an unreadable record — registers no command, spawns no runner process,
  reaches no plugin-to-runner call site, and creates nothing beneath the
  redirected svibe user configuration root. Ordinary `svibe sync`, `status`,
  and `validate` against a pre-M2 tree likewise invoke no runner and create no
  capability cache. Evidence is filesystem and process observation, never code
  review alone. These guards ship as automated regression tests and remain by
  default. The absent-record, fallback-mode-record, and unreadable-record cases
  are ongoing fail-closed invariants, not phase guards scheduled for retirement.
  A later bead may narrow or remove any guard only after recording the concrete
  reason it no longer applies and proving against the then-current code that
  equivalent safety coverage remains; activation of M2 or M3 alone is not such
  a reason. A failing guard is a delivery-gate failure, not a flake to be
  silenced (R-65).
- The outcome is recorded per delivery mode: a probe-cleared channel is proven,
  or none is available. A mode with no cleared channel is ineligible; the aggregate
  state is `delivery_unsafe` only when no eligible safe mode remains (D7, D9).
- If no probe-cleared channel exists for any mode, work stops and escalates: the epic
  cannot deliver commands that carry a target.
- The result gates the fallback work in M3. If no fallback channel is cleared,
  the fallback publication, user-authority, and transition outcomes below are
  not built; fallback remains a retained mechanism with no eligible runtime and
  the spec records that state rather than shipping machinery for it
  (human decision 2026-09-12, revision 8).

### M2 — Managed templates and snapshot generation

**Status: not built.** M1b cleared no channel on OpenCode 1.18.30, so D10
withholds this milestone. The outcomes below stand as the contract a future
host must satisfy before any template ships.

**Dependencies:** M1 and M1b (the recorded mechanism, naming, and probe-cleared
target channel decide the generated format).

**Testable outcomes:**

- `core/commands/` contains seven definition sources; the managed manifest is
  regenerated through repository tooling and integrity checks cover them.
- Each template requests its skill by name, carries the canonical target
  opaquely through the channel M1b cleared, and contains no workflow
  instructions — enforced by a test asserting template shape, not by
  convention. The shape test fails a template that routes the target through
  an expansion path M1b did not clear.
- `svibe sync` publishes command definitions into the snapshot. Publication
  keeps the registered `skills` path stable and requires no host
  re-registration. It either swaps one unit containing the generation or
  performs ordered renames under the held lock with metadata published last,
  so any crash window reads as stale, never as falsely current.
  Fault-injection tests cover every publication boundary.
- Command definition content feeds the sync fingerprint: a template-only
  change flips `svibe status` to stale.
- `svibe validate` reports structurally invalid command definitions (missing
  name, empty template, duplicate names) with named diagnostics.
- Existing environments without command payload continue to sync and validate
  cleanly.

### M3 — Deliver commands to the host

**Status: not built.** D10 withholds delivery on OpenCode 1.18.30. No probe
runner, capability cache, delivery record, artifact derivation, or shared
target-channel implementation is created while this status holds; the M1b
dormancy attestation is moot because none of those primitives exist. The
outcomes below remain the contract for a future host.

**Dependencies:** M1, M1b, and M2.

**Testable outcomes:**

**Mode-independent outcomes.**

- The release-managed probe contract, isolated fixture payload, and
  probe-contract version are embedded or packaged with the matched CLI/plugin
  release and covered by managed integrity checks. They do not come from the
  project snapshot or mutable host command directories.
- The shared runner safely probes the immutable identified host artifact set in
  an owned temporary host
  and atomically caches complete pass or deterministic failure results under the
  full D3 key. Tests vary the reported version, artifact-set digest, plugin/API
  contract, and probe-contract version independently and prove no result is
  reused across a key change.
- Concurrent invocations of the runner for one cache key are single-flighted:
  tests prove a second caller waits within its own bound or receives
  unavailable, that duplicate isolated hosts are not spawned for one key, and
  that atomic persistence resolves any residual race (R-47).
- A structural test proves the production plugin and the probe fixture consume
  one shared target-channel implementation, and the release gate fails when
  that implementation changes without a probe-contract version bump (R-46).
- Positive-control failure, process launch failure, timeout, malformed evidence,
  and cache corruption are unavailable rather than deterministic capability
  failures. They publish no fallback and cause no primary injection, are not
  cached as failed capability, and can succeed on a later automatic retry.
- Sync chooses a mode only when its sync-visible prerequisites satisfy D3 and
  records the chosen mode, naming scheme, target channel, observation, and any
  capability-result key. Status reports incompatible, unsafe, probe-unavailable,
  or changed artifacts rather than inferring compatibility from version ranges.
- An unknown sync-time observation writes a primary record and withholds
  fallback; tests prove both halves, and prove the plugin's own check still
  refuses an unidentified artifact or one without a passing exact-key result.
- Sync-time and plugin-side artifact-set derivations are exercised against one
  artifact set of the tested packaging form and produce the same digest. A
  forced derivation mismatch produces the unknown-observation behavior — a
  primary record with fallback withheld — rather than a false capability miss
  reported as incompatibility (R-53).
- `delivery_unsafe` is emitted only when no eligible mode is safe. Tests prove a
  deterministic unsafe fallback does not suppress safe primary injection and a
  deterministic unsafe primary does not suppress an independently passing
  fallback candidate.
- No mode publishes a command set it already knows cannot carry a target.
  Tests cover the sync-time-observable obstacles — no plugin, a plugin too old,
  and no passing exact-artifact result — and assert non-publication rather than a
  degraded publication (R-32). The missing or incompatible required-plugin
  cases fail sync before mutation under architecture 13.4; they do not commit a
  partial skill-only snapshot (R-61).
- Status tests enforce the D4 boundary: `delivery_current` describes
  sync-visible desired delivery, names unknown actual-host activation as
  unconfirmed, and never claims to report a separate host process's registration
  result. A sync-visible failed or unavailable result is not current.
- Delivery tests assert end-to-end absence of every target interpretation
  represented by the probe contract for whatever mode ships, by side effect,
  not by comparing strings.

**Primary-mode outcomes.**
- Under the primary mechanism: the plugin injects all seven commands from the
  correct snapshot at load time; each is invocable and observable per D8.
- The plugin consults the delivery record before registering: fallback mode,
  an absent record, an unreadable record, and a mode incompatible with the
  plugin's own observed runtime each produce no injection, and the
  unavailable or incompatible cases produce the actionable D3 warning.
- The plugin obtains its authoritative runtime version from the current host's
  `/global/health` endpoint via `serverUrl`, identifies the exact loading
  artifact, and obtains or creates the matching capability result through the
  shared runner. Tests cover cached pass, cached deterministic failure,
  first-encounter pass and failure, unavailable and malformed health responses,
  artifact-identity failure, probe timeout, and detached TUI. Every non-pass
  produces no injection, never register-then-check (D3, R-39). A probe timeout
  terminates the run, caches nothing, and leaves no background runner alive
  past the registration decision; tests prove the next load repeats the bounded
  attempt from a cold result (R-45).
- Injection skips same-name commands visible at registration time with a
  warning, and also skips an affected phase when its alternate name is visible;
  both states are conflicted. Tests document that later plugins may replace
  svibe under host ordering. Svibe never modifies or removes a command it did
  not create.
- Missing snapshot, missing command payload, malformed payload, and detached
  TUI produce no injection and no session failure. Malformed payload produces a
  visible warning when a TUI is attached and a durable status diagnostic in all
  cases.
**Fallback outcomes, conditional on M1b.** These are built only if M1b clears a
fallback target channel (human decision 2026-09-12, revision 8). If it does not,
fallback is a retained mechanism with no eligible runtime: the work below is not
performed, the corresponding beads close as not-applicable, and status reports
the withheld state under D7.

- Fallback tests prove the target is absent from host-expanded file-template
  text and is added only after the authoritative actual-host check passes. An
  absent plugin, a plugin too old to carry a target, an unavailable or malformed
  health response, an unidentified artifact, or an artifact without a passing
  exact-key result each carries no target, causes no target-derived shell
  execution or file resolution, and leaves a pre-model message containing the
  D5 delivery-inactive instruction rather than an apparently empty invocation.
- Under fallback, sync publishes hash-tracked files to project command storage
  inside Git and user command storage outside Git. `svibe init` idempotently
  manages exact generated-file `.gitignore` entries for new and existing
  projects; fallback sync refuses publication and names that remediation when
  required entries are absent.
- A valid user fallback is inherited by repository sync without duplicate
  project files or injection. Tests cover repository primary and fallback
  candidates while user fallback is active, stale or modified user fallback,
  unowned reserved user paths, outside-Git refresh and retirement, and the
  transition back to repository-selected delivery. Repository records never
  acquire ownership of user fallback files.
- Delivery validation tests cover same-name and alternate-form definitions in
  observable user and project config, command files, and earlier plugin state.
  They include project sources shadowing an inherited user fallback and prove
  that intact owned hashes cannot produce `delivery_current` when another
  effective definition wins.
- Fallback publication, rollback, interrupted recovery, modified owned files,
  missing/lost delivery records, and cleanup are tested. User-modified or
  unowned files are never overwritten or deleted.
- Primary↔fallback and colon↔hyphen transitions, command removal, and migration
  from a pre-command snapshot remove only positively owned old output and leave
  exactly one successfully delivered command set.
**Closing outcomes.**

- Status fixtures cover every D7 delivery state that the shipped modes can
  reach, including unknown-activation and probe-unavailable advisories.
  Consecutive syncs are idempotent in every shipped mechanism.
- Before any fallback or delivery-record behavior ships, architecture sections
  2.4, 4, 12.4, 13 through 15, 17.1, 20, and 22 are amended as applicable. They
  define isolated no-inference behavioral qualification as bounded CLI
  infrastructure, permit the disposable exact-key capability cache while
  retaining the prohibition on authoritative or general-purpose runtime caches,
  preserve the missing/incompatible-integration hard sync failure, and document
  the fallback location, recovery journal, narrow transaction exception,
  isolated probe runner, capability cache, compatibility check, delivery record,
  user-level fallback authority, and sync-bounded status states. The
  authoritative architecture never contradicts shipped behavior.

### M4 — Documentation and user-facing alignment

**Status: reduced to the withheld state.** While D10 holds, the only
user-facing documentation required is that Structured Vibe ships no OpenCode
commands, why (host-side interpretation of target text), and that `svibe`
is unaffected. `docs/ROADMAP.md` records the capability as blocked on the host
rather than planned. The outcomes below apply if delivery is re-enabled.

**Dependencies:** M3.

**Testable outcomes:**

- User documentation lists the seven commands, their mode-dependent names from
  D4, and the target-argument convention. It states both D9 residuals plainly:
  the probe excludes represented host-side interpretation but cannot detect
  unknown future mechanisms, and model-side interpretation remains owned by the
  host's permission model.
- Documentation explains first-encounter capability probing, the exact-artifact
  cache key, automatic retry of unavailable probes, and remediation for each
  non-delivery state. It states that status reports sync-visible readiness while
  the plugin reports actual-host activation in-session. It also states that a
  pass covers the interpretation mechanisms represented by the shipped probe
  contract, not unknown mechanisms a future host may introduce.
- The `sv-vzf` consumer assumptions are explicitly confirmed or corrected
  against the delivered mechanism: one opaque target argument, invocation
  observability with correlation identity, and the signal payloads available
  for a terminal-record transport.
- `docs/ROADMAP.md` gains the entry for this epic or records it as delivered.
- `docs/specs/releasing.md` and the architecture release layout include
  `core/commands/` and identify where the probe contract, isolated fixture,
  shared target-channel implementation, runner, and contract-version metadata
  reside, whether embedded in existing binaries/plugin artifacts or packaged
  separately. Every platform archive is checked for all seven valid definitions
  and every required probe artifact, and a packaged-runtime test materializes
  them.
- `docs/specs/releasing.md` requires every release to validate the shipped probe
  contract and runner with working positive and negative controls against the
  release's designated test runtime. Failure blocks the release. The gate proves
  the probe and supported delivery contract, not a closed list of future host
  versions; unseen artifacts are tested locally on first encounter (human
  decision 2026-09-12, revision 10; R-44). The same document records the
  per-release obligation to review current OpenCode command-processing
  documentation and observed behavior and to update the probe contract when a
  new mechanism is known (D9), and the release checklist enforces that review
  so the R-49 residual mitigation cannot silently lapse (R-55).
- Repository validation, Go tests, integration type-checking, and managed
  payload checks pass.

## M1 Findings

Recorded 2026-09-10 by `sv-e85.1` against OpenCode runtime **1.18.30** with
pinned package **@opencode-ai/plugin 1.18.15**, using `opencode serve` in an
isolated sandbox. Prototype fixtures only; no production payload was generated.

**F-01 — Selected primary mechanism: the legacy `config` hook.** It registers
new commands by mutating `input.command`, which is a map of name to
`{ template, description?, agent?, model?, subtask? }`. Registered commands
appear in `GET /command` with `source: "command"`, and the host derives
`hints: ["$ARGUMENTS"]` automatically. `agent`, `model`, and `subtask` were
omitted and read back as `null`, satisfying the D3 exclusion.

**F-02 — The v2 `CommandHooks` transform is not viable, for two independent
reasons.** `CommandDraft` exposes only `list`, `get`, `update`, and `remove` —
there is no create path. Independently, runtime 1.18.30 refuses to load the v2
plugin shape at all: `Plugin ... must default export an object with server()`.
D3's "first candidate" framing is therefore inverted; the legacy hook is the
only registration path on this runtime.

**F-03 — Colon naming is accepted.** `svibe:probe` registered and invoked
successfully end to end. **D4 resolves to the colon form** for injected
delivery. The hyphen form also works, so fallback naming remains available.

**F-04 — The canonical target is an exact passthrough.** Verified for a bead
id, a path, leading/internal/trailing whitespace, embedded single and double
quotes, and shell metacharacters (`& | ; $HOME` backticks `#`). The host strips
nothing and alters nothing between the two observation points;
`command.execute.before`'s `arguments` string and the `$ARGUMENTS` substitution
agreed in every case. D5's caveat about quoting the host removes is vacuous on
this runtime — but retain it, since it costs nothing and the behavior is
undocumented.

**Amended in revision 6.** As originally written this finding said the host
"expands nothing". That overstates the evidence. What was measured is
*agreement between the hook value and the substituted value*. It says nothing
about what happens to the substituted text afterward, and F-18 shows that text
is reprocessed. Read F-04 as a passthrough result at the observation boundary
only; do not read it as an inertness result.

**F-05 — The host does not reject an empty target.** An empty `arguments`
value returns HTTP 200 and substitutes to the empty string. **D5's template
floor applies**: each template must carry the one-sentence stop instruction.

**F-06 — Both observability signals fire with full payloads.**
`command.execute.before` provides `{command, sessionID, arguments}` and the
mutable `output.parts` carrying the substituted template. The `command.executed`
event provides `{name, sessionID, arguments, messageID}`. Distinct session and
message identifiers were observed per invocation, and consecutive commands in
one session did not contaminate each other. This satisfies D8 and the
correlation identity `sv-vzf` assumes.

**F-07 — `/global/health` exists on 1.18.30 and `serverUrl` identifies the
loading server.** The plugin receives `serverUrl` in its input, and
`GET {serverUrl}/global/health` returned `{"healthy":true,"version":"1.18.30"}`
from the exact server that loaded the plugin. An unreachable endpoint fails
fast with a connection error. D3's plugin-side runtime check is implementable.

**F-08 — The plugin must not query its own server during initialization.** A
`fetch` to a non-health endpoint on its own server during init timed out: the
request handler waits on plugin initialization, which is waiting on the
request. The runtime check must run lazily or after init completes.

**F-09 — `client.tui.showToast()` never resolves when no TUI is attached.** It
does not throw; it hangs indefinitely. A `try`/`catch` is insufficient
protection, and an awaited toast during init will hang host startup. Warning
delivery must be fire-and-forget or timeout-raced. This is a correction to the
existing integration pattern, not only a new requirement.

**F-10 — `worktree` is unusable for detecting non-Git context.** Launched from
a nested subdirectory, `directory` was the launch directory and `worktree`
correctly resolved to the repository root, so in-repo discovery is
worktree-correct. But launched outside Git, `worktree` was `/` rather than
empty, while `directory` was the launch directory. Code of the form
`worktree || directory` therefore silently selects the filesystem root outside
Git. Discovery of the architecture 13.2 user-level snapshot must test for this
explicitly.

**F-11 — Plugin coexistence holds, and plugin failure is isolated.** The probe
registered commands correctly alongside the installed user-level integration at
`~/.config/opencode/plugins/svibe.js` and the repository source plugin. A
plugin that fails to load is logged and skipped without affecting other plugins
or server startup. Project configuration and relative plugin paths resolved
correctly from a nested subdirectory.

**Gate result: the primary mechanism passes.** Injection is viable on 1.18.30
via the `config` hook, with colon naming, exact target passthrough, and full
observability. The version skew between the pinned 1.18.15 package and the
tested 1.18.30 runtime is recorded but was not load-bearing for any result
above. Revision 10 supersedes the original tested-range conclusion: M3 keys a
behavioral result to the exact runtime artifact and plugin/probe contracts.

Recorded 2026-09-11 by `sv-e85.2` against the same runtime **1.18.30**, using
`opencode serve` in an isolated sandbox with `XDG_CONFIG_HOME` redirected so
no real user command file was read or written. Prototype fixtures only.

**F-12 — File fallback delivers working commands in both contexts, with the
same observability as injection.** A project file at
`.opencode/commands/svibe-file.md` registered and invoked successfully inside
Git; a user file at `<config>/opencode/commands/svibe-user.md` did the same
outside Git. In both cases `command.execute.before` carried the substituted
template and `command.executed` carried name, session ID, arguments, and
message ID, with exact `$ARGUMENTS` passthrough. Project command files are
correctly invisible outside their repository. The documented directories in D3
are confirmed, including the plural `commands/` spelling at both scopes. This
run also independently reconfirmed F-10: outside Git, `worktree` was `/` while
`directory` was the launch directory.

**F-13 — Effective-source precedence map.** For a single command name defined
in every source simultaneously, the host resolved them in this order, highest
first:

1. plugin injection via the `config` hook;
2. project command file (`.opencode/commands/`);
3. user command file (`<config>/opencode/commands/`);
4. project config (`opencode.json` `command`);
5. user config (`<config>/opencode/opencode.json` `command`).

Two results here are not obvious and are load-bearing for D6: **command files
outrank config entries at both scopes**, and **plugin injection outranks every
file and config source**. The latter means a plugin that writes
unconditionally silently replaces user content, which is precisely the
behavior D3 and D6 forbid svibe from exhibiting.

**F-14 — D6's skip rule is enforceable against every non-plugin source.** The
`config` hook receives a command map in which all four non-plugin sources are
already merged, including their templates. A probe injector observed the
winning project-file definition before writing its own. The plugin can
therefore detect a same-name command originating from any file or config
source and abstain, as D6 requires. This inverts the risk flagged when this
bead was finalized: the concern was that file-defined commands might be
resolved after plugin hooks and so be undetectable. They are not.

**F-15 — Alternate-form detection is enforceable.** A colon-named command
defined in user config was present in the map at plugin-hook time, and both
`svibe-clash` and `svibe:clash` were visible simultaneously in the merged
list. The plugin can therefore check for the reserved alternate form before
registering. Note the asymmetry D6 must encode: the colon form is reachable
only through config or injection, never through files, because the filename
becomes the command name.

**F-16 — The host API exposes no provenance.** Every command reports
`source: "command"` regardless of whether it came from a project file, a user
file, project config, user config, or plugin injection. The plugin can observe
a command's presence, name, and template, but cannot attribute its origin.
Consequently D6's plugin-side diagnostics can report *that* a conflict exists
and name the command, but cannot name the source; attribution must come from
sync and status inspecting the filesystem and config directly. Template
content is the only discriminator available in-process, and it is not a
trustworthy ownership signal on its own.

**F-17 — Plugin ordering is last-writer-wins.** With two injectors registering
the same name, the later one in the plugin list won, in both orderings tested.
An earlier plugin can see nothing of a later one, while a later plugin sees the
earlier definition. This confirms the host-ordering limitation D6 already
documents: svibe can detect and defer to plugins loaded before it, but cannot
prevent or observe replacement by a plugin loaded after it.

**Gate result: the fallback mechanism passes, and the precedence map is
complete.** Both delivery mechanisms are viable on 1.18.30, so the human
decision to retain a file fallback is supported by evidence rather than
assumption. No escalation condition was reached.

**Both M1 gate results above predate the inertness question.** They establish
that commands register, invoke, and are observable. They do not establish that
the target they carry is safe to carry. F-18 is the reason M1b exists.

Recorded 2026-09-11 by `sv-e85.14`, which escalated rather than completing.
Runtime **1.18.30**, pinned package **@opencode-ai/plugin 1.18.15**, in a
sandbox with `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, and `XDG_CACHE_HOME`
redirected.

**F-18 — Syntax introduced only through `$ARGUMENTS` is reprocessed, in both
delivery modes.** Direct template controls produced `INJ_CONTROL_OUTPUT` and
`FILE_CONTROL_OUTPUT` and created their sandbox markers, establishing that the
harness detects the documented behavior. In the matched target cases, where
the shell expression appeared *only* inside the arguments substituted at
`$ARGUMENTS`, `command.execute.before` still reported the original expression
in its `arguments` value — consistent with F-04 — while its output parts and
the persisted user-message parts contained `INJ_TARGET_OUTPUT` and
`FILE_TARGET_OUTPUT`, and both target-only markers were created on disk.

Two consequences follow directly. Execution occurs during template expansion,
**before** `command.execute.before` runs, so a plugin that rewrites parts at
that hook removes the evidence and not the side effect. And because the result
reproduces under markdown-file delivery, it is not a property of plugin
injection that a different registration path would avoid.

The same run found that positional placeholders behave differently: direct
positional controls substituted, while placeholders introduced through
`$ARGUMENTS` remained literal. Reprocessing is therefore selective, which is
precisely why each syntax needs its own control rather than an inference from
the others.

*Provenance caveat.* The fixture tree under `/tmp/opencode/sv-e85-14` was
subsequently removed by a temporary-directory cleaner, so the raw
`observations.jsonl`, server log, and markers backing this finding no longer
exist. The finding is recorded from the escalation record. M1b must reproduce
it independently before any mitigation depends on its details.

**F-19 — `@file` resolution through `$ARGUMENTS` is unsettled.** The `@file`
positive controls in that run did not resolve, so the negative target-case
result is inconclusive: a harness that cannot demonstrate the documented
behavior cannot prove its absence. This is an independent risk from F-18 — if
`@` references resolve after substitution, a target can pull arbitrary
readable file contents into the prompt — and it needs its own working control
under M1b.

Recorded 2026-09-12 by `sv-e85.14` during M1b execution, which escalated
before implementing any production primitive. Runtime **1.18.30**, executable
SHA-256 `87bd160e053af86b5b409daabf71f8dc05bbc3a2a3a5f563f36011cdf706a999`,
pinned package **@opencode-ai/plugin 1.18.15**. Probes ran from an immutable
private copy of that exact executable inside an owned user/PID/network
namespace with redirected `HOME`, `SVIBE_CONFIG_HOME`, `XDG_CONFIG_HOME`,
`XDG_DATA_HOME`, and `XDG_CACHE_HOME`. External network was unreachable by
construction, the configured provider pointed at an owned loopback sink that
recorded zero bytes, and the fixture aborted at `chat.params`, so no inference
ran. Evidence is retained at `/tmp/opencode/sv-e85-14/evidence-run4` and
`/tmp/opencode/sv-e85-14/evidence-run5`.

**F-20 — The command pipeline order is now known exactly, and it explains
F-18.** For every command, regardless of delivery mode, 1.18.30 performs, in
this order: positional `$n` substitution against the **template only**;
`$ARGUMENTS` substitution; **automatic append of the raw arguments when the
template contains neither a positional placeholder nor `$ARGUMENTS`**; shell
execution of every `` !`...` `` occurrence in the **already-substituted**
string; `@file` resolution over that same string; and only then
`command.execute.before`. Interpretation therefore operates on text the target
contributed, and the hook is downstream of all of it.

**F-21 — F-18 is independently reproduced, and `@file` is settled as unsafe
(F-19 closed).** With working direct-template positive controls for all three
mechanisms in both delivery modes, target-only syntax introduced through
`$ARGUMENTS` executed shell commands and created its markers, and a target-only
`@path` caused the host to invoke its Read tool and attach the file. Positional
placeholders introduced through the target remained literal, because positional
substitution reads the template only. The same results hold for plugin
injection and for markdown-file delivery.

**F-22 — Omitting the target from the template is not a safe channel.** A
static template with no target reference still received the target through the
automatic append, and the appended text was then shell-executed and `@file`
resolved exactly as the direct case. The candidate channel named in the
specification's open questions is therefore refuted as written.

**F-23 — Reassigning `output.parts` in `command.execute.before` is silently
discarded; in-place mutation works.** The host passes its own array to the hook
and then prompts with that same array, so `output.parts = [...]` has no effect
and the target is dropped entirely. Mutating the array in place does take
effect, and a text part inserted that way is **not** rescanned: a target
containing `` !`...` ``, `@path`, or `$1` reached the persisted pre-model
message verbatim, created no marker, resolved no file, and substituted nothing.

**F-24 — A target-inert channel exists on 1.18.30, but only behind an
index-bounded guard.** Because the automatic append fires whenever the template
references nothing, suppressing it requires the template to contain a
positional placeholder, and any such placeholder substitutes the target's own
tokens once the target supplies enough of them. A template carrying a
deliberately high index (`$999999`) plus in-place part insertion produced a
fully inert target in both delivery modes. A bound probe with `$5` and a
five-token target proved the guard is not structural: the tail token was
substituted, shell-executed, and `@file` resolved. The threshold is an index,
not an invariant, so this channel's safety depends on a target never reaching
the guard's token count.

**F-25 — The pre-registration enforcement point is implementable.** Inside the
`config` hook, `GET {serverUrl}/global/health` completed in 6 ms and returned
`{"healthy":true,"version":"1.18.30"}` before any command was registered,
confirming that the F-08 initialization deadlock does not apply to the health
endpoint and answering R-39's M1b outcome affirmatively.

**Gate result: no probe-cleared channel on this host; delivery withheld.** The
only inert arrangement found depends on a positional index rather than a
structural guarantee, and the fallback form of it cannot satisfy D3's
no-target-reference requirement. Human decision 2026-09-12 adjudicated this
under D9 and D5's third consequence: withhold delivery rather than ship a
bounded guard or call a target-length restriction a control. D10 records the
decision, its consequences, and the conditions for re-enablement. M1b is
complete; the candidate-channel inventory it recorded is closed for 1.18.30 and
the remaining milestones are not built for this host.

## Open Questions

All three empirical questions that gated M2 and M3 are now closed:

- **Does a probe-cleared target channel exist, per delivery mode?** **No, on
  OpenCode 1.18.30.** The injection candidate named here — a template that
  never references the target, with the plugin constructing the target-bearing
  part — is refuted by F-22: the host appends the raw arguments automatically
  and interprets them. The only inert arrangement needs a positional-index
  guard, which D10 rejects as a control. Markdown fallback has no eligible
  channel at all. D9's withholding rule applies, as adjudicated in D10.
- **Can the plugin identify the exact host artifact set that loaded the actual
  host?** **Moot while D10 holds**, since nothing is delivered and no
  capability cache exists. It returns as an open question only if a future host
  clears a channel.
- **Can a first-encounter isolated probe complete before registration without
  entering model inference or recursively loading the production plugin?**
  **Partly answered and otherwise moot.** F-25 shows the pre-registration
  observation point is reachable and that an isolated no-inference fixture is
  constructible. The remaining runner and cache lifecycle is unbuilt under D10.

The historical decisions below stand, superseded where D10 conflicts with them.
Human decision 2026-09-12:
unsafe delivery is withheld rather than shipped with a documented hazard, and
revision 8 adds that a knowingly targetless delivery is withheld on the same
reasoning. Template work, the final architecture amendment, and delivery work
are gated on the outcome, and the fallback half of M3 is gated on a cleared
fallback channel specifically. The delivery mechanism, naming fallback, command
set, loading guarantee, and recoverable fallback transaction remain as
adjudicated 2026-09-10; fallback is retained as a mechanism, and whether any
runtime is eligible for it is now M1b's result rather than an assumption. The
snapshot, capability-cache, and journal encodings are implementation details
bounded by the contracts and milestones above.

## Assumptions

- At least one in-memory command-registration path can satisfy every primary
  M1 gate on a concrete OpenCode artifact; M1 verifies rather than trusts this.
  Fallback absorbs primary failure only where fallback's independent
  registration and target-safety probe passes for the exact artifact.
- OpenCode continues reading `.opencode/commands/` as the project command
  convention directory (documented behavior; load-bearing only in fallback).
- `command.execute.before` and `command.executed` fire for injected commands
  the same way they do for file-defined commands. M1 verifies.
- One canonical opaque target string is sufficient for all seven commands;
  nothing in the current skills requires a second positional argument. This
  assumption survives F-18, but opacity is no longer free: it now requires a
  delivery channel the host does not interpret (D5).
- A target-bearing message part constructed by the plugin, or appended by the
  host from a template that never references the target, is not itself
  rescanned. This is the central assumption behind the primary candidate
  channel and is unverified. M1b verifies rather than trusts it.
- The actual host artifact set can be identified strongly enough for the plugin
  to select the matching cache key, and the isolated probe can finish before a
  generous hard liveness timeout on first encounter. M1b verifies both rather
  than treating version equality as sufficient; ordinary probe duration is not
  a compatibility criterion.
- Injecting commands at plugin load reflects snapshot state at host startup;
  a resync during a session is not visible until restart. This matches the
  existing skills model and `sv-vzf` D9's restart-aware drift handling.

## Review record

Reviewed by `sv-review` across alternating models: the revision 1 review
produced findings R-01 through R-10; the revision 2 review, on a different
model from the revision 2 author, produced R-11 through R-15 (zero blocking);
the revision 3 review produced R-16 and R-17; the revision 4 review produced
R-18 (zero blocking).

Resolutions:

- **R-01** (blocking) accepted with human adjudication: fallback remains, and
  D3 defines the narrow architecture 13.5 exception, journal, compensating
  rollback, recovery, and non-current partial state.
- **R-02** (blocking) accepted with human adjudication: the command guarantees
  prompt delivery and observability and requests a skill load; it does not
  claim a model tool call is deterministic.
- **R-03** accepted: D6 defines enforceable source- and mechanism-specific
  collision guarantees rather than universal user precedence.
- **R-04** accepted: D3 adds an explicit runtime compatibility contract,
  sync-time mode selection, status checking, and fallback for incompatibility.
- **R-05** accepted: M1 must prove snapshot discovery, legacy-hook coexistence,
  warning channels, and observability in one installed integration; the legacy
  `config` hook is evaluated alongside v2 command transforms.
- **R-06** accepted: D3 defines a versioned hash-bearing delivery record,
  ownership safety, recovery, and migrations; D7 and M3 define delivery status
  and transition tests.
- **R-07** accepted: both primary discovery and fallback delivery now cover the
  architecture 13.2 user snapshot outside Git.
- **R-08** accepted: D5 defines the hook-provided argument string as canonical,
  and M1 covers normalization, special cases, emptiness, and consecutive calls.
- **R-09** accepted: the bead acceptance criteria are updated to seven commands
  and the request-loading guarantee.
- **R-10** accepted: M4 updates the release specifications and verifies every
  platform archive contains valid command definitions.
- **R-11** accepted: D3 names the sync-time probe, treats unreachable runtimes
  as unknown, and adds the authoritative plugin-side runtime check with a
  visible warning on mismatch. Revision 7 superseded its original
  unknown-to-fallback policy after R-30's security finding; revision 8 restored
  primary publication on an unknown observation under R-33, since the plugin
  check R-11 and R-17 added is the enforcement point that makes it safe.
- **R-12** accepted: the delivery record is an explicit cross-component
  contract at the snapshot root, surviving publication; the plugin injects
  only on a readable, compatible primary record and otherwise abstains.
- **R-13** accepted: empty-target rejection is best-effort with the D5
  template floor; neither outcome fails the delivery mechanism, and D1's
  ceiling admits the one-sentence stop instruction.
- **R-14** accepted: the architecture amendments moved into M3's completion
  criteria so the authoritative documents never contradict shipped behavior.
- **R-15** accepted: M2 permits ordered renames with metadata-last crash
  semantics and requires the registered skills path to remain stable.
- **R-16** accepted with human adjudication: a valid user-level fallback is the
  one authoritative command set in every context. Repository sync inherits it,
  repository plugins abstain, and invalid or unowned global fallback state
  conflicts rather than causing dual registration.
- **R-17** accepted: the plugin observes the actual host through
  `serverUrl`'s `/global/health`, abstains when that check is unavailable or
  incompatible, and gives host-update/restart guidance instead of prescribing
  a sync that would reproduce the mismatch.
- **R-18** accepted: both name forms are reserved alternatives; M1's precedence
  probe becomes an effective-source map consumed by sync and status, and no
  mode is current when an observable unowned source shadows the selected form
  or exposes the alternate form.

The revision 5 review produced R-20 through R-29, all arising from the `sv-e85.14`
escalation. Its numbering began at R-20; **R-19 was never assigned** and has no
disposition to record (R-63). That review ran in the same session that adjudicated the
escalation, so it was not independent; revision 6 should be re-reviewed on a
different high-capability model.

- **R-20** (blocking) accepted: D5 is rewritten to separate opacity from
  inertness and to state the reprocessing behavior directly.
- **R-21** (blocking) accepted with human adjudication: D9 defines target text
  as untrusted input and states the execution boundary; scope no longer
  excludes delivery-side target handling.
- **R-22** (blocking) accepted with human adjudication 2026-09-12: if fallback
  cannot deliver an inert target it publishes nothing and reports
  `delivery_unsafe`. The 2026-09-10 decision to retain fallback stands; it is
  retained as a mechanism, not as a guarantee that it will always be usable.
- **R-23** accepted: F-04 is amended in place rather than only superseded.
- **R-24** accepted: M1's target criterion is explicitly scoped to agreement,
  and M1b and M3 add side-effect inertness gates.
- **R-25** accepted: F-19 records the inconclusive `@file` control and M1b
  gates it separately with a working positive control.
- **R-26** accepted: D7 gains `delivery_unsafe`, distinguished from
  `delivery_incompatible`.
- **R-27** accepted: D5 states that prompt text cannot mitigate host-side
  interpretation, and contrasts it with the empty-target floor.
- **R-28** accepted: the status line, open questions, and assumptions are
  corrected.
- **R-29** accepted: the sufficiency assumption is retained with its new cost
  stated.

The independent revision 6 review produced R-30 and R-31:

- **R-30** (blocking) accepted with human adjudication 2026-09-12: compatibility
  is now per mode and includes the target-safety contract. Unproven runtimes
  publish nothing, fallback requires an actual-host fail-closed path, and the
  delivery record identifies the cleared target channel. Revision 8 narrows the
  unknown-runtime half of this under R-33: unknown still withholds fallback, and
  no longer withholds the primary record whose contract the plugin enforces.
- **R-31** accepted: the architecture amendment is gated on M1b so it records
  the target-delivery capability and withholding behavior M1b actually proves
  rather than speculating ahead of the result.

The independent revision 7 review produced R-32 through R-38:

- **R-32** (blocking) accepted with human adjudication 2026-09-12: a mode never
  publishes a command set it already knows cannot carry a target. The
  sync-observable obstacles condition publication; the residual post-publication
  case degrades to a D5 model-behavior floor that reports delivery is inactive;
  and `delivery_current` now requires target carriage, not only visibility.
- **R-33** accepted with human adjudication: the unknown-runtime rule is split
  per mode. Primary injection may publish on an unknown sync-time observation
  because the plugin enforces the contract against the actual host; fallback,
  which has no such enforcement point for registration, still withholds. This
  narrows R-30's rule without weakening it, since the enforcement it relied on
  was already in the primary path.
- **R-34** accepted with human adjudication: runtime ranges become a proven band
  from versions actually probed plus a provisional band covering later patches
  of the same minor, delivered with an advisory. Beyond that, delivery is
  withheld. Each release carries a re-probe obligation. Revision 10 supersedes
  this with automatic exact-artifact capability probing after R-42 and R-44.
- **R-35** accepted: D9 states the residual explicitly. Host-side interpretation
  is closed by this epic; model-side interpretation is not, and is owned by the
  host's permission model. Delimited target blocks are adopted as hygiene where
  free, never claimed as a control.
- **R-36** accepted with human adjudication: fallback is retained as a mechanism
  per 2026-09-10, and its M3 work is gated on M1b clearing a fallback target
  channel. If none is cleared, the machinery is not built and the beads close as
  not-applicable.
- **R-37** accepted: the fallback guard is stated structurally rather than as a
  race, since F-18 places interpretation before `command.execute.before`.
- **R-38** accepted: M1b records non-reproduction of F-18 as a legitimate
  outcome under working controls, which would reopen the plain template as a
  cleared channel; non-reproduction with failing controls fails the gate.

The review also identified a pre-existing release-cancellation rollback risk.
It is unrelated to this specification and is not included as a finding here;
release automation remains governed separately by `docs/specs/releasing.md`.

The independent revision 8 review produced R-39 through R-41 (zero blocking):

- **R-39** (major) accepted with human adjudication 2026-09-12: the plugin's
  health check must complete, timeout-raced, before the registration decision;
  a check that cannot complete counts as unavailable and produces no
  injection. M1b proves the check is reachable pre-registration on the proven
  runtime; if it is not, the result escalates to planning rather than
  degrading to register-then-check. This resolves the contradiction between
  D3's "at load" enforcement claim and F-08's init constraint.
- **R-40** (major) accepted with human adjudication 2026-09-12: D9 names the
  provisional band as its one deliberate exception, with the bounds that make
  it coherent, and the advisory surfaces in-session once per plugin load when
  a TUI is attached, in addition to the delivery record and status. Revision 10
  removes the exception by replacing provisional delivery with automatic exact-
  artifact probing.
- **R-41** accepted: M4 requires `docs/specs/releasing.md` to record the
  re-probe obligation and the release pipeline to enforce it, so the
  provisional band cannot silently become permanent. Revision 10 supersedes the
  release-time version list with release validation of the reusable probe
  contract and runner.

The independent revision 9 review produced R-42 through R-44 (zero blocking):

- **R-42** (major) accepted with human adjudication 2026-09-12: compatibility is
  now a cached behavioral result for one exact host version and executable
  artifact hash under one plugin/API and probe-contract version. No interval
  between tested versions is called proven.
- **R-43** (major) accepted with human adjudication: `delivery_current` is
  explicitly sync-bounded desired-delivery state. It does not claim to know a
  separate running host's registration result; the plugin reports that decision
  in-session. `delivery_unsafe` is global only when no eligible mode is safe.
- **R-44** (major) accepted with a replacement design: future runtimes are not a
  release-authored allowlist. A shared isolated runner probes an uncached exact
  runtime artifact automatically and caches deterministic pass or failure;
  environmental unavailability is retried. Releases must validate the probe
  contract and runner with working controls, not pre-probe every future runtime.

The independent revision 10 review produced R-45 through R-48 (zero blocking):

- **R-45** (major) accepted with human adjudication 2026-09-12: first encounter
  blocks registration on a strict, bounded, synchronous probe. Success within
  the bound persists the result and registers normally; timeout or failure
  registers nothing and reports the runtime as unverified. No detached runner,
  background completion, or second-restart activation lifecycle exists. M1b
  measures the end-to-end probe duration so the bound is empirical, and a probe
  that structurally cannot fit an acceptable startup budget escalates to
  planning under the R-39 rule. Revision 12 supersedes that performance gate
  under R-51: only the hard liveness timeout makes a run unavailable.
- **R-46** (major) accepted with human adjudication 2026-09-12: the probe
  fixture and the production plugin share the target-channel implementation, so
  a passing result attests the path production actually uses. Changing that
  shared implementation bumps the probe-contract version and invalidates cached
  results; a structural test and release gate enforce both halves.
- **R-47** accepted: the runner single-flights probes per cache key; concurrent
  callers wait within their own bound or receive unavailable, and atomic
  persistence resolves any residual race.
- **R-48** accepted: artifact identity must yield both the hash and a
  launchable path; an artifact that cannot be both identified and executed is
  an unavailable outcome, never matched by version. Revision 12 strengthens
  this to the running-process-bound artifact-set invariant under R-52.

The independent revision 11 review produced R-49 through R-52:

- **R-49** (blocking) accepted with a deliberately weaker guarantee after human
  adjudication 2026-09-12: automatic qualification of unseen host artifacts is
  retained. A passing probe attests only the interpretation mechanisms
  enumerated by the release-managed probe contract; it cannot prove the absence
  of unknown syntax or stages introduced by a future host. D5, D9, scope,
  constraints, M1b, M4, and the epic acceptance contract state this residual
  rather than claiming absolute inertness.
- **R-50** (major) rejected with explicit risk acceptance: modified managed
  plugin bytes continue under architecture 16.3's unsupported-warning policy.
  They neither change the capability key nor withhold commands, and the target-
  safety guarantee excludes their behavior. Release-managed plugin changes
  remain bound by R-46's shared implementation and probe-contract bump.
- **R-51** (major) accepted with a replacement policy: successful probe duration
  is not a compatibility threshold. First encounter runs synchronously to
  completion under a generous hard timeout used only to detect hangs or
  deadlocks, with periodic normal-log progress when startup UI cannot show it.
  M1b measures duration and records the timeout basis; timeout remains an
  unavailable, retriable outcome.
- **R-52** (major) accepted: executable hash plus path becomes an artifact-set
  identity tied to the running process. It covers all load-bearing host code for
  command parsing and expansion, and the runner launches an immutable copy or
  equivalent stable handle of those same bytes. Packaging forms that cannot
  satisfy that invariant are unavailable.

The independent revision 12 review produced R-53 through R-55 (zero blocking):

- **R-53** (major) accepted: sync-time artifact-set derivation is now defined
  as the shared derivation implementation applied statically. Its digest is a
  valid cache-lookup key only where M1b proved static/process-bound digest
  equivalence for the packaging form; an unproven or disagreeing derivation is
  an unknown observation with the existing fail-closed behavior. M1b gains the
  equivalence outcome and M3 gains the agreement and forced-mismatch tests.
- **R-54** (minor) accepted: M1b measures the tested host's behavior when the
  registration hook blocks for durations approaching the liveness timeout,
  since revision 12 made host tolerance of a long synchronous hook
  load-bearing.
- **R-55** (minor) accepted: M4 requires `docs/specs/releasing.md` to record
  the per-release probe-contract review obligation alongside the validation
  gate, so the R-49 residual mitigation has an enforcement home.

The revision 12 reviewer recorded review saturation: after R-53's resolution,
remaining objections would re-litigate accepted residuals, and the next gate
is M1b's empirical result rather than a further review round.

The revision 13 review was requested explicitly in the context of `sv-e85.14`
and produced R-56 through R-61:

- **R-56** (blocking) accepted with human adjudication 2026-09-12: M1b may
  implement the minimal production target-channel, artifact-derivation, runner,
  result-cache, and plugin-to-runner primitives needed for a production-faithful
  fixture. They remain dormant outside the fixture; command definitions,
  registration, fallback publication, and delivery remain in M2 and M3.
- **R-57** (major) accepted: M1b records a finite candidate-channel inventory
  derived from documented and observed host surfaces before it can conclude
  that no channel is available.
- **R-58** (major) accepted: each supported packaging form records a
  conservative artifact-set derivation rule and completeness evidence; forms
  without a defensible boundary are unavailable.
- **R-59** (major) accepted: fallback fail-closed attestation checks raw
  pre-model message evidence for the diagnostic instruction and target absence.
  It does not claim visible model compliance without inference.
- **R-60** (major) accepted: the required architecture amendment now includes
  the CLI responsibility, persistent-state/non-goal, release-layout, and summary
  sections affected by the runner and cache.
- **R-61** (major) accepted with human adjudication 2026-09-12: a missing or
  incompatible required OpenCode integration remains a hard sync failure under
  architecture 13.4; command withholding does not create partial-success sync.

The independent revision 14 review produced R-62 (major) and R-63/R-64
(minor); zero blocking. It confirmed the revision 13→14 delta is otherwise
coherent and recorded that, once R-62 is resolved, the next gate is M1b's
empirical result rather than a further review round:

- **R-62** (major) accepted with human adjudication 2026-09-12: M1b gains a
  dormancy attestation outcome — the new production primitives are proven
  inactive outside the probe fixture by permanent regression guards that M2
  and M3 retire deliberately as each path legitimately activates. The
  release-timing half is resolved by explicit risk acceptance: a dormant,
  attestation-covered primitive may ship before the sv-e85.5 architecture
  amendment lands; the amendment still precedes any shipped M3 delivery
  behavior.
- **R-63** (minor) accepted: the record states that R-19 was never assigned —
  the revision 5 review numbered its findings R-20..R-29 — and the status
  line reflects the true ranges rather than claiming a disposition that does
  not exist.
- **R-64** (minor) accepted: D3 states explicitly that a missing plugin and a
  too-old plugin are the architecture 13.4 hard-failure cases while an absent
  passing exact-artifact fallback result is ordinary command-specific
  ineligibility.

The revision 15 review, requested in the context of `sv-e85.14`, produced R-65
(major) and no blocking or minor findings:

- **R-65** (major) accepted with human adjudication 2026-09-12: dormancy and
  fail-closed regression guards remain by default rather than being retired
  merely because M2 or M3 activates related behavior. In particular, absent,
  fallback-mode, and unreadable delivery records remain permanent no-injection
  invariants. A future bead may narrow or remove a guard only for a recorded
  concrete reason and with evidence that the current implementation retains
  equivalent safety coverage.

## M1b adjudication

`sv-e85.14` executed M1b against revision 16 and escalated rather than adopting
the only channel it found. Human decision 2026-09-12, recorded as **D10**:
withhold delivery on OpenCode 1.18.30.

The reasoning, in the human's terms: the positional guard is not an invariant,
it only moves the failure threshold, so neither `$999999` nor any target-length
bound may be treated as a security control; and because the fallback template
necessarily carries a target reference on an uncleared path, fallback does not
satisfy D3. The result is classified as a **host capability limitation for
OpenCode 1.18.30**, not as a failed implementation attempt. No production
delivery is implemented for this host path, and the feature may be enabled
later if OpenCode provides an actually inert pre-interpretation transport and
the probes establish that it satisfies the required invariants.

This supersedes, for the current host, every earlier expectation that some mode
would ship: R-30's per-mode eligibility, R-33's unknown-runtime split, R-36's
retained fallback machinery, and the R-42..R-58 probe/runner/cache design all
remain the recorded contract for a future host, but none of them is built.

## Next Step

M1b is complete and D10 withholds delivery on OpenCode 1.18.30. The remaining
work is to record that state rather than to build toward delivery.

1. Close `sv-e85.14` with its findings; M1b answered its question.
2. Close the delivery beads as not applicable under D10 rather than leaving
   them open against a host that cannot carry a target: `sv-e85.3`, `sv-e85.4`,
   `sv-e85.6` through `sv-e85.12`. `sv-e85.5` is no longer required, because
   the architecture never needs to describe behavior that does not ship.
3. Record the epic as specified-but-withheld, and note in `docs/ROADMAP.md`
   that host workflow commands are blocked on an OpenCode capability rather
   than scheduled.
4. Escalate to `sv-vzf`. Its revision 5 assumes a command that observably
   carries one opaque target, and no such command will exist on this host, so
   `sv-e85.13` becomes an input to replanning `sv-vzf` rather than a
   re-validation. `sv-7sq` is affected the same way: there are no commands to
   complete.

Re-enablement is a new planning cycle, not a resumption of this graph. It
begins by rerunning M1b's probe matrix unchanged against the candidate host and
requires the D10 conditions to be met before any template or delivery bead is
recreated.
