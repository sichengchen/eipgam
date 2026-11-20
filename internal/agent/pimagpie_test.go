package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sichengchen/eipgam/internal/edit"
	"github.com/sichengchen/eipgam/internal/provider"
)

func TestPiKeepsBothProviderIDsWithMagpieExtension(t *testing.T) {
	home := syncHome(t)
	dir := filepath.Join(home, ".pi", "agent")
	os.MkdirAll(dir, 0o755)
	models := filepath.Join(dir, "models.json")
	settings := filepath.Join(dir, "settings.json")
	os.WriteFile(settings, []byte(`{"packages":[]}`), 0o644)
	os.WriteFile(models, []byte(`{"providers":{"mine":{"baseUrl":"http://x/v1"}}}`), 0o644)
	if err := provider.Save(provider.Provider{ID: "p", Name: "P", Key: "k", Chat: "http://127.0.0.1:1/v1", Models: []string{"m"}}); err != nil {
		t.Fatal(err)
	}
	if err := pi(home).Field("model").Set("eipgam/p/m"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFile(models), `"eipgam"`) {
		t.Fatalf("eipgam provider is missing: %s", readFile(models))
	}
	if err := edit.SetJSON(models, edit.KV{Path: "providers.magpie", Value: map[string]any{"baseUrl": "http://other.example/v1"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"packages":["npm:pi-magpie"],"defaultProvider":"eipgam"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := pi(home).Sync(); err != nil {
		t.Fatal(err)
	}
	got := readFile(models)
	for _, name := range []string{`"eipgam"`, `"magpie"`, `"mine"`} {
		if !strings.Contains(got, name) {
			t.Fatalf("%s provider is missing: %s", name, got)
		}
	}
	if msg := pi(home).Check(); msg != "" {
		t.Fatalf("check: %s", msg)
	}
}
