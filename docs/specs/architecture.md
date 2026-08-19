# Structured Vibe Architecture

## 1. Purpose

Structured Vibe is a local-first methodology and toolchain for structuring AI-assisted software development.

It is designed to spend human attention and model capability where they have the most leverage:

- humans provide product intent, architectural judgment, tradeoff decisions, and adjudication;
- high-capability models handle ambiguity, planning, review, and difficult reasoning;
- lower-cost models may execute well-specified work when appropriate;
- skills provide reusable procedural guidance;
- beads describe executable work and dependency state;
- verification checks that implementation actually satisfies intent.

Structured Vibe is **not** an agent harness. It does not replace OpenCode, Claude Code, Codex, or another host's model execution, tool execution, permissions, or session management.

The core architectural rule is:

> Structured Vibe resolves, validates, materializes, and advises. The host loads skills, runs models, and executes tools.

For v1, OpenCode is the only supported host integration.

---

## 2. Design Principles

### 2.1 Local first

Structured Vibe must remain fully usable without a hosted Structured Vibe service.

A future persistent service, if one is ever justified, is expected to behave like a local build daemon: one local process per user/machine, serving multiple repositories. It must not become a remote control plane or a dependency on a hosted account.

Think **Gradle, not Diablo 4**.

### 2.2 Native host integration

Structured Vibe should use host-native skill loading and extension mechanisms whenever practical.

Structured Vibe may resolve and materialize the skill set a host should see, but it should not replace the host's loader, model runtime, permissions, or tool system.

### 2.3 Transparent, inspectable files

Skills, packs, configuration, generated output, and the model registry should be ordinary files on disk where practical.

The released binary may contain integrity metadata, but the actual runtime content should remain inspectable.

### 2.4 Stateless where practical

Manifests, pack contents, repository contents, and the installed release are the source of truth.

Structured Vibe should recompute derived state whenever practical rather than growing a persistent database.

Persistent state is limited to data with a concrete purpose, such as generated output and the minimum sync fingerprint metadata needed to detect staleness.

Transient work uses the operating system's temporary directory.

### 2.5 Transactional mutation

Commands that publish a coherent generated state should either complete successfully or leave the previous state unchanged.

`svibe sync` is transactional.

The release process is also transactional: a release/tag exists only if the release pipeline succeeds.

### 2.6 Advisory capability tiers

Model capability tiers guide routing and warn about risk. They are not authorization.

Structured Vibe may recommend that a skill or bead use a stronger model. If the current model does not meet the recommendation, Structured Vibe warns the human and continues.

### 2.7 No hidden inheritance

Skills compose by selecting multiple skills, not by inheriting Markdown from one another.

When two scopes define the same skill ID, the higher-precedence skill replaces the lower-precedence skill completely.

### 2.8 Core skills are ordinary skills

Workflow skills such as `sv-plan` and `sv-review` are not a privileged skill class.

They participate in normal resolution exactly like `java-backend`, `testing`, or any other skill.

Core provides defaults. User and project scopes may replace them.

---

## 3. V1 System Overview

V1 consists of five major parts:

1. the `svibe` Go CLI;
2. the managed core pack;
3. user and project skill packs;
4. the managed model registry;
5. a thin, user-level OpenCode integration.

Conceptually:

```text
core pack
user packs
project pack
    |
    v
svibe
  validate
  resolve
  sync
  status
    |
    v
resolved/materialized skill snapshot
    |
    v
OpenCode integration
    |
    v
OpenCode native model/tool/session runtime
```

The CLI contains Structured Vibe logic. The OpenCode integration observes host runtime state and delegates Structured Vibe decisions back to `svibe` rather than duplicating resolver or model-registry logic in JavaScript.

---

## 4. CLI Responsibilities

The canonical executable is:

```text
svibe
```

V1 commands include:

```text
svibe init
svibe validate
svibe validate <pack-path>
svibe resolve
svibe sync
svibe status

svibe admin setup
svibe admin setup opencode
svibe admin update
svibe admin update opencode
```

### 4.1 Infrastructure, not workflow execution

The CLI provides infrastructure:

- discovery;
- validation;
- resolution;
- materialization;
- integrity checks;
- status;
- installation/administrative integration work.

The CLI does **not** expose commands such as `svibe plan`, `svibe review`, or `svibe execute`.

The development workflow runs in the host through skills such as:

- `sv-plan`;
- `sv-review`;
- `sv-beads`;
- `sv-finalize`;
- `sv-execute`;
- `sv-verify`.

### 4.2 Go implementation boundary

The CLI is implemented in Go for simple cross-platform distribution as self-contained binaries.

Core logic must not be coupled to CLI/stdout concerns. A conceptual internal structure is:

```text
cmd/svibe/
    |
    v
application/core logic
    resolver
    validation
    model registry
    integrity
    sync
    status
```

This keeps open the possibility of a future local daemon or other transport without rewriting resolution logic.

No SDK is required in v1. CLI JSON output is the initial machine integration contract.

---

## 5. User Configuration Root

Structured Vibe uses the operating system's native user configuration directory.

Conceptually:

```text
<user-config>/svibe/
├── core/
│   ├── structured-vibe.yaml
│   └── skills/
├── packs/
│   └── <user-pack>/
├── config/
│   └── models.yaml
└── generated/
    └── opencode/
```

Typical platform roots are expected to follow the OS convention rather than forcing a Unix-style path on every platform.

The entire config root may be overridden with:

```text
SVIBE_CONFIG_HOME
```

When set, that path replaces the normal svibe config root completely.

No additional config-path environment variables are required in v1.

---

## 6. Project Scope

A project is anchored to the Git repository root.

Structured Vibe determines the project root using the Git top-level directory, regardless of the current working directory inside that repository.

A project may contain exactly one project pack in v1:

```text
<git-root>/
└── .structured-vibe/
    ├── structured-vibe.yaml
    ├── skills/
    └── generated/
        └── opencode/
```

Nested `.structured-vibe/` directories are ignored.

Structured Vibe does not recursively search subprojects for project packs.

Outside a Git repository:

- project scope does not exist;
- `resolve` and `sync` still work with core and user scopes;
- `init` fails because there is no project root.

`svibe init` creates the minimal project structure and adds `.structured-vibe/generated/` to `.gitignore`. The initial pack name is derived from the repository directory name and starts at version `0.1.0`.

---

## 7. Pack Model

### 7.1 One format

A pack has exactly one supported manifest filename in v1:

```text
structured-vibe.yaml
```

YAML is the only manifest format.

### 7.2 Pack layout

A typical pack is:

```text
pack-root/
├── structured-vibe.yaml
└── skills/
    ├── java-backend/
    │   ├── SKILL.md
    │   ├── references/
    │   ├── templates/
    │   └── scripts/
    └── documentation/
        └── SKILL.md
```

Only immediate child directories under `skills/` that contain `SKILL.md` are skills.

Structured Vibe does not search the rest of a pack for `SKILL.md`.

Supporting files are allowed inside a skill directory.

### 7.3 Pack manifest

V1 pack metadata is intentionally small.

Example:

```yaml
name: mike-general
version: 0.1.0
description: Mike's general engineering skills
source: https://example.invalid/mike-general
```

Rules:

- `name` is required;
- `version` is required and follows SemVer;
- `description` is optional;
- `source` is optional and informational only in v1;
- pack names use lowercase kebab-case;
- loaded pack names must be unique within the active resolution environment.

`source` does not trigger installation, cloning, update checks, or network access in v1.

### 7.4 User pack discovery

User packs are discovered one level below:

```text
<user-config>/svibe/packs/
```

Every immediate child directory containing a valid `structured-vibe.yaml` is active.

V1 has no pack enable/disable flag.

Discovery is not recursive.

### 7.5 Project pack discovery

The Git-root `.structured-vibe/` directory is the project pack.

V1 does not support multiple project packs per repository.

### 7.6 Core is a pack

Core uses the same `structured-vibe.yaml` and `skills/*/SKILL.md` structure as other packs.

Its special behavior is limited to:

- being installed and managed as part of the svibe release;
- occupying the lowest-precedence scope;
- having its shipped membership defined by the release rather than arbitrary filesystem additions.

Core skills themselves remain ordinary skills for resolution purposes.

---

## 8. Skill Contract

### 8.1 Identity

Skill IDs use lowercase kebab-case.

The skill directory name is the skill ID and must exactly match `name` in `SKILL.md` frontmatter.

Example:

```text
skills/java-backend/SKILL.md
```

must declare:

```yaml
---
name: java-backend
description: Java backend implementation conventions and practices.
---
```

A mismatch is a validation error.

There are no aliases in v1.

### 8.2 Required and optional metadata

Every skill requires:

- `name`;
- `description`.

Optional metadata includes:

```yaml
minimum_driver_tier: B
```

`minimum_driver_tier` is optional and omitted when the skill has no declared capability recommendation.

V1 tiers are:

```text
A
B
C
```

`unknown` is not a tier. It is the state produced when the current model cannot be mapped by the model registry.

### 8.3 Skill containment

A skill is self-contained.

A skill may only rely on files bundled beneath its own skill directory.

It may not depend on:

- files in another skill directory;
- pack-level shared resources outside the skill directory;
- paths that escape the skill directory.

`svibe validate` should detect obvious path escapes where practical.

Supporting files may include scripts, references, examples, or templates.

`svibe` never executes arbitrary bundled skill scripts. Execution remains the host's responsibility and is governed by the host's normal permission/sandbox system.

V1 does not implement its own runtime sandbox.

---

## 9. Scope and Resolution

### 9.1 V1 precedence

V1 provides three scopes:

```text
core < user < project
```

Higher-precedence scopes replace lower-precedence definitions of the same skill ID.

Example:

```text
core:security-review
user:security-review
project:security-review
```

selects the project skill.

The replacement is complete. There is no inheritance or merging.

### 9.2 Same-scope ambiguity

Multiple packs may exist at user scope.

If more than one skill with the same ID exists in the same scope, resolution fails.

Structured Vibe never resolves same-scope ambiguity by:

- filesystem order;
- alphabetical order;
- last-write-wins behavior;
- host-specific behavior.

Example:

```text
user:mike-general/java-backend
user:mike-experiments/java-backend
```

is a hard resolution error.

### 9.3 Generic ordered-scope implementation

Although v1 exposes only `core`, `user`, and `project`, resolver internals should operate on an ordered set of scopes rather than hard-coding three special cases throughout the codebase.

This is not a dormant org feature. It is simply enough architectural looseness to allow another scope to be inserted later without changing the pack or skill contracts.

### 9.4 Provenance

Resolution tracks provenance.

Human-readable `svibe resolve` output should show:

- selected skill;
- selected scope;
- selected source pack;
- shadowed definitions where useful.

Machine-readable output includes equivalent provenance.

Replacing a core workflow skill is normal and does not generate a special warning.

---

## 10. Validation

`svibe validate` validates the active environment.

`svibe validate <pack-path>` validates one pack in isolation.

Validation distinguishes:

### Errors

Examples:

- missing required manifest;
- malformed YAML;
- invalid SemVer;
- invalid pack or skill ID;
- skill directory/frontmatter name mismatch;
- duplicate loaded pack names;
- duplicate skill IDs within the same scope;
- obvious illegal path escapes;
- structurally invalid `SKILL.md`.

Errors cause command failure.

### Warnings

Warnings identify advisory or unsupported conditions that do not make the environment structurally unusable.

Warnings do not become errors merely to make CI strict.

V1 has no generic strict mode requirement.

---

## 11. Model Registry and Capability Tiers

### 11.1 Registry ownership

Structured Vibe owns the canonical model-to-tier mapping.

The registry lives in the source repository at:

```text
config/models.yaml
```

and is installed as managed runtime data at:

```text
<user-config>/svibe/config/models.yaml
```

Users do not have a supported model-tier override mechanism.

If a user modifies the managed registry manually, Structured Vibe may still use the modified file after issuing the managed-file integrity warning described later.

### 11.2 Canonical identity and aliases

V1 may represent a model with one canonical identity and exact external aliases.

Conceptually:

```yaml
models:
  claude-opus-example:
    tier: A
    aliases:
      - provider-a/model-id
      - provider-b/other-model-id
```

V1 uses exact alias matching.

It does not infer identity from:

- substring similarity;
- regular expressions;
- guessed provider naming conventions.

If no alias matches, the model is `unknown`.

Host adapters contributing their own alias maps is deferred.

### 11.3 Runtime discovery

The OpenCode integration observes the actual resolved model used for the current request rather than assuming that the configured default model is the active model.

The plugin passes the reported model identity to `svibe`, which owns canonicalization and tier lookup.

The plugin should remain thin and should not duplicate the model registry.

### 11.4 Warning semantics

Tier warnings happen only when a skill is actually loaded into model context.

Merely having a skill in the resolved catalog or mentioning its name does not trigger a warning.

If a loaded skill declares `minimum_driver_tier`:

- current tier meets/exceeds recommendation: continue silently;
- current tier is lower: show a visible human warning and continue;
- current model is unknown: show a visible human warning and continue.

Warnings are:

- human-only;
- visible in the active OpenCode session;
- not injected into model context;
- non-blocking;
- emitted at most once per `(skill, model)` combination per OpenCode session.

Changing models during a session causes the next use of that skill to be evaluated against the new model.

The same general philosophy applies to bead-level model recommendations: they guide routing and warn; they do not revoke the user's ability to execute.

---

## 12. OpenCode Integration

### 12.1 V1 host

OpenCode is the only v1 host integration.

Claude Code, Codex, and other hosts are deferred.

### 12.2 User-level plugin

The OpenCode integration is installed at user scope, not once per repository.

It is generic infrastructure.

Project-specific behavior comes from the active Git repository and the generated Structured Vibe snapshot, not project-specific plugin code.

### 12.3 Administrative setup

Host installation is an administrative operation:

```text
svibe admin setup opencode
svibe admin update opencode
```

`svibe admin update` with no target updates all installed host integrations.

The installed `svibe` release owns the compatible OpenCode integration version. V1 does not maintain an independent integration compatibility matrix.

The integration source is maintained in the repository, while release artifacts contain built JavaScript. Contributors do not commit generated plugin `dist/` output.

### 12.4 Plugin/CLI boundary

The OpenCode plugin may:

- observe the current repo/session/model;
- observe actual skill loading where supported;
- surface human-visible Structured Vibe warnings;
- locate the generated Structured Vibe snapshot for the current context;
- invoke `svibe` for Structured Vibe decisions.

It should not reimplement:

- pack discovery;
- precedence;
- model-tier mapping;
- integrity policy;
- Structured Vibe business logic.

V1 uses CLI/subprocess integration.

Core logic must remain transport-independent enough that a future local service could replace repeated process invocation if runtime evidence justifies it.

---

## 13. Sync and Materialization

### 13.1 Explicit synchronization

Synchronization is explicit in v1:

```text
svibe sync
```

Structured Vibe does not automatically resync on every host invocation.

### 13.2 Repo-local generated output

Inside a Git repository, the resolved OpenCode snapshot is published beneath:

```text
<git-root>/.structured-vibe/generated/opencode/
```

Outside Git, the fallback is:

```text
<user-config>/svibe/generated/opencode/
```

Generated output is disposable build output.

Users should not edit it.

### 13.3 Materialized snapshot

`sync` copies complete winning skill directories into the generated host-facing working tree, including supporting files beneath each skill.

It does not symlink source skill directories.

The resulting tree is a snapshot of the resolved state at synchronization time.

### 13.4 Sync sequence

Conceptually:

```text
acquire lock
validate
resolve
build desired snapshot
verify OpenCode integration
stage filesystem publication
publish atomically
write minimal sync metadata
release lock
```

Validation and resolution must succeed before publication.

A missing or incompatible required OpenCode integration is a hard sync failure.

`sync` does not rewrite OpenCode's global configuration on every invocation. One-time user-level integration wiring is owned by `svibe admin setup opencode`.

### 13.5 Transactionality

`svibe sync` must not change the live generated state if synchronization fails.

Resolution and snapshot planning may happen in memory.

Filesystem publication uses temporary/staging storage and swaps the complete new snapshot into place only after all preconditions succeed.

If publication cannot complete, the previous generated snapshot remains intact.

### 13.6 Locking

The entire sync transaction is protected by an OS-managed advisory file lock.

Requirements:

- lock is held from the beginning of validation through successful publication or failure cleanup;
- lock ownership is process-scoped;
- process termination releases the OS lock;
- stale lock-file existence alone never implies ownership;
- lock metadata may be written for diagnostics only;
- if another sync owns the lock, the new sync fails immediately;
- svibe does not wait, retry, or break locks based on elapsed time;
- unsupported/broken filesystem locking is an error rather than a fallback to fragile PID/TTL semantics.

---

## 14. Sync Freshness and Status

`svibe status` reports whether generated output is current.

V1 uses content hashing, not filesystem timestamps.

The sync fingerprint includes both:

1. the contents of all files in winning/materialized skill directories;
2. the relevant resolution inputs that could change which skills win, including pack manifests, scope/order inputs, and the `svibe` version.

The CLI version is also the resolver/rules version. V1 does not maintain a separate rules-version number.

Generated output may include a minimal hidden sync-state file containing only what is necessary to compare the previous snapshot to current inputs, such as:

- svibe version;
- input fingerprint;
- synchronization timestamp.

It does not cache a redundant full resolution manifest. `svibe resolve --json` remains the source for current resolution details.

`status` may report:

- current;
- stale;
- invalid;
- missing integration;
- CLI-version drift relative to generated output;
- managed-file warnings.

Staleness is informational state, not necessarily command failure.

---

## 15. JSON and CLI Output Contract

Read-only and mutating commands may expose:

```text
--json
```

V1 uses a consistent top-level envelope:

```json
{
  "ok": true,
  "warnings": [],
  "errors": [],
  "result": {}
}
```

Rules:

- `stdout` is pure JSON when `--json` is used;
- human-readable warnings/errors go to `stderr`;
- warnings also appear as structured JSON data;
- `ok` represents command success, not a claim that no advisory state exists;
- automation should inspect JSON state rather than infer domain semantics from clever exit-code conventions;
- non-zero process exit remains appropriate for actual command failure.

Before 1.0, the JSON shape should be treated as intentionally stable but not contractually frozen.

---

## 16. Managed Installation Integrity

### 16.1 Managed files

The svibe release owns managed runtime payload such as:

- the shipped core pack;
- `config/models.yaml`;
- shipped host-integration payloads used by administrative setup.

The expected managed-file path/hash set is embedded in the `svibe` binary at build time.

It is not installed as a separate authoritative manifest.

The public source repository remains the place to inspect how that data is built.

### 16.2 Integrity check on every invocation

Every `svibe` invocation checks the expected managed runtime payload before executing the requested command.

This deliberately includes innocuous commands such as:

```text
svibe status
```

The rule is simple enough to apply universally and avoids commands accidentally forgetting integrity checks.

### 16.3 Modified managed file

If a shipped managed file exists but its content hash differs:

- emit a visible warning;
- continue using the modified on-disk content;
- do not restore it automatically;
- state that local changes are unsupported;
- state that a future svibe upgrade will overwrite managed files without warning.

Modification is allowed in the sense that svibe does not prevent it, but the resulting behavior is unsupported.

### 16.4 Missing managed file

If a required shipped managed file is missing:

- fail before running the requested command;
- do not auto-repair in v1;
- direct the user toward reinstalling/updating the release.

Deletion is treated differently from modification because the installed release is incomplete.

### 16.5 Extra files

Unexpected extra files beneath managed directories are ignored.

They do not generate warnings and are not automatically deleted.

### 16.6 Closed core membership

Core skill membership comes from the shipped release's embedded managed-file metadata.

Structured Vibe does not discover new core skills merely because someone drops another `SKILL.md` directory into the installed core tree.

For a shipped core skill:

- modified shipped file: warn, then use it;
- missing shipped file: hard integrity failure;
- extra neighboring files/directories: ignore;
- extra skill directories: do not discover as core skills.

Persistent customization belongs in user or project packs, where normal precedence applies.

### 16.7 Upgrade behavior

Managed files are replaced unconditionally by a svibe upgrade.

Upgrade does not:

- merge local modifications;
- preserve modified managed files;
- create automatic backups;
- ask whether locally modified managed files should be retained.

Core changes only when the CLI/release itself changes.

---

## 17. Installation and Distribution

### 17.1 Release artifact

A platform release archive contains a matched release unit:

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
        └── <built plugin artifact>
```

The CLI, core, registry, and OpenCode integration belong to one svibe release version.

### 17.2 Cross-platform binaries

Go release builds target the supported OS/architecture matrix, including common Intel/AMD64 and ARM64 platforms.

Users do not need Go installed to run svibe.

### 17.3 Installer

A v1 installer may support the familiar developer-tool flow:

```text
curl -fsSL <install-url> | bash
```

The script should remain simple:

- detect platform/architecture;
- download the matching release artifact;
- verify its published checksum;
- install the binary;
- install the matching managed payload;
- print PATH/setup instructions where necessary.

The installer is not the only supported installation path. Manual download/install remains possible.

Checksum mismatch is a hard failure with no override flag.

V1 does not require cryptographic release signing.

### 17.4 Updates

V1 does not automatically check for updates.

Rerunning the installer is sufficient to update the CLI and managed payload.

Self-update is deferred but the release packaging should not preclude it.

`svibe admin update` updates installed host integrations to the version bundled with the currently installed CLI. It does not independently upgrade the core/CLI release.

---

## 18. Repository Layout

The repository is expected to evolve approximately along these boundaries:

```text
structured-vibe/
├── AGENTS.md
├── LICENSE
├── README.md
├── config/
│   └── models.yaml
├── core/
│   ├── structured-vibe.yaml
│   └── skills/
├── cmd/
│   └── svibe/
├── internal/
│   └── ...
├── integrations/
│   └── opencode/
│       └── src/
├── docs/
│   └── specs/
│       ├── structured-vibe-spec.md
│       ├── architecture.md
│       └── releasing.md
└── ...
```

This is a boundary guide, not a requirement to create empty speculative directories.

Directories should be added when real implementation requires them.

---

## 19. AI Contributor Guidance

The repository is expected to be developed heavily with AI assistance.

Root `AGENTS.md` is therefore part of the contributor control surface.

It should remain short and point agents to authoritative documents rather than duplicating them.

At minimum:

- `docs/specs/structured-vibe-spec.md` is authoritative for product/workflow intent;
- `docs/specs/architecture.md` is authoritative for architectural constraints;
- `docs/specs/releasing.md` is authoritative for release mechanics.

Universal repository rules may also live in `AGENTS.md`, such as:

- keep changes scoped;
- run relevant tests/validation;
- do not hand-edit generated output;
- do not duplicate authoritative rules across multiple documents.

---

## 20. Explicit V1 Non-Goals

V1 intentionally does **not** implement:

- a hosted Structured Vibe service;
- a persistent local daemon;
- autonomous model execution;
- a custom agent harness;
- Claude Code/Codex host adapters;
- organization scope or organization enforcement;
- skill inheritance;
- hard skill dependencies;
- pack-to-pack dependency resolution;
- abstract capability matching;
- automatic pack installation/update from `source`;
- user model-tier overrides;
- automatic model identity guessing from fuzzy strings;
- adapter-contributed model aliases;
- automatic update checks;
- self-update;
- a Structured Vibe runtime sandbox;
- arbitrary execution of bundled skill scripts by `svibe`;
- a persistent database/cache;
- recursive project-pack discovery;
- recursive user-pack discovery;
- multiple project packs;
- pack enable/disable state;
- pack compatibility constraints such as `requires_svibe`.

These may be revisited only when real usage demonstrates the need.

---

## 21. Deferred / V2-Friendly Extensions

V1 should not implement these features, but its contracts should avoid unnecessarily preventing them.

### 21.1 Organization scope and enforcement

A future resolver may insert organization scope into the ordered scopes, for example:

```text
core < org < user < project
```

A future policy mechanism may allow organization-level enforcement that cannot be replaced by ordinary user/project precedence.

No v1 configuration field should pretend this feature already exists.

### 21.2 Abstract capabilities

V1 recommendations reference exact skill IDs.

A future capability system may allow a skill to request a capability such as `documentation` or `security-review` and resolve one of several providers.

This should only be built if exact skill references become a demonstrated limitation.

### 21.3 Local daemon

If repeated CLI subprocess calls become materially inefficient, a future local daemon may expose the same core logic through local IPC.

Expected characteristics:

- one daemon per user/machine;
- multiple repo contexts;
- local-only by default;
- daemon state is cache/coordination, not authoritative project data;
- deleting/restarting the daemon must not destroy important state.

### 21.4 Additional hosts

Future adapters may support Claude Code, Codex, or other agent harnesses.

Adapters should translate Structured Vibe's resolved output into host-native mechanisms rather than move host semantics into the core resolver.

### 21.5 Adapter-contributed model aliases

If maintaining every provider/host spelling in the central registry becomes painful, adapters may contribute exact external identifier mappings while the core registry continues to own canonical model identities and tiers.

### 21.6 Pack install/update

A future package/update mechanism may use informational `source` metadata and SemVer.

V1 deliberately does not turn packs into a package-manager ecosystem.

### 21.7 Self-update

A future `svibe` self-update flow may consume the same release artifacts/checksums as the installer.

Update checks remain explicit unless a later design deliberately changes that principle.

---

## 22. Core Architectural Summary

Structured Vibe v1 is intentionally small:

```text
ordinary skill packs
        |
core < user < project
        |
validate + resolve
        |
transactional materialization
        |
thin OpenCode integration
        |
host-native execution
```

Core rules:

- packs say where skills come from;
- scopes determine which same-ID skill wins;
- same-scope ambiguity is an error;
- skills do not inherit;
- recommendations are hints;
- model tiers advise but do not gate;
- generated output is disposable;
- managed release files are inspectable but unsupported when modified;
- missing managed files are fatal;
- synchronization and releases are transactional;
- important state lives in files, not hidden services;
- future capability should be earned by real usage.

Structured Vibe should remain a tool that helps humans and models work at the right altitude, not another runtime that tries to own the entire development environment.
