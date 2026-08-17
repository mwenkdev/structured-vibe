package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mwenkdev/structured-vibe/internal/cliout"
	"github.com/mwenkdev/structured-vibe/internal/diag"
	"github.com/mwenkdev/structured-vibe/internal/pack"
	"github.com/mwenkdev/structured-vibe/internal/paths"
	"github.com/mwenkdev/structured-vibe/internal/scope"
)

// initialVersion is the starting version for a new project pack.
const initialVersion = "0.1.0"

type initResult struct {
	ProjectRoot string   `json:"project_root"`
	PackDir     string   `json:"pack_dir"`
	PackName    string   `json:"pack_name"`
	Version     string   `json:"version"`
	Created     []string `json:"created"`
	AlreadyDone bool     `json:"already_initialized"`
}

func (r *initResult) PrintHuman(w io.Writer) {
	if r.AlreadyDone {
		fmt.Fprintf(w, "project pack already exists at %s\n", r.PackDir)
		return
	}
	fmt.Fprintf(w, "initialized project pack %q (%s)\n", r.PackName, r.Version)
	fmt.Fprintf(w, "  %s\n", r.PackDir)
	for _, c := range r.Created {
		fmt.Fprintf(w, "  created %s\n", c)
	}
	fmt.Fprintf(w, "\nAdd skills under %s\n", filepath.Join(r.PackDir, pack.SkillsDir))
}

func runInit(e *Env, args []string) error {
	fs := newFlagSet("init", e.Stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON on stdout")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return &ExitError{Code: 2}
	}
	if len(rest) > 0 {
		fmt.Fprintln(e.Stderr, "svibe: init takes no positional arguments")
		return &ExitError{Code: 2}
	}

	out := cliout.New(e.Stdout, e.Stderr, *asJSON)
	var d diag.Diagnostics

	root, err := paths.ProjectRoot(e.cwd())
	if err != nil {
		// init fails outside a Git repository because there is no project
		// root to anchor to (architecture 6).
		if errors.Is(err, paths.ErrNoProject) {
			d.Errorf("init.no-project", e.cwd(),
				"not inside a git repository; svibe init requires a project root")
		} else {
			d.Errorf("init.project-root", e.cwd(), "cannot determine project root: %v", err)
		}
		out.Emit(false, d, nil)
		return Failure
	}

	packDir := paths.ProjectPackDir(root)
	manifestPath := filepath.Join(packDir, pack.ManifestName)
	packName := pack.DeriveName(filepath.Base(root))

	res := &initResult{
		ProjectRoot: root,
		PackDir:     packDir,
		PackName:    packName,
		Version:     initialVersion,
	}

	if _, err := os.Stat(manifestPath); err == nil {
		res.AlreadyDone = true
		if !out.Emit(true, d, res) {
			return Failure
		}
		return nil
	}

	skillsDir := filepath.Join(packDir, pack.SkillsDir)
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		d.Errorf("init.mkdir", skillsDir, "cannot create project pack directory: %v", err)
		out.Emit(false, d, nil)
		return Failure
	}
	res.Created = append(res.Created, skillsDir)

	manifest := fmt.Sprintf(""+
		"name: %s\n"+
		"version: %s\n"+
		"description: Project skills for %s\n",
		packName, initialVersion, packName)

	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		d.Errorf("init.manifest", manifestPath, "cannot write manifest: %v", err)
		out.Emit(false, d, nil)
		return Failure
	}
	res.Created = append(res.Created, manifestPath)

	// Generated output is disposable build output and must not be committed.
	if changed, err := ensureGitignore(root); err != nil {
		d.Warnf("init.gitignore", filepath.Join(root, ".gitignore"),
			"cannot update .gitignore: %v", err)
	} else if changed {
		res.Created = append(res.Created, filepath.Join(root, ".gitignore"))
	}

	_ = scope.Project // project scope is what this pack will occupy

	if !out.Emit(true, d, res) {
		return Failure
	}
	return nil
}

// gitignoreEntry is the path excluded from version control.
const gitignoreEntry = paths.ProjectDirName + "/generated/"

// ensureGitignore adds the generated-output exclusion if it is absent.
func ensureGitignore(root string) (bool, error) {
	path := filepath.Join(root, ".gitignore")

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == gitignoreEntry {
			return false, nil
		}
	}

	var b strings.Builder
	if len(existing) > 0 {
		b.Write(existing)
		if !strings.HasSuffix(string(existing), "\n") {
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	b.WriteString("# Generated Structured Vibe host output (disposable)\n")
	b.WriteString(gitignoreEntry + "\n")

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return false, err
	}
	return true, nil
}
