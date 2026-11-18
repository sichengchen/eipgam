package pii

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/settings"
)

// The model is downloaded through the proxy set in magpie, as magpie's
// other requests are (hh0592821 on Discord: a client made before
// netproxy.Install went direct, and in China huggingface.co answered with
// a *.facebook.com certificate while their proxy got the file).
func TestFetchGoesThroughMagpiesProxy(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, ".config"))
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy"} {
		t.Setenv(k, "")
	}
	body := bytes.Repeat([]byte("model bytes "), 1000)
	var mu sync.Mutex
	var seen []string
	// the proxy: a plain http:// request comes to it whole, by its URL
	px := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL.String())
		mu.Unlock()
		w.Write(body)
	}))
	defer px.Close()
	if err := settings.Save(settings.Settings{Proxy: px.URL}); err != nil {
		t.Fatal(err)
	}
	// TEST-NET-1: nothing answers there but the proxy, and no name is
	// looked up for it
	raw := "http://192.0.2.1/QuantumNous/astrlink-guard/resolve/r/int8/tokenizer.json"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "tokenizer.json")
	var n atomic.Int64
	if err := fetch(ctx, raw, path, int64(len(body)), sum(body), &n); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(seen, []string{raw}) {
		t.Fatalf("through the proxy: %v, want %s", seen, raw)
	}
}
