# Structured Vibe Roadmap

This roadmap tracks product capabilities Structured Vibe may add over time.

It is intentionally **not** a release plan. Roadmap items are candidates for
Beads epics; releases are cut from the set of completed work that is ready to
ship.

Implementation details, sequencing, acceptance criteria, and decomposition
belong in the corresponding Beads epic and its plan, not in this document.

## Near-term capabilities

### Autonomous epic execution

Add a host workflow such as `/svibe:do <epic>` that carries an approved epic
forward until either:

- all child work is complete and verified, or
- meaningful human input is required.

The workflow should consume ready Beads work, execute it, verify it, repair
straightforward failures, and continue without requiring the human to advance
each mechanical lifecycle step manually.

### Document-to-Beads decomposition

Allow Structured Vibe to turn a roadmap, specification, design document, or
similar artifact into a proposed Beads structure.

The workflow should propose epics, tasks, dependencies, and acceptance criteria
for human review before materializing them. It should avoid creating a second
project-management system alongside Beads.

### Blocker diagnosis and unblock guidance

When work cannot proceed, explain why rather than merely reporting that no task
is ready.

Structured Vibe should be able to trace blocking dependencies, distinguish
technical blockers from external dependencies and human decisions, and suggest
a concrete path to resume work.

### Command and identifier completion

Improve host-side completion where supported, including:

- `/svibe:verify`
- `/svibe:do`
- epic IDs
- task IDs

Completion behavior should be generated from durable project state rather than
hard-coded examples.

### Deterministic next-step guidance

Workflow commands should report the recommended next action explicitly when a
human action is required.

The workflow should not depend on an individual model remembering to suggest
the correct next command.

### Model capability metadata

Refresh model capability and tier handling across supported providers and make
it resilient to provider catalog changes.

Known needs include:

- correctly classifying current frontier models such as GPT-5.6 Sol,
- accounting for newer model families and revisions,
- reducing brittle model-name matching,
- preferring capability metadata where the host or provider exposes it.

## Execution and context capabilities

### Durable execution-ready epic state

Ensure an approved epic contains or references enough durable state to execute
without depending on the planning conversation remaining in context.

This may include the approved plan, child tasks, dependencies, acceptance
criteria, completion criteria, known external dependencies, and human
checkpoints. Beads should remain the primary task graph rather than being
replaced by a parallel Structured Vibe project-management layer.

### Fresh-context task execution

Allow child tasks to execute in fresh contexts while reconstructing the
necessary task context from durable project artifacts.

This should reduce stale assumptions and context pollution during long-running
epics.

### Independent planning and review contexts

Where the host supports it, allow planning and review to use independent
contexts and potentially different models so reviews evaluate the artifact
rather than inherit the conversational momentum that produced it.

### Harness capability detection

Detect which orchestration capabilities the active host provides, such as:

- model switching,
- fresh contexts,
- subagents,
- structured handoff,
- persistent orchestration state.

Structured Vibe should adapt to the host's capabilities rather than assume all
harnesses expose the same execution model.

### Capability-based model routing

Select or recommend models based on task requirements and available model
capabilities rather than provider-specific name checks.

The routing system should degrade cleanly when a model or capability cannot be
identified.

## Future orchestration capabilities

These are intentionally separate roadmap items rather than one broad "smart
orchestration" feature.

### Resumable epic execution

Allow an interrupted autonomous epic run to reconstruct its state and continue
without replaying completed work.

### Task prioritization

Choose among multiple ready tasks using useful execution signals such as
blocking depth, task risk, and context locality.

### Failure and retry policy

Make retry behavior explicit and configurable, including when repeated failure
should stop and request human judgment.

### Parallel task execution

Execute independent child tasks concurrently where the host supports it and
where doing so does not create unsafe repository or dependency interactions.

### Cost-aware model routing

Consider model cost alongside required capability when selecting among valid
execution models.

### Richer execution status

Provide clearer visibility into epic progress, active work, completed work,
blocked work, verification state, and the reason for any stop.

## Candidate integrations

### Additional host integrations

Expand beyond OpenCode where another host can support Structured Vibe's core
workflow and capability model without compromising the local-first design.

Claude Code is an obvious candidate for evaluation.

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
