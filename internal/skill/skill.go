// Package skill loads and validates individual skills.
//
// A skill is a directory containing SKILL.md. The directory name is the skill
// ID and must exactly match the name declared in frontmatter (architecture
// 8.1). A skill is self-contained: it may only rely on files bundled beneath
// its own directory (architecture 8.3).
package skill

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/mwenkdev/structured-vibe/internal/diag"
)

// FileName is the exact required filename. The host loader is case-sensitive.
const FileName = "SKILL.md"

// MaxNameLen and MaxDescriptionLen mirror the limits enforced by the
// OpenCode skill loader. Exceeding them means the skill would be rejected or
// silently dropped by the host, so svibe treats them as validation errors
// rather than letting a broken skill reach a synchronized snapshot.
const (
	MaxNameLen        = 64
	MaxDescriptionLen = 1024
)

// idPattern is the shared identifier form for skill IDs: lowercase
// kebab-case, no leading or trailing hyphen, no consecutive hyphens.
var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidID reports whether s is a well-formed lowercase kebab-case identifier.
func ValidID(s string) bool { return idPattern.MatchString(s) }

// ValidTiers are the v1 capability tiers. "unknown" is not a tier; it is the
// state produced when the model registry cannot map the current model
// (architecture 8.2).
var ValidTiers = []string{"A", "B", "C"}

// Frontmatter is the recognized SKILL.md metadata.
//
// name and description are required. minimum_driver_tier is optional and
// omitted when empty. The host ignores unknown frontmatter fields, so the
// Structured Vibe fields travel safely inside a snapshot the host consumes
// natively.
type Frontmatter struct {
	Name              string `yaml:"name"`
	Description       string `yaml:"description"`
	MinimumDriverTier string `yaml:"minimum_driver_tier,omitempty"`
}

// Skill is one loaded, validated skill.
type Skill struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	MinimumDriverTier string `json:"minimum_driver_tier,omitempty"`

	// Dir is the skill directory; Path is its SKILL.md.
	Dir  string `json:"-"`
	Path string `json:"path"`
}

// Load reads and validates the skill rooted at dir.
//
// It returns the skill when it is structurally usable. A nil skill with error
// diagnostics means the skill could not be loaded at all.
func Load(dir string) (*Skill, diag.Diagnostics) {
	var d diag.Diagnostics
	path := filepath.Join(dir, FileName)

	raw, err := os.ReadFile(path)
	if err != nil {
		d.Errorf("skill.unreadable", path, "cannot read %s: %v", FileName, err)
		return nil, d
	}

	fm, fmDiags := parseFrontmatter(raw, path)
	d.Extend(fmDiags)
	if fm == nil {
		return nil, d
	}

	wantName := filepath.Base(dir)

	switch {
	case fm.Name == "":
		d.Errorf("skill.name.missing", path, "frontmatter is missing required field \"name\"")
	case !ValidID(fm.Name):
		d.Errorf("skill.name.invalid", path,
			"skill name %q must be lowercase kebab-case matching %s", fm.Name, idPattern)
	case len(fm.Name) > MaxNameLen:
		d.Errorf("skill.name.too-long", path,
			"skill name %q is %d characters; the maximum is %d", fm.Name, len(fm.Name), MaxNameLen)
	case fm.Name != wantName:
		// A mismatch is a validation error (architecture 8.1). The host keys
		// skills by frontmatter name and does not enforce this, so svibe must.
		d.Errorf("skill.name.mismatch", path,
			"frontmatter name %q does not match skill directory name %q", fm.Name, wantName)
	}

	switch {
	case strings.TrimSpace(fm.Description) == "":
		// The host silently filters skills without a description; they are
		// never surfaced to the model. Fail loudly instead.
		d.Errorf("skill.description.missing", path,
			"frontmatter is missing required field \"description\"; the host silently drops skills without one")
	case len(fm.Description) > MaxDescriptionLen:
		d.Errorf("skill.description.too-long", path,
			"description is %d characters; the maximum is %d", len(fm.Description), MaxDescriptionLen)
	}

	if fm.MinimumDriverTier != "" && !validTier(fm.MinimumDriverTier) {
		d.Errorf("skill.tier.invalid", path,
			"minimum_driver_tier %q is not one of %s", fm.MinimumDriverTier, strings.Join(ValidTiers, ", "))
	}

	d.Extend(checkContainment(dir))

	if d.HasErrors() {
		return nil, d
	}

	return &Skill{
		Name:              fm.Name,
		Description:       fm.Description,
		MinimumDriverTier: fm.MinimumDriverTier,
		Dir:               dir,
		Path:              path,
	}, d
}

var frontmatterDelim = []byte("---")

// parseFrontmatter extracts the leading YAML frontmatter block.
func parseFrontmatter(raw []byte, path string) (*Frontmatter, diag.Diagnostics) {
	var d diag.Diagnostics

	// Tolerate a UTF-8 BOM and CRLF line endings.
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	normalized := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))

	if !bytes.HasPrefix(normalized, append(append([]byte{}, frontmatterDelim...), '\n')) {
		d.Errorf("skill.frontmatter.missing", path,
			"%s must begin with a YAML frontmatter block delimited by ---", FileName)
		return nil, d
	}

	rest := normalized[len(frontmatterDelim)+1:]
	end := bytes.Index(rest, []byte("\n---"))
	if end < 0 {
		d.Errorf("skill.frontmatter.unterminated", path,
			"frontmatter block is not terminated by a closing ---")
		return nil, d
	}

	var fm Frontmatter
	if err := yaml.Unmarshal(rest[:end], &fm); err != nil {
		d.Errorf("skill.frontmatter.invalid", path, "frontmatter is not valid YAML: %v", err)
		return nil, d
	}
	return &fm, d
}

// checkContainment enforces that a skill is self-contained.
//
// Two rules matter here:
//
//   - No nested SKILL.md. The OpenCode loader scans configured skill paths
//     recursively for **/SKILL.md, so a SKILL.md bundled beneath a skill's
//     supporting files would be registered as a separate phantom skill.
//   - No symlink escaping the skill directory. A skill may not depend on
//     files outside its own directory.
func checkContainment(dir string) diag.Diagnostics {
	var d diag.Diagnostics

	root, err := filepath.Abs(dir)
	if err != nil {
		d.Errorf("skill.path.unresolvable", dir, "cannot resolve skill directory: %v", err)
		return d
	}

	err = filepath.WalkDir(root, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			d.Errorf("skill.walk.failed", p, "cannot inspect skill contents: %v", err)
			return nil
		}
		if p == root {
			return nil
		}

		if entry.Type()&fs.ModeSymlink != 0 {
			if escapes(root, p) {
				d.Errorf("skill.path.escape", p,
					"symlink escapes the skill directory; a skill may only rely on files beneath itself")
			}
			// Do not follow the link; its target is validated by the check above.
			return nil
		}

		if !entry.IsDir() && entry.Name() == FileName && filepath.Dir(p) != root {
			rel, relErr := filepath.Rel(root, p)
			if relErr != nil {
				rel = p
			}
			d.Errorf("skill.nested-skill", p,
				"nested %s at %s would be discovered by the host as a separate skill; "+
					"supporting files must not be named %s", FileName, rel, FileName)
		}
		return nil
	})
	if err != nil {
		d.Errorf("skill.walk.failed", root, "cannot inspect skill contents: %v", err)
	}
	return d
}

// escapes reports whether the symlink at p resolves outside root.
func escapes(root, p string) bool {
	target, err := filepath.EvalSymlinks(p)
	if err != nil {
		// A broken link cannot be shown to escape; treat it as contained and
		// let the host deal with it.
		return false
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = root
	}
	rel, err := filepath.Rel(realRoot, target)
	if err != nil {
		return true
	}
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func validTier(t string) bool {
	for _, v := range ValidTiers {
		if v == t {
			return true
		}
	}
	return false
}

// IsSkillDir reports whether dir immediately contains a SKILL.md.
func IsSkillDir(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, FileName))
	return err == nil && !info.IsDir()
}
