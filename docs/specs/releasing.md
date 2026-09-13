# Releasing Structured Vibe

This document defines the release mechanics for Structured Vibe.

For architecture and product intent, see:

- `docs/specs/architecture.md`
- `docs/specs/structured-vibe-spec.md`

## Release Principle

Structured Vibe releases are transactional.

> A full release and its tag should exist only if the entire release pipeline completes successfully.

Release creation initiates the release transaction. For a full release, any non-success conclusion the workflow can observe — a step failure, manual cancellation of the run, or the job exceeding its timeout — causes rollback to delete the GitHub release and its associated tag. Cancellation and timeout coverage was validated empirically against real GitHub infrastructure; see [Observed rollback behavior](#observed-rollback-behavior).

**Prereleases are exempt** from this automatic rollback. See [Prerelease exemption](#prerelease-exemption). The transactional guarantee above applies to full releases only.

If rollback itself fails, the workflow reports the rollback failure. No recursive rollback-of-the-rollback mechanism is required.

The failed GitHub Actions run remains the troubleshooting record.

### Human post-release verification

Rollback runs inside the workflow, so it cannot fire when no workflow step runs ([Residual risks](#residual-risks)), and a retained prerelease looks like any other release on the releases page. The human who creates a release is therefore responsible for confirming the transaction settled. A release is not done when the workflow is dispatched; it is done when this checklist passes:

1. The release workflow run reached a conclusion — it is not still running and did not disappear without one.
2. On success, the release and tag exist and carry the expected asset set and checksums.
3. For a full release, on any non-success conclusion, the release and tag are gone. If they are not — for any of the residual-risk reasons — delete them by hand, fix the cause, and create the release again.
4. For a prerelease, confirm that retention was intended and that the asset set is what you expect, since rollback deletes nothing and a non-success pipeline may have left a partial set.

## Versioning

Structured Vibe uses SemVer.

Release names/tags use the plain SemVer value without a `v` prefix:

```text
0.1.0
1.1.0
2.0.0-beta.1
```

The release version is the single version for:

- the `svibe` CLI;
- the core pack;
- managed configuration such as the model registry;
- the bundled OpenCode integration;
- platform release archives.

## Release Initiation

A human intentionally creates a GitHub Release.

Typical CLI flow:

```bash
gh release create 1.1.0 \
  --target main \
  --generate-notes \
  --title "1.1.0"
```

`--target` may identify another intended release branch/ref when appropriate.

There is no tag-triggered release workflow and no required `workflow_dispatch` release step.

The release creation creates the release/tag and triggers the release workflow.

## Exact Commit

The release workflow operates against the commit referenced by the release tag.

Release verification, build, packaging, and artifacts must all correspond to that exact commit.

The workflow must not quietly rebuild current `main` if `main` has moved after release creation.

## Prerequisite Validation

Before packaging/distribution, the release workflow verifies that required prerequisite validation has succeeded for the exact release commit.

The exact implementation of prerequisite-check discovery may evolve, but the invariant is:

> A successful check for a different commit is not sufficient.

The release workflow should fail if the exact release commit does not meet the repository's required validation contract.

## Release Pipeline

The release workflow is expected to perform, in order:

1. identify the release version and exact tagged commit;
2. validate the version format;
3. verify prerequisite checks for that exact commit;
4. run release-specific tests/validation as needed;
5. build `svibe` for the supported OS/architecture matrix;
6. build the OpenCode integration from source;
7. package the matching core pack and managed configuration;
8. assemble platform archives;
9. generate checksums;
10. verify the expected artifact set;
11. upload release assets;
12. complete successfully.

Any non-success conclusion of the release job — step failure, manual cancellation, or job timeout — triggers rollback.

## Release Artifacts

A platform archive is expected to contain a matched release unit similar to:

```text
svibe_<version>_<os>_<arch>/
├── svibe
├── core/
│   ├── structured-vibe.yaml
│   └── skills/
├── config/
│   └── models.yaml
└── integrations/
    └── opencode/
        └── <built JavaScript plugin>
```

The OpenCode plugin source lives in the repository.

Generated `dist/` output is produced by the release workflow and is not committed to source control.

## Checksums

Every distributable release artifact receives a published checksum.

The installer must refuse an artifact whose checksum does not match.

V1 does not require cryptographic release signing.

## Rollback

For a full release, if the release pipeline reaches any non-success conclusion — a step fails, the run is cancelled by hand, or the job exceeds its timeout — rollback removes:

1. the GitHub Release;
2. the associated release tag.

Conceptually:

```text
release created
      |
      v
release workflow
      |
      +-- success --------------------------> keep release + tag
      |
      +-- failure / cancellation / timeout
              |
              +-- full release --> delete release + tag
              |
              +-- prerelease ----> retain, report exemption
```

Full-release rollback is idempotent and verifies its end state: it succeeds only when neither the release nor the tag survives, treats already-deleted objects as success, and fails loudly naming any surviving object. Rerunning it against an already-cleaned release is safe.

Rollback is intentionally simple.

Do not:

- clean individual assets one by one before deleting the release;
- create alternate retry tags;
- move an existing release tag to a new commit;
- create special release branches solely to repair release transaction state;
- recursively attempt to repair a failed rollback.

After rollback, fix the underlying branch/code/configuration and create the same intended release again.

### Prerelease exemption

A prerelease is exempt from automatic rollback. On a non-success conclusion the rollback step runs, detects the prerelease, and retains the release and its tag instead of deleting them, reporting the exemption in the workflow log (`docs/specs/release-rollback-coverage.md`, D4).

- **The GitHub `prerelease` flag is the authoritative signal**, not the SemVer prerelease suffix. When the two disagree, the flag decides and the rollback script logs a warning naming both values: a flagged `1.2.3` is retained; an unflagged `2.0.0-beta.1` is rolled back like any full release.
- **A prerelease may exist without a successful pipeline.** It still runs the full pipeline — exact-commit verification, prerequisite CI, asset upload on success — but a non-success conclusion leaves it published.
- **It may therefore carry an incomplete or missing asset set.** Retention is a complete no-op: no release deletion, no tag deletion, no per-asset cleanup. Whatever was uploaded before the non-success conclusion stays.
- **It is not marked or annotated in any way.** The workflow reports retention only in its own log and does not mutate release notes, titles, or labels. The Actions run is the record of whether the pipeline succeeded.

A consumer who installs a prerelease by explicit version is responsible for verifying it.

### Residual risks

The in-workflow rollback cannot fire when no workflow step runs, or when the runner disappears before cleanup completes (`docs/specs/release-rollback-coverage.md`, D6):

- the runner is lost or the infrastructure fails mid-run;
- the cancellation grace period expires before the rollback step completes;
- the run is cancelled while queued, before any step executes;
- the workflow never triggers, because Actions are disabled for the repository or `release.yml` is absent or invalid on the triggering ref.

In each case a full release and tag can survive without a successful pipeline. The remedy is manual: delete the release and tag by hand, then create the release again after fixing the cause. The [human post-release verification](#human-post-release-verification) checklist is the compensating control that detects these cases.

### Observed rollback behavior

The rollback guard is `failure() || cancelled()`. That `cancelled()` half rests on
a runtime assumption GitHub documents for manual cancellation but not for job
timeout, so it was validated empirically against real infrastructure
(`docs/specs/release-rollback-coverage.md`, D7). Both runs used a throwaway
prerelease, which the D4 exemption retains rather than deletes, so the evidence
sought was that the rollback step executed at all.

| Case | Run | Job conclusion | Guard that fired | Rollback step |
| --- | --- | --- | --- | --- |
| Manual cancellation before checkout | 34726280788 | `cancelled` | `cancelled()` | executed, success |
| Job timeout exceeded | 34726338633 | `cancelled` | `cancelled()` | executed, success |

Both observed 2026-09-12 against implementation commit `7fa87af`.

What each run establishes:

- **Manual cancellation** was issued roughly seven seconds after the release was
  created, while the job was between starting and finishing checkout. `Validate
  version format` and `Check out the exact tagged commit` both report `skipped`,
  yet the rollback step still ran and reported the exemption. That is the
  checkout-independent script bootstrap working: with no workspace, the step
  fetched the script from the contents API and executed it.
- **Job timeout** is the load-bearing observation. The job started 23:47:48Z and
  completed 00:18:03Z, hitting the 30-minute `timeout-minutes` bound. The
  long-running step reports `cancelled` with `The operation was canceled`, the
  job conclusion is `cancelled` rather than `failure`, and the rollback step
  still ran. **A job-level timeout therefore does reach a `cancelled()`-guarded
  step**, so `failure()` alone would have missed it and the widened guard is
  sufficient on its own.

Because the timeout case reached rollback, D7's fallback — step-level
`timeout-minutes` on the long-running steps to convert a timeout into an
ordinary step failure — was **not** required and was not applied.

Re-validate these observations if the guard, either timeout, or the script
bootstrap changes. `TestReleaseWorkflow*` in `internal/release` fails CI if the
workflow regresses, but a static check cannot detect a guard that never fires.

## Rerunning

A failed release transaction is not resumed against a retained tag.

Because rollback removes a full release and its tag, retrying means:

1. fix the underlying issue;
2. ensure prerequisite validation succeeds;
3. create the release again.

GitHub Actions history from the failed transaction remains available for troubleshooting.

## Immutability

Do not enable a release mode that prevents rollback of a newly created release/tag while this transactional release design is in use.

If GitHub release immutability semantics change or the project later adopts immutable releases, the release architecture must be revisited deliberately.

## Installer Relationship

V1 installation may use:

```bash
curl -fsSL <install-url> | bash
```

The installer:

1. detects OS/architecture;
2. selects the matching release artifact;
3. downloads it;
4. verifies the published checksum;
5. installs the CLI and matching managed payload;
6. prints any required PATH/next-step information.

Manual installation remains supported.

V1 updates are performed by rerunning the installer.

Automatic update checks and self-update are intentionally deferred.
