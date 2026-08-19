---
name: sv-review
description: Use when independently challenging an existing specification, design document, or plan before work is decomposed or implemented. Triggers on "review this spec", "poke holes in this", "critique this design", or a handoff from sv-plan. Produces bounded, severity-classified findings and is explicitly allowed to report that nothing material is wrong.
minimum_driver_tier: A
---

# sv-review

Independently challenge a specification.

Run this with a different model than the one that authored the specification. The value is independence; reviewing your own work reproduces your own blind spots.

## When to use

Use on a specification or design that is about to be committed to: before decomposition, before implementation, before a decision becomes expensive to reverse.

## What you are looking for

- contradictions between sections;
- missing requirements;
- risky or unstated assumptions;
- unnecessary complexity;
- missing acceptance criteria;
- ambiguous implementation requirements that will force the implementer to redesign;
- architectural concerns;
- security concerns;
- ordering problems, where a milestone needs something no earlier milestone establishes.

## You are allowed to pass

This matters more than the finding list.

Review models generate objections because they were asked to review. That reflex produces noise, and noise costs human attention, which is the thing this workflow exists to protect.

**"No material findings" is a valid and useful result.** Say it plainly when it is true.

## Finding budget

- at most **5 blocking** findings;
- at most **8 major** findings;
- minor findings **only** when they materially improve correctness, usability, maintainability, security, or implementation clarity.

These are limits, not quotas. Producing five blocking findings because five are allowed is a failure of the skill.

If you find more than the budget allows, report the most consequential ones and say that the specification has systemic problems rather than padding a list.

## Severity

- **BLOCKING** — implementation cannot proceed correctly without resolving this.
- **MAJOR** — implementation can proceed but the result will likely be wrong, unsafe, or need rework.
- **MINOR** — genuinely worth fixing, and you can say why in one sentence.

Do not classify by how confident you feel. Classify by consequence.

## Finding format

```text
R-12 — BLOCKING
Authentication lifecycle is unspecified.

Reason:
Milestones M4 and M7 require identity but no earlier milestone establishes it.

Suggested resolution:
Use the existing session model or explicitly define a token-based model.
```

Every finding states what is wrong, why it matters, and a suggested resolution. A finding without a "why" is an opinion.

## Output shape

1. blocking findings;
2. major findings;
3. minor findings, if any genuinely qualify;
4. **what appears sound and complete** — name the parts that are well specified.

Section 4 is required. It tells the author what not to churn, and it is evidence that the review was calibrated rather than reflexive.

## Constraints

- **Do not rewrite the specification** unless explicitly asked. Produce findings; the author revises.
- Do not restate the same objection in multiple findings.
- Do not raise stylistic preferences as findings.
- Do not require unanimous agreement. The author may accept, reject, or resolve each finding, and the human is the tie-breaker.

## When to stop reviewing

Review rounds stop when remaining proposed changes are pedantic, subjective, duplicates, already addressed, intentionally rejected, or not worth their cost and complexity.

Say so when you reach that point. Do not manufacture a further round.
