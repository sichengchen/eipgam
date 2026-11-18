package agent

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// A catalog sync runs a ListOnly agent's Sync only while it reads as
// connected: every one of them connected by magpie reads so, or its list
// would never follow the catalog again.
func TestListOnlyAgentsReadConnected(t *testing.T) {
	codexHome(t, "", "")
	n := 0
	for _, a := range All() {
		if !a.ListOnly || a.WSL != "" {
			continue
		}
		n++
		if err := a.Connect(); err != nil {
			t.Errorf("%s: %v", a.ID, err)
			continue
		}
		if !a.Wired() {
			t.Errorf("%s connected but not Wired: a catalog sync would skip its list", a.ID)
		}
	}
	// 5 with the agents magpie keeps
	if n < 4 {
		t.Fatalf("only %d ListOnly agents", n)
	}
}

// yyddbds on Discord: in an OpenCode-based app they added a custom provider
// pointing at magpie and picked a few of its models by hand. It wasn't connected
// on magpie's Agents page, yet after an update the provider had every one
// of magpie's models: the start's catalog sync wrote magpie's whole list
// over any block named magpie. Magpie's default never goes over a list the
// user wrote; one magpie wrote still follows the catalog.
func TestCatalogSyncLeavesAHandWrittenMagpieBlock(t *testing.T) {
	home := syncHome(t)
	if err := provider.Save(provider.Provider{ID: "added", Name: "Added", Key: "k", Chat: "http://127.0.0.1:1/v1", Models: []string{"m2"}}); err != nil {
		t.Fatal(err)
	}
	cfgDir := filepath.Join(home, ".config")
	oc := filepath.Join(cfgDir, "opencode", "opencode.json")
	// as the app writes a custom provider: magpie's address, a key, the
	// models the user picked; the model they run is their own
	own := `{
  "model": "anthropic/claude-x",
  "provider": {
    "magpie": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "magpie",
      "options": {"baseURL": "http://127.0.0.1:3425/v1", "apiKey": "sk-mine"},
      "models": {"relay/glm-4.6": {"name": "GLM 4.6"}}
    }
  }
}
`
	writeFile(t, oc, own)
	// a sibling on the same sync: Pi with a magpie provider written by
	// hand in models.json, its default its own
	piModels := filepath.Join(home, ".pi", "agent", "models.json")
	piOwn := `{"providers":{"magpie":{"baseUrl":"http://127.0.0.1:3425/v1","api":"openai-completions","models":[{"id":"relay/glm-4.6"}]}}}`
	writeFile(t, piModels, piOwn)
	writeFile(t, filepath.Join(home, ".pi", "agent", "settings.json"), `{"defaultProvider":"anthropic","defaultModel":"claude-x"}`)

	SyncCatalog()
	if got := readFile(oc); got != own {
		t.Errorf("OpenCode's own magpie provider written over:\n%s", got)
	}
	if got := readFile(piModels); got != piOwn {
		t.Errorf("Pi's own magpie provider written over:\n%s", got)
	}

	// connected through magpie, the list follows the catalog
	a := opencode(home, cfgDir)
	if err := a.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := provider.Save(provider.Provider{ID: "later", Name: "Later", Key: "k", Chat: "http://127.0.0.1:1/v1", Models: []string{"m3"}}); err != nil {
		t.Fatal(err)
	}
	SyncCatalog()
	if got := readFile(oc); !strings.Contains(got, `"later/m3"`) || !strings.Contains(got, `"added/m2"`) {
		t.Errorf("a connected OpenCode's list not kept current:\n%s", got)
	}

	// switched off on the Agents page, then a provider of that name
	// written by hand again: it is the user's
	if err := a.Disconnect(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, oc, own)
	SyncCatalog()
	if got := readFile(oc); got != own {
		t.Errorf("after Disconnect, OpenCode's own magpie provider written over:\n%s", got)
	}

	// a model of magpie's picked in OpenCode on that provider of its own
	// doesn't connect it (JFunfunfun on X: handWired); connecting it in
	// magpie does, and the list is magpie's again
	picked := strings.Replace(own, `"anthropic/claude-x"`, `"magpie/relay/glm-4.6"`, 1)
	writeFile(t, oc, picked)
	SyncCatalog()
	if got := readFile(oc); got != picked {
		t.Errorf("a magpie model picked on OpenCode's own magpie provider, written over:\n%s", got)
	}
	if err := a.Connect(); err != nil {
		t.Fatal(err)
	}
	SyncCatalog()
	if got := readFile(oc); !strings.Contains(got, `"later/m3"`) {
		t.Errorf("a connected OpenCode's list not kept current:\n%s", got)
	}
}
