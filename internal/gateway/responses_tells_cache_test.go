package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// The usage bodies sparrowbits got from ModelScope's API-Inference for the
// same ~12K-token prompt asked twice (#1578): Responses says 12288 of it
// was read from the cache, Chat says nothing of the cache at all.
const (
	modelScopeResponsesUsage = `{"input_tokens": 12396, "input_tokens_details": {"cached_tokens": 12288}, "output_tokens": 32, "total_tokens": 12428}`
	modelScopeChatUsage      = `{"prompt_tokens": 12364, "completion_tokens": 293, "total_tokens": 12657}`
)

// fakeModelScope answers on Chat and Responses with the reporter's usage,
// streamed when asked, and records each request's path. A model in
// notOnResponses is answered 404 on Responses.
func fakeModelScope(t *testing.T, chatUsage string, notOnResponses ...string) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		var body struct{ Model string }
		json.Unmarshal(b, &body)
		stream := streamOf(b)
		switch {
		case r.URL.Path == "/v1/responses" && strings.Contains(strings.Join(notOnResponses, "\x00"), body.Model):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(404)
			io.WriteString(w, `{"error":{"message":"Not Found"}}`)
		case r.URL.Path == "/v1/responses" && stream:
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"i\",\"output_index\":0,\"content_index\":0,\"delta\":\"ok\"}\n\n"+
				"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"r\",\"object\":\"response\",\"status\":\"completed\",\"model\":\"x\",\"output\":[{\"type\":\"message\",\"id\":\"i\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"ok\"}]}],\"usage\":"+modelScopeResponsesUsage+"}}\n\n")
		case r.URL.Path == "/v1/responses":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"id":"r","object":"response","status":"completed","model":"x","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":`+modelScopeResponsesUsage+`}`)
		case r.URL.Path == "/v1/chat/completions" && stream:
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, "data: {\"id\":\"c\",\"object\":\"chat.completion.chunk\",\"model\":\"x\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"}}]}\n\n"+
				"data: {\"id\":\"c\",\"object\":\"chat.completion.chunk\",\"model\":\"x\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":"+chatUsage+"}\n\ndata: [DONE]\n\n")
		case r.URL.Path == "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"id":"c","object":"chat.completion","model":"x","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":`+chatUsage+`}`)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		p := paths
		paths = nil
		return p
	}
}

// messagesUsage is the usage a Messages reply gave the client, whole or
// streamed: the last value of each field the stream named.
func messagesUsage(t *testing.T, reply string) map[string]int {
	t.Helper()
	out := map[string]int{}
	take := func(u map[string]any) {
		for k, v := range u {
			if n, ok := v.(float64); ok {
				out[k] = int(n)
			}
		}
	}
	var whole struct {
		Usage map[string]any `json:"usage"`
	}
	if json.Unmarshal([]byte(reply), &whole) == nil && whole.Usage != nil {
		take(whole.Usage)
		return out
	}
	for _, line := range strings.Split(reply, "\n") {
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}
		var ev struct {
			Usage   map[string]any `json:"usage"`
			Message struct {
				Usage map[string]any `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			continue
		}
		take(ev.Message.Usage)
		take(ev.Usage)
	}
	return out
}

// A provider with Chat and Responses that isn't ModelScope keeps Chat for a
// translated request (nothing is known of its Responses API), and a Chat
// API that tells its cache in prompt_tokens_details.cached_tokens reaches a
// Messages client as cache_read_input_tokens.
func TestOtherChatAndResponsesKeepChat(t *testing.T) {
	srv, seen := fakeModelScope(t, `{"prompt_tokens": 12364, "completion_tokens": 293, "total_tokens": 12657, "prompt_tokens_details": {"cached_tokens": 12288}}`)
	for _, stream := range []string{"true", "false"} {
		fresh(t)
		if err := provider.Save(provider.Provider{ID: "relay", Name: "Relay", Key: "k",
			Chat: srv.URL + "/v1", Responses: srv.URL + "/v1", Models: []string{"glm-5"}}); err != nil {
			t.Fatal(err)
		}
		seen()
		code, reply := post(t, "/v1/messages", `{"model":"relay/glm-5","max_tokens":100,"stream":`+stream+`,"messages":[{"role":"user","content":"hi"}]}`)
		if code != 200 {
			t.Fatalf("%d %s", code, reply)
		}
		if paths := seen(); len(paths) != 1 || paths[0] != "/v1/chat/completions" {
			t.Fatalf("stream=%s: went to %v, want Chat", stream, paths)
		}
		if u := messagesUsage(t, reply); u["cache_read_input_tokens"] != 12288 || u["input_tokens"] != 76 {
			t.Errorf("stream=%s: usage %v, want 12288 read from cache, 76 input", stream, u)
		}
	}
}
