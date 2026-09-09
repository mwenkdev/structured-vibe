---
name: sv-progress
description: Use when reporting execution progress for a bead subtree, checking verification coverage, or explaining a structural stop. Invokes svibe progress and presents its deterministic projection without re-deriving work state.
minimum_driver_tier: C
---

# sv-progress

Report execution progress from Structured Vibe's deterministic projection.

The CLI owns the projection. This skill invokes it and frames the result; it does not reconstruct progress from Beads or model reasoning.

## When to use

Use when someone asks for the progress of an epic or other bead subtree, its completed and remaining work, verification coverage, lifecycle anomalies, or the structural reason no work is available.

Do not use this skill for generated-snapshot freshness. That is reported by `svibe status`.

## Procedure

1. Run `svibe progress <bead-id>` from the project repository.
2. Present the emitted projection as the current result.
3. If the command fails, report its diagnostics. Do not fall back to reconstructing the result with direct `bd` calls.

Use `svibe progress <bead-id> --json` only when the host or another tool needs the stable machine-readable envelope.

## Boundaries

Do not independently compute subtree membership, buckets, completion ratios, verification coverage, anomalies, blockers, or stop reasons. Do not reinterpret verification as readiness or blockage. The command already owns those decisions and reads current Beads state.

The projection is structural status, not semantic diagnosis or advice about how to unblock work.

## Handoff

Hand the command's emitted projection back to the requester, preserving its distinction between execution progress and verification coverage.
