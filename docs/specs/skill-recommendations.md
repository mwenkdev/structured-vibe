# Decision: Remove `recommends` from the Skill Contract

**Status:** Decided, pending review
**Produced by:** `sv-plan`, adjudicated by Mike

## Purpose

A skill may currently declare related skills:

```yaml
recommends:
  - testing
  - documentation
```

Structured Vibe parses this field, validates each ID, carries it through
resolution, and exposes it in `svibe resolve --json`.

Nothing ever shows it to a model or a human.

Architecture 8.3 says a recommendation "means those skills should be visible as
potentially useful collaborators if they are available." That requirement was
never implemented. Our own core skills declare recommendations, and those
declarations affect nothing.

This document records the decision to **remove the field** rather than finish
implementing it.

## What forced the decision

The OpenCode host advertises skills to the model as `<available_skills>`
containing **only** `name` and `description`. A skill's body reaches the model
only after the `skill` tool loads it. Established by reading the loader, not by
assumption.

So a recommendation could only surface in one of two places, each with a real
cost:

- **In the body**, visible only *after* the model has already chosen the skill
  — which is after the composition decision it was meant to inform.
- **In the description**, visible while choosing, but the description is the
  model's primary selection signal. It is repeated for every skill on every
  request. Padding it with skill names dilutes the trigger keywords that make
  selection accurate.

Implementing it would also have required sync to *render* skill content rather
than copy it, amending architecture 13.3 and making the snapshot no longer a
faithful copy of its source.

## Decision

Remove `recommends` from the skill contract.

The field costs parsing, validation, a resolution field, a JSON field, test
surface, and documentation. In exchange it delivers nothing today, and every
way of delivering something carried a cost out of proportion to the benefit.

A skill author who wants to point at a collaborating skill can simply write so
in the skill body, in prose, where it reads better than a machine-rendered
list and costs the toolchain nothing.

**Alternatives considered and rejected:**

- *Render into the body.* Real but weak benefit; requires sync to mutate
  content and amend architecture 13.3.
- *Render into the description.* Risks degrading skill selection for every
  skill on every request, which is a worse outcome than no feature.
- *Leave it parsed but unused.* Rejected. A field that validates input and
  produces no behavior is a promise the tool does not keep.

This is a deliberate reduction. Architecture 8.2 and 8.3 are amended to match,
rather than the code being left to contradict them.

## Scope

**In scope**

- Remove `recommends` from skill frontmatter parsing, validation, the resolved
  skill representation, and JSON output.
- Remove the declarations from the six core skills.
- Amend architecture 8.2 and 8.3.

**Out of scope**

- Any replacement mechanism, including abstract capabilities. Architecture 21.2
  already defers that until exact skill references become a demonstrated
  limitation. Removing the unused field does not change that position.

## Constraints

- A user pack that still declares `recommends:` must keep working. Unknown
  frontmatter keys are ignored, so such skills must continue to load and
  resolve without an error or a warning.
- No change to sync, fingerprinting, integrity, or the resolution rules.
- `svibe resolve --json` loses the `recommends` key. The JSON shape is
  intentionally stable but not contractually frozen before 1.0, and no consumer
  reads it.

## Milestones

### M-A — Remove the field

**Testable outcomes**

- No Go source outside tests references a skill `Recommends` field.
- A skill declaring `recommends:` loads and resolves with no error and no
  warning, proving user packs are unaffected.
- A YAML-valid `recommends:` value that validation previously rejected, such as
  a malformed skill ID, no longer fails, because the key is no longer
  interpreted. Syntactically invalid YAML still fails, as it must: the parser
  rejects it before any key can be ignored.
- `svibe resolve --json` contains no `recommends` key.
- The six core skills no longer declare the field, and the managed manifest is
  regenerated to match.

### M-B — Amend the architecture

**Testable outcomes**

- Architecture 8.2 no longer lists `recommends` as optional metadata.
- Architecture 8.3 no longer describes recommendation semantics.
- No document still describes a field the code does not implement.

## Assumptions

- No user pack depends on `recommends` producing behavior, because it never
  produced any.
- Prose in a skill body is a sufficient substitute for naming collaborators.

## Review record

Reviewed by `sv-review` running on a different model (`openai/gpt-5.4`).

Outcome: **no material findings**. One minor finding accepted.

- **R-1 (minor, accepted).** The acceptance criterion "an invalid value under
  `recommends:` no longer fails validation" was broader than the
  implementation can support: malformed YAML still fails before any key can be
  ignored. Narrowed to YAML-valid values that were previously rejected only
  because the key was interpreted.

The reviewer confirmed the decision is grounded in the actual host contract,
that the scope covers the full implementation surface, and that the
compatibility constraint matches the parser.
