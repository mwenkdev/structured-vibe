---
name: roadmap-sync
description: Reconcile this repository's human-facing roadmap with Beads roadmap epics. Use when adding roadmap ideas, seeding roadmap epics, refreshing roadmap state from Beads, or checking roadmap/backlog drift.
---

# Roadmap Sync

Maintain consistency between the repository's human-facing roadmap and the subset of Beads epics that represent roadmap capabilities.

This is repository-specific product workflow.

## Goals

The skill should make it inexpensive to:

- capture a new product or architecture capability,
- turn roadmap capabilities into durable Beads epics,
- refresh roadmap state from Beads,
- detect drift between the two,
- preserve stable identity across title changes,
- and keep implementation/project state out of the human-facing roadmap.

The desired relationship is:

```text
docs/ROADMAP.md
      ↕
 roadmap-sync
      ↕
Beads epics labeled "roadmap"
```

Neither side is wholly generated from the other.

This is a reconciliation workflow with explicit ownership rules.

---

# Sources

Primary sources:

- Human-facing roadmap: `docs/ROADMAP.md`
- Durable roadmap/backlog state: Beads
- Product principles: `docs/PRINCIPLES.md`
- Architecture constraints: `docs/specs/architecture.md`
- Relevant specifications when explicitly referenced by a roadmap item or epic

Use existing repository guidance for Beads operations when available.

If the repository defines a Beads skill, helper, wrapper, or documented command convention, defer to it.

Otherwise use standard `bd` behavior conservatively.

Do not duplicate or fossilize Beads CLI mechanics in this skill.

---

# Scope

Only Beads epics explicitly participating in roadmap synchronization are in scope.

A Beads epic is considered roadmap-scoped when at least one of the following is true:

1. It has the `roadmap` label.
2. It is referenced by explicit roadmap metadata.
3. The human explicitly asks to promote it into roadmap scope.

Do **not** infer that an epic belongs on the roadmap merely because it is an epic.

Implementation epics, cleanup epics, migrations, experiments, release mechanics, and other internal work must not leak into the product roadmap unless deliberately promoted.

New epics created from the roadmap must receive the `roadmap` label.

---

# Ownership Rules

## ROADMAP.md owns

The roadmap is authoritative for human-facing product presentation:

- capability title,
- canonical concise capability summary,
- grouping/section,
- presentation order,
- product-facing wording,
- roadmap intent such as near-term, future, or deferred classification.

When roadmap-owned fields drift, the roadmap value wins, subject to the priority rules below.

For example:

- if the roadmap title changes, update the matching Beads epic title;
- if the roadmap summary changes, update only the dedicated roadmap-summary block in the epic description;
- if the roadmap classification changes between near-term, future, and deferred, update Beads priority according to the priority rules in this skill unless the epic is currently P1.

All such updates must be reported.

## Beads owns

Beads is authoritative for operational state:

- bead ID,
- open/closed state,
- close reason,
- P1 active-work override,
- dependency relationships,
- implementation/decomposition state,
- all epic description content outside the roadmap-summary block.

ROADMAP.md may display some operational state, but must not become a second operational database.

## Shared identity

The stable join key is the Beads epic ID.

Titles are presentation fields, not durable identity.

---

# Roadmap Metadata

Each synchronized roadmap capability should carry its Beads ID using exactly this format:

```md
### Autonomous epic execution

<!-- bd: sv-142 -->

Execute and verify an approved epic until it completes or requires meaningful human input.
```

Use exactly:

```text
<!-- bd: <bead-id> -->
```

Do not invent alternate metadata formats.

The metadata must appear directly beneath the roadmap item's `###` heading, before the capability summary.

---

# Canonical Roadmap Summary in Beads

The roadmap's concise capability summary must be stored inside a dedicated delimited block in the Beads epic description.

Use exactly this format:

```md
<!-- roadmap-summary -->
Execute and verify an approved epic until it completes or requires meaningful human input.
<!-- /roadmap-summary -->
```

The text inside this block is owned by `ROADMAP.md`.

Everything outside this block is Beads-owned unless another repository convention explicitly says otherwise.

A typical epic description may therefore look like:

```md
<!-- roadmap-summary -->
Execute and verify an approved epic until it completes or requires meaningful human input.
<!-- /roadmap-summary -->

## Context

Source: docs/ROADMAP.md, Autonomous epic execution.

## Additional Detail

Further durable context may live here without being overwritten by roadmap sync.
```

## Summary drift

Summary drift is determined only by comparing:

- the canonical concise capability summary in `ROADMAP.md`,
- with the exact contents of the matching epic's `<!-- roadmap-summary -->` block.

Do not compare the roadmap summary against the entire epic description.

Do not use fuzzy or subjective "material change" reasoning for summary drift.

Normalize only insignificant line-ending differences when necessary.

Otherwise treat the summary comparison as deterministic text comparison.

When the roadmap summary changes:

- update only the contents of the `roadmap-summary` block,
- preserve all content outside that block exactly,
- report the update.

Do not overwrite:

- context,
- acceptance criteria,
- notes,
- implementation history,
- architecture constraints,
- or other Beads-owned description content.

## Missing summary block

If an existing matched roadmap epic does not yet contain a `roadmap-summary` block:

- derive the canonical summary from `ROADMAP.md`,
- add the block without deleting existing description content,
- report the self-healing update.

After the block exists, future syncs must compare only that block.

---

# Self-Healing Identity

Matching should become more stable over time.

Use this matching order:

1. Explicit `<!-- bd: ... -->` metadata
2. Exact normalized title match among roadmap-scoped epics
3. Strong semantic candidate
4. Unmatched

## Exact metadata match

If valid bead metadata exists and refers to a roadmap-scoped epic, use it.

If metadata refers to a missing or non-roadmap epic, report the problem and do not silently repair identity.

## Exact title match

If exactly one roadmap-scoped epic matches the normalized title:

- treat it as a match,
- write its Beads ID metadata back into `ROADMAP.md`,
- ensure its epic description contains the canonical `roadmap-summary` block,
- report the self-healing identity update.

After this succeeds, future runs should match by ID.

## Semantic candidates

Semantic matching is advisory only.

If title and ID matching fail but one or more epics appear semantically related:

- report the candidate match or matches,
- explain the evidence,
- do **not** merge automatically,
- require human resolution.

Do not raise the model tier merely to permit automatic semantic merging.

Ambiguous identity is a product-management decision, not a model-capability problem.

---

# Priority Semantics

Use the following priority model:

- **P0** — reserved; never assigned or changed by this skill
- **P1** — explicitly selected next / actively intended work
- **P2** — normal roadmap candidate; valid backlog work but not scheduled
- **P3** — future or exploratory capability
- **P4** — deferred or speculative architecture extension

Roadmap classification maps as follows:

```text
default / candidate / near-term → P2
future                          → P3
deferred                        → P4
```

## Priority ownership rule

Roadmap classification is authoritative for **P2 through P4**.

P1 is a special Beads-owned active-work override.

Therefore:

- near-term/default/candidate + P2 → consistent
- future + P3 → consistent
- deferred + P4 → consistent
- any roadmap classification + P1 → consistent

When an existing non-P1 roadmap epic changes classification:

```text
near-term → future    => P2 → P3
future → near-term    => P3 → P2
future → deferred     => P3 → P4
deferred → future     => P4 → P3
deferred → near-term  => P4 → P2
near-term → deferred  => P2 → P4
```

Apply these changes automatically when writes are requested.

## P1 override

Never automatically demote P1.

An epic at P1 is considered priority-consistent regardless of whether its roadmap item currently appears under:

- near-term,
- future,
- deferred,
- or another roadmap classification.

P1 means the human explicitly selected the work as active/next.

Roadmap classification may continue to describe the feature's broader product horizon while P1 temporarily represents active execution intent.

Only leave P1 when the human explicitly changes it or existing repository workflow does so.

After an epic is no longer P1, normal roadmap classification → P2/P3/P4 synchronization resumes.

Never infer P1 from:

- roadmap order,
- roadmap wording,
- section placement,
- perceived importance,
- dependency position,
- model judgment.

Never assign P0.

---

# Dependency Rules

Beads dependency edges created by this skill must use only the hard blocking relationship.

Do not create:

- parent-child relationships,
- `related` edges,
- `discovered-from` edges,
- speculative sequencing dependencies.

A blocking dependency may be created only when an authoritative source explicitly establishes that one capability cannot meaningfully exist without another.

Valid sources include:

- explicit roadmap text,
- architecture documentation,
- an approved specification,
- an existing durable dependency already present in Beads,
- explicit human instruction.

Do not infer blocking dependencies merely because:

- one feature would be cleaner to build first,
- two capabilities are closely related,
- one is architecturally foundational,
- one likely precedes another,
- one is a refinement of another,
- one would reduce implementation risk.

When a dependency seems plausible but is not explicitly supported:

- report it as a suggested relationship,
- do not write it.

---

# Modes

The skill supports four conceptual modes:

```text
dry-run
roadmap → Beads
Beads → roadmap refresh
reconcile
```

The human does not need to use those exact words.

Infer the intended mode from the request when clear.

When unclear, prefer `dry-run` / reconcile reporting over writes.

---

# Dry Run

Dry-run must perform all discovery, matching, comparison, and proposed-action reasoning without modifying:

- `docs/ROADMAP.md`,
- Beads,
- labels,
- priorities,
- dependencies,
- descriptions,
- or repository files.

A dry run should report:

- matched items,
- self-healing ID metadata that would be added,
- missing roadmap-summary blocks,
- roadmap-only items,
- Beads-only roadmap items,
- title drift,
- canonical summary drift,
- priority drift,
- status drift,
- ambiguous semantic candidates,
- proposed epic creations,
- proposed priority changes,
- proposed dependency changes,
- proposed roadmap refresh changes.

Dry-run output should be sufficient for a human to approve a later write run.

---

# Roadmap → Beads

When syncing roadmap capabilities into Beads:

1. Read the entire `docs/ROADMAP.md`.
2. Read all relevant open and closed roadmap-scoped Beads epics.
3. Match by stable metadata first.
4. Use exact normalized title matching only when no metadata exists.
5. Self-heal successful title matches by writing Beads ID metadata into the roadmap.
6. Report semantic candidates but do not merge them automatically.
7. Create Beads epics only for genuinely unmatched roadmap capabilities.
8. Label created epics `roadmap`.
9. Create epics only.
10. Do not create child tasks.
11. Store the roadmap's canonical concise capability summary in the exact `roadmap-summary` block format defined above.
12. Keep context, acceptance criteria, notes, and other richer detail outside the roadmap-summary block.
13. Preserve product/architecture constraints in notes or other Beads-owned fields.
14. Assign or reconcile priority using the priority rules in this skill.
15. Add blocking dependency edges only under the dependency rules above.
16. Report all changes.

New roadmap epics should receive:

```text
default / candidate / near-term → P2
future                          → P3
deferred                        → P4
```

Existing P2/P3/P4 epics should be updated when roadmap classification changes.

Existing P1 epics must not be automatically demoted.

---

# Beads → Roadmap Refresh

This is a **refresh**, not a regeneration.

The roadmap is not disposable generated output.

When refreshing from Beads:

1. Read the complete existing roadmap.
2. Read all roadmap-scoped open and closed Beads epics.
3. Match using Beads ID metadata.
4. Preserve roadmap-owned:
   - title,
   - canonical summary,
   - grouping,
   - ordering,
   - explanatory prose,
   - classification.
5. Use the Beads `roadmap-summary` block only to detect whether the Beads copy of the roadmap-owned summary is stale.
6. Do not overwrite roadmap summary text from a divergent Beads summary block; roadmap wins.
7. Pull only Beads-owned operational state into the roadmap where the roadmap format explicitly supports displaying it.
8. Add missing stable ID metadata where identity is certain.
9. Do not dump implementation details into the roadmap.
10. Do not render:
   - child tasks,
   - implementation plans,
   - detailed acceptance criteria,
   - internal retry state,
   - full dependency graphs,
   - verification logs.
11. Always show the proposed `ROADMAP.md` diff before writing human-facing roadmap changes.
12. Do not perform destructive restructuring without explicit human approval.

---

# Closed Epic Handling

Closing an epic does not automatically remove its roadmap capability.

Interpret closure using the Beads close reason where possible.

## Completed / shipped

If the epic clearly closed because the capability was completed:

- preserve the roadmap identity,
- move or render it according to the roadmap's existing shipped/completed convention,
- if no such convention exists, propose a `Shipped` section rather than inventing one silently.

## Won't do / abandoned / superseded

If the close reason clearly indicates the capability will not be pursued:

- do not silently delete it,
- report the state,
- propose an appropriate product-facing disposition such as:
  - Not planned,
  - Superseded,
  - Removed from roadmap.

Require human approval before moving or removing the item.

## Ambiguous close reason

If the close reason does not clearly distinguish completion from abandonment:

- preserve the roadmap entry,
- report the ambiguity,
- require human judgment.

---

# Reconcile

When asked to reconcile roadmap and Beads:

1. Read both complete sources.
2. Match roadmap items and roadmap-scoped epics.
3. Compare only fields under their defined ownership rules.
4. Produce these categories:

```text
Matched
Roadmap-only
Beads-only roadmap-scoped
Missing/stale bead metadata
Missing roadmap-summary block
Title drift
Canonical summary drift
Priority drift
Status drift
Dependency drift
Ambiguous semantic candidates
```

5. Apply only explicitly safe updates unless operating in dry-run mode.
6. Show the proposed roadmap diff before modifying `docs/ROADMAP.md`.
7. Stop for ambiguous identity, deletion, merge, product disposition, or conflicting human intent.

Do not report drift merely because the full Beads epic description differs from the roadmap summary.

Only the dedicated `roadmap-summary` block participates in summary drift.

Do not report P1 as priority drift against a P2/P3/P4 roadmap classification.

P1 is always considered consistent until explicitly removed.

---

# Safe Automatic Updates

The following updates are considered safe when writes are requested:

- add stable bead-ID metadata after a unique exact-title match,
- create an unmatched roadmap epic,
- add the `roadmap` label to an epic created by this skill,
- add a missing `roadmap-summary` block to a matched roadmap epic,
- update an epic title from the roadmap-owned title,
- refresh only the `roadmap-summary` block from roadmap-owned wording,
- assign P2/P3/P4 to a newly created roadmap epic according to explicit roadmap classification,
- reconcile an existing non-P1 epic between P2/P3/P4 when roadmap classification changes,
- reflect unambiguous Beads-owned operational state in existing roadmap status presentation,
- add an explicitly sourced hard blocking dependency.

These changes must still be reported.

---

# Changes Requiring Human Approval

Do not automatically:

- merge two capabilities,
- split one capability,
- delete roadmap items,
- delete Beads epics,
- remove the `roadmap` label from an existing epic,
- promote any epic to P1,
- demote an epic from P1,
- assign P0,
- infer a semantic identity match,
- convert an ambiguous close reason into shipped or abandoned state,
- reorder roadmap groups for aesthetic reasons,
- create speculative blocking dependencies,
- substantially restructure `ROADMAP.md`,
- change roadmap product intent based solely on Beads implementation state.

---

# Architecture and Specification Context

Supporting documentation may contribute:

- constraints,
- known contradictions,
- human decision gates,
- architectural boundaries,
- rationale,
- deferred design intent.

It may enrich an existing roadmap epic's notes.

Do not automatically create a new roadmap capability merely because a future-looking idea appears somewhere in architecture or specification documents unless:

- the roadmap already includes it,
- the epic is already roadmap-scoped,
- or the human explicitly asks to promote it.

---

# No Second Project-Management System

Beads remains the durable operational work graph.

Do not introduce parallel operational state into:

- `ROADMAP.md`,
- arbitrary JSON/YAML mapping files,
- generated state directories,

when the same fact already exists reliably in Beads.

Stable identity should be stored directly in `ROADMAP.md` using the `<!-- bd: ... -->` metadata described above.

The canonical roadmap summary should be duplicated into Beads only inside the explicitly delimited `roadmap-summary` block for deterministic reconciliation.

Do not maintain a separate roadmap-to-Beads mapping database.

---

# Idempotency

Idempotency is a hard requirement.

After a successful synchronization or reconciliation run, immediately performing the same operation again without intervening human or repository changes should:

- make no file changes,
- make no Beads changes,
- create no duplicate epics,
- create no duplicate dependency edges,
- rewrite no equivalent metadata,
- rewrite no unchanged roadmap-summary block,
- oscillate no priority between roadmap classification and Beads state,
- report zero actionable drift.

If a second identical run produces changes, treat that as a defect in the reconciliation logic.

---

# Verification

After any write operation:

1. Re-read `docs/ROADMAP.md`.
2. Re-read relevant roadmap-scoped Beads epics.
3. Confirm every synchronized roadmap capability has at most one stable Beads identity.
4. Confirm no duplicate roadmap epics were created.
5. Confirm newly created epics have the `roadmap` label.
6. Confirm every synchronized epic has exactly one valid `roadmap-summary` block.
7. Confirm each roadmap-summary block matches the corresponding roadmap canonical summary.
8. Confirm no Beads-owned description content outside the roadmap-summary block was overwritten.
9. Confirm roadmap classification and non-P1 priority agree:
   - near-term/default/candidate → P2
   - future → P3
   - deferred → P4
10. Confirm P1 epics were not reported as inconsistent solely because of roadmap classification.
11. Confirm P0 was not assigned.
12. Confirm P1 was not assigned or removed without explicit human instruction.
13. Confirm only explicitly supported hard blocking dependencies were created.
14. Confirm no child tasks were created by this skill.
15. Confirm no unrelated epics were pulled into roadmap scope.
16. Perform an idempotency check.

The idempotency check should find zero remaining changes attributable to the just-completed operation.

---

# Completion Report

Always report:

- roadmap items matched,
- Beads epics created,
- epics updated,
- stable ID metadata added,
- roadmap-summary blocks added or refreshed,
- priority changes,
- P1 overrides preserved,
- dependency changes,
- closed/shipped items encountered,
- unresolved roadmap-only items,
- unresolved Beads-only roadmap items,
- ambiguous semantic candidates,
- human decisions required,
- files changed,
- Beads state changed,
- idempotency result.

Do not report success if unresolved ambiguity was silently ignored.

---

# Safety

Never:

- silently merge capabilities,
- silently delete roadmap entries,
- silently delete epics,
- create child implementation tasks,
- infer P1,
- assign P0,
- automatically demote P1,
- treat every epic as roadmap work,
- create speculative dependency edges,
- use title text as permanent identity after stable metadata exists,
- compare the roadmap summary against the entire epic description,
- overwrite Beads-owned description content outside the roadmap-summary block,
- overwrite roadmap-owned wording from Beads,
- overwrite Beads-owned operational state from roadmap prose except for the explicit P2/P3/P4 classification reconciliation defined by this skill,
- maintain a second project-management database beside Beads,
- commit,
- push,
- or `bd dolt push`

unless the human explicitly requests the relevant repository operation.

When product intent is unclear, preserve both sides and report the conflict rather than guessing.