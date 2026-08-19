---
name: sv-verify
description: Use when independently checking whether a completed implementation actually satisfies its finalized bead, before the bead is closed. Triggers on "verify this bead", "check this implementation", "is this actually done", or a handoff from sv-execute. Compares the diff against acceptance criteria in fresh context, looks for weakened tests and scope drift, and may approve, reject, or escalate.
minimum_driver_tier: B
---

# sv-verify

Independently determine whether an implementation satisfies its finalized bead.

A non-trivial bead is not closed because the model that implemented it believes it is complete.

## Run in fresh context

Verification is worth roughly nothing if it inherits the executor's reasoning. The executor already convinced itself. Start from the bead and the diff, not from the executor's explanation of why the diff is correct.

You are allowed to reject completion. You are equally allowed to conclude the implementation is correct.

## What to compare

- the finalized bead;
- the approved specification, where relevant;
- the implementation diff;
- the tests;
- the verification output.

## Checklist

Look specifically for:

- **weakened or removed tests** — assertions loosened, cases deleted, tests skipped or marked expected-failure to obtain a green run;
- **partial implementation of acceptance criteria** — three of four criteria satisfied, the fourth quietly unaddressed;
- **unintended scope changes** — work the bead did not ask for;
- **missing edge cases**;
- **error-handling regressions**;
- **API or compatibility changes**;
- **implementation that passes tests but violates requested behavior** — the most common failure, and the reason tests alone are not verification.

Read the test diff before the implementation diff. It is where completion pressure shows up first.

## Verify the claims

Do not take the execution result's word for it. Run the validation yourself where you can. A claim that tests pass is not evidence that tests pass, and a passing suite is not evidence that the suite still tests anything.

## Outcome

A bead may close when:

- required tests pass;
- required validation passes;
- independent verification passes when required;
- acceptance criteria are satisfied.

Report one of:

- **PASS** — with a brief statement of what you checked and why you are satisfied. Name the acceptance criteria and where each is met.
- **REJECT** — with specific, actionable defects. Say which acceptance criterion is unmet and what evidence shows it.
- **ESCALATE** — when the problem is not the implementation. The bead may be underspecified, the specification may be wrong, or the conflict may need a human decision.

Do not reject for style. Do not reject to demonstrate diligence. An implementation that satisfies the bead should be approved, plainly and without hedging.

## Trivial work

Truly trivial work may use self-verification instead of an independent pass.

Keep this exception narrow. The threshold between trivial and independently verified is deliberately unsettled and should be learned from use, not assumed generously.

## Human review

Independent verification does not replace human judgment; it replaces mechanical human review. Flag for human review when:

- a milestone or epic reaches an integration boundary;
- you found a material concern;
- the executor escalated;
- the change is unusually consequential or risky;
- the implementation contains a design tradeoff requiring human judgment.

Routine low-risk beads that pass verification do not need line-by-line human review.

## Record the result

Associate the verification outcome with the bead and the implementation, and record the commit or commit range. For each executed bead, a compact observational record is useful:

```text
bead_id
planned_minimum_tier
selected_skills
effective_tier
model
attempts
verification: pass/fail
escalated: yes/no
human_intervention: none/light/material
```

This is observational data, not a telemetry platform. Record it because it is cheap and answers real questions later.
