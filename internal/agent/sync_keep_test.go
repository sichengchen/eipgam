package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// providerAt is the object at providers.<id> (or provider.<id>) of a JSON
// file, decoded.
func providerAt(t *testing.T, path, top string) map[string]any {
	t.Helper()
	var f map[string]json.RawMessage
	var ps map[string]map[string]any
	if err := json.Unmarshal([]byte(readFile(path)), &f); err != nil || json.Unmarshal(f[top], &ps) != nil {
		t.Fatalf("%s: %v\n%s", path, err, readFile(path))
	}
	return ps[magpieID]
}

// #1103: pi-cache-optimizer's /cache-optimizer fix adds
// providers.eipgam.compat.sendSessionAffinityHeaders to Pi's models.json.
// A catalog sync, and a pick of a eipgam model in Pi, keep it; the model
// list is still eipgam's, so a model gone from the catalog leaves it, and
// a sync with nothing new writes nothing.
func TestPiKeepsTheirKeysInMagpiesProvider(t *testing.T) {
	home := syncHome(t)
	piModels := filepath.Join(home, ".pi", "agent", "models.json")
	// the block as eipgam wrote it, with the optimizer's key added
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
	// Pi runs a model of eipgam's: connected, so its list follows
	writeFile(t, filepath.Join(home, ".pi", "agent", "settings.json"), `{"defaultProvider":"eipgam","defaultModel":"relay/glm-4.6"}`)
	setUpInMagpie("pi")
	SyncCatalog()
	p := providerAt(t, piModels, "providers")
	compat, _ := p["compat"].(map[string]any)
	if compat["sendSessionAffinityHeaders"] != true {
		t.Fatalf("compat.sendSessionAffinityHeaders dropped by the sync:\n%s", readFile(piModels))
	}
	ms, _ := json.Marshal(p["models"])
	if strings.Contains(string(ms), "gone/old-model") || !strings.Contains(string(ms), "relay/glm-4.6") {
		t.Fatalf("models not eipgam's list: %s", ms)
	}

	// synced already: the next sync writes nothing
	st, _ := os.Stat(piModels)
	old := st.ModTime().Add(-1e12)
	os.Chtimes(piModels, old, old)
	SyncCatalog()
	if now, _ := os.Stat(piModels); !now.ModTime().Equal(old) {
		t.Errorf("models.json rewritten though nothing changed:\n%s", readFile(piModels))
	}

	// picking a eipgam model in Pi writes the block too
	if err := pi(home).Fields[0].Set("eipgam/relay/glm-4.6"); err != nil {
		t.Fatal(err)
	}
	compat, _ = providerAt(t, piModels, "providers")["compat"].(map[string]any)
	if compat["sendSessionAffinityHeaders"] != true {
		t.Fatalf("compat.sendSessionAffinityHeaders dropped by a pick:\n%s", readFile(piModels))
	}
}
