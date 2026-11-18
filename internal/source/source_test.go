package source

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/testenv"
)

func TestMain(m *testing.M) {
	os.Exit(testenv.Run(m))
}

// down answers every request 502, as an official address that is down,
// and keeps each address asked.
type down struct{ asked []string }

func (d *down) RoundTrip(req *http.Request) (*http.Response, error) {
	d.asked = append(d.asked, req.URL.String())
	return &http.Response{
		StatusCode: http.StatusBadGateway,
		Status:     "502 Bad Gateway",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}, nil
}

// magpie asks no mirror of its own: npm's registry, Bun's releases, the
// community plugin list, GitHub's API and the masking models' files are
// asked of their official addresses alone, and an official address that
// fails gives its own answer back. MAGPIE_MIRRORS, which once turned on a
// fallback to npmmirror and gh-proxy, does nothing.
func TestDoAsksTheOfficialAddressAlone(t *testing.T) {
	t.Setenv("MAGPIE_MIRRORS", "on")
	for _, raw := range []string{
		"https://registry.npmjs.org/@magpie-community%2fopencode-kiro-auth",
		"https://github.com/oven-sh/bun/releases/download/bun-v1.4.2/bun-darwin-aarch64.zip",
		"https://raw.githubusercontent.com/magpie-community/plugins/main/registry.json",
		"https://api.github.com/repos/oven-sh/bun/releases/latest",
		"https://huggingface.co/QuantumNous/astrlink-guard/resolve/7216b6a/int8/tokenizer.json",
	} {
		d := &down{}
		req, _ := http.NewRequest("GET", raw, nil)
		resp, err := Do(&http.Client{Transport: d}, req)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadGateway || len(d.asked) != 1 || d.asked[0] != raw {
			t.Errorf("%s: answered %d after asking %q; want the official address alone, its 502 given back", raw, resp.StatusCode, d.asked)
		}
	}
}
