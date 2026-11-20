package gateway

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/sichengchen/eipgam/internal/provider"
	"github.com/sichengchen/eipgam/internal/settings"
)

// Cheapest first is a group's: a provider saved with it keeps Smart, and
// a group keeps it.
func TestCheapestFirstIsAGroupsOnly(t *testing.T) {
	fresh(t)
	if err := provider.Save(provider.Provider{ID: "rl", Name: "Relay", Key: "k", Chat: "https://relay.example/v1", Models: []string{"m"}, Routing: provider.Cheapest}); err != nil {
		t.Fatal(err)
	}
	if p, err := provider.Find("rl"); err != nil || p.Routing != "" {
		t.Fatalf("a provider kept routing cheap, want Smart: %v %+v", err, p)
	}
	if err := provider.SaveGroup(provider.Group{Name: "C", Members: []string{"rl/m"}, Routing: provider.Cheapest}); err != nil {
		t.Fatal(err)
	}
	gs := provider.Groups()
	if len(gs) != 1 || gs[0].Routing != provider.Cheapest {
		t.Fatalf("the group didn't keep Cheapest first: %+v", gs)
	}
}

// Ollama Cloud bills deepseek-v4.1-flash at half outside 12:00 to 18:00
// UTC on weekdays and all of Saturday and Sunday (hayden4608 on Discord,
// https://ollama.com/pricing): $0.15/$0.60 then, $0.30/$1.20 in its peak
// hours. A relay at one $0.22/$0.88 goes behind Ollama Cloud off-peak and
// ahead of it at peak, at each edge a minute either side and on a
// Saturday, on the clock of a far zone too, and the relay's price is the
// same at every hour.
func TestCheapestFirstGroupFollowsOllamaOffPeak(t *testing.T) {
	fresh(t)
	up := httptest.NewServer(http.HandlerFunc(streamed))
	t.Cleanup(up.Close)
	to, _ := url.Parse(up.URL)
	for _, p := range []provider.Provider{
		{ID: "ollama-cloud", Preset: "ollama-cloud", Name: "Ollama Cloud", Key: "sk-oc", Chat: "https://ollama.com/v1", Models: []string{"deepseek-v4.1-flash"}},
		{ID: "rl", Name: "Relay", Key: "sk-rl", Chat: "https://relay.example/v1", Models: []string{"deepseek-v4.1-flash"}},
	} {
		if err := provider.Save(p); err != nil {
			t.Fatal(err)
		}
	}
	f := func(v float64) *float64 { return &v }
	if err := settings.Save(settings.Settings{ModelPrices: map[string]settings.ModelPrice{
		"ollama-cloud/deepseek-v4.1-flash": {Input: f(0.15), Output: f(0.60), CacheRead: f(0.003), CacheWrite: f(0)},
		"rl/deepseek-v4.1-flash":           {Input: f(0.22), Output: f(0.88), CacheRead: f(0.01), CacheWrite: f(0)},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := provider.SaveGroup(provider.Group{
		Name: "O", Members: []string{"rl/deepseek-v4.1-flash", "ollama-cloud/deepseek-v4.1-flash"}, Routing: provider.Cheapest,
	}); err != nil {
		t.Fatal(err)
	}
	s := New()
	s.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme, r.URL.Host = to.Scheme, to.Host
		return http.DefaultTransport.RoundTrip(r)
	})}
	ask := func(at time.Time) *Route {
		t.Helper()
		old := priceClock
		priceClock = func() time.Time { return at }
		defer func() { priceClock = old }()
		code, body := postAs(t, s, "", `{"model":"group/o","stream":true,"messages":[{"role":"user","content":"x"}]}`)
		if code != 200 {
			t.Fatalf("code %d: %s", code, body)
		}
		return s.trace.routes[len(s.trace.routes)-1]
	}
	for _, c := range []struct {
		at   time.Time
		peak bool
	}{
		{time.Date(2026, 10, 28, 11, 59, 0, 0, time.UTC), false}, // a Wednesday
		{time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC), true},
		{time.Date(2026, 10, 28, 17, 59, 0, 0, time.UTC), true},
		{time.Date(2026, 10, 28, 18, 0, 0, 0, time.UTC), false},
		{time.Date(2026, 10, 31, 14, 0, 0, 0, time.UTC), false}, // a Saturday
		// 12:00 UTC on the Wednesday written in UTC+14 (Thursday 02:00
		// there), and 17:59 UTC in UTC-11 (Wednesday 06:59)
		{time.Date(2026, 10, 29, 2, 0, 0, 0, time.FixedZone("+14", 14*3600)), true},
		{time.Date(2026, 10, 28, 6, 59, 0, 0, time.FixedZone("-11", -11*3600)), true},
		// Saturday 13:00 UTC, which is Sunday 03:00 in UTC+14
		{time.Date(2026, 11, 1, 3, 0, 0, 0, time.FixedZone("+14", 14*3600)), false},
	} {
		r := ask(c.at)
		want := "ollama-cloud"
		if c.peak {
			want = "rl"
		}
		if r.Order[0].Provider != want {
			t.Fatalf("%v (%v UTC): Cheapest first went to %q first, want %q: %+v", c.at, c.at.UTC(), r.Order[0].Provider, want, r.Order)
		}
		for _, w := range r.Order {
			if w.Price == nil {
				t.Fatalf("%v: %s has no price: %+v", c.at, w.Provider, w)
			}
			switch w.Provider {
			case "ollama-cloud":
				in, out := 0.15, 0.60
				if c.peak {
					in, out = 0.30, 1.20
				}
				if w.Price.Input != in || w.Price.Output != out || w.Price.Peak != c.peak {
					t.Fatalf("%v: Ollama Cloud's row %+v, want %v/%v peak %v", c.at, w.Price, in, out, c.peak)
				}
			case "rl":
				if w.Price.Input != 0.22 || w.Price.Output != 0.88 || w.Price.Peak {
					t.Fatalf("%v: the relay's row %+v, want its one price", c.at, w.Price)
				}
			}
		}
	}
}
