package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sichengchen/eipgam/internal/catalog"
	"github.com/sichengchen/eipgam/internal/provider"
)

// ownOutputUpstream answers every API eipgam may ask on, whole, and keeps
// each body it was sent. A Chat body naming max_tokens past refuseOver is
// turned away as a vendor whose limit is lower than its list says does.
type ownOutputUpstream struct {
	mu         sync.Mutex
	got        []map[string]any
	refuseOver int
}

func (u *ownOutputUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	var q map[string]any
	json.Unmarshal(b, &q)
	u.mu.Lock()
	u.got = append(u.got, q)
	refuse := u.refuseOver
	u.mu.Unlock()
	stream := q["stream"] == true
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	switch {
	case strings.HasSuffix(r.URL.Path, "/responses") && stream:
		io.WriteString(w, "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"r\",\"status\":\"in_progress\",\"output\":[]}}\n\n"+
			"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"output_index\":0,\"content_index\":0,\"delta\":\"ok\"}\n\n"+
			"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"r\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"id\":\"o\",\"role\":\"assistant\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"text\":\"ok\",\"annotations\":[]}]}],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n")
	case strings.HasSuffix(r.URL.Path, "/responses"):
		io.WriteString(w, `{"id":"r","object":"response","status":"completed","model":"m","output":[{"type":"message","id":"o","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok","annotations":[]}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`)
	default:
		if n, _ := q["max_tokens"].(float64); refuse > 0 && int(n) > refuse {
			w.WriteHeader(400)
			io.WriteString(w, `{"error":{"message":"Invalid max_tokens value, the valid range of max_tokens is [1, 32768]","type":"invalid_request_error"}}`)
			return
		}
		if stream {
			io.WriteString(w, "data: {\"id\":\"c\",\"object\":\"chat.completion.chunk\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":null}]}\n\n"+
				"data: {\"id\":\"c\",\"object\":\"chat.completion.chunk\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\n"+
				"data: [DONE]\n\n")
			return
		}
		io.WriteString(w, `{"id":"c","object":"chat.completion","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}
}

func (u *ownOutputUpstream) take() []map[string]any {
	u.mu.Lock()
	defer u.mu.Unlock()
	got := u.got
	u.got = nil
	return got
}

// Translated onto another vendor's Chat or Responses, a request naming no
// reply limit is sent its model's own, as on Anthropic's (#1792): the
// client's API reads none as the model's whole, which the vendor's needn't.
// A vendor that turns the figure away (a list saying more than it takes) is
// asked again naming none, as the client did, and from then on.
func TestTranslatedAsksTheModelsOutput(t *testing.T) {
	fresh(t)
	u := &ownOutputUpstream{}
	up := httptest.NewServer(u)
	t.Cleanup(up.Close)
	os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o755)
	if err := os.WriteFile(catalog.CachePath(), []byte(`{"kimi":{"id":"kimi","models":{
		"k-big":{"id":"k-big","limit":{"context":262144,"output":65536}}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)
	// a vendor serving Chat alone: Codex's turn is translated
	if err := provider.Save(provider.Provider{ID: "v", Name: "V", Key: "k", Catalog: "kimi", Models: []string{"k-big"}, Chat: up.URL + "/v1"}); err != nil {
		t.Fatal(err)
	}
	s := New()
	codex := `{"model":"v/k-big","input":"hi"}`
	ask := func() []map[string]any {
		t.Helper()
		if code, raw := postJSON(t, s, "/v1/responses", codex); code != 200 {
			t.Fatalf("%d %s", code, raw)
		}
		return u.take()
	}
	got := ask()
	if len(got) != 1 || got[0]["max_tokens"] != float64(65536) {
		t.Fatalf("translated onto Chat: %v, want max_tokens 65536", got)
	}
	if code, _ := postJSON(t, s, "/v1/responses", `{"model":"v/k-big","input":"hi","max_output_tokens":700}`); code != 200 {
		t.Fatal(code)
	}
	if got := u.take(); len(got) != 1 || got[0]["max_tokens"] != float64(700) {
		t.Fatalf("the client's own limit: %v, want max_tokens 700", got)
	}

	u.mu.Lock()
	u.refuseOver = 32768
	u.mu.Unlock()
	got = ask()
	if len(got) != 2 || got[0]["max_tokens"] != float64(65536) {
		t.Fatalf("refused: %v, want 65536 then none", got)
	}
	if _, ok := got[1]["max_tokens"]; ok {
		t.Fatalf("asked again with a limit: %v", got[1])
	}
	got = ask()
	if len(got) != 1 {
		t.Fatalf("after a refusal: %d requests, want 1 naming none", len(got))
	}
	if _, ok := got[0]["max_tokens"]; ok {
		t.Fatalf("filled in again after the vendor turned it away: %v", got[0])
	}
}
