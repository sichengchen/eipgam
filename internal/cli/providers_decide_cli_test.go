package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// A System One API of another vendor is added with decide= (#647).
func TestProviderDecidePairs(t *testing.T) {
	c := provider.Provider{Name: "My Decider"}
	if err := applyPairs(&c, []string{"decide=https://decide.example.com/v1", "models=my-decision-model", "key=sk-1"}); err != nil {
		t.Fatal(err)
	}
	if !c.DecideOnly() || c.Jev() != "my-decision-model" || !c.DecidesModel("my-decision-model") {
		t.Fatalf("custom: %+v asked as %s", c, c.Jev())
	}
}

func TestRemoteMagpieAddReportsModels(t *testing.T) {
	for _, tc := range []struct{ name, list, want string }{
		{"unreachable", "", "no models exposed yet"},
		{"chat only", `{"data":[{"id":"lib/m1"}]}`, "1 models in the catalog"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groupsHome(t)
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, tc.list)
			}))
			t.Cleanup(up.Close)
			if tc.list == "" {
				up.Close()
			}
			out, err := stdoutOf(t, func() error {
				return providerCmd([]string{"provider", "add", "remote-magpie", "remote-key", "url=" + up.URL, "id=office"})
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, tc.want) || strings.Contains(out, "classifier=office/") {
				t.Errorf("remote add said %q; want %q and no empty classifier", out, tc.want)
			}
			if tc.list == "" && !strings.Contains(out, "refused") {
				t.Errorf("remote list error hidden: %q", out)
			}
		})
	}
}
