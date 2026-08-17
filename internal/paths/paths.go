// Package paths resolves the Structured Vibe configuration root and the
// active project root.
//
// Structured Vibe uses the operating system's native user configuration
// directory rather than forcing a Unix-style path on every platform
// (architecture 5). A project is anchored to the Git repository root
// regardless of the current working directory inside it (architecture 6).
package paths

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConfigHomeEnv overrides the entire svibe config root when set. When
// present it replaces the normal root completely. No additional config-path
// environment variables exist in v1.
const ConfigHomeEnv = "SVIBE_CONFIG_HOME"

// ProjectDirName is the project pack directory at the repository root.
const ProjectDirName = ".structured-vibe"

// ErrNoProject indicates the working directory is not inside a Git
// repository, so project scope does not exist.
var ErrNoProject = errors.New("not inside a git repository: project scope does not exist")

// ConfigHome returns the svibe configuration root.
func ConfigHome() (string, error) {
	if v := os.Getenv(ConfigHomeEnv); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "svibe"), nil
}

// CoreDir is the managed core pack inside the config root.
func CoreDir(configHome string) string { return filepath.Join(configHome, "core") }

// UserPacksDir is the directory whose immediate children are user packs.
func UserPacksDir(configHome string) string { return filepath.Join(configHome, "packs") }

// ConfigDir holds managed configuration such as the model registry.
func ConfigDir(configHome string) string { return filepath.Join(configHome, "config") }

// FallbackGeneratedDir is where generated output is published when the
// working directory is not inside a Git repository.
func FallbackGeneratedDir(configHome string) string {
	return filepath.Join(configHome, "generated")
}

// ProjectRoot returns the Git top-level directory containing dir.
//
// It shells out to git so that worktrees, submodules and the various
// .git-file indirections behave exactly as git itself defines them, then
// falls back to walking up for a .git entry when git is unavailable.
func ProjectRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("git", "-C", abs, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err == nil {
		root := strings.TrimSpace(string(out))
		if root != "" {
			// Resolve symlinks so the result compares equal to paths derived
			// from the working directory.
			if resolved, rerr := filepath.EvalSymlinks(root); rerr == nil {
				return resolved, nil
			}
			return root, nil
		}
	}

	if root, ok := walkUpForGit(abs); ok {
		return root, nil
	}
	return "", ErrNoProject
}

func walkUpForGit(dir string) (string, bool) {
	cur := dir
	for {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			return cur, true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", false
		}
		cur = parent
	}
}

// ProjectPackDir is the single project pack for a repository. Nested
// .structured-vibe directories are ignored and v1 supports exactly one
// project pack per repository.
func ProjectPackDir(projectRoot string) string {
	return filepath.Join(projectRoot, ProjectDirName)
}

// GeneratedDir is the repo-local generated output root.
func GeneratedDir(projectRoot string) string {
	return filepath.Join(ProjectPackDir(projectRoot), "generated")
}

// OpenCodeSnapshotDir is where the resolved OpenCode snapshot is published.
func OpenCodeSnapshotDir(projectRoot string) string {
	return filepath.Join(GeneratedDir(projectRoot), "opencode")
}

// OpenCodeSkillsRelPath is the repo-relative path registered in the project
// opencode.json skills.paths array.
//
// It is deliberately relative so a committed opencode.json stays portable
// across machines. The host resolves relative skills.paths entries against
// its launch working directory, so the OpenCode integration detects and
// reports the case where opencode was started outside the repository root.
const OpenCodeSkillsRelPath = ProjectDirName + "/generated/opencode/skills"
