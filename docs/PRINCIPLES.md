# Structured Vibe Product Principles

This document captures durable product and workflow principles for Structured
Vibe. These principles guide design decisions; they are not roadmap items and
do not imply implementation order or release scope.

## Human judgment over human scheduling

Structured Vibe should spend human attention where judgment matters.

Humans should make product, architecture, scope, safety, and risk decisions.
They should not have to manually advance a predictable workflow simply because
the next mechanical step is ready.

A good workflow interrupts the human because a decision is needed, not because
a state machine needs another button press.

## Beads are the durable work graph

Beads describe what needs to be done: coherent units of work, dependencies,
acceptance criteria, constraints, and specification references.

Structured Vibe should build on that graph rather than create a second project
management system beside it.

## Skills describe reusable execution knowledge

Skills capture how classes of work should be performed.

They should be reusable, composable, and resolved independently from the
specific Beads that consume them.

## Planning and execution have different needs

Planning benefits from deliberation, challenge, iteration, and human judgment.
Execution benefits from durable state, bounded context, and automation of
routine mechanics.

Structured Vibe should not optimize planning for autonomy at the expense of
plan quality, nor force execution to remain interactive when the approved plan
already answers the relevant questions.

## Approved plans become durable execution state

Execution should not depend on preserving the conversation that produced the
plan.

An approved plan should be represented in durable project artifacts and Beads
state strongly enough that a later context can reconstruct what it needs to do.

## Fresh context is useful isolation

Long-lived model context can accumulate stale assumptions, irrelevant history,
and accidental momentum.

Where the host supports it, Structured Vibe should be able to use fresh
contexts for independent review and for execution of sufficiently isolated
child tasks.

Fresh context is not a goal by itself; it is a tool for reliability and
isolation.

## Verification is part of doing the work

Verification is a first-class Structured Vibe capability, not an optional
cleanup step.

For routine execution, verification should normally be part of the execution
loop rather than requiring the human to invoke it manually after every task.
Explicit verification should remain available for diagnosis, reruns, manual
changes, and external workflows.

## Escalate decisions, not ordinary repairs

Structured Vibe should normally handle implementation details that are already
within an approved plan, including straightforward test failures, lint errors,
type errors, retries, and verification fixes.

It should stop when proceeding requires judgment, including cases such as:

- material scope changes,
- unresolved product or architecture decisions,
- destructive or risky operations requiring approval,
- missing credentials or external access,
- explicit human checkpoints,
- blockers it cannot safely resolve,
- repeated failures that suggest the plan itself may be wrong.

## Models are capabilities, not brand-name constants

Model catalogs change quickly.

Structured Vibe should reason in terms of capability requirements and known
model metadata rather than scattering provider-specific model-name assumptions
through the workflow.

Unknown models should degrade to an explicit unknown state instead of being
silently misclassified.

## The host owns execution mechanics

Structured Vibe is not an agent harness.

It does not replace the host's model execution, tool execution, permissions,
or session management. Structured Vibe should detect and use host capabilities
where available while continuing to work within the boundaries the host
provides.

> Structured Vibe resolves, validates, materializes, and advises.
> The host loads skills, runs models, and executes tools.

## Local-first by default

Structured Vibe's project state, configuration, generated integration output,
and durable planning artifacts should remain inspectable and usable locally.

External services may be integrated where useful, but the core workflow should
not depend on an opaque hosted control plane.

## Keep the scope narrow

Structured Vibe exists to improve the lifecycle around:

```text
plan -> review -> decompose -> execute -> verify -> complete
```

It should not become a general-purpose PR system, code-review product, QA
platform, or agent harness unless a capability is directly required by that
core lifecycle.

## Prefer correctness before sophistication

Reliable sequential execution is more valuable than premature parallelism.
Durable state is more valuable than clever prompt chaining. Clear blocker
reporting is more valuable than hidden retry loops.

Add optimization only after the underlying workflow is trustworthy.
