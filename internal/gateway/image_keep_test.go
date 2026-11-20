package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sichengchen/eipgam/internal/provider"
	"github.com/sichengchen/eipgam/internal/sessions"
)

// dsBigPNG is a picture's base64 of about n bytes, unlike any other of
// the same n by its tag.
func dsBigPNG(n int, tag string) string {
	raw := make([]byte, n/4*3)
	copy(raw, "\x89PNG\r\n\x1a\n"+tag)
	for i := 8 + len(tag); i < len(raw); i++ {
		raw[i] = byte(i * 7)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// picturesUp is a vendor that takes any size of body on Chat, Responses
// and Anthropic messages, and keeps the last one each path was sent.
type picturesUp struct {
	mu   sync.Mutex
	last map[string]string
}

func (u *picturesUp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	u.mu.Lock()
	u.last[r.URL.Path] = string(b)
	u.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.HasSuffix(r.URL.Path, "/chat/completions"):
		io.WriteString(w, chatWholeOK)
	case strings.HasSuffix(r.URL.Path, "/responses"):
		io.WriteString(w, `{"id":"resp_1","object":"response","status":"completed","model":"m1","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok","annotations":[]}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`)
	default:
		io.WriteString(w, anthropicWholeOK)
	}
}

func (u *picturesUp) sent(path string) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.last[path]
}

// picturesBody is a request with one picture on each API, the path it is
// asked at and the path its vendor is sent it at.
func picturesBody(api, model, img string) (path, upPath, body string) {
	switch api {
	case "chat":
		return "/v1/chat/completions", "/v1/chat/completions", `{"model":"` + model + `","stream":false,"messages":[{"role":"user","content":[{"type":"text","text":"what is it"},{"type":"image_url","image_url":{"url":"data:image/png;base64,` + img + `"}}]}]}`
	case "responses":
		return "/v1/responses", "/v1/responses", `{"model":"` + model + `","stream":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"what is it"},{"type":"input_image","image_url":"data:image/png;base64,` + img + `","detail":"auto"}]}]}`
	}
	return "/v1/messages", "/anthropic/v1/messages", `{"model":"` + model + `","max_tokens":100,"stream":false,"messages":[{"role":"user","content":[{"type":"text","text":"what is it"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"` + img + `"},"cache_control":{"type":"ephemeral"}}]}]}`
}

// A conversation of pictures grows past what the views keep of a body;
// its pictures and text still show in the Routing card, from memory and
// from the archive, and its turns on the Sessions page (matrix0487 on
// Discord).
func TestPictureConversationStillShows(t *testing.T) {
	for _, api := range []string{"chat", "responses", "anthropic"} {
		t.Run(api, func(t *testing.T) {
			fresh(t)
			u := &picturesUp{last: map[string]string{}}
			up := httptest.NewServer(u)
			t.Cleanup(up.Close)
			if err := provider.Save(provider.Provider{ID: "relay", Name: "Relay", Key: "k1", Models: []string{"m1"},
				Chat: up.URL + "/v1", Responses: up.URL + "/v1", Anthropic: up.URL + "/anthropic"}); err != nil {
				t.Fatal(err)
			}
			s := New()
			if err := sessions.SetGatewayRecording(true, false); err != nil {
				t.Fatal(err)
			}
			// three pictures, the body over promptBodyMax
			imgs := []string{dsBigPNG(12<<20, "a"), dsBigPNG(12<<20, "b"), dsBigPNG(12<<20, "c")}
			path, upPath, body := picturesBody(api, "relay/m1", imgs[0])
			_, _, one := picturesBody(api, "relay/m1", "IMG")
			for _, im := range imgs[1:] {
				// the same turn with the next picture too
				body = strings.Replace(body, `"what is it"}`, `"what is it"}`+between(one, im), 1)
			}
			if len(body) <= promptBodyMax || len(body) <= conversationBodyMax {
				t.Fatalf("body only %d", len(body))
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", path, strings.NewReader(body))
			req.Header.Set(SessionHeader, "pics-1")
			req.Header.Set("User-Agent", "opencode/1.0")
			s.Handler().ServeHTTP(rec, req)
			if rec.Code != 200 {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			if got := u.sent(upPath); !strings.Contains(got, imgs[2][:64]) {
				t.Fatalf("the vendor wasn't sent the pictures: %.300s", got)
			}
			rs := s.Trace(context.Background(), 0, 0).Routes
			if len(rs) == 0 {
				t.Fatal("no route")
			}
			id := rs[0].ID
			var pt PromptText
			for range 200 { // the prompt is read beside the request
				var ok bool
				if pt, ok = s.PromptText(id, PartFiles, true); ok {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			picturesDrawn(t, "memory", pt, imgs)
			chat, ok := s.PromptText(id, PartChat, true)
			if b, _ := json.Marshal(chat); !ok || !strings.Contains(string(b), "what is it") {
				t.Errorf("memory: no text: %v %.300s", ok, b)
			}
			from := map[string]provider.Protocol{"chat": provider.Chat, "responses": provider.Responses, "anthropic": provider.Anthropic}[api]
			arch, ok := ArchivedPromptText(from, []byte(body), id+1000, PartFiles, true, 0)
			if !ok {
				t.Fatal("archive: not read")
			}
			picturesDrawn(t, "archive", arch, imgs)
			tr, err := sessions.GatewayTranscript("opencode", "pics-1")
			if err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(tr)
			if !strings.Contains(string(b), "what is it") || strings.Count(string(b), "[image]") != 3 || strings.Contains(string(b), imgs[0][:64]) {
				t.Errorf("sessions: %.600s", b)
			}
		})
	}
}

// between is what one's single turn holds after its text: the picture's
// block, put in again with im, comma first.
func between(one, im string) string {
	_, rest, _ := strings.Cut(one, `"what is it"}`)
	block, _, _ := strings.Cut(rest, `]}]`)
	return strings.Replace(block, "IMG", im, 1)
}

func picturesDrawn(t *testing.T, from string, pt PromptText, imgs []string) {
	t.Helper()
	var srcs []string
	for _, it := range pt.Items {
		for _, pc := range it.Pieces {
			if pc.Src != "" {
				srcs = append(srcs, pc.Src)
			}
		}
	}
	if len(srcs) != len(imgs) {
		t.Fatalf("%s: %d pictures drawn, want %d", from, len(srcs), len(imgs))
	}
	for _, im := range imgs {
		found := false
		for _, s := range srcs {
			found = found || s == "data:image/png;base64,"+im
		}
		if !found {
			t.Errorf("%s: picture %.20s… not drawn", from, im[8:])
		}
	}
}

// leanImages swaps only a picture's base64, and keeps a body without one
// as it is.
func TestLeanImages(t *testing.T) {
	big := dsBigPNG(4<<10, "x")
	plain := `{"messages":[{"role":"user","content":"` + strings.Repeat("hello ", 400) + `"}]}`
	if got := leanImages([]byte(plain), true); string(got) != plain {
		t.Errorf("plain changed")
	}
	sig := strings.Repeat("QUJD", 600) // base64, not a picture
	esc := `data:image/png;base64,` + big[:20] + `\/` + big[22:]
	in := `{"a":"data:image/png;base64,` + big + `","b":{"type":"base64","data":"` + big + `"},"s":"` + sig + `","e":"` + esc + `","small":"data:image/png;base64,iVBORw0KGgo="}`
	got := string(leanImages([]byte(in), true))
	var v map[string]any
	if err := json.Unmarshal([]byte(got), &v); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	a, _ := v["a"].(string)
	ref, ok := strings.CutPrefix(a, "data:image/png;base64,"+imageRef)
	if !ok {
		t.Fatalf("a: %.80s", a)
	}
	if b64, ok := keptImageOf(ref); !ok || b64 != big {
		t.Errorf("a not kept")
	}
	if d := v["b"].(map[string]any)["data"]; d != imageRef+ref {
		t.Errorf("b: %.80v, want the same ref", d)
	}
	if v["s"] != sig || v["e"] != strings.ReplaceAll(esc, `\/`, "/") || v["small"] != "data:image/png;base64,iVBORw0KGgo=" {
		t.Errorf("swapped what isn't a picture's base64: %.300s", got)
	}
	if dataImage("data:image/png;base64,"+imageRef+"999999") != "" {
		t.Error("a gone ref drawn")
	}
}
