package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sichengchen/eipgam/internal/catalog"
	"github.com/sichengchen/eipgam/internal/provider"
)

// The same for Pi's and StepCode's model entries (piModelJSON): a model
// best served on Responses carries compat.supportsStrictMode, a Chat
// model's entry carries no compat at all. Pencil's entries stay on Chat
// throughout, so it takes none.
func TestPiTellsAResponsesToolsStrictIsOff(t *testing.T) {
	home := syncHome(t)
	if err := provider.Save(provider.Provider{ID: "resp", Name: "Resp", Key: "k", Responses: "http://127.0.0.1:1/v1", Models: []string{"gpt-5.5"}}); err != nil {
		t.Fatal(err)
	}
	if err := catalog.SaveLive("resp", "http://127.0.0.1:1/v1", []catalog.Model{{ID: "gpt-5.5", Efforts: []string{"high"}}}); err != nil {
		t.Fatal(err)
	}
	piModels := filepath.Join(home, ".pi", "agent", "models.json")
	// the provider block as eipgam wrote it before this provider was, with
	// a key of pi-cache-optimizer's in it
	writeFile(t, piModels, `{
  "providers": {
    "eipgam": {
      "api": "openai-completions",
      "apiKey": "eipgam",
      "baseUrl": "http://127.0.0.1:3425/v1",
      "compat": {
        "sendSessionAffinityHeaders": true
      },
      "models": [
        {
          "id": "gone/old-model",
          "name": "Old"
        }
      ],
      "name": "eipgam"
    }
  }
}
`)
	// the models.json the test writes is one eipgam wrote while Pi was
	// connected: a sync only tends a ListOnly agent's file while it
	// reads as connected (Connect), and a block eipgam never set up is
	// the user's and is left alone
	if err := pi(home).Connect(); err != nil {
		t.Fatal(err)
	}
	SyncCatalog()
	b, err := os.ReadFile(piModels)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Providers struct {
			Eipgam struct {
				Models []struct {
					ID     string         `json:"id"`
					API    string         `json:"api"`
					Compat map[string]any `json:"compat"`
				} `json:"models"`
			} `json:"eipgam"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("%v:\n%s", err, b)
	}
	strict := map[string]bool{}
	for _, m := range f.Providers.Eipgam.Models {
		if m.API == "openai-responses" && m.Compat["supportsStrictMode"] != true {
			t.Fatalf("%s is asked on Responses and isn't told strict is off:\n%s", m.ID, b)
		}
		strict[m.ID] = m.Compat["supportsStrictMode"] == true
	}
	if !strict["resp/gpt-5.5"] {
		t.Fatalf("no responses model of eipgam's carries supportsStrictMode:\n%s", b)
	}
	if strict["relay/glm-4.6"] {
		t.Fatalf("a Chat model was told strict is off:\n%s", b)
	}
}
