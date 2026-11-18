package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

func keyPlanHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
}

// tcMixed is Tencent Cloud at its Token Plan 企业版, with a pay-as-you-go
// key beside the plan's own (yckonnnn on X had the same on Volcengine).
func tcMixed() Provider {
	pr := Preset("tencent-cloud")
	var plan Region
	for _, r := range pr.Regions {
		if r.ID == "enterprise-intl" {
			plan = r
		}
	}
	return Provider{ID: "tc", Name: "TC", Preset: "tencent-cloud", Chat: plan.Chat, Responses: plan.Responses, Anthropic: plan.Anthropic,
		Key: "k-plan", Keys: []KeyAccount{{Key: "k-api", Plan: "intl"}}}
}

// Each key goes to its own plan's endpoints, on every protocol; a key that
// names no plan stays on the provider's.
func TestKeyPlanEndpoints(t *testing.T) {
	p := tcMixed()
	if !p.PlansMixed() {
		t.Fatal("keys on two plans read as one")
	}
	const h = "https://tokenhub-intl.tencentmaas.com"
	for _, c := range []struct{ key, chat, responses, anthropic string }{
		{"k-plan", h + "/plan/v3", h + "/plan/v3", h + "/plan/anthropic"},
		{"k-api", h + "/v1", h + "/v1", h},
	} {
		var k KeyAccount
		for _, x := range p.KeysOn() {
			if x.Key == c.key {
				k = x
			}
		}
		q := p.WithKey(k)
		if q.Chat != c.chat || q.Responses != c.responses || q.Anthropic != c.anthropic {
			t.Errorf("%s at %q %q %q", c.key, q.Chat, q.Responses, q.Anthropic)
		}
		if q.PlansMixed() {
			t.Errorf("%s: one key's copy judged mixed again", c.key)
		}
	}
	// at a host of its own (a mirror, a fake vendor) each plan is asked there
	p.Chat, p.Responses, p.Anthropic = "http://127.0.0.1:9/plan/v3", "http://127.0.0.1:9/plan/v3", "http://127.0.0.1:9/plan/anthropic"
	if q := p.WithKey(p.Keys[0]); q.Chat != "http://127.0.0.1:9/v1" || q.Anthropic != "http://127.0.0.1:9" {
		t.Errorf("pay-as-you-go key at its own host: %q %q", q.Chat, q.Anthropic)
	}
}

// A model only one plan has goes only to that plan's key; one every plan
// has goes to every key; a model the user set a key to serve it serves.
func TestKeyPlanServing(t *testing.T) {
	p := tcMixed()
	serves := func(model string) string {
		f := p.PlanServing(model)
		var on []string
		for _, k := range p.KeysOn() {
			if f(k) {
				on = append(on, k.Key)
			}
		}
		return strings.Join(on, ",")
	}
	for model, want := range map[string]string{
		"qwen3-max": "k-api",        // pay as you go's open list alone
		"kimi-k3":   "k-plan,k-api", // the plan's, and the open list
		"Kimi-K3":   "k-plan,k-api",
	} {
		if got := serves(model); got != want {
			t.Errorf("%s: %s, want %s", model, got, want)
		}
	}
	p.AccountModels = map[string][]string{keyID("k-plan"): {"qwen3-max"}}
	if got := serves("qwen3-max"); got != "k-plan,k-api" {
		t.Errorf("a key set to serve a model: %s", got)
	}
}

// Old configs are as they were: keys that name no plan, a plan that is the
// provider's own, a plan the preset doesn't have, or a preset whose keys
// are all on one plan.
func TestKeyPlanOldConfigsUnchanged(t *testing.T) {
	keyPlanHome(t)
	for name, p := range map[string]Provider{
		"no plans": func() Provider { p := tcMixed(); p.Keys[0].Plan = ""; return p }(),
		"own plan": func() Provider {
			p := tcMixed()
			p.KeyPlan, p.Keys[0].Plan = "enterprise-intl", "enterprise-intl"
			return p
		}(),
		"gone":     func() Provider { p := tcMixed(); p.Keys[0].Plan = "lite"; return p }(),
		"one plan": {ID: "q", Name: "Q", Preset: "qwen", Chat: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1", Key: "a", Keys: []KeyAccount{{Key: "b", Plan: "api"}}},
	} {
		if p.PlansMixed() {
			t.Errorf("%s: mixed", name)
		}
		for _, k := range p.KeysOn() {
			if q := p.WithKey(k); q.Chat != p.Chat || q.Anthropic != p.Anthropic {
				t.Errorf("%s: key %s moved to %q", name, k.Key, q.Chat)
			}
			if !p.PlanServing("qwen3-max")(k) {
				t.Errorf("%s: key %s barred", name, k.Key)
			}
		}
		if got := planViews([]Provider{p}); len(got) != 1 || got[0].Key != p.Key || len(got[0].Keys) != len(p.Keys) {
			t.Errorf("%s: views %+v", name, got)
		}
	}
	// a saved provider that names no plan is written as before
	p := tcMixed()
	p.Keys[0].Plan = ""
	if err := Save(p); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"plan"`) || strings.Contains(string(b), `"keyPlan"`) {
		t.Errorf("a plan written for keys that name none:\n%s", b)
	}
}

// SetKeyPlan keeps the provider's own plan as none, refuses a plan the
// preset doesn't have and a preset whose keys are on one plan.
func TestSetKeyPlan(t *testing.T) {
	keyPlanHome(t)
	p := tcMixed()
	p.Keys[0].Plan = ""
	if err := Save(p); err != nil {
		t.Fatal(err)
	}
	if err := SetKeyPlan("tc", keyID("k-api"), "intl"); err != nil {
		t.Fatal(err)
	}
	if err := SetKeyPlan("tc", keyID("k-plan"), "intl"); err != nil {
		t.Fatal(err)
	}
	got, err := Find("tc")
	if err != nil {
		t.Fatal(err)
	}
	if got.KeyPlan != "intl" || got.Keys[0].Plan != "intl" {
		t.Fatalf("plans %q %q", got.KeyPlan, got.Keys[0].Plan)
	}
	if l := got.KeyList(); l[0].Plan != "intl" || l[1].Plan != "intl" {
		t.Errorf("KeyList %+v", l)
	}
	// the provider's own plan is none
	if err := SetKeyPlan("tc", keyID("k-api"), "enterprise-intl"); err != nil {
		t.Fatal(err)
	}
	if got, _ := Find("tc"); got.Keys[0].Plan != "" {
		t.Fatalf("the provider's own plan kept as %q", got.Keys[0].Plan)
	}
	if err := SetKeyPlan("tc", keyID("k-api"), "nope"); err == nil {
		t.Error("a plan the preset doesn't have")
	}
	if err := Save(Provider{ID: "q", Name: "Q", Preset: "qwen", Chat: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1", Key: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := SetKeyPlan("q", keyID("a"), "api"); err == nil {
		t.Error("a plan for a preset whose keys are on one")
	}
}

// A provider of keys on mixed plans lists each key's plan from its own
// /models, with its own key, and marks which keys see each model.
func TestKeyPlanModelList(t *testing.T) {
	keyPlanHome(t)
	var mu sync.Mutex
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.Header.Get("Authorization")+" "+r.URL.Path)
		mu.Unlock()
		if strings.HasPrefix(r.URL.Path, "/plan/") {
			_, _ = w.Write([]byte(`{"data":[{"id":"hy4-preview"},{"id":"kimi-k3"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"qwen3-max"},{"id":"kimi-k3"}]}`))
	}))
	defer srv.Close()
	p := Provider{ID: "tc", Name: "TC", Preset: "tencent-cloud", Chat: srv.URL + "/plan/v3", Responses: srv.URL + "/plan/v3", Anthropic: srv.URL + "/plan/anthropic",
		Key: "k-plan", Keys: []KeyAccount{{Key: "k-api", Plan: "intl"}}}
	ms, err := p.fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// each key at its own plan's endpoints alone
	for _, a := range asked {
		if strings.HasPrefix(a, "Bearer k-plan ") != strings.Contains(a, " /plan/") {
			t.Fatalf("asked %v", asked)
		}
	}
	if !slices.Contains(asked, "Bearer k-api /v1/models") || !slices.Contains(asked, "Bearer k-plan /plan/v3/models") {
		t.Fatalf("asked %v", asked)
	}
	have := map[string]bool{}
	for _, m := range ms {
		have[m.ID] = true
	}
	if !have["qwen3-max"] || !have["hy4-preview"] || !have["kimi-k3"] {
		t.Fatalf("models %v", have)
	}
	if p.Serves(p.first(), "qwen3-max") || !p.Serves(p.Keys[0], "qwen3-max") {
		t.Error("pay as you go's model served by the plan's key")
	}
	if !p.Serves(p.first(), "kimi-k3") || !p.Serves(p.Keys[0], "kimi-k3") {
		t.Error("a model both plans have")
	}
}

// Each plan a provider has keys of is its own view, with those keys alone,
// at that plan's endpoints: windows read per plan.
func TestKeyPlanViews(t *testing.T) {
	v := planViews([]Provider{tcMixed()})
	if len(v) != 2 {
		t.Fatalf("%d views", len(v))
	}
	for i, want := range []string{"k-plan /plan/v3", "k-api /v1"} {
		if got := v[i].Key + " " + strings.TrimPrefix(v[i].Chat, "https://tokenhub-intl.tencentmaas.com"); got != want || len(v[i].Keys) != 0 {
			t.Errorf("view %d: %s (%d more keys), want %s", i, got, len(v[i].Keys), want)
		}
	}
}
