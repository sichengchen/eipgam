package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yetone/magpie/internal/agentenv"
)

func keepSecretsHome(t *testing.T) {
	t.Helper()
	isolate(t)
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(h, ".cache"))
	t.Setenv("PATH", h)
	for _, v := range agentenv.Vars {
		t.Setenv(v, "")
	}
}

// #1821: a Volcengine provider's AccessKey ID and Secret went missing from
// providers.json. Sync and a restore bring the provider whole from another
// computer; one that knows no access key (a magpie before v0.1.1142, or
// one it was never given on) sends the same key and nothing else, and
// that wiped it here. Kept now while the key is the one here, as a copy
// without keys keeps its secret headers too; another key is another
// account, whose access key it isn't.
func TestSyncKeepsAccessKey(t *testing.T) {
	for _, op := range []struct {
		name string
		put  func([]Provider) error
	}{
		{"restore", func(ps []Provider) error { _, _, err := Restore(ps); return err }},
		{"mirror", func(ps []Provider) error { return Mirror(ps, nil) }},
	} {
		t.Run(op.name, func(t *testing.T) {
			for _, c := range []struct {
				name             string
				incoming         Provider
				wantAK, wantTok  string
				wantKey, wantHdr string
			}{
				{"same key, no access key", Provider{Key: "ark-here"}, "AK-here", "tok-here", "ark-here", ""},
				{"same key, its own access key", Provider{Key: "ark-here", AccessKeyID: "AK-new", SecretAccessKey: "SK-new"}, "AK-new", "tok-here", "ark-here", ""},
				{"without keys", Provider{}, "AK-here", "tok-here", "ark-here", "hdr-here"},
				{"another key", Provider{Key: "ark-other"}, "", "", "ark-other", ""},
			} {
				t.Run(c.name, func(t *testing.T) {
					keepSecretsHome(t)
					here := Provider{ID: "volcengine", Name: "Volcengine Ark", Chat: "https://ark.cn-beijing.volces.com/api/coding/v3",
						Key: "ark-here", BalanceToken: "tok-here", AccessKeyID: "AK-here", SecretAccessKey: "SK-here",
						Headers: map[string]string{"X-Api-Token": "hdr-here", "X-Plain": "plain-here"}}
					if err := Save(here); err != nil {
						t.Fatal(err)
					}
					p := c.incoming
					p.ID, p.Name, p.Chat = "volcengine", "Ark synced", here.Chat
					p.Headers = map[string]string{"X-Plain": "plain-new"}
					list := []Provider{p}
					if err := op.put(list); err != nil {
						t.Fatal(err)
					}
					if len(list[0].Headers) != 1 {
						t.Fatalf("the caller's headers were written into: %v", list[0].Headers)
					}
					ps, _ := Stored()
					if len(ps) != 1 {
						t.Fatalf("providers: %+v", ps)
					}
					got := ps[0]
					wantSK := strings.Replace(c.wantAK, "AK", "SK", 1)
					if got.AccessKeyID != c.wantAK || got.SecretAccessKey != wantSK {
						t.Errorf("access key %q %q, want %q %q", got.AccessKeyID, got.SecretAccessKey, c.wantAK, wantSK)
					}
					if got.BalanceToken != c.wantTok || got.Key != c.wantKey || got.Name != "Ark synced" {
						t.Errorf("got %+v, want token %q key %q", got, c.wantTok, c.wantKey)
					}
					if got.Headers["X-Api-Token"] != c.wantHdr || got.Headers["X-Plain"] != "plain-new" {
						t.Errorf("headers %v, want the secret one %q and the incoming plain one", got.Headers, c.wantHdr)
					}
				})
			}
		})
	}
}

// A providers.json an editor saved with a byte-order mark is read as it
// is, not taken for a broken file whose .bak (an older generation without
// the access key given since) stands in for it (#1821).
func TestProvidersFileWithBOM(t *testing.T) {
	keepSecretsHome(t)
	if err := Save(Provider{ID: "volcengine", Name: "Volcengine Ark", Chat: "https://ark.cn-beijing.volces.com/api/coding/v3", Key: "ark-k"}); err != nil {
		t.Fatal(err)
	}
	// a second save leaves the first as the .bak
	if err := Save(Provider{ID: "volcengine", Name: "Volcengine Ark", Chat: "https://ark.cn-beijing.volces.com/api/coding/v3", Key: "ark-k", AccessKeyID: "AK1", SecretAccessKey: "SK1"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(), append([]byte("\xef\xbb\xbf"), b...), 0o600); err != nil {
		t.Fatal(err)
	}
	ps, err := Stored()
	if err != nil || len(ps) != 1 || ps[0].AccessKeyID != "AK1" || ps[0].SecretAccessKey != "SK1" {
		t.Fatalf("read %+v %v, want the file's access key", ps, err)
	}
	// and a write after it keeps it
	if err := Save(Provider{ID: "other", Name: "Other", Chat: "https://other.example/v1", Key: "sk-o"}); err != nil {
		t.Fatal(err)
	}
	if p, err := Find("volcengine"); err != nil || p.AccessKeyID != "AK1" || p.SecretAccessKey != "SK1" {
		t.Fatalf("after a save: %+v %v", p, err)
	}
}

// A copy made in the editor sends the access key's ID as the form shows
// it, never the saved Secret: the copy keeps both, as with a blank ID
// (#1821). An ID of its own is its own, Secret and all.
func TestAddCopyKeepsAccessKeySecret(t *testing.T) {
	keepSecretsHome(t)
	src := Provider{ID: "volcengine", Name: "Volcengine Ark", Chat: "https://ark.cn-beijing.volces.com/api/coding/v3", Key: "ark-k", AccessKeyID: "AK1", SecretAccessKey: "SK1"}
	if err := Save(src); err != nil {
		t.Fatal(err)
	}
	id, err := AddCopy(Provider{Name: "Ark copy", Chat: src.Chat, AccessKeyID: "AK1"}, "volcengine")
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := Find(id); p.AccessKeyID != "AK1" || p.SecretAccessKey != "SK1" {
		t.Fatalf("copy: %q %q", p.AccessKeyID, p.SecretAccessKey)
	}
	id, err = AddCopy(Provider{Name: "Ark copy 2", Chat: src.Chat, AccessKeyID: "AK2", SecretAccessKey: "SK2"}, "volcengine")
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := Find(id); p.AccessKeyID != "AK2" || p.SecretAccessKey != "SK2" {
		t.Fatalf("copy with its own: %q %q", p.AccessKeyID, p.SecretAccessKey)
	}
}

// #1821: an Alibaba Cloud or Tencent plan whose provider has no access
// key was no card at all, so a key gone from providers.json looked like a
// card gone for no reason. It is a card that says what to give, once a
// plan, and none when another provider on the plan has the key.
func TestPlanCardWithoutAccessKey(t *testing.T) {
	keepSecretsHome(t)
	var keyedIn atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !keyedIn.Load() {
			t.Errorf("asked %s %s with no access key", r.Header.Get("X-Host"), r.URL)
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = rewrite{srv}
	t.Cleanup(func() { http.DefaultClient.Transport = old })
	forgetPlanQuotas()
	t.Cleanup(forgetPlanQuotas)
	const aliPlan = "https://token-plan.ap-southeast-1.maas.aliyuncs.com/compatible-mode/v1"
	for _, p := range []Provider{
		{ID: "alibaba-token-plan", Name: "Alibaba Cloud Token Plan", Preset: "alibaba-token-plan", Chat: aliPlan, Key: "sk-sp-k"},
		// the same plan again: one card
		{ID: "ali-copy", Name: "Alibaba copy", Preset: "alibaba-token-plan", Anthropic: "https://token-plan.ap-southeast-1.maas.aliyuncs.com/apps/anthropic", Key: "sk-sp-k", AccessKeyID: "AK-half"},
		// pay-as-you-go reads nothing with an access key
		{ID: "qwen-payg", Name: "Qwen", Preset: "qwen", Chat: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1", Key: "sk-k"},
		{ID: "ali-off", Name: "Alibaba off", Preset: "alibaba-token-plan", Chat: aliPlan, Key: "sk-sp-k", Off: true},
		{ID: "tencent-cloud", Name: "Tencent Cloud", Preset: "tencent-cloud", Chat: "https://tokenhub-intl.tencentmaas.com/plan/v3", Key: "sk-tp-1"},
		// TokenHub's China host is no plan magpie reads: no card asks it
		// for an access key
		{ID: "tc-gz", Name: "Tencent China", Preset: "tencent-cloud", Chat: "https://tokenhub.tencentmaas.com/plan/v3", Key: "sk-tp-2"},
		// a custom provider at a plan's address is the user's own: no card
		// asks it for an access key either (ysm119)
		{ID: "my-intl", Name: "My TokenHub", Chat: "https://tokenhub-intl.tencentmaas.com/plan/v3", Key: "sk-tp-3"},
		{ID: "my-ali", Name: "My Alibaba", Chat: aliPlan, Key: "sk-sp-3"},
	} {
		if err := Save(p); err != nil {
			t.Fatal(err)
		}
	}
	got := map[string]SubscriptionQuota{}
	for _, q := range PlanQuotas(context.Background()) {
		if q.NeedsAccessKey {
			if _, dup := got[q.Provider]; dup {
				t.Errorf("two cards for %s", q.Provider)
			}
			got[q.Provider] = q
		}
	}
	ali, okA := got["alibaba-token-plan"]
	if !okA {
		ali, okA = got["ali-copy"]
	}
	tc, okT := got["tencent-cloud"]
	if len(got) != 2 || !okA || !okT {
		t.Fatalf("cards: %+v", got)
	}
	if !strings.Contains(ali.Error, "AccessKey ID and Secret") || ali.Plan == "" || ali.Windows == nil {
		t.Errorf("alibaba: %+v", ali)
	}
	if !strings.Contains(tc.Error, "SecretId and SecretKey") || tc.Plan != tcPlanName {
		t.Errorf("tencent: %+v", tc)
	}

	// another provider on the plan with the key: its card, not this one
	keyedIn.Store(true)
	if err := Save(Provider{ID: "ali-keyed", Name: "Alibaba keyed", Preset: "alibaba-token-plan", Chat: aliPlan, Key: "sk-sp-k2", AccessKeyID: "AK1", SecretAccessKey: "SK1"}); err != nil {
		t.Fatal(err)
	}
	forgetPlanQuotas()
	var cards []string
	for _, q := range aliPlanQuotas(context.Background()) {
		cards = append(cards, q.Provider)
		if q.NeedsAccessKey {
			t.Errorf("a keyless card beside the keyed one: %+v", q)
		}
	}
	if len(cards) != 1 || cards[0] != "ali-keyed" {
		t.Errorf("cards: %v", cards)
	}
}
