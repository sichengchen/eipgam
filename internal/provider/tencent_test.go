package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sichengchen/eipgam/internal/catalog"
)

const (
	tcIntlHost = "https://tokenhub-intl.tencentmaas.com"
	tcIntlKeys = "https://console.tencentcloud.com/tokenhub/apikey"
	tcIntlDocs = "https://www.tencentcloud.com/document/product/1300/78939"
	tcEntKeys  = "https://console.tencentcloud.com/tokenhub/tokenplan"
	tcEntDocs  = "https://cloud.tencent.com/document/product/1823/130659"
	// TokenHub's China host, its plans there, and where its Token Plan 个人版
	// was before Tencent moved it (ysm119): mainland China's, which eipgam
	// leaves out
	tcCNHost      = "https://tokenhub.tencentmaas.com"
	tcCNPlanChat  = tcCNHost + "/plan/v3"
	tcCNPlanMsgs  = tcCNHost + "/plan/anthropic"
	tcOldPlanChat = "https://api.lkeap.cloud.tencent.com/plan/v3"
	tcOldPlanMsgs = "https://api.lkeap.cloud.tencent.com/plan/anthropic"
)

// tcRegions are Tencent Cloud International's two: TokenHub's pay as you
// go, the default, and its Token Plan 企业版, and what a provider there is
// made with.
var tcRegions = []struct {
	region                     string
	chat, responses, anthropic string
	catalog, keys, website     string
}{
	{"intl", tcIntlHost + "/v1", tcIntlHost + "/v1", tcIntlHost, "tencent-tokenhub", tcIntlKeys, tcIntlDocs},
	{"enterprise-intl", tcIntlHost + "/plan/v3", tcIntlHost + "/plan/v3", tcIntlHost + "/plan/anthropic", "", tcEntKeys, tcEntDocs},
}

// Tencent Cloud is one preset (Jorben on Discord), TokenHub's global pay
// as you go and its Token Plan 企业版 its regions: TokenHub's at its host
// with Responses, models.dev's tencent-tokenhub and no models given, the
// plan's under /plan with its own models. TokenHub's old preset id still
// names it; its China presets are gone, and an endpoint of theirs is no
// longer taken for Singapore's.
func TestTencentCloud(t *testing.T) {
	var tc []PresetDef
	for _, pr := range Presets() {
		if pr.Icon == "tencentcloud-color" {
			tc = append(tc, pr)
		}
	}
	if len(tc) != 1 || tc[0].ID != "tencent-cloud" || tc[0].Name != "Tencent Cloud" || tc[0].Kind != KindVendor || !tc[0].Hosts || tc[0].NoList || tc[0].NoKey {
		t.Fatalf("Tencent Cloud's presets: %+v", tc)
	}
	pr := &tc[0]
	if len(pr.Regions) != len(tcRegions) {
		t.Fatalf("regions: %+v", pr.Regions)
	}
	for i, c := range tcRegions {
		r := pr.Regions[i]
		if r.ID != c.region || r.Chat != c.chat || r.Responses != c.responses || r.Anthropic != c.anthropic ||
			r.Catalog != c.catalog || r.KeysURL != c.keys || r.Website != c.website {
			t.Fatalf("region %d: %+v", i, r)
		}
	}
	// TokenHub's old id names the preset, and builds a provider at its region
	intl := tcRegions[0]
	if Preset("tencent-tokenhub") == nil || Preset("tencent-tokenhub").ID != pr.ID {
		t.Fatalf("tencent-tokenhub: %+v", Preset("tencent-tokenhub"))
	}
	p, err := FromPreset("tencent-tokenhub")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "tencent-cloud" || p.Preset != "tencent-cloud" || p.Chat != intl.chat || p.Responses != intl.responses || p.Anthropic != intl.anthropic ||
		p.Catalog != intl.catalog || p.KeysURL != intl.keys || p.Website != intl.website || p.Icon != "tencentcloud-color" {
		t.Fatalf("FromPreset(tencent-tokenhub): %+v", p)
	}
	if n := normalize(p); n.Catalog != intl.catalog || n.KeysURL != intl.keys || n.Website != intl.website {
		t.Fatalf("normalized tencent-tokenhub: %+v", n)
	}
	// the China presets' ids name nothing now
	for _, old := range []string{"tencent-token-plan", "tencent-tokenhub-cn"} {
		if pr := Preset(old); pr != nil {
			t.Fatalf("%s names %s", old, pr.ID)
		}
	}
	// the default is TokenHub's global pay as you go, with its catalog once
	// saved, and none of the plan's models: its list is its own
	p, _ = FromPreset("tencent-cloud")
	if n := normalize(p); n.Chat != intl.chat || n.Responses != intl.responses || n.Anthropic != intl.anthropic || n.Catalog != intl.catalog || n.KeysURL != intl.keys {
		t.Fatalf("default: %+v", n)
	}
	if got := p.planModels(nil); got != nil {
		t.Fatalf("pay as you go given the plan's models: %+v", got)
	}
	// the plan before its list is fetched: its models; a list it gave is
	// kept whole
	plan := pr.Regions[1]
	for _, id := range plan.Models {
		if id != strings.ToLower(id) {
			t.Fatalf("model ids are lowercase on the plan: %q", id)
		}
	}
	p.Chat, p.Responses, p.Anthropic = plan.Chat, plan.Responses, plan.Anthropic
	if got := p.planModels(nil); len(got) != len(plan.Models) || got[0].ID != "auto" {
		t.Fatalf("plan's: %+v", got)
	}
	if got := p.planModels([]catalog.Model{{ID: "glm-5.3"}, {ID: "hy3"}}); len(got) != 2 || got[0].ID != "glm-5.3" {
		t.Fatalf("listed: %+v", got)
	}
	// models.dev's tencent-tokenhub is still Tencent Cloud's logo
	if IconForCatalog("tencent-tokenhub") != "tencentcloud-color" {
		t.Fatal("tencent-tokenhub has no icon")
	}
	// an entry imported from another app is the preset at the region its
	// endpoints are, whichever API it was set up on
	for _, c := range tcRegions {
		var es []endpoints
		if c.region == "enterprise-intl" {
			es = []endpoints{{anthropic: c.anthropic}, {chat: c.chat}}
		} else {
			es = []endpoints{{chat: c.chat + "/"}, {responses: c.responses}, {anthropic: c.anthropic}}
		}
		for _, e := range es {
			im, _ := imported("Tencent", "sk-x", e, nil)
			if im.Preset != "tencent-cloud" || im.Icon != "tencentcloud-color" || im.Catalog != c.catalog || im.KeysURL != c.keys {
				t.Fatalf("imported from %+v: %+v", e, im)
			}
			if got := slices.DeleteFunc([]string{im.Chat, im.Responses, im.Anthropic}, func(s string) bool { return s == "" }); !slices.Contains([]string{c.chat, c.responses, c.anthropic}, got[0]) {
				t.Fatalf("imported from %+v at %+v", e, im)
			}
		}
	}
	// one set up at a China endpoint stays the user's own, asked where it
	// was: its key is never sent to Singapore's host
	for _, e := range []endpoints{{chat: tcCNHost + "/v1"}, {anthropic: tcCNHost}, {chat: tcCNPlanChat}, {anthropic: tcCNPlanMsgs}, {chat: tcOldPlanChat}} {
		im, _ := imported("Tencent", "sk-x", e, nil)
		if im.Preset != "" || strings.Contains(im.Chat+im.Responses+im.Anthropic, "tokenhub-intl") || im.Chat+im.Anthropic != e.chat+e.anthropic {
			t.Fatalf("imported from %+v: %+v", e, im)
		}
	}
	// a eipgam://import link: Tencent Cloud at a region, an old link by
	// TokenHub's old id, and the old id at another region, each where it
	// says
	for _, c := range tcRegions {
		links := []string{
			"eipgam://import?preset=tencent-cloud&region=" + c.region,
			"eipgam://import?preset=tencent-tokenhub&region=" + c.region,
		}
		if c.region == "intl" {
			links = append(links, "eipgam://import?preset=tencent-tokenhub")
		}
		for _, link := range links {
			p, err := ParseImport(link)
			if err != nil {
				t.Fatalf("%s: %v", link, err)
			}
			if p.Preset != "tencent-cloud" || p.Chat != c.chat || p.Responses != c.responses || p.Anthropic != c.anthropic ||
				p.Catalog != c.catalog || p.KeysURL != c.keys || p.Website != c.website {
				t.Fatalf("%s: %+v", link, p)
			}
		}
	}
	// a link made for a China region, or by a China preset's id, lands on
	// no China endpoint
	for _, link := range []string{"eipgam://import?preset=tencent-cloud&region=plan", "eipgam://import?preset=tencent-cloud&region=cn", "eipgam://import?preset=tencent-token-plan"} {
		if p, err := ParseImport(link); err == nil && strings.Contains(p.Chat+p.Responses+p.Anthropic, "tokenhub.tencentmaas.com") {
			t.Fatalf("%s: %+v", link, p)
		}
	}
}

// Token Plan 企业版 (#1574) is at TokenHub's Singapore host under /plan
// with chat completions, Responses and Anthropic messages, with its own
// models before a list: an entry imported at its endpoints, a link and a
// provider moved there in the editor each land on it, and TokenHub's pay
// as you go at the same host stays where it was.
func TestTencentCloudEnterprisePlan(t *testing.T) {
	pr := Preset("tencent-cloud")
	for _, c := range []struct{ region, host, keys, docs, first string }{
		{"enterprise-intl", tcIntlHost, tcEntKeys, tcEntDocs, "auto"},
	} {
		var r *Region
		for i := range pr.Regions {
			if pr.Regions[i].ID == c.region {
				r = &pr.Regions[i]
			}
		}
		if r == nil || r.Chat != c.host+"/plan/v3" || r.Responses != c.host+"/plan/v3" || r.Anthropic != c.host+"/plan/anthropic" ||
			r.Catalog != "" || r.KeysURL != c.keys || r.Website != c.docs ||
			len(r.Models) == 0 || r.Models[0] != c.first {
			t.Fatalf("%s: %+v", c.region, r)
		}
		for _, id := range r.Models {
			if id != strings.ToLower(id) {
				t.Fatalf("%s: model ids are lowercase: %q", c.region, id)
			}
		}
		for _, e := range []endpoints{{chat: r.Chat}, {responses: r.Responses}, {anthropic: r.Anthropic}} {
			im, _ := imported("Tencent", "sk-x", e, nil)
			if im.Preset != "tencent-cloud" || im.KeysURL != c.keys || im.Catalog != "" {
				t.Fatalf("imported from %+v: %+v", e, im)
			}
			if got := im.regionOf(pr); got == nil || got.ID != c.region {
				t.Fatalf("imported from %+v at %+v", e, got)
			}
		}
		p, err := ParseImport("eipgam://import?preset=tencent-cloud&region=" + c.region)
		if err != nil || p.Chat != r.Chat || p.Responses != r.Responses || p.Anthropic != r.Anthropic || p.KeysURL != c.keys {
			t.Fatalf("link: %v %+v", err, p)
		}
		if got := p.planModels(nil); len(got) != len(r.Models) || got[0].ID != c.first {
			t.Fatalf("%s's models: %+v", c.region, got)
		}
		// pay as you go at the same host moved here takes the plan's key
		// page and docs, and no catalog
		q, _ := FromPreset("tencent-cloud")
		q.Chat, q.Responses, q.Anthropic, q.Catalog, q.KeysURL, q.Website = c.host+"/v1", c.host+"/v1", c.host, "tencent-tokenhub", "", ""
		if got := q.regionOf(pr); got == nil || got.ID == c.region {
			t.Fatalf("pay as you go at %+v", got)
		}
		q.Chat, q.Responses, q.Anthropic, q.Catalog = r.Chat, r.Responses, r.Anthropic, ""
		if n := normalize(q); n.KeysURL != c.keys || n.Website != r.Website || n.Catalog != "" {
			t.Fatalf("moved to %s: %+v", c.region, n)
		}
	}
}

// A provider added under any of the three presets Tencent Cloud was keeps
// working with nothing done: TokenHub's global one as Tencent Cloud's,
// its id, key, endpoints, models, catalog (names and prices), key page and
// docs as they were; the two China ones, whose presets are gone, with
// their id, key, endpoints and models as they were, asked where they
// always were and never moved onto Singapore's host; the routing group
// naming them finding all three. A save from the editor (which sends no
// key page or docs) keeps them, and moving TokenHub's to the plan takes
// the plan's.
func TestTencentCloudOldPresetsKeepWorking(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	// providers.json as an older eipgam wrote it
	old := `{"providers":[
{"id":"tencent-token-plan","name":"Tencent Cloud Token Plan","icon":"tencentcloud-color","preset":"tencent-token-plan","key":"sk-tp-plan",
 "chat":"` + tcOldPlanChat + `","anthropic":"` + tcOldPlanMsgs + `","models":["glm-5.3","hy3"]},
{"id":"tencent-tokenhub-cn","name":"Tencent Cloud TokenHub (China)","icon":"tencentcloud-color","preset":"tencent-tokenhub-cn","key":"sk-cn",
 "chat":"` + tcCNHost + `/v1","responses":"` + tcCNHost + `/v1","anthropic":"` + tcCNHost + `","models":["hy3","deepseek-v4-pro"],"catalog":"tencent-tokenhub"},
{"id":"tencent-tokenhub","name":"Tencent Cloud TokenHub","icon":"tencentcloud-color","preset":"tencent-tokenhub","key":"sk-intl",
 "chat":"` + tcIntlHost + `/v1","responses":"` + tcIntlHost + `/v1","anthropic":"` + tcIntlHost + `","models":["hy3"],"catalog":"tencent-tokenhub","website":"` + tcIntlDocs + `","keysUrl":"` + tcIntlKeys + `"}],
"groups":[{"id":"hy","name":"Hy","members":["tencent-token-plan/hy3","tencent-tokenhub-cn/hy3","tencent-tokenhub/hy3"]}]}`
	if err := os.MkdirAll(filepath.Dir(Path()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	type was struct {
		key, chat, responses, anthropic string
		models                          []string
	}
	saved := []struct {
		id string
		was
	}{
		{"tencent-token-plan", was{"sk-tp-plan", tcOldPlanChat, "", tcOldPlanMsgs, []string{"glm-5.3", "hy3"}}},
		{"tencent-tokenhub-cn", was{"sk-cn", tcCNHost + "/v1", tcCNHost + "/v1", tcCNHost, []string{"hy3", "deepseek-v4-pro"}}},
		{"tencent-tokenhub", was{"sk-intl", tcIntlHost + "/v1", tcIntlHost + "/v1", tcIntlHost, []string{"hy3"}}},
	}
	intl := tcRegions[0]
	check := func(when string) {
		t.Helper()
		for _, c := range saved {
			p, err := Find(c.id)
			if err != nil {
				t.Fatalf("%s: %s: %v", when, c.id, err)
			}
			if p.ID != c.id || p.Key != c.key || !slices.Equal(p.Models, c.models) ||
				p.Chat != c.chat || p.Responses != c.responses || p.Anthropic != c.anthropic || p.Icon != "tencentcloud-color" {
				t.Fatalf("%s: %s: %+v", when, c.id, p)
			}
		}
		// TokenHub's global one is Tencent Cloud's, at its region
		p, _ := Find("tencent-tokenhub")
		if p.Preset != "tencent-cloud" || p.Catalog != intl.catalog || p.KeysURL != intl.keys || p.Website != intl.website {
			t.Fatalf("%s: tencent-tokenhub: %+v", when, p)
		}
		if got := p.planModels(nil); got != nil {
			t.Fatalf("%s: TokenHub given the plan's models: %+v", when, got)
		}
		if r := p.regionOf(Preset(p.Preset)); r == nil || r.ID != intl.region {
			t.Fatalf("%s: tencent-tokenhub at %+v", when, r)
		}
		// the China ones are no preset's now, and nothing asks them for an
		// access key
		for _, id := range []string{"tencent-token-plan", "tencent-tokenhub-cn"} {
			p, _ := Find(id)
			if Preset(p.Preset) != nil || AsksAccessKey(*p) != "" {
				t.Fatalf("%s: %s: preset %q, asks %q", when, id, p.Preset, AsksAccessKey(*p))
			}
		}
		// the routing group naming each by its id finds all three
		_, ms, ok := FindGroup(GroupPrefix + "hy")
		if !ok || len(ms) != 3 {
			t.Fatalf("%s: group: %v %+v", when, ok, ms)
		}
		for i, c := range saved {
			if ms[i].Provider.ID != c.id || ms[i].Model != "hy3" || ms[i].Provider.Chat != c.chat {
				t.Fatalf("%s: member %d: %+v", when, i, ms[i])
			}
		}
	}
	check("read")
	// the editor's Save: the provider's own fields, no key page or docs
	for _, c := range saved {
		p, _ := Find(c.id)
		p.Website, p.KeysURL = "", ""
		if err := Save(*p); err != nil {
			t.Fatal(err)
		}
	}
	check("saved")

	// TokenHub's moved to the plan in the editor takes the plan's key page
	// and docs and no catalog, and back again TokenHub's
	plan := tcRegions[1]
	p, _ := Find("tencent-tokenhub")
	p.Chat, p.Responses, p.Anthropic, p.Website, p.KeysURL = plan.chat, plan.responses, plan.anthropic, "", ""
	n := normalize(*p)
	if n.Catalog != "" || n.KeysURL != plan.keys || n.Website != plan.website {
		t.Fatalf("to the plan: %+v", n)
	}
	n.Chat, n.Responses, n.Anthropic, n.Website, n.KeysURL = intl.chat, intl.responses, intl.anthropic, "", ""
	if n := normalize(n); n.Catalog != intl.catalog || n.KeysURL != intl.keys || n.Website != intl.website {
		t.Fatalf("to pay as you go: %+v", n)
	}
	// a catalog the user gave stays wherever it is
	p.Catalog = "moonshotai"
	if n := normalize(*p); n.Catalog != "moonshotai" {
		t.Fatalf("own catalog: %q", n.Catalog)
	}
}

// TokenHub's /v1/models, as its docs show it, asked with the key as a
// Bearer: its language models are kept, the embedding one kept for
// /v1/embeddings, out of the chat models (#5339).
func TestTencentTokenHubList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer sk-test" {
			rw.WriteHeader(http.StatusUnauthorized)
			return
		}
		rw.Write([]byte(`{"object":"list","data":[
{"id":"hy3","object":"model","name":"Hy3","created":1783267200,"status":"online"},
{"id":"deepseek-v4-pro","object":"model","name":"DeepSeek-V4-Pro","created":1776960000,"status":"online"},
{"id":"kinfra-text-embedding-0.6b","object":"model","name":"KInfra Embedding","created":1776960000,"status":"online"}]}`))
	}))
	defer srv.Close()
	ms, _, err := catalog.FetchAt(context.Background(), srv.URL+"/v1", "sk-test", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if chat := catalog.Chat(ms); len(chat) != 2 || chat[0].ID != "hy3" || chat[1].ID != "deepseek-v4-pro" {
		t.Fatalf("%+v", ms)
	}
	if len(ms) != 3 || ms[2].ID != "kinfra-text-embedding-0.6b" || ms[2].Serves != catalog.ServesEmbedding {
		t.Fatalf("the embedding model: %+v", ms)
	}
}
