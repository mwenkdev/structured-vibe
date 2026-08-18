// Package pack discovers and validates skill packs.
//
// A pack has exactly one supported manifest filename in v1,
// structured-vibe.yaml, and YAML is the only manifest format
// (architecture 7.1).
//
// Only immediate child directories under skills/ that contain SKILL.md are
// skills. Structured Vibe does not search the rest of a pack for SKILL.md
// (architecture 7.2). This is deliberately stricter than the host loader,
// which scans configured skill paths recursively.
package pack

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/scope"
	"github.com/mwenkdev/structured-vibe/internal/skill"
)

// ManifestName is the only supported manifest filename.
const ManifestName = "structured-vibe.yaml"

// SkillsDir is the only directory searched for skills within a pack.
const SkillsDir = "skills"

// semverPattern is the SemVer 2.0.0 grammar.
var semverPattern = regexp.MustCompile(
	`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
		`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
		`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// Manifest is the v1 pack metadata. It is intentionally small.
type Manifest struct {
	Name        string `yaml:"name" json:"name"`
	Version     string `yaml:"version" json:"version"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Source is optional and informational only in v1. It does not trigger
	// installation, cloning, update checks, or network access.
	Source string `yaml:"source,omitempty" json:"source,omitempty"`
}

// Pack is a loaded pack and its skills.
type Pack struct {
	Manifest Manifest      `json:"manifest"`
	Root     string        `json:"root"`
	Scope    scope.Scope   `json:"-"`
	Skills   []skill.Skill `json:"skills"`
}

// Options adjusts how a pack is loaded.
type Options struct {
	// AllowSkill, when non-nil, restricts which skill names are recognized.
	//
	// This implements closed core membership: core skills come from the
	// shipped release manifest, so an extra skill directory dropped into the
	// installed core tree is not discovered (architecture 16.6).
	AllowSkill func(name string) bool
}

// Load reads the pack rooted at root and validates it in isolation.
func Load(root string, sc scope.Scope) (*Pack, diag.Diagnostics) {
	return LoadWithOptions(root, sc, Options{})
}

// LoadWithOptions reads a pack with explicit loading options.
func LoadWithOptions(root string, sc scope.Scope, opts Options) (*Pack, diag.Diagnostics) {
	var d diag.Diagnostics
	manifestPath := filepath.Join(root, ManifestName)

	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			d.Errorf("pack.manifest.missing", manifestPath,
				"pack is missing its required %s manifest", ManifestName)
		} else {
			d.Errorf("pack.manifest.unreadable", manifestPath, "cannot read manifest: %v", err)
		}
		return nil, d
	}

	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		d.Errorf("pack.manifest.invalid", manifestPath, "manifest is not valid YAML: %v", err)
		return nil, d
	}

	switch {
	case m.Name == "":
		d.Errorf("pack.name.missing", manifestPath, "manifest is missing required field \"name\"")
	case !skill.ValidID(m.Name):
		d.Errorf("pack.name.invalid", manifestPath,
			"pack name %q must be lowercase kebab-case", m.Name)
	}

	switch {
	case m.Version == "":
		d.Errorf("pack.version.missing", manifestPath, "manifest is missing required field \"version\"")
	case !semverPattern.MatchString(m.Version):
		d.Errorf("pack.version.invalid", manifestPath,
			"pack version %q is not valid SemVer", m.Version)
	}

	p := &Pack{Manifest: m, Root: root, Scope: sc}

	skills, skillDiags := loadSkills(root, opts)
	d.Extend(skillDiags)
	p.Skills = skills

	if d.HasErrors() {
		return nil, d
	}
	return p, d
}

// loadSkills reads immediate child directories of skills/ that contain a
// SKILL.md. Anything else beneath skills/ is ignored rather than reported,
// so a pack may keep unrelated material there.
func loadSkills(root string, opts Options) ([]skill.Skill, diag.Diagnostics) {
	var d diag.Diagnostics
	skillsRoot := filepath.Join(root, SkillsDir)

	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			// A pack with no skills is structurally valid.
			return nil, d
		}
		d.Errorf("pack.skills.unreadable", skillsRoot, "cannot read skills directory: %v", err)
		return nil, d
	}

	var out []skill.Skill
	seen := map[string]string{}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(skillsRoot, e.Name())
		if !skill.IsSkillDir(dir) {
			continue
		}
		// Membership is closed for scopes that declare it. An unlisted
		// directory is silently not a skill, not an error: extra files
		// beneath managed directories are ignored.
		if opts.AllowSkill != nil && !opts.AllowSkill(e.Name()) {
			continue
		}

		s, sd := skill.Load(dir)
		d.Extend(sd)
		if s == nil {
			continue
		}

		// Two directories in one pack cannot claim the same ID. The directory
		// name is the ID and directory names are unique, so this can only
		// happen on a case-insensitive filesystem.
		if prev, dup := seen[s.Name]; dup {
			d.Errorf("pack.skill.duplicate", s.Path,
				"skill %q is defined twice in this pack (also at %s)", s.Name, prev)
			continue
		}
		seen[s.Name] = s.Path
		out = append(out, *s)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, d
}

// DiscoverUserPacks returns the packs found one level below dir.
//
// Every immediate child directory containing a valid manifest is active.
// Discovery is not recursive and there is no enable/disable flag
// (architecture 7.4).
func DiscoverUserPacks(dir string) ([]*Pack, diag.Diagnostics) {
	var d diag.Diagnostics

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, d
		}
		d.Errorf("packs.unreadable", dir, "cannot read user packs directory: %v", err)
		return nil, d
	}

	var out []*Pack
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		root := filepath.Join(dir, e.Name())
		if _, err := os.Stat(filepath.Join(root, ManifestName)); err != nil {
			// Not a pack. Ignore quietly; the directory may be anything.
			continue
		}
		p, pd := Load(root, scope.User)
		d.Extend(pd)
		if p != nil {
			out = append(out, p)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Manifest.Name < out[j].Manifest.Name })
	return out, d
}

// DeriveName converts a directory name into a usable kebab-case pack name.
func DeriveName(dirName string) string {
	s := strings.ToLower(dirName)
	var b strings.Builder
	lastHyphen := true // suppress a leading hyphen
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "project"
	}
	return out
}
