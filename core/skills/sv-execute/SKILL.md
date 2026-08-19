---
name: sv-execute
description: Use when implementing a finalized bead from an execution packet produced by sv-finalize. Triggers on "execute this bead", "implement this packet", or picking up finalized work. Implements the specified behavior using the selected skills, validates it, and escalates consequential conflicts rather than redesigning approved decisions.
minimum_driver_tier: C
---

# sv-execute

Implement a finalized bead using the selected skills.

Favor implementation over redesign. You are here to build the thing that was decided, not to relitigate the decision.

## When to use

Use when you have an execution packet from `sv-finalize`. If you do not have one, the bead has not been checked against current `HEAD` and you may be implementing stale assumptions — run `sv-finalize` first.

## Procedure

1. **Read the execution packet** — objective, acceptance criteria, constraints, verification expectations, HEAD reference.
2. **Load the selected skills** and follow their procedures and conventions.
3. **Inspect the relevant code** before writing any. The packet points at locations; go read them.
4. **Implement the requested behavior.**
5. **Add or modify tests** per the testing expectations.
6. **Run validation** — build, tests, linters, whatever the project requires.
7. **Inspect your own diff.** Read it as if someone else wrote it.
8. **Iterate** until the acceptance criteria appear satisfied.
9. **Produce an execution result.**

## What you may do

- inspect the repository freely;
- make ordinary local implementation decisions a competent developer would make without asking;
- implement the specified behavior;
- add or modify tests;
- fix defects directly related to the bead;
- run validation;
- iterate on your own work.

## What you may not do silently

- change approved architectural decisions;
- expand product scope;
- weaken acceptance criteria;
- change external contracts without authorization;
- rewrite dependent beads merely to fit your implementation;
- disable, delete, or weaken tests to obtain a passing result;
- mark work complete when required validation fails.

The test-weakening prohibition is absolute. If a test fails, either the implementation is wrong or the test encodes a decision you are not authorized to change. Both cases escalate. Deleting the test is neither.

## When to stop and escalate

If implementation reveals a consequential conflict, the correct result is to stop:

```text
BLOCKED

The bead assumes FooService exposes getFoo(), but the current approved
service contract does not. Satisfying the bead requires a contract change.
```

Escalate when you encounter a decision affecting product behavior, architecture, external contracts, data ownership or persistence, security, compatibility, meaningful scope, or acceptance criteria.

**Do not make unresolved consequential assumptions.** A blocked bead with a clear reason is a better outcome than a completed bead built on a guess.

## Carry the failed attempt

If you escalate or fail, do not throw away what you learned. The next model should not restart from zero. Include, where useful:

- the current diff;
- compiler and test output;
- relevant logs;
- the approach you attempted;
- what failed;
- the assumption you suspect is invalid;
- why you escalated;
- the selected skills and effective executor tier.

A failed low-cost attempt is acceptable when escalation is efficient and preserves useful information.

## Execution result

Report:

- what you implemented;
- which acceptance criteria you believe are satisfied, and how;
- tests added or changed;
- validation commands run and their output;
- anything you decided that a reviewer might disagree with;
- anything you could not do.

State what you did, not that it is correct. Deciding correctness is `sv-verify`'s job, and it runs in fresh context so it does not inherit your reasoning.

## Traceability

Reference the bead ID in the work, and make sure the bead can be associated with the resulting commit or commit range. One bead should map cleanly to an identifiable commit or commit range.
