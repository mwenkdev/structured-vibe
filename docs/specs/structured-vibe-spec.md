# Structured Vibe — Working Specification

**Status:** Working baseline for implementation and dogfooding  
**Revision:** 2  
**Purpose:** Define a practical, specification-driven workflow for AI-assisted software development that keeps human attention focused on high-value decisions, progressively reduces ambiguity, composes reusable skills around the work, independently verifies implementation, and selects the lowest-cost model capable of meeting the required reliability threshold.

---

## 1. Core Idea

Structured Vibe is a collaborative AI software-development workflow.

The goal is **not full autonomy**.

Successful software development is not autonomous even when performed entirely by humans. Good developers ask questions, challenge assumptions, discover constraints, and escalate when implementation reality conflicts with the design. AI-assisted development should behave the same way.

The goal is:

> **Human attention should be spent deliberately at the highest-value level appropriate to the work.**

Human involvement is desirable when it contributes:

- product judgment;
- architectural judgment;
- tradeoff decisions;
- risk decisions;
- clarification of intent;
- adjudication between credible alternatives.

Human involvement is undesirable when it consists primarily of:

- repeatedly restating settled requirements;
- supervising mechanical implementation;
- rediscovering repository context for a model;
- correcting avoidable scope drift;
- rescuing poorly specified tasks;
- manually checking work that structured verification could check.

A useful shorthand is:

> **Spend intelligence and human attention where they create leverage; make execution as routine as the problem allows.**

---

## 2. What Structured Vibe Optimizes

Structured Vibe does not optimize for autonomy, cheapest tokens, or minimum human interaction in isolation.

It optimizes the **whole development process**.

Relevant costs include:

- human attention;
- planning inference;
- specification inference;
- review inference;
- finalization inference;
- implementation inference;
- verification inference;
- retries;
- failed approaches;
- context reconstruction;
- debugging;
- integration failures;
- rework caused by unclear requirements.

The desired outcome is:

> **The lowest total effort and effective cost required to produce a verified change with acceptable quality.**

Human attention is expected to remain part of the process, but should move upward toward:

- what should be built;
- why it should be built;
- which tradeoffs are acceptable;
- which architectural decisions matter;
- whether the resulting software actually solves the intended problem.

Inference cost remains a real engineering concern.

The workflow should assign work to:

> **The lowest-cost model that meets the required reliability threshold for that work.**

For some work that may be a low-cost local or hosted model. For other work it may be a frontier model. A stronger model winning on total cost after retries is a valid and successful outcome.

---

## 3. Long-Term Vision

Structured Vibe should make development effort roughly commensurate with idea complexity.

For example:

1. Des describes an idea.
2. Mike supplies the idea and any relevant artifacts.
3. A planning model explores the idea and asks consequential questions.
4. Mike participates at the product and architecture level where useful.
5. A second model independently reviews the specification.
6. The author revises the specification until remaining objections are low-value, intentionally rejected, or no longer material.
7. The approved design is compiled into a `bd` epic and dependency graph.
8. A ready bead is finalized against current repository state.
9. Finalization determines which reusable skills are required and what model capability floor applies.
10. A general execution model loads the finalized bead and selected skills.
11. The implementation is independently verified when required.
12. The bead is closed only after required verification succeeds.
13. Design conflicts or capability failures are escalated rather than silently improvised.

For appropriately sized ideas, a working prototype should be achievable in a day or two with human effort concentrated primarily on product judgment and consequential decisions.

The desired experience is not:

> Press a button and software magically appears.

It is closer to:

> Have the necessary high-value conversations, then allow the workflow to perform as much repeatable engineering work as the problem safely and economically permits.

---

## 4. Primary Outputs

Structured Vibe should produce three durable outputs.

### 4.1 Decision Clarity

The process should preserve important answers to:

- What are we building?
- Why are we building it this way?
- Which alternatives were considered?
- Which constraints matter?
- Which assumptions remain?

### 4.2 Executable Specification

The approved design should be converted into implementation work detailed enough that the assigned coding model can execute it without redesigning the feature.

### 4.3 Traceability

The process should preserve a path from:

```text
intent
  ↓
specification
  ↓
reviewed decisions
  ↓
bead
  ↓
finalized execution packet
  ↓
implementation
  ↓
verification
  ↓
commit / PR
```

Model routing and cost optimization sit on top of these outputs rather than replacing them.

---

## 5. System Model

Structured Vibe separates **what**, **how**, **who**, and **state**.

### 5.1 Bead — What

A bead describes the coherent unit of work to be completed.

It owns:

- objective;
- acceptance criteria;
- constraints;
- specification references;
- dependencies;
- sufficient execution context;
- a planned minimum executor capability when known.

A bead should not normally encode redundant domain labels such as:

```text
runner: frontend
```

when the work itself makes the domain evident.

---

### 5.2 Skill — How

A skill describes a reusable way to perform a class of work.

Examples may eventually include:

- React/frontend implementation;
- Java backend implementation;
- database migrations;
- Kubernetes changes;
- QA;
- security review;
- API design conventions;
- project-specific testing procedures.

A skill may declare the minimum model capability required to drive it reliably.

Example conceptual metadata:

```yaml
name: kubernetes-change
minimum_driver_tier: B
```

Skills should be created and evolved because repeated work demonstrates their usefulness, not because every technical category needs a skill in advance.

---

### 5.3 Finalizer / Dispatcher — Which

The finalization step determines:

- whether the bead is still valid against current `HEAD`;
- which skills the bead requires;
- whether the bead needs additional execution context;
- whether the original capability estimate is still valid;
- whether work can proceed or should be blocked/escalated.

Skill selection is normally **derived**, not redundantly authored.

The finalizer may select multiple skills for one bead.

Example:

```text
Bead:
"Add the preferences endpoint and wire the setting into the profile page."

Derived skills:
- java-backend
- react-frontend
- testing
```

An optional bead-level skill override may exist for cases where inference would be wrong or a project-specific skill must be forced into context.

---

### 5.4 Model — Who

The model is the worker selected to execute or review a step.

Models are selected by required capability rather than permanent identity such as "frontend agent."

The same general coding model may execute:

```text
finalized bead
+ backend skill
+ java skill
+ database skill
```

and later execute:

```text
different finalized bead
+ frontend skill
+ react skill
+ testing skill
```

Specialization should live primarily in skills and context rather than a growing zoo of permanently specialized agents.

---

### 5.5 `bd` — State

`bd` is the work-state and dependency layer.

It represents:

- epics;
- beads;
- dependencies;
- readiness;
- blocked state;
- closure;
- traceability references.

---

## 6. Guiding Principles

### 6.1 Ask Rather Than Invent

A model should ask or escalate when an unresolved answer materially affects:

- product behavior;
- architecture;
- externally visible contracts;
- data ownership or persistence;
- security;
- compatibility;
- meaningful implementation scope;
- acceptance criteria.

Questions are part of engineering and are not a failure of autonomy.

Models should still make ordinary local implementation decisions expected of a competent developer.

The rule is:

> **Do not make unresolved consequential assumptions.**

---

### 6.2 Human Attention Belongs at the Right Altitude

Structured Vibe should not attempt to remove Mike from the process.

It should reduce attention required at the level of ordinary implementation work while preserving or increasing attention at higher-value levels.

Preferred human touchpoints include:

- product intent;
- consequential requirements;
- architecture and major technology choices;
- disagreement between credible alternatives;
- risk acceptance;
- specification adjudication;
- integration-level judgment;
- deciding whether the result actually solves the intended problem.

The workflow should actively reduce human demand for:

- repetitive implementation supervision;
- mechanical code review;
- task-state tracking;
- repeated repository explanation;
- preventable clarification caused by weak decomposition.

---

### 6.3 Progressive Reduction of Ambiguity

Each phase removes a different class of uncertainty.

```text
Idea / artifacts
      ↓
What are we actually building?
      ↓
What decisions matter?
      ↓
What are the milestones and dependencies?
      ↓
What coherent work units implement them?
      ↓
Is this bead still valid against current HEAD?
      ↓
Which skills are required?
      ↓
What capability is required to drive those skills and this bead?
      ↓
Can the selected model implement it without redesigning it?
      ↓
Does an independent verifier agree that it satisfies the bead?
      ↓
Does the integrated result satisfy the original intent?
```

---

### 6.4 Capability-Based Model Tiers

Workflow definitions should describe capabilities rather than model brands.

#### Tier A — High Reasoning

Use for:

- ambiguous product design;
- architecture;
- difficult tradeoffs;
- high-level specification;
- adversarial specification review;
- difficult escalation.

#### Tier B — Strong Engineering

Use for:

- detailed decomposition;
- JIT finalization and judgment;
- complex implementation;
- difficult debugging;
- complex verification;
- escalation from lower-cost execution.

#### Tier C — Cost-Efficient Execution

Use where demonstrated reliable for:

- sufficiently specified implementation;
- mechanical refactors;
- test scaffolding;
- repetitive changes;
- constrained verification.

The tier definitions are intentionally abstract. Models may move between practical tiers as capabilities and pricing change.

---

### 6.5 Independent Specification Review

Important specifications should normally involve:

- an authoring model;
- an independent reviewing model;
- a human adjudicator.

The reviewer should produce findings rather than silently rewrite the specification.

Example:

```text
R-12 — BLOCKING
Authentication lifecycle is unspecified.

Reason:
Milestones M4 and M7 require identity but no earlier milestone establishes it.

Suggested resolution:
Use the existing session model or explicitly define a token-based model.
```

The author may accept, reject, or resolve each finding.

The human remains the tie-breaker.

---

### 6.6 Reviewers Are Allowed to Pass

Review models tend to generate objections because they were asked to review.

`sv-review` must explicitly allow:

> **No material findings.**

Default review output should contain:

1. blocking findings;
2. major findings;
3. optionally a small number of genuinely useful minor findings;
4. a short statement of what appears sound and complete.

A reasonable initial finding budget is:

- maximum 5 blocking findings;
- maximum 8 major findings;
- minor findings only when they materially improve correctness, usability, maintainability, security, or implementation clarity.

These are limits, not quotas.

---

### 6.7 Beads as Executable Specification

The approved design should be compiled into a `bd` graph.

A bead should represent a coherent unit of work that can be implemented and verified.

Avoid pathological over-decomposition such as:

```text
Create DTO
Add constructor
Add getter
Write one test
```

Prefer coherent units that produce meaningful, observable behavior.

---

### 6.8 Plan Early, Finalize Late

The dependency graph should be created early enough to understand sequencing and scope.

Detailed execution assumptions may become stale as earlier beads land.

Therefore:

> **Plan the bead graph early, but finalize each bead just before execution.**

The JIT finalization step owns freshness judgment and skill selection.

---

## 7. Scaled Execution-Ready Bead Contract

All beads share one conceptual schema, but detail scales with risk, ambiguity, and intended executor capability.

The methodology should not require ceremonial sections for trivial work.

### 7.1 Minimum Bead

Every executable bead should include:

#### Objective
What this bead accomplishes.

#### Acceptance Criteria
Observable conditions defining success.

#### Constraints
Important things that must not be violated. This may explicitly state that only existing project conventions apply.

#### Specification Reference
Pointer to the relevant approved design, issue, milestone, or artifact.

#### Minimum Executor Tier
The minimum capability level the bead author believes can reliably execute the bead at the level of detail provided.

Example:

```yaml
minimum_executor_tier: C
```

This is a **planning estimate**, not a permanent routing command.

---

### 7.2 Expanded Bead

Add as needed:

#### Context
Relevant architectural or repository context.

#### Relevant Files / Components
Likely implementation locations or analogous code.

#### Required Behavior
Specific requirements not obvious from acceptance criteria.

#### Testing Expectations
Tests that should be added, changed, or run.

#### Verification
Known commands or procedures.

#### Dependencies
Required beads, infrastructure, or external conditions.

#### Escalation Conditions
Known situations where the executor should stop rather than improvise.

#### Optional Skill Override
A manually specified skill or set of skills when automatic derivation should be overridden.

Example:

```yaml
skills_override:
  - project-specific-auth-migration
```

This is an escape hatch, not required metadata.

---

### 7.3 Full Bead

A bead targeted at a context-limited executor, or touching consequential areas, should normally include enough expanded detail that the executor does not need to perform product or architecture design.

The execution-readiness test is:

> **Can the intended executor tier implement this bead without inventing a consequential decision?**

If not, the bead is either underspecified or assigned too low a capability floor.

---

## 8. Skill Contract

Skills are reusable execution knowledge.

A skill should define, where useful:

- purpose;
- when to apply it;
- required inputs/context;
- procedure or heuristics;
- project/domain conventions;
- verification expectations;
- known failure/escalation conditions;
- minimum driver capability.

Example conceptual frontmatter:

```yaml
name: react-frontend
minimum_driver_tier: C
```

A more judgment-heavy skill might declare:

```yaml
name: kubernetes-production-migration
minimum_driver_tier: B
```

The skill owns the capability needed to use that procedure correctly.

The bead does not duplicate this information.

---

## 9. `sv-finalize` — JIT Finalization and Dispatch

`sv-finalize` is a judgment-bearing workflow skill and is distinct from implementation.

Its responsibilities are:

1. read the ready bead;
2. inspect the current repository state;
3. verify the bead remains consistent with current `HEAD`;
4. validate that dependencies actually produced the expected state;
5. refresh stale but non-consequential execution context;
6. determine which domain/project skills are required;
7. apply any explicit skill override;
8. inspect the selected skills' minimum driver tiers;
9. reassess implementation complexity against current code;
10. compute the effective minimum executor tier;
11. produce a finalized execution packet or block/escalate the bead.

`sv-finalize` itself requires sufficient reasoning capability to make these judgments. The initial working assumption is **Tier B or better**.

This is intentionally not delegated to a low-capability executor merely because that executor will later write the code.

---

## 10. Capability Calculation and Routing

The bead author records a planned minimum executor tier.

Selected skills record their own minimum driver tiers.

JIT finalization may discover additional complexity.

Conceptually:

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

Finalization may **raise** the capability floor when current conditions require it.

Automatic lowering of an authored capability floor should be conservative and is not required initially.

The router then selects the lowest-cost available model that satisfies the effective floor and other environment constraints.

---

## 11. Execution Packet

`sv-finalize` should produce a compact execution packet containing at least:

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

The packet should avoid copying large quantities of source code that the executor can inspect directly.

The packet should provide high information density, not maximal token volume.

---

## 12. `sv-execute` — Execution Responsibilities

`sv-execute` receives the finalized execution packet and selected skills.

The executor may:

- inspect the repository;
- make ordinary local implementation decisions;
- implement specified behavior;
- add or modify tests;
- fix defects directly related to the bead;
- run validation;
- inspect its own diff;
- iterate until acceptance criteria appear satisfied.

The executor may **not** silently:

- change approved architectural decisions;
- expand product scope;
- weaken acceptance criteria;
- change external contracts without authorization;
- rewrite dependent beads merely to fit its implementation;
- disable, delete, or weaken tests to obtain a passing result;
- mark work complete when required validation fails.

If implementation reveals a consequential conflict, the correct result may be:

```text
BLOCKED

The bead assumes FooService exposes getFoo(), but the current approved
service contract does not. Satisfying the bead requires a contract change.
```

The executor should favor implementation over redesign.

---

## 13. Independent Verification

Non-trivial beads should not be closed solely because the implementing model believes they are complete.

A fresh verification pass should independently compare:

- the finalized bead;
- the approved specification where relevant;
- the implementation diff;
- tests;
- verification output.

Preferably the verifier should:

- run in fresh context;
- not inherit the executor's rationalizations;
- be allowed to reject completion;
- be allowed to say the implementation is correct.

The verifier should look specifically for:

- weakened or removed tests;
- partial implementation of acceptance criteria;
- unintended scope changes;
- missing edge cases;
- error-handling regressions;
- API or compatibility changes;
- implementation that passes tests but violates requested behavior.

### 13.1 Trivial Work

Truly trivial work may use self-verification.

The initial implementation should keep this exception narrow and evolve it through use.

### 13.2 Verification Outcome

A bead may close when:

- required tests pass;
- required validation passes;
- independent verification passes when required;
- acceptance criteria are satisfied.

---

## 14. Human Code Review Policy

Human diff review is deliberate rather than automatically required for every bead.

Mike should normally review code when:

- a milestone or epic reaches an integration boundary;
- independent verification flags a material concern;
- an executor or verifier escalates;
- the change is unusually consequential or risky;
- the implementation contains a design tradeoff that requires human judgment;
- Mike simply wants to inspect the work.

Routine low-risk beads that pass independent verification do not automatically require line-by-line human review.

The intent is to preserve human judgment without recreating ordinary implementation-level supervision as a mandatory bottleneck.

---

## 15. Escalation Carries the Failed Attempt

When an executor fails or escalates, the next model should not restart from zero unless there is a reason to discard prior work.

The escalation package should carry forward, where useful:

- current diff;
- compiler/test output;
- relevant logs;
- attempted approach;
- what failed;
- suspected invalid assumption;
- why escalation occurred;
- selected skills;
- effective executor tier.

A stronger model may:

- continue the implementation;
- partially or completely revert the failed attempt;
- identify that the bead is underspecified;
- escalate the issue back to finalization, decomposition, or planning.

---

## 16. Git Traceability Contract

Structured Vibe should make the relationship between intent and code mechanically discoverable.

At minimum:

- implementation work references the bead ID;
- completed bead work records the relevant commit SHA or commit range;
- commits include the bead ID in a predictable form;
- PRs reference relevant epic/beads when appropriate;
- verification results can be associated with the bead and implementation.

The methodology does **not** require one branch per bead.

Branch strategy may vary by project.

A useful default is:

> **One bead maps cleanly to an identifiable commit or commit range.**

---

## 17. Workflow Skills

The initial Structured Vibe workflow skill set is:

### `sv-plan`

**Purpose:** Turn an idea, repository, and supporting artifacts into a high-level specification.

Responsibilities:

- inspect artifacts and relevant source;
- understand the requested outcome;
- identify consequential product decisions;
- identify consequential architectural decisions;
- identify milestones and dependencies;
- ask useful questions;
- distinguish decisions needed now from implementation details that can wait;
- produce a structured high-level specification.

---

### `sv-review`

**Purpose:** Independently challenge an existing specification.

Responsibilities:

- identify contradictions;
- find missing requirements;
- identify risky assumptions;
- detect unnecessary complexity;
- find missing acceptance criteria;
- identify ambiguous implementation requirements;
- identify architectural or security concerns;
- classify findings by severity;
- explain why findings matter;
- respect the finding budget;
- explicitly say when no material findings remain;
- identify portions already sound.

The reviewer should not rewrite the specification unless explicitly asked.

---

### `sv-beads`

**Purpose:** Convert an approved specification or milestone into a coherent `bd` dependency graph.

Responsibilities:

- create appropriate epics and beads;
- establish dependencies;
- order work correctly;
- avoid unnecessary over-decomposition;
- create the minimum bead contract for every bead;
- assign a planned minimum executor tier;
- add detail according to ambiguity, consequence, and intended executor;
- preserve specification references;
- create a seed script or directly populate `bd`, depending on environment;
- validate the graph against the approved plan.

`sv-beads` plans the work but does not assume detailed repository context will remain current forever.

---

### `sv-finalize`

**Purpose:** Prepare one ready bead for execution against current reality.

Responsibilities:

- perform freshness judgment;
- refresh safe stale context;
- derive required skills;
- apply optional overrides;
- compute effective executor capability;
- produce an execution packet;
- block/escalate when current reality invalidates consequential assumptions.

Initial minimum driver tier: **B**.

---

### `sv-execute`

**Purpose:** Implement a finalized bead using the selected skills.

Responsibilities:

- read the execution packet;
- load/use required skills;
- inspect relevant code;
- implement the requested behavior;
- test and validate;
- self-inspect;
- produce an execution result;
- escalate rather than redesign consequential decisions.

---

### `sv-verify`

**Purpose:** Independently determine whether the implementation satisfies the finalized bead and required quality checks.

Responsibilities:

- inspect fresh context;
- compare diff to acceptance criteria;
- inspect tests and validation;
- detect weakened tests or scope drift;
- approve, reject, or escalate completion.

The appropriate verifier capability may vary with bead complexity and should be learned through dogfooding.

---

## 18. Domain and Project Skills

Domain skills are composed into execution as needed.

Potential examples include:

```text
frontend
react
backend
java
database
qa
devops
kubernetes
security
api-design
migration
project-specific-build
project-specific-testing
```

These are examples, not an initial checklist.

Do **not** create all of them before there is repeated work that justifies them.

A useful evolution rule is:

> If the same procedural guidance must be explained repeatedly, it is a candidate skill.

Domain specialization should primarily be expressed through these reusable skills rather than permanent specialized agents.

---

## 19. End-to-End Workflow

### Phase 0 — Gather Inputs

Collect as appropriate:

- idea/summary;
- user stories;
- tear sheets;
- mocks;
- diagrams;
- relevant documentation;
- existing codebase;
- known constraints.

---

### Phase 1 — High-Level Specification

Run `sv-plan` with a high-reasoning model.

The planner:

1. inspects artifacts and repository;
2. asks consequential questions;
3. establishes major product and architectural decisions;
4. defines milestones;
5. identifies dependencies;
6. defines testable outcomes.

Output: proposed high-level specification.

---

### Phase 2 — Independent Review and Consensus

Run `sv-review` with a different high-quality reasoning model.

The reviewer returns bounded findings and identifies sound portions.

The author revises.

The human adjudicates disagreements.

Repeat while review is finding material improvements.

Stop when remaining proposed changes are:

- pedantic;
- subjective;
- duplicates;
- already addressed;
- intentionally rejected;
- or not worth their cost/complexity.

Unanimous stylistic agreement is not required.

---

### Phase 3 — Bead Graph Compilation

Run `sv-beads`.

For each milestone:

1. identify coherent work units;
2. create epic/bead hierarchy;
3. establish dependencies;
4. create minimum bead contracts;
5. set planned executor floors;
6. add detail according to risk, ambiguity, and expected executor;
7. preserve specification references.

---

### Phase 4 — JIT Finalization

For each ready bead:

```text
ready bead
   ↓
sv-finalize
   ↓
freshness judgment against current HEAD
   ↓
refresh safe context OR escalate design conflict
   ↓
derive required skills
   ↓
compute effective minimum executor tier
   ↓
finalized execution packet
```

---

### Phase 5 — Execution

```text
finalized execution packet
   +
selected skills
   ↓
route to lowest-cost qualifying model
   ↓
sv-execute
   ↓
implementation + tests + result package
```

---

### Phase 6 — Verification and Closure

```text
implementation result
   ↓
sv-verify when required
   ↓
pass?
 /    \
yes    no
 ↓      ↓
record   fix / escalate
trace
 ↓
close bead
```

---

### Phase 7 — Integration Review

At appropriate epic or milestone boundaries:

- run broader tests;
- examine interactions between completed beads;
- verify milestone outcomes;
- compare integrated behavior to approved specification;
- perform human code review at the appropriate altitude;
- surface integration defects or design drift.

---

## 20. Model Escalation Ladder

Routing should allow economical models to attempt appropriate work without encouraging improvisation beyond their capability.

```text
finalized bead
      ↓
lowest-cost model satisfying effective floor
      ↓
success?
  ┌───────┴────────┐
 yes               no / blocked
  ↓                     ↓
verify             carry failed attempt upward
                        ↓
                 stronger engineering model
                        ↓
                 execution problem?
                    /         \
                  yes          no
                   ↓            ↓
                continue     finalization/spec escalation
                                ↓
                       planning/decomposition if needed
```

A failed low-cost attempt is acceptable when escalation is efficient and preserves useful information.

---

## 21. Lightweight Initial Metrics

Do not build a telemetry platform before the workflow has enough use to justify one.

For each executed bead, record a compact result such as:

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

Approximate cost or elapsed minutes may be recorded when easy.

These data are initially observational.

Detailed metrics should be added only to answer concrete questions such as:

- Does a fuller bead contract reduce retries?
- Is Tier C economical for this class of work?
- Which skills consistently force Tier B?
- Is `sv-verify` catching meaningful defects?
- How often does JIT finalization raise the planned capability floor?
- Where is Mike's attention actually being consumed?

---

## 22. Skill Evolution

Skills are hypotheses about repeatable process, not permanent law.

When something goes wrong:

```text
Use workflow / skill
   ↓
Observe problem
   ↓
Classify cause
   ├── workflow skill deficiency → improve workflow skill
   ├── domain skill deficiency → improve domain skill
   ├── spec/bead deficiency → improve contract/decomposition
   ├── model capability issue → change routing/floor
   ├── stale task context → improve JIT finalization
   ├── project context issue → improve shared context
   └── one-off oddity → do not create a permanent rule
```

Not every incident deserves another instruction.

Overfitting skills to isolated failures will make them bloated and less effective.

---

## 23. Skills Before Agents

Initial development should focus on:

- useful workflow skills;
- reusable domain skills;
- stable information contracts;
- repeatable review behavior;
- reliable finalization;
- reliable execution;
- independent verification;
- practical escalation.

Possible progression:

```text
rough skills
    ↓
real-world use
    ↓
stable contracts
    ↓
reliable role behavior
    ↓
optional agents
    ↓
workflow orchestration
    ↓
adaptive model routing
```

Agents may eventually own stable portions of the process.

No part of Structured Vibe requires full agent autonomy to be valuable.

---

## 24. Proportional Process

Process should scale with risk, ambiguity, and size.

### Tiny Change

```text
brief clarification if needed
→ minimum bead
→ lightweight finalization
→ execute
→ self-verify if truly trivial
```

### Medium Feature

```text
structured planning
→ independent spec review
→ bead graph
→ JIT finalization + skill selection
→ implementation
→ independent verification
→ integration check
```

### Large Project

```text
substantial specification
→ multi-round reviewed milestones
→ dependency graph
→ staged JIT finalization
→ skill-composed execution
→ model-tiered routing
→ independent verification
→ milestone integration review
```

The methodology exists to eliminate wasted effort, not create ceremony.

---

## 25. Definition of Success

Structured Vibe is successful when:

- human attention is concentrated on product, architecture, tradeoffs, risk, and integration judgment;
- consequential decisions are made deliberately;
- models ask useful questions instead of inventing consequential assumptions;
- specifications survive independent challenge;
- reviewer noise is bounded;
- bead detail scales with the work instead of becoming ceremony;
- beads state what must happen without redundant domain metadata;
- JIT finalization catches stale assumptions before implementation;
- required skills are derived and composed automatically when practical;
- skill capability requirements participate in model routing;
- execution work remains traceable to specification and bead;
- non-trivial work receives independent verification;
- failed attempts carry useful context into escalation;
- low-cost models are used where they are actually economical and reliable;
- stronger models are used when their additional capability reduces total effort;
- domain expertise accumulates in reusable skills rather than permanent agent identities;
- prototype and feature turnaround decreases without sacrificing confidence in the result.

---

## 26. Initial Implementation Plan

### SV-1 — Create the Core Workflow Skills

Create rough initial versions of:

- `sv-plan`
- `sv-review`
- `sv-beads`
- `sv-finalize`
- `sv-execute`
- `sv-verify`

Keep them concise enough to evolve easily.

### SV-2 — Operationalize the Bead Contract

Turn Section 7 into practical instructions/checks for `sv-beads` and `sv-finalize`.

### SV-3 — Define Initial Skill Metadata

Establish the minimal metadata required for reusable domain skills, including:

```text
name
minimum_driver_tier
```

Avoid designing a large skill taxonomy.

### SV-4 — Dogfood on Real Work

Use the workflow on actual projects.

Observe:

- where Mike's attention was valuable;
- where Mike's attention was wasted;
- where executors lacked information;
- where beads became stale;
- where skill selection was wrong;
- where capability floors were too high or too low;
- where reviewers generated noise;
- where independent verification caught defects;
- where cheap models succeeded or failed;
- where stronger models were cheaper in practice.

### SV-5 — Add Domain Skills Only from Repetition

Create domain/project skills when repeated procedural knowledge emerges during dogfooding.

Do not build the entire skill library speculatively.

### SV-6 — Revise Contracts and Routing

Improve recurring deficiencies and refine capability routing from observed outcomes.

### SV-7 — Automate Stable Portions

Only automate workflow sections that have demonstrated reliable behavior.

---

## 27. Open Questions for Dogfooding

The following are intentionally left empirical:

1. What threshold should distinguish trivial from independently verified work?
2. What capability tier is sufficient for `sv-verify` across different bead classes?
3. How often should `sv-finalize` be allowed to revise bead execution context?
4. Should the planned minimum executor tier ever be automatically lowered?
5. Which domain skills emerge naturally from repeated work?
6. Which project context should be persistent versus rediscovered?
7. What task characteristics best predict the optimal executor model?
8. At what point should `sv-execute` loop automatically over multiple ready beads?
9. When does an agent add enough value over skill-driven execution to justify its complexity?
10. At what scale does milestone integration review deserve its own dedicated workflow skill?

These should be answered through use rather than additional speculative design wherever practical.

---

## 28. Working Philosophy

Structured Vibe should remain practical, empirical, and replaceable in parts.

No skill, model tier, workflow step, or contract should be protected merely because it appears in this document.

The methodology should evolve toward a development process in which:

> **Beads say what. Skills say how. Finalization determines what knowledge and capability are needed now. Models do the work. Verification checks the work. Humans provide judgment at the altitude where their attention has the most leverage.**

This Revision 2 is the working baseline.

The next step is implementation and dogfooding, not another abstract review cycle.
