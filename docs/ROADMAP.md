# Structured Vibe Roadmap

This roadmap tracks product capabilities Structured Vibe may add over time.

It is intentionally **not** a release plan. Roadmap items are candidates for
Beads epics; releases are cut from the set of completed work that is ready to
ship.

Implementation details, sequencing, acceptance criteria, and decomposition
belong in the corresponding Beads epic and its plan, not in this document.

## Near-term capabilities

### Autonomous epic execution

<!-- bd: sv-bpq -->

Add a host workflow such as `/svibe:do <epic>` that carries an approved epic
forward until either:

- all child work is complete and verified, or
- meaningful human input is required.

The workflow should consume ready Beads work, execute it, verify it, repair
straightforward failures, and continue without requiring the human to advance
each mechanical lifecycle step manually.

### Document-to-Beads decomposition

<!-- bd: sv-33d -->

Allow Structured Vibe to turn a roadmap, specification, design document, or
similar artifact into a proposed Beads structure.

The workflow should propose epics, tasks, dependencies, and acceptance criteria
for human review before materializing them. It should avoid creating a second
project-management system alongside Beads.

### Blocker diagnosis and unblock guidance

<!-- bd: sv-gx4 -->

When work cannot proceed, explain why rather than merely reporting that no task
is ready.

Structured Vibe should be able to trace blocking dependencies, distinguish
technical blockers from external dependencies and human decisions, and suggest
a concrete path to resume work.

### Host workflow commands

<!-- bd: sv-e85 -->

**Blocked on an OpenCode capability, not scheduled.**

Real host commands for the core workflow skills are specified in
`docs/specs/host-workflow-commands.md`, but delivery is withheld on OpenCode
1.18.30. The host interprets command target text after substitution, executing
shell expressions and resolving `@file` references the target supplies, and the
only arrangement that avoids it depends on a positional-index guard that moves
the failure threshold rather than removing it (D10, findings F-20..F-25).

This unblocks only when a host provides an inert pre-interpretation transport
for the target and the recorded probe matrix confirms it.

The two capabilities below depend on this one and are blocked with it.

### Command and identifier completion

<!-- bd: sv-7sq -->

Improve host-side completion where supported, including:

- `/svibe:verify`
- `/svibe:do`
- epic IDs
- task IDs

Completion behavior should be generated from durable project state rather than
hard-coded examples.

### Deterministic next-step guidance

<!-- bd: sv-vzf -->

Workflow commands should report the recommended next action explicitly when a
human action is required.

The workflow should not depend on an individual model remembering to suggest
the correct next command.

### Model capability metadata

<!-- bd: sv-m75 -->

Refresh model capability and tier handling across supported providers and make
it resilient to provider catalog changes.

Known needs include:

- correctly classifying current frontier models such as GPT-5.6 Sol,
- accounting for newer model families and revisions,
- reducing brittle model-name matching,
- preferring capability metadata where the host or provider exposes it.

## Execution and context capabilities

### Durable execution-ready epic state

<!-- bd: sv-dwx -->

Ensure an approved epic contains or references enough durable state to execute
without depending on the planning conversation remaining in context.

This may include the approved plan, child tasks, dependencies, acceptance
criteria, completion criteria, known external dependencies, and human
checkpoints. Beads should remain the primary task graph rather than being
replaced by a parallel Structured Vibe project-management layer.

### Fresh-context task execution

<!-- bd: sv-ot3 -->

Allow child tasks to execute in fresh contexts while reconstructing the
necessary task context from durable project artifacts.

This should reduce stale assumptions and context pollution during long-running
epics.

### Independent planning and review contexts

<!-- bd: sv-w75 -->

Where the host supports it, allow planning and review to use independent
contexts and potentially different models so reviews evaluate the artifact
rather than inherit the conversational momentum that produced it.

### Harness capability detection

<!-- bd: sv-t9i -->

Detect which orchestration capabilities the active host provides, such as:

- model switching,
- fresh contexts,
- subagents,
- structured handoff,
- persistent orchestration state.

Structured Vibe should adapt to the host's capabilities rather than assume all
harnesses expose the same execution model.

### Capability-based model routing

<!-- bd: sv-w9t -->

Select or recommend models based on task requirements and available model
capabilities rather than provider-specific name checks.

The routing system should degrade cleanly when a model or capability cannot be
identified.

## Future orchestration capabilities

These are intentionally separate roadmap items rather than one broad "smart
orchestration" feature.

### Resumable epic execution

<!-- bd: sv-jlw -->

Allow an interrupted autonomous epic run to reconstruct its state and continue
without replaying completed work.

### Task prioritization

<!-- bd: sv-zas -->

Choose among multiple ready tasks using useful execution signals such as
blocking depth, task risk, and context locality.

### Failure and retry policy

<!-- bd: sv-j13 -->

Make retry behavior explicit and configurable, including when repeated failure
should stop and request human judgment.

### Parallel task execution

<!-- bd: sv-eym -->

Execute independent child tasks concurrently where the host supports it and
where doing so does not create unsafe repository or dependency interactions.

### Cost-aware model routing

<!-- bd: sv-1i0 -->

Consider model cost alongside required capability when selecting among valid
execution models.

### Richer execution status

<!-- bd: sv-08e -->

Provide clearer visibility into epic progress, active work, completed work,
blocked work, verification state, and the reason for any stop.

## Candidate integrations

### Additional host integrations

<!-- bd: sv-sc7 -->

Expand beyond OpenCode where another host can support Structured Vibe's core
workflow and capability model without compromising the local-first design.

Claude Code is an obvious candidate for evaluation.

## Deferred

These are speculative architecture extensions, recorded so the design intent is
not lost. They are not scheduled work, and several are conditional on a problem
that has not yet appeared.

### svibe self-update

<!-- bd: sv-j6i -->

A future svibe self-update flow may consume the same release artifacts and
checksums as the installer.

### Pack install and update

<!-- bd: sv-52h -->

A future package or update mechanism may use the informational source metadata
and SemVer to install and update skill packs.

### Adapter-contributed model aliases

<!-- bd: sv-7mk -->

If maintaining every provider and host spelling in the central registry becomes
painful, adapters may contribute exact external identifier mappings while the
core registry continues to own canonical model identities and tiers.

### Local daemon

<!-- bd: sv-okt -->

If repeated CLI subprocess calls become materially inefficient, a future local
daemon may expose the same core logic through local IPC - one daemon per user or
machine, multiple repo contexts, local-only by default.

### Organization scope and enforcement

<!-- bd: sv-8wq -->

A future resolver may insert an organization scope into the ordered scopes, for
example core < org < user < project, with a policy mechanism allowing
organization-level enforcement that cannot be replaced by ordinary user or
project precedence.

### Abstract capability matching

<!-- bd: sv-r8e -->

A future capability system may allow a skill to request a capability such as
documentation or security-review and resolve one of several providers, rather
than referencing exact skill IDs.

## Roadmap maintenance

Roadmap entries describe **capabilities**, not implementation plans.

When work on an item begins:

1. create a Beads epic for the capability,
2. plan and review the epic using Structured Vibe,
3. decompose it into executable child work,
4. implement and verify it,
5. assign the resulting work to a release when it is ready to ship.

A release may contain one roadmap capability, several, or only part of a larger
capability if the shipped increment is independently useful.
