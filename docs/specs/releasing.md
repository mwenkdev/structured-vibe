# Releasing Structured Vibe

This document defines the release mechanics for Structured Vibe.

For architecture and product intent, see:

- `docs/specs/architecture.md`
- `docs/specs/structured-vibe-spec.md`

## Release Principle

Structured Vibe releases are transactional.

> A release/tag should exist only if the entire release pipeline completes successfully.

Release creation initiates the release transaction. If any release step fails, rollback deletes the GitHub release and its associated tag.

If rollback itself fails, the workflow reports the rollback failure. No recursive rollback-of-the-rollback mechanism is required.

The failed GitHub Actions run remains the troubleshooting record.

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

Any failure in any release step triggers rollback.

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

If any release-pipeline step fails, rollback removes:

1. the GitHub Release;
2. the associated release tag.

Conceptually:

```text
release created
      |
      v
release workflow
      |
      +-- success --> keep release + tag
      |
      +-- failure --> delete release + tag
```

Rollback is intentionally simple.

Do not:

- clean individual assets one by one before deleting the release;
- create alternate retry tags;
- move an existing release tag to a new commit;
- create special release branches solely to repair release transaction state;
- recursively attempt to repair a failed rollback.

After rollback, fix the underlying branch/code/configuration and create the same intended release again.

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

Because rollback removes the release and tag, retrying means:

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
