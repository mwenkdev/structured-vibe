// Package hostskills enumerates skills the OpenCode host discovers on its
// own, so Structured Vibe can warn about name collisions.
//
// This exists because of a concrete host behavior: when two discovered
// skills share a name, the loader logs a warning and then overwrites, with
// the load running at unbounded concurrency. The winner is therefore
// nondeterministic. Structured Vibe's core < user < project precedence can be
// silently defeated by an unrelated skill directory outside its control, so
// the collision must be surfaced to the human rather than resolved.
//
// This package only reports what it finds. It never edits host directories.
package hostskills

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mwenkdev/structured-vibe/internal/skill"
)

// Found is one skill discovered in a host-owned location.
type Found struct {
	Name string
	Path string
	// Origin describes the discovery location in human terms.
	Origin string
}

// ExtraDir is an additional configured location to scan, such as another
// entry in the host's skills.paths.
type ExtraDir struct {
	Dir    string
	Origin string
	// Recursive matches how the host scans configured skill paths, which use
	// a recursive **/SKILL.md glob rather than a fixed depth.
	Recursive bool
}

// Scan returns skills the host discovers outside the Structured Vibe
// snapshot, keyed by skill name.
//
// projectRoot may be empty when not inside a Git repository. home may be
// empty to skip global locations. excludeRoots lists directories whose
// contents should be ignored, normally the generated snapshot itself. extra
// lists additional configured locations, such as other skills.paths entries.
func Scan(projectRoot, home string, excludeRoots []string, extra ...ExtraDir) map[string][]Found {
	out := map[string][]Found{}

	type location struct {
		dir    string
		origin string
	}
	var locations []location

	if projectRoot != "" {
		locations = append(locations,
			location{filepath.Join(projectRoot, ".opencode", "skills"), "project .opencode/skills"},
			location{filepath.Join(projectRoot, ".opencode", "skill"), "project .opencode/skill"},
			location{filepath.Join(projectRoot, ".claude", "skills"), "project .claude/skills"},
			location{filepath.Join(projectRoot, ".agents", "skills"), "project .agents/skills"},
		)
	}
	if home != "" {
		locations = append(locations,
			location{filepath.Join(home, ".config", "opencode", "skills"), "global opencode skills"},
			location{filepath.Join(home, ".config", "opencode", "skill"), "global opencode skill"},
			location{filepath.Join(home, ".claude", "skills"), "global .claude/skills"},
			location{filepath.Join(home, ".agents", "skills"), "global .agents/skills"},
		)
	}

	for _, loc := range locations {
		if excluded(loc.dir, excludeRoots) {
			continue
		}
		for _, f := range scanDir(loc.dir, loc.origin, false) {
			out[f.Name] = append(out[f.Name], f)
		}
	}

	// Other configured skills.paths entries are just as capable of colliding
	// as the default locations, and the host resolves the collision by race.
	for _, ex := range extra {
		if ex.Dir == "" || excluded(ex.Dir, excludeRoots) {
			continue
		}
		for _, f := range scanDir(ex.Dir, ex.Origin, ex.Recursive) {
			out[f.Name] = append(out[f.Name], f)
		}
	}

	for name := range out {
		found := out[name]
		sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
		out[name] = found
	}
	return out
}

// scanDir finds skill directories beneath dir.
//
// The default host locations use a fixed-depth pattern, while configured
// skills.paths entries are scanned recursively for **/SKILL.md. recursive
// selects which shape to match so collision detection reflects what the host
// would actually discover.
func scanDir(dir, origin string, recursive bool) []Found {
	if !recursive {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		var out []Found
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			sub := filepath.Join(dir, e.Name())
			if !skill.IsSkillDir(sub) {
				continue
			}
			out = append(out, Found{
				Name:   e.Name(),
				Path:   filepath.Join(sub, skill.FileName),
				Origin: origin,
			})
		}
		return out
	}

	var out []Found
	_ = filepath.WalkDir(dir, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() || entry.Name() != skill.FileName {
			return nil
		}
		out = append(out, Found{
			Name:   filepath.Base(filepath.Dir(p)),
			Path:   p,
			Origin: origin,
		})
		return nil
	})
	return out
}

func excluded(dir string, roots []string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	for _, r := range roots {
		if r == "" {
			continue
		}
		rabs, err := filepath.Abs(r)
		if err != nil {
			continue
		}
		if abs == rabs {
			return true
		}
		// abs is excluded when it lies beneath rabs.
		rel, err := filepath.Rel(rabs, abs)
		if err != nil {
			continue
		}
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
