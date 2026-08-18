package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mwenkdev/structured-vibe/internal/hostint"
	"github.com/mwenkdev/structured-vibe/internal/managed"
)

const pluginBody = "export default async () => ({})\n"

// newRelease writes a config root containing a shipped OpenCode integration.
func newRelease(t *testing.T, configHome string) managed.Manifest {
	t.Helper()
	dst := filepath.Join(configHome, filepath.FromSlash(hostint.ManagedPluginPath))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte(pluginBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return manifestFor(t, configHome, hostint.ManagedPluginPath)
}

func adminResultOf(t *testing.T, stdout string) adminResult {
	t.Helper()
	var env struct {
		Result adminResult `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout)
	}
	return env.Result
}

func TestAdminSetupInstallsPlugin(t *testing.T) {
	configHome := t.TempDir()
	m := newRelease(t, configHome)
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := runCLIWithManifest(t, configHome, t.TempDir(), m, "admin", "setup", "opencode", "--json")
	if got.err != nil {
		t.Fatalf("setup failed: %v\n%s", got.err, got.stderr)
	}

	res := adminResultOf(t, got.stdout)
	if len(res.Targets) != 1 || !res.Targets[0].Installed || !res.Targets[0].Changed {
		t.Fatalf("targets = %+v", res.Targets)
	}

	installed := hostint.InstalledPluginPath(home)
	raw, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("plugin not installed: %v", err)
	}
	if string(raw) != pluginBody {
		t.Errorf("installed content = %q", raw)
	}
}

// TestAdminSetupInstallsReadablePlugin: the host must be able to read it.
func TestAdminSetupInstallsReadablePlugin(t *testing.T) {
	configHome := t.TempDir()
	m := newRelease(t, configHome)
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got := runCLIWithManifest(t, configHome, t.TempDir(), m, "admin", "setup", "opencode"); got.err != nil {
		t.Fatalf("setup failed: %v", got.err)
	}

	info, err := os.Stat(hostint.InstalledPluginPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o044 == 0 {
		t.Errorf("plugin mode %v is not group/world readable", info.Mode().Perm())
	}
}

func TestAdminSetupIsIdempotent(t *testing.T) {
	configHome := t.TempDir()
	m := newRelease(t, configHome)
	home := t.TempDir()
	t.Setenv("HOME", home)

	runCLIWithManifest(t, configHome, t.TempDir(), m, "admin", "setup", "opencode")
	got := runCLIWithManifest(t, configHome, t.TempDir(), m, "admin", "setup", "opencode", "--json")
	if got.err != nil {
		t.Fatalf("second setup failed: %v", got.err)
	}

	res := adminResultOf(t, got.stdout)
	if res.Targets[0].Changed {
		t.Error("second setup should report no change")
	}
	if !res.Targets[0].Installed {
		t.Error("plugin should still be reported installed")
	}
}

// TestAdminSetupRequiresTarget: setup is deliberate, not a spray across hosts.
func TestAdminSetupRequiresTarget(t *testing.T) {
	configHome := t.TempDir()
	m := newRelease(t, configHome)

	got := runCLIWithManifest(t, configHome, t.TempDir(), m, "admin", "setup")
	if got.err == nil {
		t.Fatal("expected setup to require a target")
	}
	if !strings.Contains(got.stderr, "requires a target host") {
		t.Errorf("stderr = %q", got.stderr)
	}
}

func TestAdminRejectsUnknownHost(t *testing.T) {
	configHome := t.TempDir()
	m := newRelease(t, configHome)

	got := runCLIWithManifest(t, configHome, t.TempDir(), m, "admin", "setup", "emacs")
	if got.err == nil {
		t.Fatal("expected an unknown host to fail")
	}
	if !strings.Contains(got.stderr, "unknown host integration") {
		t.Errorf("stderr = %q", got.stderr)
	}
}

// TestAdminUpdateRefreshesStalePlugin: the installed release owns the
// compatible integration version.
func TestAdminUpdateRefreshesStalePlugin(t *testing.T) {
	configHome := t.TempDir()
	m := newRelease(t, configHome)
	home := t.TempDir()
	t.Setenv("HOME", home)

	runCLIWithManifest(t, configHome, t.TempDir(), m, "admin", "setup", "opencode")

	// Simulate an older integration left by a previous release.
	installed := hostint.InstalledPluginPath(home)
	if err := os.WriteFile(installed, []byte("export default async () => ({}) // old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runCLIWithManifest(t, configHome, t.TempDir(), m, "admin", "update", "--json")
	if got.err != nil {
		t.Fatalf("update failed: %v\n%s", got.err, got.stderr)
	}

	res := adminResultOf(t, got.stdout)
	if !res.Targets[0].Changed {
		t.Error("update should have refreshed the stale integration")
	}

	raw, _ := os.ReadFile(installed)
	if string(raw) != pluginBody {
		t.Errorf("integration not refreshed: %q", raw)
	}
}

// TestAdminUpdateSkipsUninstalledHost: update means "bring what I have in
// line with this release", not "adopt every host svibe knows about".
func TestAdminUpdateSkipsUninstalledHost(t *testing.T) {
	configHome := t.TempDir()
	m := newRelease(t, configHome)
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := runCLIWithManifest(t, configHome, t.TempDir(), m, "admin", "update", "--json")
	if got.err != nil {
		t.Fatalf("update failed: %v\n%s", got.err, got.stderr)
	}

	res := adminResultOf(t, got.stdout)
	if !res.Targets[0].Skipped {
		t.Errorf("uninstalled host should be skipped, got %+v", res.Targets[0])
	}
	if _, err := os.Stat(hostint.InstalledPluginPath(home)); err == nil {
		t.Error("update must not install a host that was never set up")
	}
}

// TestAdminUpdateExplicitTargetInstalls: naming a host is an explicit request.
func TestAdminUpdateExplicitTargetInstalls(t *testing.T) {
	configHome := t.TempDir()
	m := newRelease(t, configHome)
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := runCLIWithManifest(t, configHome, t.TempDir(), m, "admin", "update", "opencode", "--json")
	if got.err != nil {
		t.Fatalf("update failed: %v\n%s", got.err, got.stderr)
	}
	if _, err := os.Stat(hostint.InstalledPluginPath(home)); err != nil {
		t.Errorf("explicit target should install: %v", err)
	}
}

// TestAdminSetupFailsWithoutShippedPayload: an incomplete release cannot
// install an integration it does not have.
func TestAdminSetupFailsWithoutShippedPayload(t *testing.T) {
	configHome := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := runCLIWithManifest(t, configHome, t.TempDir(), managed.Manifest{},
		"admin", "setup", "opencode")
	if got.err == nil {
		t.Fatal("expected setup to fail without a shipped integration")
	}
	if !strings.Contains(got.stderr, "hostint.payload-missing") {
		t.Errorf("stderr = %q", got.stderr)
	}
}

func TestAdminRequiresSubcommand(t *testing.T) {
	got := runCLI(t, t.TempDir(), t.TempDir(), "admin")
	if got.err == nil {
		t.Fatal("expected admin to require a subcommand")
	}
	if !strings.Contains(got.stderr, "setup or update") {
		t.Errorf("stderr = %q", got.stderr)
	}
}

func TestAdminRejectsUnknownSubcommand(t *testing.T) {
	got := runCLI(t, t.TempDir(), t.TempDir(), "admin", "reinstall")
	if got.err == nil {
		t.Fatal("expected an unknown subcommand to fail")
	}
}
