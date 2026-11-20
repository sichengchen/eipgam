package provider

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sichengchen/eipgam/internal/catalog"
	"github.com/sichengchen/eipgam/internal/settings"
)

// ccPlugin is the Command Code plan as its plugin serves it: its
// requests go to plugin://, which names no vendor (treeman on Discord).
var ccPlugin = Provider{ID: CommandCodePlanID, Chat: pluginBase + CommandCodePlanID + "/v1", Responses: pluginBase + CommandCodePlanID + "/v1"}

// Only the vendors whose own pricing bills DeepSeek's models by the hour
// are: DeepSeek's API, Command Code's DeepSeek V4 models (not its flat
// deepseek-v4-flash-fast) and OpenCode Go's two. OpenCode Zen, a relay, a
// lookalike host and every other model keep one price.
func TestPeakOf(t *testing.T) {
	preset := func(id string) Provider {
		p, err := FromPreset(id)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	for _, c := range []struct {
		name  string
		p     Provider
		model string
		want  catalog.Peak
	}{
		{"commandcode", preset("commandcode"), "deepseek/deepseek-v4.1-flash", catalog.PeakWeekdays},
		{"commandcode pro", preset("commandcode"), "deepseek/deepseek-v4-pro", catalog.PeakWeekdays},
		{"commandcode flash-fast", preset("commandcode"), "deepseek/deepseek-v4-flash-fast", ""},
		{"commandcode other", preset("commandcode"), "moonshotai/kimi-k3", ""},
		{"commandcode plan", Provider{ID: CommandCodePlanID}, "deepseek/deepseek-v4.1-flash", catalog.PeakWeekdays},
		{"commandcode plan's plugin", ccPlugin, "deepseek/deepseek-v4.1-flash", catalog.PeakWeekdays},
		{"commandcode plan's plugin flash-fast", ccPlugin, "deepseek/deepseek-v4-flash-fast", ""},
		{"another plugin", Provider{ID: "zen-free", Chat: pluginBase + "zen-free/v1"}, "deepseek-v4.1-flash", ""},
		{"opencode go", preset("opencode-go"), "deepseek-v4.1-flash", catalog.PeakWeekdays},
		{"opencode go other", preset("opencode-go"), "kimi-k3", ""},
		{"opencode zen", preset("opencode-zen"), "deepseek-v4.1-flash", ""},
		{"relay", Provider{ID: "relay", Chat: "https://relay.example/v1"}, "deepseek-v4.1-flash", ""},
		{"relay named deepseek", Provider{ID: "deepseek", Preset: "deepseek", Chat: "https://relay.example/v1"}, "deepseek-flash", ""},
		{"lookalike host", Provider{ID: "x", Chat: "https://api.deepseek.com.evil.example/v1"}, "deepseek-flash", ""},
		{"openai", preset("openai"), "gpt-6-astra", ""},
		// Ollama Cloud: the two models its pricing page gives an Off-Peak
		// row, their dated tags too, and nothing else it serves
		{"ollama cloud flash", preset("ollama-cloud"), "deepseek-v4.1-flash", catalog.PeakOllama},
		{"ollama cloud pro", preset("ollama-cloud"), "deepseek-v4-pro", catalog.PeakOllama},
		{"ollama cloud dated flash", preset("ollama-cloud"), "deepseek-v4.1-flash:1017", catalog.PeakOllama},
		{"ollama cloud dated pro", preset("ollama-cloud"), "deepseek-v4-pro:0813", catalog.PeakOllama},
		{"ollama cloud v4-flash", preset("ollama-cloud"), "deepseek-v4-flash", ""},
		{"ollama cloud glm", preset("ollama-cloud"), "glm-5.3", ""},
		{"ollama cloud gpt-oss", preset("ollama-cloud"), "gpt-oss:120b", ""},
		{"ollama cloud lookalike tag", preset("ollama-cloud"), "deepseek-v4-pro-max:1", ""},
		{"removed ollama cloud", Provider{ID: "ollama-cloud"}, "deepseek-v4-pro", catalog.PeakOllama},
		{"ollama local", preset("ollama"), "deepseek-v4-pro", ""},
		{"relay named ollama cloud", Provider{ID: "ollama-cloud", Preset: "ollama-cloud", Chat: "https://relay.example/v1"}, "deepseek-v4-pro", ""},
		{"ollama lookalike host", Provider{ID: "x", Chat: "https://ollama.com.evil.example/v1"}, "deepseek-v4-pro", ""},
		{"opencode go at ollama's model", preset("opencode-go"), "deepseek-v4-pro:0813", ""},
	} {
		if got := c.p.PeakOf(c.model); got != c.want {
			t.Errorf("%s %s: %q, want %q", c.name, c.model, got, c.want)
		}
	}
}

// ollamaCloudCatalog is models.dev's api.json as eipgam had it on
// 2026-11-01, these entries byte for byte (descriptions and the like left
// out): Ollama Cloud's DeepSeek models at their Off-Peak row of
// https://ollama.com/pricing, its GLM at its one price, and DeepSeek's own
// API beside it.
const ollamaCloudCatalog = `{
"ollama-cloud":{"id":"ollama-cloud","api":"https://ollama.com/v1","name":"Ollama Cloud","models":{
"deepseek-v4.1-flash":{"id":"deepseek-v4.1-flash","cost":{"input":0.15,"output":0.6,"cache_read":0.003}},
"deepseek-v4.1-flash:1017":{"id":"deepseek-v4.1-flash:1017","cost":{"input":0.15,"output":0.6,"cache_read":0.003}},
"deepseek-v4-pro":{"id":"deepseek-v4-pro","cost":{"input":0.66,"output":1.98,"cache_read":0.022}},
"glm-5.3":{"id":"glm-5.3","cost":{"input":1.4,"output":4.4,"cache_read":0.26}}}},
"deepseek":{"id":"deepseek","api":"https://api.deepseek.com","name":"DeepSeek","models":{
"deepseek-v4-pro":{"id":"deepseek-v4-pro","cost":{"input":0.66,"output":1.98,"cache_read":0.013}}}}
}`

// Ollama Cloud's DeepSeek V4.1 Flash and V4 Pro cost their listed
// (off-peak) price outside 12:00 to 18:00 UTC on weekdays and all of
// Saturday and Sunday, and double in those hours, which is its pricing page's peak
// row ($0.30/$1.20, $1.32/$3.96). Every edge is checked a minute either
// side, on the clock of the far zones a call can be written in. Its GLM,
// and DeepSeek's own API in Ollama's peak hours, cost what they did.
func TestOllamaCloudOffPeakPrice(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalog.CachePath(), []byte(ollamaCloudCatalog), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)
	oc, err := FromPreset("ollama-cloud")
	if err != nil {
		t.Fatal(err)
	}
	none := settings.Settings{}
	price := func(p Provider, model string, tm time.Time) catalog.Price {
		t.Helper()
		pr, ok := p.PriceAt(none, model, tm)
		if !ok {
			t.Fatalf("%s %s: unpriced", p.ID, model)
		}
		return pr.In(tm)
	}
	zones := []*time.Location{time.UTC, time.FixedZone("+14", 14*3600), time.FixedZone("-11", -11*3600)}
	utc := func(day, hm string) time.Time {
		v, err := time.Parse("2006-01-02 15:04", day+" "+hm)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, c := range []struct {
		day, hm string
		peak    bool
	}{
		{"2026-10-28", "11:59", false}, // a Wednesday
		{"2026-10-28", "12:00", true},
		{"2026-10-28", "17:59", true},
		{"2026-10-28", "18:00", false},
		{"2026-10-28", "02:00", false}, // DeepSeek's peak, not Ollama's
		{"2026-10-26", "12:00", true},  // a Monday
		{"2026-10-30", "17:59", true},  // a Friday
		{"2026-10-05", "13:00", true},  // a Chinese holiday: Ollama names none
		{"2026-10-31", "12:00", false}, // a Saturday
		{"2026-11-01", "17:59", false}, // a Sunday
	} {
		for _, z := range zones {
			tm := utc(c.day, c.hm).In(z)
			name := c.day + " " + c.hm + " UTC in " + z.String()
			f, pro := price(oc, "deepseek-v4.1-flash", tm), price(oc, "deepseek-v4-pro", tm)
			dated := price(oc, "deepseek-v4.1-flash:1017", tm)
			wantF := catalog.Price{Input: 0.15, Output: 0.6, CacheRead: 0.003}
			wantP := catalog.Price{Input: 0.66, Output: 1.98, CacheRead: 0.022}
			if c.peak {
				wantF, wantP = wantF.Times(2), wantP.Times(2)
			}
			if !f.Same(wantF) || !dated.Same(wantF) {
				t.Errorf("%s: flash %+v, dated %+v, want %+v", name, f, dated, wantF)
			}
			if !pro.Same(wantP) {
				t.Errorf("%s: pro %+v, want %+v", name, pro, wantP)
			}
			if g := price(oc, "glm-5.3", tm); !g.Same(catalog.Price{Input: 1.4, Output: 4.4, CacheRead: 0.26}) {
				t.Errorf("%s: glm-5.3 %+v, want its one price", name, g)
			}
		}
	}
	// the page's peak row, to the cent
	peak := price(oc, "deepseek-v4-pro", utc("2026-10-28", "15:00"))
	if peak.Input != 1.32 || peak.Output != 3.96 || peak.CacheRead != 0.044 {
		t.Errorf("deepseek-v4-pro at peak: %+v, want $1.32/$3.96/$0.044", peak)
	}
}

// commandCodeCatalog is models.dev's api.json as eipgam had it on
// 2026-10-30, these entries byte for byte (descriptions and the like left
// out). DeepSeek lists V4.1 Flash only as deepseek-flash, by a dated
// canonical id; Command Code's own catalog lists its undated
// deepseek/deepseek-v4.1-flash, which comes first for its plan.
const commandCodeCatalog = `{
"deepseek":{"id":"deepseek","api":"https://api.deepseek.com","name":"DeepSeek","models":{
"deepseek-flash":{"id":"deepseek-flash","name":"DeepSeek Flash","cost":{"input":0.15,"output":0.6,"cache_read":0.003},"canonical_model_id":"deepseek/deepseek-v4.1-flash-1017"},
"deepseek-v4-pro":{"id":"deepseek-v4-pro","name":"DeepSeek V4 Pro","cost":{"input":0.66,"output":1.98,"cache_read":0.013},"canonical_model_id":"deepseek/deepseek-v4-pro"}}},
"commandcode":{"id":"commandcode","env":["CMD_API_KEY"],"npm":"@ai-sdk/openai-compatible","api":"https://api.commandcode.ai/provider/v1","name":"CommandCode","doc":"https://commandcode.ai/docs/provider","models":{
"deepseek/deepseek-v4.1-flash":{"id":"deepseek/deepseek-v4.1-flash","name":"DeepSeek V4.1 Flash","family":"deepseek-flash","cost":{"input":0.15,"output":0.6,"cache_read":0.003},"canonical_model_id":"deepseek/deepseek-v4.1-flash"},
"deepseek/deepseek-v4-pro":{"id":"deepseek/deepseek-v4-pro","name":"DeepSeek V4 Pro","cost":{"input":0.9,"output":2.7,"cache_read":0.02},"canonical_model_id":"deepseek/deepseek-v4-pro"}}}
}`

// The Command Code plan, served by its plugin, prices DeepSeek V4.1 Flash
// at Command Code's list price, doubled in its peak hours, where it showed
// no price at all and its calls were counted at nothing (treeman on
// Discord). A model DeepSeek prices keeps DeepSeek's price, and a price
// the user set is doubled at peak too.
func TestCommandCodePlanPrice(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalog.CachePath(), []byte(commandCodeCatalog), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)
	peak := time.Date(2026, 10, 29, 2, 0, 0, 0, time.UTC) // a Thursday
	off := time.Date(2026, 10, 29, 11, 0, 0, 0, time.UTC)
	price := func(s settings.Settings, model string) catalog.Price {
		t.Helper()
		pr, ok := priceOf(s, ccPlugin, true, model, time.Time{})
		if !ok {
			t.Fatalf("%s: unpriced", model)
		}
		return pr
	}
	none := settings.Settings{}
	if pr, ok := ccPlugin.ListPrice("deepseek/deepseek-v4.1-flash"); !ok || pr.Input != 0.15 || pr.Output != 0.6 || pr.CacheRead != 0.003 {
		t.Errorf("list price: %+v %v; want 0.15/0.6/0.003", pr, ok)
	}
	pr := price(none, "deepseek/deepseek-v4.1-flash")
	if a, b := pr.In(peak), pr.In(off); a.Input != 0.3 || a.Output != 1.2 || b.Input != 0.15 || b.Output != 0.6 {
		t.Errorf("deepseek-v4.1-flash: %+v at peak, %+v off-peak", a, b)
	}
	// DeepSeek's own price comes first, as before
	if pr := price(none, "deepseek/deepseek-v4-pro"); pr.In(off).Input != 0.66 || pr.In(peak).Input != 1.32 {
		t.Errorf("deepseek-v4-pro: %+v", pr)
	}
	in, out, cache := 0.1, 0.4, 0.0
	for _, key := range []string{CommandCodePlanID + "/deepseek/deepseek-v4.1-flash", AnyPriceKey("deepseek/deepseek-v4.1-flash")} {
		s := settings.Settings{ModelPrices: map[string]settings.ModelPrice{key: {Input: &in, Output: &out, CacheRead: &cache, CacheWrite: &cache}}}
		pr := price(s, "deepseek/deepseek-v4.1-flash")
		if a, b := pr.In(peak), pr.In(off); a.Input != 0.2 || a.Output != 0.8 || b.Input != 0.1 || b.Output != 0.4 {
			t.Errorf("set at %s: %+v at peak, %+v off-peak", key, a, b)
		}
	}
	// another provider's id says nothing of Command Code's catalog: a relay
	// has its maker's price, DeepSeek's deepseek-flash, whose dated
	// canonical id (deepseek-v4.1-flash-1017) is this model's snapshot
	if pr, ok := (Provider{ID: "relay", Chat: "https://relay.example/v1"}).ListPrice("deepseek/deepseek-v4.1-flash"); !ok || pr.Input != 0.15 || pr.Output != 0.6 || pr.CacheRead != 0.003 {
		t.Errorf("a relay's price: %+v %v; want DeepSeek's 0.15/0.6/0.003", pr, ok)
	}
}
