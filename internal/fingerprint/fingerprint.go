// Package fingerprint computes the sync freshness fingerprint.
//
// V1 uses content hashing, not filesystem timestamps (architecture 14).
// Timestamps lie: a checkout, a copy, or a clock skew changes them without
// changing content, and touching a file changes them without changing meaning.
//
// The fingerprint covers both:
//
//  1. the contents of all files in the winning materialized skill directories;
//  2. the resolution inputs that could change which skills win, including pack
//     manifests, scope and ordering inputs, and the svibe version.
//
// The CLI version is also the resolver/rules version, so a svibe upgrade
// invalidates the fingerprint even when no file changed. That is intentional:
// new resolution rules can select a different winner from identical inputs.
package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Input describes everything that can change the resolved snapshot.
type Input struct {
	// SvibeVersion is the CLI version, which is also the rules version.
	SvibeVersion string
	// Scopes are the active scope names in precedence order.
	Scopes []string
	// Packs are the loaded packs as "scope:name@version:root".
	Packs []string
	// Skills are the winning skills to be materialized.
	Skills []SkillInput
}

// SkillInput is one winning skill directory.
type SkillInput struct {
	Name string
	// Scope and Pack identify the winner, so a change of winner between two
	// byte-identical definitions still changes the fingerprint.
	Scope string
	Pack  string
	// Dir is the source directory whose full contents are hashed.
	Dir string
}

// Compute returns the hex-encoded fingerprint for the given inputs.
func Compute(in Input) (string, error) {
	h := sha256.New()

	// Field-length prefixes prevent ambiguity: without them, adjacent fields
	// could be re-split to produce the same byte stream from different inputs.
	write := func(label, value string) {
		fmt.Fprintf(h, "%s:%d:%s\n", label, len(value), value)
	}

	write("version", in.SvibeVersion)

	for _, s := range in.Scopes {
		write("scope", s)
	}

	packs := append([]string(nil), in.Packs...)
	sort.Strings(packs)
	for _, p := range packs {
		write("pack", p)
	}

	skills := append([]SkillInput(nil), in.Skills...)
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })

	for _, s := range skills {
		write("skill", s.Name)
		write("skill.scope", s.Scope)
		write("skill.pack", s.Pack)

		files, err := hashTree(s.Dir)
		if err != nil {
			return "", fmt.Errorf("hashing skill %q: %w", s.Name, err)
		}
		for _, f := range files {
			write("file", f.rel)
			write("hash", f.sum)
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

type fileHash struct {
	rel string
	sum string
}

// hashTree hashes every regular file beneath dir, sorted by relative path so
// the result does not depend on filesystem iteration order.
func hashTree(dir string) ([]fileHash, error) {
	var out []fileHash

	err := filepath.WalkDir(dir, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		// Only regular files contribute. A symlink's target is outside the
		// skill by definition of containment, and validation rejects escapes.
		if !entry.Type().IsRegular() {
			return nil
		}

		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}

		sum, err := hashFile(p)
		if err != nil {
			return err
		}
		out = append(out, fileHash{rel: filepath.ToSlash(rel), sum: sum})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
