package cli

import (
	"fmt"
	"io"

	"github.com/mwenkdev/structured-vibe/internal/cliout"
	"github.com/mwenkdev/structured-vibe/internal/env"
	"github.com/mwenkdev/structured-vibe/internal/models"
	"github.com/mwenkdev/structured-vibe/internal/resolve"
)

// adviseResult is the capability comparison for one skill and model.
//
// This is the decision endpoint the OpenCode integration calls. The plugin
// observes which skill was loaded and which model is active; svibe owns
// canonicalization, tier lookup, and the warning text. The plugin must not
// reimplement any of it.
type adviseResult struct {
	Skill string `json:"skill"`
	// SkillKnown is false when the skill is not in the resolved set.
	SkillKnown bool `json:"skill_known"`
	// RequiredTier is empty when the skill declares no recommendation.
	RequiredTier string `json:"required_tier,omitempty"`

	Model string `json:"model"`
	// ModelID is the canonical identity, empty when the model is unknown.
	ModelID string `json:"model_id,omitempty"`
	// ModelTier is "unknown" when the registry cannot map the model.
	ModelTier string `json:"model_tier"`

	// Meets reports whether the current model satisfies the recommendation.
	Meets bool `json:"meets"`
	// Warn reports whether the host should show a warning to the human.
	Warn bool `json:"warn"`
	// Message is the human-facing warning text, empty when Warn is false.
	Message string `json:"message,omitempty"`
}

func (r *adviseResult) PrintHuman(w io.Writer) {
	fmt.Fprintf(w, "skill:     %s\n", r.Skill)
	if r.RequiredTier != "" {
		fmt.Fprintf(w, "requires:  tier %s\n", r.RequiredTier)
	} else {
		fmt.Fprintln(w, "requires:  (no declared minimum)")
	}
	fmt.Fprintf(w, "model:     %s\n", r.Model)
	if r.ModelID != "" {
		fmt.Fprintf(w, "canonical: %s\n", r.ModelID)
	}
	fmt.Fprintf(w, "tier:      %s\n", r.ModelTier)
	if r.Warn {
		fmt.Fprintf(w, "\nWARNING: %s\n", r.Message)
		return
	}
	fmt.Fprintln(w, "\nok")
}

func runAdvise(e *Env, args []string) error {
	fs := newFlagSet("advise", e.Stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON on stdout")
	skillName := fs.String("skill", "", "skill being loaded into model context")
	modelID := fs.String("model", "", "model identifier reported by the host")

	rest, err := parseMixed(fs, args)
	if err != nil {
		return &ExitError{Code: 2}
	}
	if len(rest) > 0 {
		fmt.Fprintln(e.Stderr, "svibe: advise takes no positional arguments")
		return &ExitError{Code: 2}
	}
	if *skillName == "" {
		fmt.Fprintln(e.Stderr, "svibe: advise requires --skill")
		return &ExitError{Code: 2}
	}

	out := cliout.New(e.Stdout, e.Stderr, *asJSON)
	d := e.baseDiags()

	environment, ed := env.LoadWithOptions(e.cwd(), env.Options{Manifest: e.manifest()})
	d.Extend(ed)
	if environment == nil || d.HasErrors() {
		out.Emit(false, d, nil)
		return Failure
	}

	resolution, rd := resolve.Resolve(resolve.Input{Packs: environment.Packs})
	d.Extend(rd)
	if resolution == nil {
		out.Emit(false, d, nil)
		return Failure
	}

	res := buildAdvice(resolution, environment.Registry, *skillName, *modelID)

	if !out.Emit(!d.HasErrors(), d, res) {
		return Failure
	}
	return nil
}

// buildAdvice computes the capability comparison.
//
// Tier warnings happen only when a skill is actually loaded into model
// context. Merely having a skill in the resolved catalog does not warrant a
// warning, so the caller decides when to ask (architecture 11.4).
func buildAdvice(r *resolve.Resolution, registry *models.Registry, skillName, modelID string) *adviseResult {
	res := &adviseResult{
		Skill:     skillName,
		Model:     modelID,
		ModelTier: models.TierUnknown.String(),
		Meets:     true,
	}

	var required models.Tier
	for _, s := range r.Skills {
		if s.Name != skillName {
			continue
		}
		res.SkillKnown = true
		if t, ok := models.ParseTier(s.MinimumDriverTier); ok {
			required = t
			res.RequiredTier = string(t)
		}
		break
	}

	// A skill with no declared recommendation never warns, whatever the model.
	if !required.Valid() {
		return res
	}

	current := models.TierUnknown
	if m, ok := registry.Lookup(modelID); ok {
		current = m.Tier
		res.ModelID = m.ID
	}
	res.ModelTier = current.String()
	res.Meets = current.Meets(required)

	switch {
	case !current.Valid():
		res.Warn = true
		res.Message = fmt.Sprintf(
			"skill %q recommends a tier %s model, but the current model %q is not in "+
				"the Structured Vibe model registry so its capability cannot be checked",
			skillName, required, modelID)
	case !res.Meets:
		res.Warn = true
		res.Message = fmt.Sprintf(
			"skill %q recommends a tier %s model, but the current model %q is tier %s",
			skillName, required, modelID, current)
	}

	return res
}
