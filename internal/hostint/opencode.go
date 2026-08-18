// Package hostint owns the OpenCode-specific parts of Structured Vibe:
// locating the host's config, registering the generated snapshot, and
// verifying the installed integration.
//
// Host semantics live here rather than in the resolver, so a future adapter
// translates resolved output into host-native mechanisms without moving host
// behavior into core logic (architecture 21.4).
package hostint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/paths"
)

// ProjectConfigNames are the project config filenames OpenCode accepts, in
// the order svibe prefers them.
var ProjectConfigNames = []string{"opencode.json", "opencode.jsonc"}

// PluginFileName is the built integration artifact installed at user scope.
const PluginFileName = "svibe.js"

// UserPluginDir is the OpenCode user-level plugin directory.
func UserPluginDir(home string) string {
	return filepath.Join(home, ".config", "opencode", "plugins")
}

// InstalledPluginPath is where the svibe integration is installed.
func InstalledPluginPath(home string) string {
	return filepath.Join(UserPluginDir(home), PluginFileName)
}

// UserConfigPath is the OpenCode global config.
func UserConfigPath(home string) string {
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

// ManagedPluginPath is the shipped integration inside the svibe config root,
// relative to that root. It is part of the managed runtime payload.
const ManagedPluginPath = "integrations/opencode/" + PluginFileName

// Install copies the shipped integration to the host's user-level plugin
// directory.
//
// The integration is installed at user scope rather than once per repository:
// it is generic infrastructure, and project-specific behavior comes from the
// active repository and the generated snapshot (architecture 12.2).
func Install(configHome, home string) (installedTo string, d diag.Diagnostics) {
	if home == "" {
		d.Errorf("hostint.no-home", "", "cannot determine the user home directory")
		return "", d
	}

	src := filepath.Join(configHome, filepath.FromSlash(ManagedPluginPath))
	raw, err := os.ReadFile(src)
	if err != nil {
		d.Errorf("hostint.payload-missing", src,
			"the svibe release does not contain an OpenCode integration to install: %v; "+
				"reinstall or update the release", err)
		return "", d
	}

	dir := UserPluginDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		d.Errorf("hostint.install.mkdir", dir, "cannot create the plugin directory: %v", err)
		return "", d
	}

	dst := filepath.Join(dir, PluginFileName)

	// Write through a temporary file so a partial write never leaves the host
	// loading a truncated plugin.
	tmp, err := os.CreateTemp(dir, "."+PluginFileName+".tmp-*")
	if err != nil {
		d.Errorf("hostint.install.temp", dir, "cannot stage the integration: %v", err)
		return "", d
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		d.Errorf("hostint.install.write", tmpName, "cannot write the integration: %v", err)
		return "", d
	}
	if err := tmp.Close(); err != nil {
		d.Errorf("hostint.install.write", tmpName, "cannot write the integration: %v", err)
		return "", d
	}
	// CreateTemp uses 0600. The host must be able to read the plugin under a
	// normal umask, so widen it to the mode an ordinary install would have.
	if err := os.Chmod(tmpName, 0o644); err != nil {
		d.Errorf("hostint.install.chmod", tmpName, "cannot set integration permissions: %v", err)
		return "", d
	}
	if err := os.Rename(tmpName, dst); err != nil {
		d.Errorf("hostint.install.publish", dst, "cannot install the integration: %v", err)
		return "", d
	}
	return dst, d
}

// InstalledMatchesRelease reports whether the installed integration is byte
// identical to the one shipped with the running svibe.
//
// The installed release owns the compatible integration version; v1 does not
// maintain an independent compatibility matrix (architecture 12.3).
func InstalledMatchesRelease(configHome, home string) (matches bool, installed bool) {
	if home == "" {
		return false, false
	}
	want, err := os.ReadFile(filepath.Join(configHome, filepath.FromSlash(ManagedPluginPath)))
	if err != nil {
		return false, false
	}
	got, err := os.ReadFile(InstalledPluginPath(home))
	if err != nil {
		return false, false
	}
	return bytes.Equal(want, got), true
}

// VerifyIntegration reports whether the OpenCode integration is installed.
//
// A missing or unusable integration is a hard sync failure (architecture
// 13.4): publishing a snapshot the host cannot report on would leave the
// human with skills loaded and no capability warnings.
func VerifyIntegration(home string) diag.Diagnostics {
	var d diag.Diagnostics

	if home == "" {
		d.Errorf("hostint.no-home", "",
			"cannot determine the user home directory, so the OpenCode integration "+
				"cannot be verified")
		return d
	}

	path := InstalledPluginPath(home)
	info, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		d.Errorf("hostint.missing", path,
			"the OpenCode integration is not installed; run \"svibe admin setup opencode\"")
	case err != nil:
		d.Errorf("hostint.unreadable", path, "cannot inspect the OpenCode integration: %v", err)
	case info.IsDir():
		d.Errorf("hostint.not-a-file", path,
			"expected the OpenCode integration to be a file, found a directory")
	case info.Size() == 0:
		d.Errorf("hostint.empty", path,
			"the installed OpenCode integration is empty; reinstall it with "+
				"\"svibe admin setup opencode\"")
	}
	return d
}

// FindProjectConfig returns the existing project config path, or the
// preferred path to create when none exists.
func FindProjectConfig(projectRoot string) (path string, exists bool) {
	for _, name := range ProjectConfigNames {
		p := filepath.Join(projectRoot, name)
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return filepath.Join(projectRoot, ProjectConfigNames[0]), false
}

// RegisterSnapshotPath ensures the project config lists the generated
// snapshot in skills.paths.
//
// The entry is repo-relative so a committed config stays portable across
// machines. The host resolves relative skills.paths entries against its launch
// working directory, which the integration detects and reports at runtime.
//
// Existing entries are preserved. The host REPLACES the skills.paths array at
// the higher-precedence scope rather than concatenating, so overwriting the
// project array would silently destroy entries the user placed there.
func RegisterSnapshotPath(projectRoot string) (changed bool, d diag.Diagnostics) {
	path, exists := FindProjectConfig(projectRoot)
	want := paths.OpenCodeSkillsRelPath

	cfg := map[string]any{}
	if exists {
		raw, err := os.ReadFile(path)
		if err != nil {
			d.Errorf("hostint.config.unreadable", path, "cannot read project config: %v", err)
			return false, d
		}
		if len(strings.TrimSpace(string(raw))) > 0 {
			if err := json.Unmarshal(stripJSONComments(raw), &cfg); err != nil {
				// Do not clobber a config svibe cannot parse.
				d.Errorf("hostint.config.invalid", path,
					"cannot parse project config as JSON: %v; add %q to skills.paths by hand",
					err, want)
				return false, d
			}
		}
	}

	skills, _ := cfg["skills"].(map[string]any)
	if skills == nil {
		skills = map[string]any{}
	}

	var list []any
	if existing, ok := skills["paths"].([]any); ok {
		list = existing
	}
	for _, item := range list {
		if s, ok := item.(string); ok && s == want {
			return false, d // already registered
		}
	}

	list = append(list, want)
	skills["paths"] = list
	cfg["skills"] = skills

	// Keep the schema reference so editors validate the file.
	if _, ok := cfg["$schema"]; !ok {
		cfg["$schema"] = "https://opencode.ai/config.json"
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		d.Errorf("hostint.config.encode", path, "cannot encode project config: %v", err)
		return false, d
	}
	out = append(out, '\n')

	if err := os.WriteFile(path, out, 0o644); err != nil {
		d.Errorf("hostint.config.write", path, "cannot write project config: %v", err)
		return false, d
	}
	return true, d
}

// HasSnapshotPath reports whether the config at path lists the generated
// snapshot in skills.paths.
func HasSnapshotPath(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	var cfg struct {
		Skills struct {
			Paths []string `json:"paths"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(stripJSONComments(raw), &cfg); err != nil {
		return false, err
	}

	for _, p := range cfg.Skills.Paths {
		if p == paths.OpenCodeSkillsRelPath {
			return true, nil
		}
	}
	return false, nil
}

// ConfiguredSkillPaths returns the skills.paths entries declared by the
// project config, resolved to absolute directories.
//
// The svibe snapshot entry is excluded: it is what svibe publishes, not a
// competing source.
func ConfiguredSkillPaths(projectRoot, home string) []string {
	path, exists := FindProjectConfig(projectRoot)
	if !exists {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var cfg struct {
		Skills struct {
			Paths []string `json:"paths"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(stripJSONComments(raw), &cfg); err != nil {
		return nil
	}

	var out []string
	for _, p := range cfg.Skills.Paths {
		if p == paths.OpenCodeSkillsRelPath {
			continue
		}
		expanded := p
		if strings.HasPrefix(p, "~/") && home != "" {
			expanded = filepath.Join(home, p[2:])
		}
		if !filepath.IsAbs(expanded) {
			// The host resolves relative entries against its launch working
			// directory, which is normally the repository root.
			expanded = filepath.Join(projectRoot, expanded)
		}
		out = append(out, expanded)
	}
	return out
}

// CheckGlobalSkillsPathsShadowed warns when a global skills.paths exists and
// would be discarded by the project one.
//
// The host deep-merges configs but REPLACES arrays at the higher-precedence
// scope. A user with global skill paths silently loses them once any project
// declares its own. svibe cannot change that, but it can stop the user losing
// skills with no explanation.
func CheckGlobalSkillsPathsShadowed(home, projectRoot string) diag.Diagnostics {
	var d diag.Diagnostics
	if home == "" || projectRoot == "" {
		return d
	}

	globalPath := UserConfigPath(home)
	raw, err := os.ReadFile(globalPath)
	if err != nil {
		return d
	}

	var cfg struct {
		Skills struct {
			Paths []string `json:"paths"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(stripJSONComments(raw), &cfg); err != nil {
		return d
	}
	if len(cfg.Skills.Paths) == 0 {
		return d
	}

	projectPath, exists := FindProjectConfig(projectRoot)
	if !exists {
		return d
	}

	d.Warnf("hostint.global-skills-shadowed", globalPath,
		"the global OpenCode config declares %d skills.paths entry/entries, but the "+
			"project config at %s declares its own; OpenCode replaces this array at the "+
			"higher-precedence scope rather than merging, so the global entries (%s) will "+
			"not be loaded in this project",
		len(cfg.Skills.Paths), projectPath, strings.Join(cfg.Skills.Paths, ", "))
	return d
}

// stripJSONComments removes // and /* */ comments so JSONC parses as JSON.
//
// It tracks string state so a URL such as "https://example" is not mangled.
func stripJSONComments(raw []byte) []byte {
	var out []byte
	inString, escaped := false, false

	for i := 0; i < len(raw); i++ {
		c := raw[i]

		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}

		if c == '/' && i+1 < len(raw) {
			if raw[i+1] == '/' {
				for i < len(raw) && raw[i] != '\n' {
					i++
				}
				if i < len(raw) {
					out = append(out, '\n')
				}
				continue
			}
			if raw[i+1] == '*' {
				i += 2
				for i+1 < len(raw) && (raw[i] != '*' || raw[i+1] != '/') {
					i++
				}
				i++
				continue
			}
		}

		out = append(out, c)
	}
	return out
}

// Describe renders the snapshot registration for human output.
func Describe(projectRoot string) string {
	return fmt.Sprintf("%s -> %s",
		filepath.Join(projectRoot, ProjectConfigNames[0]), paths.OpenCodeSkillsRelPath)
}
