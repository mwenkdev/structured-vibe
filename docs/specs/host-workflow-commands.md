# Specification: Host Workflow Command Definitions

Bead: `sv-e85`
Status: revision 5 — findings R-01..R-18 addressed; awaiting review
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
- Parsing, validating, or completing the target argument inside the command.
  Commands pass it through opaquely.
- Host integrations other than OpenCode.
- Any change to skill resolution, precedence, or the ordinary-skill model.

## Decisions

### D1 — Commands are pointers to skills, not a second behavior definition

Each command's template requests that the model load the named `sv-*` skill via
the host's normal skill tool and apply it to the target in `$ARGUMENTS`. The
template contains no workflow instructions, no outcome vocabulary, and no
procedural content. One sentence of intent, the skill request, and D5's
one-sentence empty-target stop instruction are the ceiling.

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
fallback). When the installed OpenCode runtime is outside the release's tested
injection contract, or injection fails the M1 gate, `svibe sync` publishes one
markdown file per command into the documented OpenCode command directory:
`.opencode/commands/` inside Git and the user command directory outside Git.
Fallback uses hyphenated names on every platform (D4), because the file name
becomes the command name and colons are invalid in Windows filenames.

The release carries an explicit injection compatibility contract derived from
M1: tested OpenCode runtime range, plugin API/package version, and required
capabilities. Sync observes the runtime by probing the `opencode` executable's
reported version where one is reachable; an unreachable or unparseable runtime
is *unknown*. Sync selects a delivery mode from that observation and stores it
in a versioned delivery record. Status repeats the same check. An unknown or
incompatible runtime selects fallback rather than silently losing commands.
Changing runtime or release may switch modes on the next sync.

The sync-time probe can disagree with the runtime that actually loads the
plugin — multiple installs, GUI-launched hosts, or a PATH sync never saw. The
plugin is the only component that observes the true runtime, so at load it
queries the current host server's `/global/health` endpoint relative to the
`serverUrl` OpenCode supplied and uses that response's version as the
authoritative observation. M1 proves this endpoint identifies the same server
that loaded the plugin rather than trusting the SDK declaration alone.

For a primary record, an unavailable or unparseable health response, or an
observed version outside the recorded contract, produces no injection. When a
TUI is attached the plugin warns with the observed version when available, the
supported range, and the actionable remediation: update or restart the actual
OpenCode host into that range. It names `svibe sync` only when another sync is
also required after the host and PATH observations have been aligned. The
sync-time probe selects; the host-local plugin check protects.

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

The versioned delivery record contains at least: schema version, delivery mode,
naming scheme, svibe release, tested host contract, generated paths, and each
published content hash. It is a cross-component contract, not an
implementation detail: sync writes it, status validates it, and the plugin
reads it to decide whether to inject. It lives at the snapshot root beside the
sync state file, outside any directory replaced during snapshot publication,
so it and the recovery journal survive the swap.

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
uses the colon form only when M1 and the current runtime contract permit it;
fallback uses hyphens. A valid user fallback forces every context to inherit
the hyphen form. A successful mode migration removes the old owned form.
`delivery_current` means exactly one effective Structured Vibe command set is
visible in the selected form at the publication and registration boundaries
svibe can observe; an observable unowned definition in either form prevents
that state and is reported as conflicted. A journaled interruption may expose
both owned forms temporarily and is reported as `delivery_pending` until
recovered.

### D5 — One opaque target argument

Each command passes `$ARGUMENTS` through without semantic parsing, validation,
or completion. The canonical target is the exact `arguments` string OpenCode
exposes to `command.execute.before` after its own command-line parsing; the
template receives that same value through `$ARGUMENTS`. A bead identifier, a
specification path, or a free-text planning idea are all legal; meaning is
phase-specific and owned by the skill and its consumers (`sv-vzf` revision 5,
D6). The contract makes no claim to preserve quoting syntax that OpenCode
removes before producing the canonical string.

This preserves the consumer contract `sv-vzf` was specified against: one opaque
canonical target per invocation, delivered as a distinct value rather than
recovered from prose.

Empty-target handling is best-effort with a defined floor. If M1 finds a host
mechanism that can reject submission of an empty target, it is used. If not,
each template carries one sentence instructing the model to stop and ask for a
target when `$ARGUMENTS` is empty — within D1's ceiling — and consumers
receive the empty canonical string and apply their own validation (`sv-vzf`
D6 already treats it as an invalid record). M1 records which behavior was
achieved; neither outcome fails the delivery mechanism.

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
degradation is silent only inside the affected host session; `svibe status`
remains the durable diagnostic surface.

Input fingerprinting reports desired-state staleness but cannot prove delivery.
Status therefore also validates snapshot payload shape, plugin/release byte
compatibility, the observed OpenCode runtime contract, the delivery record, and
every expected fallback file and hash. It distinguishes at least:
`delivery_current`, `delivery_pending`, `delivery_missing`,
`delivery_modified`, `delivery_conflicted`, and `delivery_incompatible`.
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

## Constraints

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
- The plugin injects only snapshot-derived definitions, never executes a
  command, and never mutates commands it did not create.
- `svibe` remains usable without the OpenCode integration; commands are a host
  convenience, not a workflow dependency.
- New persistent state is limited to the versioned delivery record and a
  short-lived recovery journal required for safe fallback publication. Neither
  stores workflow state.

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
- Colon naming (`svibe:plan`) empirically accepted or rejected; the naming
  decision (D4) is recorded.
- The tested OpenCode runtime range, plugin API/package version, and required
  injection capabilities are recorded as the release compatibility contract.
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

### M2 — Managed templates and snapshot generation

**Dependencies:** M1 (the recorded mechanism and naming decide the generated
format).

**Testable outcomes:**

- `core/commands/` contains seven definition sources; the managed manifest is
  regenerated through repository tooling and integrity checks cover them.
- Each template requests its skill by name, carries the canonical target
  opaquely, and contains no workflow instructions — enforced by a test
  asserting template shape, not by convention.
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

**Dependencies:** M1 and M2.

**Testable outcomes:**

- Sync chooses primary or fallback from the explicit runtime compatibility
  contract via the sync-time probe, records the chosen mode and naming scheme,
  and status reports incompatible, unknown, or changed runtimes rather than
  silently assuming injection.
- Under the primary mechanism: the plugin injects all seven commands from the
  correct snapshot at load time; each is invocable and observable per D8.
- The plugin consults the delivery record before registering: fallback mode,
  an absent record, an unreadable record, and a mode incompatible with the
  plugin's own observed runtime each produce no injection, and the
  unavailable or incompatible cases produce the actionable D3 warning.
- The plugin obtains its authoritative runtime version from the current host's
  `/global/health` endpoint via `serverUrl`; tests cover valid, unavailable,
  malformed, and incompatible responses and prove the warning does not
  prescribe a sync that would simply recreate the same primary record.
- Injection skips same-name commands visible at registration time with a
  warning, and also skips an affected phase when its alternate name is visible;
  both states are conflicted. Tests document that later plugins may replace
  svibe under host ordering. Svibe never modifies or removes a command it did
  not create.
- Missing snapshot, missing command payload, malformed payload, and detached
  TUI produce no injection and no session failure. Malformed payload produces a
  visible warning when a TUI is attached and a durable status diagnostic in all
  cases.
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
- Status fixtures cover every D7 delivery state. Consecutive syncs are
  idempotent in either mechanism.
- Before any fallback or delivery-record behavior ships, architecture 12.4 is
  amended for the new plugin capability and sections 13 through 15 document
  the fallback location, recovery journal, narrow transaction exception,
  compatibility check, delivery record, user-level fallback authority, and
  status states. The authoritative architecture never contradicts shipped
  behavior.

### M4 — Documentation and user-facing alignment

**Dependencies:** M3.

**Testable outcomes:**

- User documentation lists the seven commands, their mode-dependent names from
  D4, and the target-argument convention.
- The `sv-vzf` consumer assumptions are explicitly confirmed or corrected
  against the delivered mechanism: one opaque target argument, invocation
  observability with correlation identity, and the signal payloads available
  for a terminal-record transport.
- `docs/ROADMAP.md` gains the entry for this epic or records it as delivered.
- `docs/specs/releasing.md` and the architecture release layout include
  `core/commands/`; every platform archive is checked for all seven valid
  definitions, and a packaged-runtime test materializes them.
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
nothing, expands nothing, and interpolates nothing; `command.execute.before`'s
`arguments` string and the `$ARGUMENTS` substitution agreed in every case.
D5's caveat about quoting the host removes is vacuous on this runtime — but
retain it, since it costs nothing and the behavior is undocumented.

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
above; M3 should pin the tested range deliberately.

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

## Open Questions

None requiring product adjudication before decomposition. The delivery
mechanism, naming fallback, command set, loading guarantee, and recoverable
fallback transaction were adjudicated 2026-09-10. The snapshot's on-disk
definition encoding and journal encoding are implementation details bounded by
the contracts and milestones above.

## Assumptions

- At least one in-memory command-registration path can satisfy every primary
  M1 gate on a bounded OpenCode runtime range; M1 verifies rather than trusts
  this, and fallback absorbs failure or incompatibility.
- OpenCode continues reading `.opencode/commands/` as the project command
  convention directory (documented behavior; load-bearing only in fallback).
- `command.execute.before` and `command.executed` fire for injected commands
  the same way they do for file-defined commands. M1 verifies.
- One canonical opaque target string is sufficient for all seven commands;
  nothing in the current skills requires a second positional argument.
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
  as unknown → fallback, and adds the authoritative plugin-side runtime check
  with a visible warning on mismatch.
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

The review also identified a pre-existing release-cancellation rollback risk.
It is unrelated to this specification and is not included as a finding here;
release automation remains governed separately by `docs/specs/releasing.md`.

## Next Step

Run `/svibe:review sv-e85` on a different high-capability model against
revision 5. After review and human approval, `sv-beads` compiles this
specification into a dependency graph. Completion of this epic unblocks
re-validation of `sv-vzf` revision 5 against the delivered mechanism.
