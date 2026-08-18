// Package managed enforces integrity of the managed runtime payload shipped
// with a svibe release.
//
// Policy (architecture 16):
//
//   - Every svibe invocation checks the payload before executing the
//     requested command, including innocuous ones such as status. The rule is
//     simple enough to apply universally and avoids commands accidentally
//     forgetting integrity checks.
//   - A modified managed file warns and is then used as-is. Modification is
//     allowed in the sense that svibe does not prevent it, but the resulting
//     behavior is unsupported and a future upgrade will overwrite it without
//     warning.
//   - A missing managed file is fatal before the command runs, because the
//     installed release is incomplete. Deletion is treated differently from
//     modification.
//   - Extra files beneath managed directories are ignored.
//   - Core skill membership comes from this manifest, so dropping another
//     SKILL.md directory into the installed core tree does not create a core
//     skill.
package managed

//go:generate go run ./gen

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mwenkdev/structured-vibe/internal/diag"
)

// ModelRegistryPath is the managed model registry, relative to the config root.
const ModelRegistryPath = "config/models.yaml"

// coreSkillPrefix is the managed path prefix for core skills.
const coreSkillPrefix = "core/skills/"

// Manifest is an expected managed payload: config-root-relative slash paths
// mapped to SHA-256 hex digests.
type Manifest map[string]string

// Embedded returns the manifest built into this binary at release time.
func Embedded() Manifest { return Manifest(expected) }

// Report is the outcome of an integrity check.
type Report struct {
	// Missing lists expected files that are absent. Any entry here is fatal.
	Missing []string `json:"missing,omitempty"`
	// Modified lists expected files whose content differs. These are used
	// anyway, unsupported.
	Modified []string `json:"modified,omitempty"`
	// Checked is how many expected files were examined.
	Checked int `json:"checked"`
}

// OK reports whether the installation is complete.
func (r Report) OK() bool { return len(r.Missing) == 0 }

// Check verifies the managed payload beneath configHome.
//
// The manifest is a parameter rather than a package global so the policy can
// be tested against a constructed installation.
func Check(configHome string, m Manifest) (Report, diag.Diagnostics) {
	var d diag.Diagnostics
	var rep Report

	paths := make([]string, 0, len(m))
	for p := range m {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, rel := range paths {
		rep.Checked++
		full := filepath.Join(configHome, filepath.FromSlash(rel))

		raw, err := os.ReadFile(full)
		if err != nil {
			if os.IsNotExist(err) {
				rep.Missing = append(rep.Missing, rel)
				continue
			}
			rep.Missing = append(rep.Missing, rel)
			d.Errorf("managed.unreadable", full, "cannot read managed file: %v", err)
			continue
		}

		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != m[rel] {
			rep.Modified = append(rep.Modified, rel)
			d.Warnf("managed.modified", full,
				"managed file has been modified locally; svibe will use the modified "+
					"content, but local changes to managed files are unsupported and a "+
					"future svibe upgrade will overwrite them without warning")
		}
	}

	if len(rep.Missing) > 0 {
		d.Errorf("managed.missing", configHome,
			"the installed svibe release is incomplete: %d managed file(s) are missing "+
				"(%s); reinstall or update the release rather than replacing them by hand",
			len(rep.Missing), summarize(rep.Missing))
	}

	return rep, d
}

// CoreSkills returns the closed set of core skill names for this release.
//
// Membership comes from the shipped manifest, not the filesystem, so an extra
// skill directory dropped into the installed core tree is not discovered.
// Persistent customization belongs in user or project packs, where normal
// precedence applies.
func CoreSkills(m Manifest) map[string]bool {
	out := map[string]bool{}
	for p := range m {
		if !strings.HasPrefix(p, coreSkillPrefix) {
			continue
		}
		rest := strings.TrimPrefix(p, coreSkillPrefix)
		// Only <name>/SKILL.md declares a core skill. Supporting files nested
		// deeper belong to a skill but do not create one.
		if path.Dir(rest) == "." {
			continue
		}
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[1] == "SKILL.md" {
			out[parts[0]] = true
		}
	}
	return out
}

func summarize(items []string) string {
	const limit = 3
	if len(items) <= limit {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:limit], ", ") + ", ..."
}
