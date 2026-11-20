package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sichengchen/eipgam/internal/edit"
	"github.com/sichengchen/eipgam/internal/provider"
)

// wevnal is the #2055 reporter's provider, its models the ones Pi warned
// about once deselected, beside two that stay.
func wevnalSave(t *testing.T, models ...string) {
	t.Helper()
	if err := provider.Save(provider.Provider{ID: "wevnal", Name: "Wevnal", Chat: "https://wevnal.invalid/v1", Key: "k", Models: models}); err != nil {
		t.Fatal(err)
	}
}

var wevnalAll = []string{"glm-5.2", "kimi-k3", "mimo-v2.6-flash-free", "muse-spark-1.3-contributor-free", "nemotron-3.5-lightning-free", "space-bunny-free"}

func piList(t *testing.T, settings string) string {
	t.Helper()
	raw, ok := edit.GetJSON(settings, "enabledModels")
	if !ok {
		return "(none)"
	}
	return strings.Join(strings.Fields(raw), "")
}

// A list with nothing left is removed, not written empty, and comes back
// when its model does, with the model picked meanwhile. A list the user
// removed themselves stays removed; one on other providers alone, or a
// sync that can't read what eipgam gives, leaves the file byte for byte.
func TestPiEnabledModelsEmptiedAndBack(t *testing.T) {
	home := syncHome(t)
	wevnalSave(t, wevnalAll...)
	settings := filepath.Join(home, ".pi", "agent", "settings.json")
	os.MkdirAll(filepath.Dir(settings), 0o755)
	os.WriteFile(settings, []byte(`{"enabledModels": ["eipgam/wevnal/space-bunny-free"]}`+"\n"), 0o644)
	a := pi(home)
	if err := a.Apply("model", "eipgam/wevnal/space-bunny-free"); err != nil {
		t.Fatal(err)
	}
	wevnalSave(t, "glm-5.2", "kimi-k3")
	if err := a.Sync(); err != nil {
		t.Fatal(err)
	}
	if got := piList(t, settings); got != "(none)" {
		t.Fatalf("emptied: %s", got)
	}
	if err := a.Apply("model", "eipgam/wevnal/kimi-k3"); err != nil {
		t.Fatal(err)
	}
	wevnalSave(t, "glm-5.2", "kimi-k3", "space-bunny-free")
	if err := a.Sync(); err != nil {
		t.Fatal(err)
	}
	if got, want := piList(t, settings), `["eipgam/wevnal/space-bunny-free","eipgam/wevnal/kimi-k3"]`; got != want {
		t.Fatalf("back: %s, want %s", got, want)
	}

	// removed by the user, then the model goes and comes back: no list made
	if err := edit.DelJSON(settings, "enabledModels"); err != nil {
		t.Fatal(err)
	}
	wevnalSave(t, "glm-5.2", "kimi-k3")
	a.Sync()
	wevnalSave(t, wevnalAll...)
	a.Sync()
	if got := piList(t, settings); got != "(none)" {
		t.Fatalf("user's removal undone: %s", got)
	}

	const theirs = `{"enabledModels": ["xai/grok-4.6", "eipgam/*/*"], "defaultProvider": "eipgam", "defaultModel": "wevnal/kimi-k3"}` + "\n"
	os.WriteFile(settings, []byte(theirs), 0o644)
	wevnalSave(t, "kimi-k3")
	if err := a.Sync(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(settings); got != theirs {
		t.Fatalf("other providers' and globs rewritten: %s", got)
	}

	// models.json unreadable: unknown, so nothing goes
	const one = `{"enabledModels": ["eipgam/wevnal/kimi-k3"]}` + "\n"
	os.WriteFile(settings, []byte(one), 0o644)
	os.WriteFile(filepath.Join(home, ".pi", "agent", "models.json"), []byte(`{"providers": {"eipgam": {"models": "not a list"`), 0o644)
	a.Sync()
	if got := readFile(settings); got != one {
		t.Fatalf("unreadable models.json pruned: %s", got)
	}
}

// #2055 (misaka9981): models deselected in a provider's editor leave
// eipgam's provider in Pi's models.json, and the entries naming them in
// enabledModels go too, in Pi and the forks that keep its settings.json:
// Pi warned "No models match pattern" for each. The user's other
// entries, a glob on eipgam, entries on a model still there (one at a
// thinking level), their order and comments stay. The ones taken out are
// remembered: selected again, a model's entry is back, and a rename takes
// a remembered one with it.
func TestPiEnabledModelsLoseDeselected(t *testing.T) {
	for _, c := range []struct {
		id  string
		dir []string
		a   func(string) *Agent
	}{
		{"pi", []string{".pi", "agent"}, pi},
	} {
		t.Run(c.id, func(t *testing.T) {
			home := syncHome(t)
			wevnalSave(t, wevnalAll...)
			settings := filepath.Join(append([]string{home}, append(c.dir, "settings.json")...)...)
			os.MkdirAll(filepath.Dir(settings), 0o755)
			os.WriteFile(settings, []byte(`{
  // the user's own
  "enabledModels": [
    "xai/grok-4.6:high",
    "eipgam/wevnal/mimo-v2.6-flash-free",
    "eipgam/wevnal/kimi-k3:low",
    "eipgam/wevnal/muse-spark-1.3-contributor-free",
    "eipgam/wevnal/*-lightning-*",
    "eipgam/wevnal/nemotron-3.5-lightning-free",
    "eipgam/wevnal/space-bunny-free:high"
  ],
  "defaultProvider": "xai",
  "defaultModel": "grok-4.6"
}
`), 0o644)
			a := c.a(home)
			if err := a.Apply("model", "eipgam/wevnal/glm-5.2"); err != nil {
				t.Fatal(err)
			}
			mine := `["xai/grok-4.6:high","eipgam/wevnal/mimo-v2.6-flash-free","eipgam/wevnal/kimi-k3:low","eipgam/wevnal/muse-spark-1.3-contributor-free","eipgam/wevnal/*-lightning-*","eipgam/wevnal/nemotron-3.5-lightning-free","eipgam/wevnal/space-bunny-free:high","eipgam/wevnal/glm-5.2"]`
			if got := piList(t, settings); got != mine {
				t.Fatalf("after the pick: %s", got)
			}
			// a sync with every model still there changes nothing
			if err := a.Sync(); err != nil {
				t.Fatal(err)
			}
			if got := piList(t, settings); got != mine {
				t.Fatalf("sync, nothing gone: %s", got)
			}

			// the reporter's four deselected
			wevnalSave(t, "glm-5.2", "kimi-k3")
			if err := a.Sync(); err != nil {
				t.Fatal(err)
			}
			want := `["xai/grok-4.6:high","eipgam/wevnal/kimi-k3:low","eipgam/wevnal/*-lightning-*","eipgam/wevnal/glm-5.2"]`
			if got := piList(t, settings); got != want {
				t.Fatalf("enabledModels = %s, want %s", got, want)
			}
			if !strings.Contains(readFile(settings), "// the user's own") {
				t.Fatalf("comment lost: %s", readFile(settings))
			}
			if got := a.Field("model").Get(); got != "eipgam/wevnal/glm-5.2" {
				t.Fatalf("model = %q", got)
			}

			// one selected again comes back, at its level
			wevnalSave(t, "glm-5.2", "kimi-k3", "space-bunny-free")
			if err := a.Sync(); err != nil {
				t.Fatal(err)
			}
			want = `["xai/grok-4.6:high","eipgam/wevnal/kimi-k3:low","eipgam/wevnal/*-lightning-*","eipgam/wevnal/glm-5.2","eipgam/wevnal/space-bunny-free:high"]`
			if got := piList(t, settings); got != want {
				t.Fatalf("selected again: %s, want %s", got, want)
			}

			// a rename takes the remembered ones with it
			if _, err := RenameProvider("wevnal", "wv"); err != nil {
				t.Fatal(err)
			}
			if err := provider.Save(provider.Provider{ID: "wv", Name: "Wevnal", Chat: "https://wevnal.invalid/v1", Key: "k", Models: wevnalAll}); err != nil {
				t.Fatal(err)
			}
			if err := a.Sync(); err != nil {
				t.Fatal(err)
			}
			want = `["xai/grok-4.6:high","eipgam/wv/kimi-k3:low","eipgam/wv/*-lightning-*","eipgam/wv/glm-5.2","eipgam/wv/space-bunny-free:high","eipgam/wv/mimo-v2.6-flash-free","eipgam/wv/muse-spark-1.3-contributor-free","eipgam/wv/nemotron-3.5-lightning-free"]`
			if got := piList(t, settings); got != want {
				t.Fatalf("after the rename: %s, want %s", got, want)
			}
		})
	}
}
