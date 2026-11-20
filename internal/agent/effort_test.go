package agent

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sichengchen/eipgam/internal/provider"
)

// effortHome is a sandbox HOME with a provider for eipgam to serve.
func effortHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	// Crush's data file, the sandbox's rather than the package's shared one
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	// where Windows keeps Goose's, Crush's and Devin's: the sandbox's, never the machine's
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Setenv("HERMES_HOME", "")
	t.Setenv("MIMOCODE_HOME", "")
	t.Setenv("DEVECO_CONFIG_DIR", "")
	t.Setenv("HANA_HOME", "")
	t.Setenv("DSH_HOME", "")
	if err := provider.Save(provider.Provider{ID: "deepseek", Name: "DeepSeek", Chat: "https://api.deepseek.com/v1", Key: "k", Models: []string{"pro", "flash"}}); err != nil {
		t.Fatal(err)
	}
	return home
}

// setEffort sets, reads back and resets an agent's effort, checking what
// else is in its file stays.
func setEffort(t *testing.T, a *Agent, v, keep string) {
	t.Helper()
	f := a.Field("effort")
	if f == nil {
		t.Fatalf("%s has no effort field", a.ID)
	}
	if err := f.Set(v); err != nil {
		t.Fatalf("%s: %v", a.ID, err)
	}
	if got := f.Get(); got != v {
		t.Fatalf("%s: effort %q, want %q\n%s", a.ID, got, v, readFile(a.Path))
	}
	found := false
	for _, o := range f.Options(map[string]string{}) {
		found = found || o.Value == v
	}
	if !found {
		t.Fatalf("%s: %q not among the options", a.ID, v)
	}
	if !strings.Contains(readFile(a.Path), keep) {
		t.Fatalf("%s lost %q:\n%s", a.ID, keep, readFile(a.Path))
	}
	if err := f.Set(""); err != nil {
		t.Fatal(err)
	}
	if got := f.Get(); got != "" {
		t.Fatalf("%s: reset left %q\n%s", a.ID, got, readFile(a.Path))
	}
	if !strings.Contains(readFile(a.Path), keep) {
		t.Fatalf("%s lost %q on reset:\n%s", a.ID, keep, readFile(a.Path))
	}
}

func TestEffortFields(t *testing.T) {
	home := effortHome(t)

	writeFile(t, filepath.Join(home, ".claude", "settings.json"), `{"theme":"dark","model":"opus"}`)
	c := claude(home)
	setEffort(t, c, "xhigh", `"theme"`)
	// max lasts a session in Claude Code's /effort; eipgam keeps it as its
	// env (TestClaudeEffortMax)
	setEffort(t, c, "max", `"theme"`)

	writeFile(t, filepath.Join(home, ".copilot", "settings.json"), `{"model":"gpt-5.5","contextTier":"long"}`)
	setEffort(t, copilot(home, filepath.Join(home, ".config")), "xhigh", `"contextTier"`)
}
