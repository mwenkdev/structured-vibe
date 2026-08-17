---
name: sv-finalize
description: Use when preparing one ready bead for execution against the current repository state, just before implementation begins. Triggers on "finalize this bead", "get this ready to execute", or picking up the next ready bead from bd. Checks the bead against current HEAD, derives required skills, computes the effective executor tier, and produces an execution packet or blocks the bead.
recommends:
  - sv-beads
  - sv-execute
minimum_driver_tier: B
---

# sv-finalize

Prepare one ready bead for execution against current reality.

This is judgment work, not clerical work. It is deliberately not delegated to a low-capability executor merely because that executor will later write the code.

## When to use

Use on a single bead that `bd` reports as ready, immediately before execution. Not in bulk, not in advance. The whole point is that it runs against current `HEAD` rather than the repository state that existed when the bead was authored.

## Procedure

1. **Read the ready bead** — objective, acceptance criteria, constraints, specification reference.
2. **Inspect current repository state.** Look at the code the bead actually touches.
3. **Verify the bead is still consistent with current `HEAD`.** Do its assumptions still hold?
4. **Validate that dependencies produced the expected state.** A closed dependency is a claim, not proof. Check that the thing it was supposed to build actually exists in the shape this bead expects.
5. **Refresh stale but non-consequential execution context.** File paths moved, a helper was renamed, an analogous example is now a better one. Fix these silently.
6. **Determine which skills are required.** Derive them from the work. Do not expect the bead to have declared them.
7. **Apply any explicit skill override** the bead carries.
8. **Inspect the selected skills' minimum driver tiers.**
9. **Reassess implementation complexity against the current code.** The bead's author estimated complexity from the specification; you can see the actual code.
10. **Compute the effective minimum executor tier.**
11. **Produce the execution packet, or block and escalate.**

## Refresh versus escalate

This is the distinction that matters most.

**Refresh** when the change is non-consequential: renamed files, moved directories, a better analogous example, a command that changed name.

**Block and escalate** when current reality invalidates a consequential assumption:

```text
BLOCKED

The bead assumes FooService exposes getFoo(), but the current approved
service contract does not. Satisfying the bead requires a contract change.
```

You are not authorized to redesign the bead so it fits the code. If the bead and the code disagree about something consequential, that is a decision, and decisions escalate.

## Capability calculation

```text
effective minimum executor tier =
    max(
        bead minimum executor tier,
        minimum tiers of selected skills,
        JIT complexity floor
    )
```

Example:

```text
bead minimum:          C
react skill:           C
testing skill:         C
current-code judgment: B

effective minimum:     B
```

You may **raise** the floor when current conditions require it. Be conservative about lowering an authored floor; you are not required to lower it at all.

Tiers are advisory. They guide routing and warn about risk. They are not authorization, and they do not revoke the human's ability to execute anyway.

## Execution packet

Produce a compact packet containing at least:

```text
bead ID
current objective / acceptance criteria
refreshed execution context as needed
selected skills
effective minimum executor tier
relevant constraints
verification expectations
current commit / HEAD reference
```

**Information density, not token volume.** Do not copy large quantities of source code the executor can read for itself. Point at it.

## Escalation

Block the bead rather than proceeding when:

- a consequential assumption no longer holds;
- a dependency did not produce the state this bead needs;
- the bead cannot be executed without a product or architecture decision;
- no available model meets the effective floor and the gap is material.

Blocking is a successful outcome for this skill. Silently patching a broken bead is not.

## Next step

Hand the execution packet and the selected skills to `sv-execute`, routed to the lowest-cost model that satisfies the effective floor.
