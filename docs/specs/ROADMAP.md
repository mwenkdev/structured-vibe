# Structured Vibe Roadmap

## Purpose

Structured Vibe should help a human and AI produce a high-confidence implementation plan, then execute that plan with as little mechanical human intervention as possible.

The human should make decisions.

The human should not have to act as a workflow scheduler.

The core direction is to separate Structured Vibe into two distinct modes:

1. **Planning** — deliberate, interactive, review-heavy.
2. **Execution** — autonomous, durable, and epic-scoped.

---

## Guiding Principles

### 1. Human judgment over human button-pushing

Structured Vibe should interrupt the user when judgment is required, not simply because another workflow phase is ready.

Bad:

```text
/svibe:finalize
/svibe:execute
/svibe:verify
/svibe:execute
/svibe:verify
```

Target:

```text
/svibe:plan <epic>
/svibe:review <epic>
...repeat as needed...
/svibe:do <epic>
```

After `/svibe:do`, Structured Vibe should continue until:

- the epic is complete, or
- it reaches a blocker that requires human input.

### 2. The epic is the unit of execution

Planning produces an executable epic.

Execution consumes the executable epic.

The epic should contain or reference enough durable state that execution does not depend on preserving the original planning conversation.

### 3. Fresh context is a feature

Planning and review benefit from independent perspectives.

Execution tasks should be able to run in fresh contexts using durable artifacts rather than inheriting an ever-growing chat history.

### 4. Verification is part of execution

Verification should remain a first-class operation, but should not normally require the human to invoke it manually during an execution run.

### 5. Interrupt only for meaningful decisions

Structured Vibe should automatically handle routine implementation and repair work.

It should stop for:

- material scope changes,
- unresolved product or architecture decisions,
- destructive or risky operations that require approval,
- missing credentials or external access,
- plan-defined human checkpoints,
- blockers it cannot safely resolve,
- repeated verification failures that indicate the plan may be wrong.

---

# Target Workflow

## Planning Workflow

Planning remains intentionally interactive.

```text
1. /svibe:plan <epic>
2. /svibe:review <epic>
3. Return to step 1 until the human and AI are satisfied
```

This loop should optimize for plan quality, not automation.

A typical flow may look like:

```text
/svibe:plan PM-12
/svibe:review PM-12
/svibe:plan PM-12
/svibe:review PM-12

# human accepts plan

/svibe:do PM-12
```

## Review Independence

Where supported by the agent harness, planning and review should use independent models and/or fresh contexts.

Desired shape:

```text
Plan:    Model A, Context A
Review:  Model B, Fresh Context B
Revise:  Model A or C, Context C
```

The reviewer should primarily evaluate the artifact itself rather than inherit the conversational momentum that produced it.

This should be capability-driven rather than hard-coded to specific provider/model names.

---

# Executable Epic

Before execution begins, the epic should be considered an executable artifact.

An execution-ready epic should include:

- approved plan,
- child tasks,
- task dependencies,
- acceptance criteria,
- epic-level completion criteria,
- known external dependencies,
- known human checkpoints,
- execution policy,
- enough context to reconstruct task-specific prompts.

Example conceptual structure:

```text
Epic
├── approved plan
├── acceptance criteria
├── execution policy
├── child task graph
├── known blockers
├── human checkpoints
└── completion criteria
```

The exact storage format is TBD.

Possible sources of truth include:

- Beads epic/task metadata,
- checked-in Structured Vibe artifact files,
- a small `.svibe/` state directory,
- or a combination of the above.

Avoid creating a second project-management system if Beads already provides the durable task graph.

---

# `/svibe:do <epic>`

## Contract

`/svibe:do <epic>` means:

> Carry this approved epic forward until it is complete or meaningful human input is required.

It replaces the current user-facing finalize/execute/verify ceremony.

Finalize may still exist internally, but should not normally be a user-visible lifecycle step.

## Execution Loop

Conceptually:

```text
while epic is open:
    ready = find_ready_subtasks(epic)

    if ready tasks exist:
        select next task
        execute task
        verify task

        if verification passes:
            mark task complete
            continue

        if verification finds a straightforward fix:
            fix
            re-verify
            continue

        if verification requires judgment:
            report and stop

    else if all epic tasks are complete:
        run epic-level verification

        if epic verification passes:
            close epic
            report completion
            stop

        if findings are straightforward:
            create/activate repair work
            continue

        otherwise:
            report and stop

    else:
        diagnose blockers
        report blocked state
        propose an unblock plan
        stop
```

---

# Blocked-State Behavior

A blocked epic should never result in a useless message such as:

```text
No tasks are ready.
```

Structured Vibe should explain:

1. which tasks remain,
2. why they are blocked,
3. what dependency chain caused the stop,
4. whether the blocker is technical, external, or a decision,
5. what Structured Vibe recommends doing next.

Example:

```text
Execution stopped.

Blocked tasks:
- PM-42 depends on PM-37
- PM-51 requires Stripe credentials

Why no work is ready:
PM-37 requires an unresolved schema ownership decision.

Recommended unblock:
1. Decide whether billing ownership belongs to organization or workspace.
2. Update PM-37 acceptance criteria.
3. Re-run /svibe:do PM-12.
```

Where possible, Structured Vibe should propose a concrete unblock plan rather than merely expose task graph state.

---

# Context Strategy

## Problem

The planning workflow may contain important reasoning that cannot safely be assumed to remain in context throughout a long execution run.

## Direction

The approved plan should become durable, compiled execution state.

Execution contexts should be reconstructable from artifacts.

A fresh task context should generally contain:

```text
epic summary
approved plan
current task
task acceptance criteria
relevant dependency outcomes
relevant completed-task results
repo state
execution policy
```

It should not require the complete planning conversation.

## Context Isolation

Where supported, each child task may run in a fresh context.

Benefits:

- less context pollution,
- fewer stale assumptions,
- more predictable token usage,
- easier retry behavior,
- better isolation between unrelated subtasks.

The orchestration layer remains responsible for maintaining durable state between contexts.

---

# Verification

Verification remains a first-class Structured Vibe capability.

## During `/svibe:do`

Verification should normally happen automatically after task execution.

Expected loop:

```text
EXECUTE
   ↓
VERIFY
   ├── pass ───────→ next task
   ├── fixable ────→ fix → verify again
   └── judgment ───→ ask human
```

## Explicit `/svibe:verify`

Keep `/svibe:verify` available for manual use.

Use cases:

- rerun verification after manual changes,
- verify an existing epic or task without executing it,
- diagnose a suspicious implementation,
- CI/workflow integration,
- debugging Structured Vibe itself.

Add command completion/autocomplete support for `/svibe:verify`.

---

# Execution Policy

Structured Vibe should have an explicit execution policy rather than relying on model personality.

Conceptual example:

```yaml
execution_policy:
  autonomy: high

  require_user_for:
    - destructive_operations
    - material_scope_change
    - ambiguous_product_decision
    - credential_or_external_access
    - plan_defined_checkpoint

  auto_handle:
    - implementation_choices_within_plan
    - test_failures
    - lint_failures
    - type_errors
    - straightforward_review_findings
    - retries
    - verification_fix_loops
```

This may eventually be configurable globally, per repo, or per epic.

---

# Task Selection

`/svibe:do` should consume ready work from the epic task graph.

Initial behavior can be simple:

1. query ready tasks belonging to the epic,
2. choose the next valid task,
3. execute it,
4. verify it,
5. mark it complete,
6. continue.

Future selection strategies may account for:

- dependency depth,
- task risk,
- context locality,
- estimated cost,
- opportunities for parallel execution,
- model capability requirements.

Do not optimize prematurely.

Correct durable sequential execution is more important than parallelism.

---

# Model and Harness Capabilities

Structured Vibe should detect and use harness capabilities where available.

Potential capabilities:

- switching models,
- spawning subagents,
- starting fresh contexts,
- assigning a model by task type,
- structured handoff between agents,
- persistent orchestration state.

Model routing should be capability-based and resilient to provider catalog changes.

Avoid hard-coding model assumptions that quickly become stale.

## Model Tiering

Refresh the model/version capability map across currently available providers.

Known requirement:

- GPT-5.6 Sol should not produce an incorrect lower-tier warning during review.
- Account for newer model families and versions rather than patching individual names.
- Prefer capability/tier metadata over brittle string matching.

---

# Command UX

## Near-Term Commands

```text
/svibe:plan <epic>
/svibe:review <epic>
/svibe:do <epic>
/svibe:status <epic>
/svibe:verify <epic-or-task>
```

## Command Completion

Add shell/command completion where supported.

Priority:

- `/svibe:verify`
- epic IDs
- task IDs
- `/svibe:do <epic>`

## Deterministic Next-Step Guidance

Until `/svibe:do` fully owns the lifecycle, every command should explicitly report the recommended next action.

Do not rely on individual models to remember to suggest the next command.

Example:

```text
Next recommended action: /svibe:review PM-12
```

or:

```text
Plan approved. Ready to run: /svibe:do PM-12
```

---

# Proposed Implementation Phases

## Phase 1 — Execution-Ready Planning

Goal: make the plan artifact sufficient for autonomous execution.

- Define what makes an epic execution-ready.
- Ensure child tasks and dependencies are explicit.
- Add task-level acceptance criteria.
- Add epic-level completion criteria.
- Record known blockers and human checkpoints.
- Make `/svibe:review` evaluate execution readiness.
- Add deterministic next-step guidance.

### Exit Criteria

An approved epic can be understood without the original planning conversation.

---

## Phase 2 — `/svibe:do` Sequential Runner

Goal: remove mechanical lifecycle button-pushing.

- Add `/svibe:do <epic>`.
- Query ready tasks for the epic.
- Execute one task at a time.
- Automatically verify after execution.
- Auto-fix straightforward findings.
- Re-verify.
- Mark successful tasks complete.
- Continue until complete or blocked.
- Close the epic after successful epic-level verification.

### Exit Criteria

A user can start `/svibe:do <epic>` and does not need to manually invoke finalize/execute/verify for each task.

---

## Phase 3 — Blocker Intelligence

Goal: make stops useful.

- Detect when no ready tasks remain.
- Trace blocking dependencies.
- Distinguish technical blockers from human decisions and external dependencies.
- Generate a proposed unblock plan.
- Clearly report what user input is required.
- Allow `/svibe:do` to resume cleanly after resolution.

### Exit Criteria

A blocked run explains both the problem and the recommended path forward.

---

## Phase 4 — Fresh Context / Agent Harness Support

Goal: improve execution reliability and context management.

- Detect harness model-switching capabilities.
- Support fresh context per task.
- Rehydrate task context from durable epic state.
- Support independent model/context for review.
- Preserve execution state across agent/context boundaries.
- Add capability-based model routing.

### Exit Criteria

Execution no longer depends on preserving a single long-lived model context.

---

## Phase 5 — Smarter Orchestration

Goal: improve efficiency after the basic runner is reliable.

Potential work:

- task prioritization,
- model selection by task,
- limited parallel execution,
- cost-aware routing,
- failure/retry policies,
- context locality optimization,
- richer status reporting,
- resumable interrupted runs.

Do not begin this phase until sequential execution is trustworthy.

---

# Non-Goals

Structured Vibe should not expand into general-purpose code review, PR automation, or QA workflow management unless those capabilities are directly required by the Structured Vibe planning/execution lifecycle.

Broader code-review, PR, and QA automation should remain separate from Structured Vibe.

Structured Vibe should stay focused on:

```text
plan
review
execute
verify
complete
```

with the minimum human intervention necessary to preserve judgment and safety.

---

# Open Questions

- Where should approved plan state live?
- How much state belongs in Beads versus `.svibe/` artifacts?
- Should `/svibe:do` require an explicitly approved/reviewed epic?
- How should approval be represented durably?
- Should verification findings create new child tasks or remain internal repair loops?
- What retry threshold should escalate a failure to the human?
- Can current agent harnesses switch models reliably from within a workflow?
- Which harnesses support fresh-context subagents?
- How should Structured Vibe detect model capabilities without maintaining a brittle static catalog?
- Should task selection be strictly deterministic initially?
- How should interrupted `/svibe:do` runs resume?
- What constitutes enough epic-level verification to automatically close an epic?

---

# North Star

The intended experience is:

```text
/svibe:plan PM-12
/svibe:review PM-12
/svibe:plan PM-12
/svibe:review PM-12

# Human approves the plan

/svibe:do PM-12
```

Then one of two outcomes:

```text
Epic PM-12 complete.
All tasks executed and verified.
Epic-level verification passed.
```

or:

```text
Execution stopped.
The remaining work is blocked.

Here is what is blocking it.
Here is why.
Here is the recommended unblock plan.
Here is the decision/input needed from you.
```

The human should be making decisions, not advancing workflow state.
