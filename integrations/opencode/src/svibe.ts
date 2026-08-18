/**
 * Structured Vibe integration for OpenCode.
 *
 * This plugin is deliberately thin. It observes host runtime state and asks
 * the `svibe` CLI for every Structured Vibe decision. It does not reimplement
 * pack discovery, scope precedence, model-tier mapping, or integrity policy
 * (architecture 12.4).
 *
 * What it does:
 *
 *   - tracks the model actually resolved for each request, rather than
 *     assuming the configured default is active;
 *   - detects when a skill is genuinely loaded into model context, and asks
 *     `svibe advise` whether the current model meets that skill's declared
 *     capability recommendation;
 *   - surfaces the answer as a TUI toast for the human only.
 *
 * Warnings are advisory. They never block execution, never throw into the
 * host, and are never injected into model context.
 */

import type { Plugin } from "@opencode-ai/plugin"

/** The built-in tool that loads a skill into context. */
const SKILL_TOOL = "skill"

/** Repo-relative location of the generated snapshot, owned by svibe. */
const SNAPSHOT_REL_PATH = ".structured-vibe/generated/opencode/skills"

const TOAST_TITLE = "Structured Vibe"

/**
 * Agents the host runs for its own bookkeeping.
 *
 * These fire `chat.params` inside the user's session using the configured
 * small_model, not the model doing the work. Observed in a single turn:
 *
 *   agent=title  model=anthropic/claude-haiku-4-5-20251001
 *   agent=build  model=anthropic/claude-opus-5
 *
 * Both carry the same sessionID, so tracking "the last model seen" would
 * attribute a skill load to the title model and warn about a capability
 * shortfall that does not exist.
 */
const INTERNAL_AGENTS = new Set(["title", "summary", "compaction"])

/** Shape of `svibe advise --json`. */
interface AdviseEnvelope {
  ok: boolean
  result?: {
    skill: string
    skill_known: boolean
    required_tier?: string
    model: string
    model_id?: string
    model_tier: string
    meets: boolean
    warn: boolean
    message?: string
  }
}

export const SvibePlugin: Plugin = async ({ client, directory, worktree, $ }) => {
  /**
   * Model resolved for each session's real work, keyed by session.
   *
   * Keyed rather than global so concurrent sessions do not overwrite one
   * another, and populated only from non-internal agents.
   */
  const modelBySession = new Map<string, string>()

  /**
   * Advice already handled, keyed by skill and model.
   *
   * A warning is emitted at most once per (skill, model) per session. Because
   * the model is part of the key, switching models re-evaluates the same skill
   * on its next use. Non-warning results are cached too, which avoids running
   * a subprocess on every skill load.
   */
  const seen = new Map<string, boolean>()

  /** Shows a human-visible toast. Never throws. */
  async function toast(message: string, variant: "warning" | "info"): Promise<void> {
    try {
      await client.tui.showToast({
        body: { title: TOAST_TITLE, message, variant },
      })
    } catch {
      // The TUI may not be attached. A missing warning must never break a session.
    }
  }

  /** Runs `svibe` and returns stdout, or null when it is unavailable. */
  async function svibe(args: string[], cwd: string): Promise<string | null> {
    try {
      const result = await $`svibe ${args}`.cwd(cwd).quiet().nothrow()
      if (result.exitCode !== 0) return null
      return result.stdout.toString()
    } catch {
      // svibe is not installed or not on PATH. Stay silent: this plugin is
      // advisory infrastructure, not a dependency of the host.
      return null
    }
  }

  /**
   * Warns when OpenCode was launched somewhere that makes the configured
   * snapshot path miss.
   *
   * The host resolves a relative `skills.paths` entry against its launch
   * working directory, with no walk up to the worktree. Launching from a
   * subdirectory therefore makes the snapshot silently invisible, and the
   * host records that only in its log. This turns it into something the
   * human can see.
   */
  async function checkSnapshotVisible(): Promise<void> {
    if (!worktree || !directory || directory === worktree) return

    const expected = join(worktree, SNAPSHOT_REL_PATH)
    const asHostResolves = join(directory, SNAPSHOT_REL_PATH)
    if (expected === asHostResolves) return

    // Only complain when a snapshot actually exists to be missed.
    if (!(await exists(expected))) return
    if (await exists(asHostResolves)) return

    await toast(
      `OpenCode was started in ${directory}, so the Structured Vibe skills at ` +
        `${SNAPSHOT_REL_PATH} were not loaded. Relative skills.paths entries resolve ` +
        `against the launch directory. Restart OpenCode from ${worktree}.`,
      "warning",
    )
  }

  async function exists(path: string): Promise<boolean> {
    try {
      const result = await $`test -e ${path}`.quiet().nothrow()
      return result.exitCode === 0
    } catch {
      return false
    }
  }

  function join(base: string, rel: string): string {
    return base.endsWith("/") ? `${base}${rel}` : `${base}/${rel}`
  }

  // Run the visibility check without delaying plugin initialization.
  void checkSnapshotVisible()

  return {
    /**
     * Records the model actually resolved for this request. The configured
     * default may not be what is running (architecture 11.3).
     */
    "chat.params": async (input) => {
      try {
        if (INTERNAL_AGENTS.has(input.agent)) return
        // The host spells a model as "<providerID>/<id>", which is the form
        // the svibe model registry matches exactly.
        modelBySession.set(input.sessionID, `${input.model.providerID}/${input.model.id}`)
      } catch {
        // Leave the previous value; an unknown model still produces a warning.
      }
    },

    /**
     * Evaluates capability only when a skill is genuinely loaded into model
     * context. Merely listing a skill in the catalog does not warrant a
     * warning (architecture 11.4).
     */
    "tool.execute.before": async (input, output) => {
      try {
        if (input.tool !== SKILL_TOOL) return

        const skill = output?.args?.name
        if (typeof skill !== "string" || skill.length === 0) return

        const model = modelBySession.get(input.sessionID) ?? ""
        const key = `${input.sessionID}\u0000${skill}\u0000${model}`
        if (seen.has(key)) return

        const stdout = await svibe(
          ["advise", "--skill", skill, "--model", model, "--json"],
          worktree || directory,
        )
        if (stdout === null) {
          // Do not cache: svibe may become available later in the session.
          return
        }

        let envelope: AdviseEnvelope
        try {
          envelope = JSON.parse(stdout) as AdviseEnvelope
        } catch {
          return
        }

        seen.set(key, true)

        const advice = envelope.result
        if (!advice?.warn || !advice.message) return

        await toast(advice.message, "warning")
      } catch {
        // Advisory only. Never fail a tool call because a warning failed.
      }
    },
  }
}

export default SvibePlugin
