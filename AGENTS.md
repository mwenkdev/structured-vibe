# AGENTS.md

Structured Vibe is expected to be developed heavily with AI assistance.

Keep this file short. It is an entry point, not a duplicate specification.

## Read Before Changing Things

Before making changes, read the authoritative document for the area you are touching:

- **Product/workflow behavior:** `docs/spec.md`
- **Architecture and invariants:** `docs/architecture.md`
- **Release automation/distribution:** `docs/releasing.md`

If instructions conflict, stop and surface the conflict rather than silently choosing whichever file was read last.

## Universal Repository Rules

- Keep changes scoped to the requested work.
- Preserve existing architectural boundaries unless the task explicitly changes them.
- Do not invent speculative abstractions merely because they may be useful later.
- Prefer the simplest design that preserves an explicit future extension point.
- Run relevant tests and validation before declaring work complete.
- Do not hand-edit generated output.
- Do not duplicate authoritative rules across multiple documents; link to the authoritative source instead.
- Do not treat workflow skills such as `sv-plan` as privileged implementation classes. Core skills are ordinary skills.
- Do not turn `svibe` into an agent harness. Hosts own model execution, tools, permissions, and sessions.
- Keep Structured Vibe local-first. Do not add a hosted runtime dependency without an explicit architectural decision.

## Generated and Managed Files

Generated host-facing output is disposable and should be recreated through Structured Vibe tooling.

Managed files installed by a svibe release are inspectable but local modifications are unsupported and may be overwritten by an upgrade.

## Release Changes

Do not modify release automation without reading `docs/releasing.md`.

The release invariant is transactional: any release-pipeline failure rolls back the GitHub release and associated tag.

## When Unsure

Ask about consequential ambiguity.

Do not ask questions merely to avoid making a reasonable implementation decision that is already constrained by the spec and architecture.
