package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sichengchen/eipgam/internal/edit"
)

// #304: PI_CODING_AGENT_DIR moves Pi's agent folder (config.js,
// getAgentDir, "~" expanded), and a model set on Pi's row goes to
// settings.json and models.json there; ~/.pi is left alone. A relative
// one, or this machine's variable for a WSL distro's Pi, is not taken.
func TestPiAgentDir(t *testing.T) {
	home := syncHome(t)
	custom := filepath.Join(home, "dotfiles", "pi")
	for _, c := range []struct{ env, want string }{
		{"", filepath.Join(home, ".pi", "agent")},
		{custom, custom},
		{"~/dotfiles/pi", custom},
		{"rel/pi", filepath.Join(home, ".pi", "agent")},
	} {
		t.Setenv("PI_CODING_AGENT_DIR", c.env)
		if a := pi(home); a.Dir != c.want || a.Path != filepath.Join(c.want, "settings.json") {
			t.Errorf("%q: %s", c.env, a.Dir)
		}
	}

	t.Setenv("PI_CODING_AGENT_DIR", custom)
	var a *Agent
	for _, x := range All() {
		if x.ID == "pi" {
			a = x
		}
	}
	if a == nil {
		t.Fatal("no pi in All()")
	}
	os.MkdirAll(custom, 0o755)
	os.WriteFile(filepath.Join(custom, "settings.json"), []byte(`{"theme": "dark", "defaultProvider": "xai", "defaultModel": "grok-4.6"}`), 0o644)
	if f := a.Field("model"); f.Get() != "xai/grok-4.6" {
		t.Fatalf("model %q", f.Get())
	}
	if err := a.Field("model").Set("eipgam/relay/glm-4.6"); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(custom, "settings.json")
	for k, want := range map[string]string{"defaultProvider": "eipgam", "defaultModel": "relay/glm-4.6", "theme": "dark"} {
		if v, _ := edit.GetJSON(settings, k); v != want {
			t.Fatalf("%s = %q:\n%s", k, v, readFile(settings))
		}
	}
	if v, _ := edit.GetJSON(filepath.Join(custom, "models.json"), "providers.eipgam.baseUrl"); v == "" {
		t.Fatalf("models.json:\n%s", readFile(filepath.Join(custom, "models.json")))
	}
	if c := a.Check(); c != "" {
		t.Fatal(c)
	}
	if _, err := os.Stat(filepath.Join(home, ".pi")); !os.IsNotExist(err) {
		t.Fatalf("~/.pi was touched: %v", err)
	}

	// a distro's Pi is at its home
	root := t.TempDir()
	d := distro{Name: "Ubuntu", Root: root, Home: "/home/me", Has: map[string]bool{"dir:.pi": true}, Mirrored: true, Running: true}
	if w := wslAgent(wslKindOf("pi"), d); w.Dir != filepath.Join(root, "home", "me", ".pi", "agent") {
		t.Fatalf("wsl: %s", w.Dir)
	}
}
