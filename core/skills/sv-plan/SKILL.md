---
name: sv-plan
description: Use when turning an idea, feature request, or pile of artifacts into a high-level specification before any code is written. Triggers on "plan this", "spec this out", "write a specification", "what should we build", or the start of a new feature or project. Produces a structured specification with explicit decisions, milestones, and open questions, and asks consequential questions rather than inventing answers.
recommends:
  - sv-review
  - sv-beads
minimum_driver_tier: A
---

# sv-plan

Turn an idea, a repository, and supporting artifacts into a high-level specification.

The output is a specification a human can adjudicate and an independent reviewer can attack. It is not an implementation plan and not a task list.

## When to use

Use at the start of work whose shape is not yet settled: a new feature, a new project, a significant refactor, or any request where "what are we building" has more than one credible answer.

Do not use for work that is already unambiguous. A one-line fix does not need a specification. Process scales with risk, ambiguity, and size.

## Procedure

1. **Gather inputs.** Read the idea or request, then the artifacts: user stories, mocks, diagrams, tear sheets, existing documentation, known constraints. Inspect the repository and the code that the work will touch.
2. **Establish what is actually being asked.** Restate the requested outcome in your own words. If your restatement and the request diverge, that divergence is your first question.
3. **Separate decisions from details.** Sort every open point into: must be decided now because later work depends on it, or can be safely deferred to implementation. Only the first group belongs in the conversation with the human.
4. **Identify consequential product decisions.** What behavior is user-visible? What is in scope and what is deliberately not? What does success look like to the person who asked?
5. **Identify consequential architectural decisions.** Data ownership and persistence, external contracts, security and identity, compatibility, major technology choices, and the boundaries between components.
6. **Surface alternatives.** Where two credible approaches exist, name both and state the tradeoff. Do not silently pick one and present it as the only option.
7. **Ask.** Put the consequential questions to the human, grouped and explained. Say why each one matters and what depends on the answer.
8. **Define milestones.** Break the work into milestones that each produce meaningful, observable behavior. Establish the dependencies between them.
9. **Define testable outcomes.** For each milestone, state the observable conditions that mean it is done.
10. **Write the specification.**

## Asking is the job

Ask or escalate when an unresolved answer materially affects product behavior, architecture, externally visible contracts, data ownership or persistence, security, compatibility, meaningful implementation scope, or acceptance criteria.

The rule is: **do not make unresolved consequential assumptions.**

Questions are part of engineering, not a failure of autonomy. But still make the ordinary local decisions a competent developer would make without asking. Do not ask questions merely to avoid deciding something the artifacts already constrain.

## Output shape

Produce a specification containing:

- **Purpose** — what this is and why it is being built.
- **Scope** — what is included, and what is explicitly excluded.
- **Decisions** — each consequential decision, the alternatives considered, and why this one. Preserve the reasoning, not just the conclusion.
- **Constraints** — what must not be violated.
- **Milestones** — ordered, with dependencies and testable outcomes.
- **Open questions** — what remains unresolved, and who must resolve it.
- **Assumptions** — what you assumed in the absence of an answer, stated plainly so a reviewer can challenge it.

Record answers to: what are we building, why this way, which alternatives were considered, which constraints matter, and which assumptions remain.

## Escalation

Stop and escalate rather than proceeding when:

- the request conflicts with an existing approved decision;
- the artifacts contradict each other on a consequential point;
- the work cannot be specified without a decision only the human can make;
- the scope is materially larger than the request implies.

## Next step

Hand the specification to `sv-review`, run with a different high-capability model, then revise. Once the specification survives review and the human has adjudicated, `sv-beads` compiles it into a dependency graph.
