---
name: sv-beads
description: Use when converting an approved specification or milestone into a bd epic and dependency graph of executable work units. Triggers on "break this down", "create the beads", "decompose this milestone", or a handoff from sv-plan after review. Produces coherent beads carrying the full execution contract, correct dependencies, and a planned minimum executor tier.
minimum_driver_tier: B
---

# sv-beads

Convert an approved specification into a `bd` dependency graph.

Beads say **what**. Skills say **how**. This skill only produces the what.

## When to use

Use after a specification has survived independent review and the human has adjudicated the findings. Decomposing an unreviewed specification propagates its defects into every bead.

## Procedure

1. Read the approved specification and the relevant milestone.
2. Identify coherent work units, each producing meaningful observable behavior.
3. Create the epic and its beads.
4. Establish dependencies and confirm the ordering is actually implementable.
5. Write the minimum bead contract for every bead.
6. Assign each bead a planned minimum executor tier.
7. Add detail in proportion to ambiguity, consequence, and intended executor.
8. Preserve specification references on every bead.
9. Validate the graph against the approved plan: every milestone outcome is covered, and nothing was invented that the specification does not call for.

## Decomposition size

A bead is a coherent unit that can be implemented and verified.

Avoid pathological over-decomposition:

```text
Create DTO
Add constructor
Add getter
Write one test
```

That is not decomposition, it is dictation, and it costs more to author and finalize than to implement.

Prefer units that produce meaningful, observable behavior. If a bead cannot be verified on its own, it is probably part of a neighbouring bead.

## Minimum bead contract

Every executable bead has:

- **Objective** — what this bead accomplishes.
- **Acceptance criteria** — observable conditions defining success.
- **Constraints** — what must not be violated. It is legitimate for this to say only that existing project conventions apply.
- **Specification reference** — pointer to the approved design, issue, milestone, or artifact.
- **Minimum executor tier** — the lowest capability you believe can reliably execute the bead *at the level of detail you provided*.

```yaml
minimum_executor_tier: C
```

This is a planning estimate, not a permanent routing command. Finalization may raise it.

## Expanded bead

Add these only when they earn their place:

- **Context** — relevant architectural or repository context.
- **Relevant files or components** — likely locations, or analogous existing code.
- **Required behavior** — requirements not obvious from the acceptance criteria.
- **Testing expectations** — tests to add, change, or run.
- **Verification** — known commands or procedures.
- **Dependencies** — required beads, infrastructure, or external conditions.
- **Escalation conditions** — situations where the executor should stop rather than improvise.
- **Skill override** — only when automatic derivation would be wrong, or a project-specific skill must be forced into context. This is an escape hatch, not required metadata.

## The execution-readiness test

> **Can the intended executor tier implement this bead without inventing a consequential decision?**

If not, the bead is either underspecified or assigned too low a capability floor. Fix one of the two. Detail and capability trade against each other: a thinner bead needs a stronger executor.

## Do not encode redundant metadata

Do not write domain labels the work itself already makes evident:

```text
runner: frontend
```

Skill selection is **derived** at finalization, not authored here. A bead that says "add the preferences endpoint and wire the setting into the profile page" already tells the finalizer it needs backend, frontend, and testing skills.

## Plan early, finalize late

Create the graph early enough to understand sequencing and scope. Do not try to make detailed execution context permanently accurate — it will go stale as earlier beads land.

Freshness judgment belongs to `sv-finalize`, which runs just before execution.

## Output

Populate `bd` directly, or produce a seed script where the environment requires it. Either way, verify the resulting graph: check that readiness and blocking reflect the intended order before handing off.
