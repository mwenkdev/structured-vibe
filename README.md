# Structured Vibe

A local-first methodology and toolchain for structuring AI-assisted software
development.

Structured Vibe combines durable work definitions, reusable skills, model
capability guidance, and independent verification so AI-assisted development
can be deliberate without becoming ceremony-heavy.

At a high level:

- **Beads say what.** A bead is a coherent unit of work with acceptance
  criteria, constraints, and a specification reference.
- **Skills say how.** A skill is reusable procedural knowledge that composes
  into execution.
- **Structured Vibe prepares and advises.** It resolves project state, derives
  execution requirements, and provides the host with durable workflow
  artifacts.
- **The host executes.** The host loads skills, runs models, executes tools,
  and owns session mechanics.
- **Verification checks the result.** Non-trivial changes are independently
  checked before work is considered complete.

Structured Vibe is **not an agent harness**. It complements one.

See [Product Principles](docs/PRINCIPLES.md) for the durable design philosophy
behind the workflow and [Roadmap](docs/ROADMAP.md) for planned capabilities.

---

## Status

Early development. The v1 host integration is OpenCode.

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/mwenkdev/structured-vibe/main/scripts/install.sh | sh
```

The installer verifies the published checksum of the release artifact. A
mismatch is a hard failure with no override.

Manual installation works too: download the archive for your platform from
[releases](https://github.com/mwenkdev/structured-vibe/releases), verify it
against `checksums.txt`, and place the binary on your `PATH` with the managed
payload in your OS configuration directory under `svibe/`.

To update, rerun the installer.

---

## Getting started

```bash
svibe admin setup opencode   # install the OpenCode integration (once per machine)

cd your-repo
svibe init                   # create the project pack and register it with OpenCode
svibe sync                   # publish the resolved skills to the host
```

Then restart OpenCode. It reads skills once at startup and does not reload
them.

The six core workflow skills become available to your agent:

| Skill | Purpose |
| --- | --- |
| `sv-plan` | Turn an idea and artifacts into a high-level specification |
| `sv-review` | Independently challenge a specification, with a bounded finding budget |
| `sv-beads` | Compile an approved specification into a `bd` dependency graph |
| `sv-finalize` | Prepare one ready bead against current `HEAD` and compute its capability floor |
| `sv-execute` | Implement a finalized bead using the selected skills |
| `sv-verify` | Independently check that the implementation satisfies the bead |

---

## Commands

`svibe` is infrastructure. It deliberately exposes no workflow commands such as
`plan` or `execute`; the workflow runs in the host through skills.

```text
svibe init                     create the project pack in the current repository
svibe validate [<pack-path>]   validate the active environment, or one pack
svibe resolve                  show the resolved skill set and its provenance
svibe sync                     publish the resolved snapshot for the host
svibe status                   report whether generated output is current
svibe advise --skill --model   compare a skill's capability recommendation to a model
svibe admin setup opencode     install the host integration
svibe admin update             update installed host integrations
```

Every command accepts `--json`, which emits a stable envelope on stdout:

```json
{ "ok": true, "warnings": [], "errors": [], "result": {} }
```

Human-readable diagnostics go to stderr, so `--json` output stays parseable.

---

## How skills resolve

Skills come from packs, which occupy ordered scopes:

```text
core < user < project
```

A higher scope **replaces** a lower definition of the same skill ID
completely. There is no inheritance and no merging. Two definitions of the
same skill ID in the same scope is a hard error: Structured Vibe never
resolves that by filesystem order, alphabetical order, or last-write-wins.

```text
<os-config-dir>/svibe/         core pack, user packs, model registry
<git-root>/.structured-vibe/   project pack and generated output
```

`svibe sync` copies the winning skill directories into a snapshot the host
loads natively. The snapshot is disposable build output; do not edit it.

---

## Capability tiers

Skills may declare a minimum driver tier:

```yaml
---
name: kubernetes-change
description: Use when changing production Kubernetes resources.
minimum_driver_tier: B
---
```

| Tier | Use for |
| --- | --- |
| A | Ambiguous product design, architecture, difficult tradeoffs, adversarial review |
| B | Decomposition, finalization, complex implementation, difficult debugging |
| C | Well-specified implementation, mechanical refactors, test scaffolding |

Tiers are **advisory**. When a loaded skill recommends more capability than the
active model provides, the OpenCode integration shows a warning and continues.
It never blocks execution and never injects the warning into model context.

A model the registry cannot map is `unknown`, which is a state rather than a
tier, and it also warns.

---

## Documentation

- [`docs/ROADMAP.md`](docs/ROADMAP.md) — planned product capabilities; roadmap items are
  candidates for Beads epics, not release commitments
- [`docs/PRINCIPLES.md`](docs/PRINCIPLES.md) — durable product and workflow
  principles
- [`docs/specs/structured-vibe-spec.md`](docs/specs/structured-vibe-spec.md) —
  product and workflow intent
- [`docs/specs/architecture.md`](docs/specs/architecture.md) — architectural
  constraints and invariants
- [`docs/specs/releasing.md`](docs/specs/releasing.md) — release mechanics

---

## Development

Requires Go and Node. Node builds the OpenCode integration, which is part of
the managed runtime payload.

```bash
make check          # gofmt, vet, lint, test
make plugin         # build the OpenCode integration
make install-dev    # build a complete local installation under .dev/
make dist VERSION=x # build release archives
```

Run the development binary against its local payload:

```bash
SVIBE_CONFIG_HOME="$PWD/.dev/svibe" ./bin/svibe status
```

**Editing `core/` does not affect your own session.** The core pack is managed
runtime payload owned by the installed release, so `svibe` resolves it from
your config root, not from this working tree. Changes to `core/skills/` ship in
the next release. To exercise them locally, run `make install-dev` and point
`SVIBE_CONFIG_HOME` at `.dev/svibe` as above.

Generated files must not be hand-edited. `make generate-check` fails if
generated output has drifted.

---

## License

[MIT](LICENSE)
