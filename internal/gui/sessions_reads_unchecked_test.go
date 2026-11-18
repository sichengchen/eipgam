package gui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/sessions"
	"github.com/yetone/magpie/internal/settings"
)

// baysonfox on X, again: magpie was installed after Antigravity 2.0, so the
// first choice ("allow selected") listed the app and it was left off beside
// agy CLI. Named off, it was never "new", so neither the Sessions page nor
// Usage asked again, and the app's calls made without magpie were counted
// nowhere while agy's were. An agent left off in that first choice is asked
// about once more, on both pages; Show/Count reads it and its calls come
// into Usage › Requests; an agent turned off on its own (Keep off,
// Settings › Privacy) is kept off and not asked again. The one left off is
// Claude Code, its calls those of sessions' testdata/claude.
func TestSessionReadsUncheckedAgentAskedAgain(t *testing.T) {
	for _, allow := range []bool{true, false} {
		t.Run(map[bool]string{true: "count", false: "keep off"}[allow], func(t *testing.T) {
			h := sandboxHome(t)
			copyTree(t, "../sessions/testdata/agy-sessions", filepath.Join(h, ".gemini", "antigravity-cli"))
			copyTree(t, "../sessions/testdata/claude", filepath.Join(h, ".claude"))
			t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(h, ".claude"))
			if err := settings.SetSessionReads(func(*settings.SessionReads) *settings.SessionReads { return &settings.SessionReads{} }); err != nil {
				t.Fatal(err)
			}
			sessions.Reset()
			t.Cleanup(sessions.Reset)
			mux := http.NewServeMux()
			sessionManageRoutes(mux, folderOnly{})
			usageRoutes(mux, folderOnly{})
			call := func(method, path, body string) []byte {
				t.Helper()
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
				if w.Code != 200 {
					t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body)
				}
				return w.Body.Bytes()
			}
			app := func(r readsJSON) readAgentJSON {
				t.Helper()
				for _, f := range r.Found {
					if f.Agent == "claude" {
						return f
					}
				}
				t.Fatalf("claude not found: %+v", r.Found)
				return readAgentJSON{}
			}
			appRows := func() int {
				t.Helper()
				var page ledgerJSON
				if err := json.Unmarshal(call("GET", "/api/usage/requests?period=all", ""), &page); err != nil {
					t.Fatal(err)
				}
				n := 0
				for _, row := range page.Rows {
					if row.Agent == "claude" {
						n++
					}
				}
				return n
			}
			// the first choice, as the Sessions page's consent posts allow
			// selected: every agent found named, Claude Code left off
			var r readsJSON
			json.Unmarshal(call("POST", "/api/sessions/reads", `{"all":false,"agents":{"agy":true,"claude":false}}`), &r)
			if a := app(r); a.On || !a.New {
				t.Fatalf("claude left off in the first choice = %+v, want off and asked about again", a)
			}
			if n := appRows(); n != 0 {
				t.Fatalf("%d of claude's calls counted before it was allowed", n)
			}
			body := `{"agents":{"claude":false}}`
			if allow {
				body = `{"agents":{"claude":true}}`
			}
			r = readsJSON{}
			json.Unmarshal(call("POST", "/api/sessions/reads", body), &r)
			if a := app(r); a.New || a.On != allow {
				t.Fatalf("after the answer: %+v", a)
			}
			if s := settings.Shared().SessionReads; s == nil || s.All || !s.Agents["agy"] {
				t.Fatalf("the answer changed the rest of the choice: %+v", s)
			}
			// the session's model calls, read from its file
			if n := appRows(); allow && n == 0 || !allow && n != 0 {
				t.Fatalf("allowed %v: %d of claude's calls in Usage › Requests", allow, n)
			}
			// asked once: a later read still says the same
			r = readsJSON{}
			json.Unmarshal(call("GET", "/api/sessions/reads", ""), &r)
			if a := app(r); a.New {
				t.Fatalf("asked about again after the answer: %+v", a)
			}
		})
	}
}

// An agent turned off on its own, in Settings › Privacy or by Keep off, is
// the user's answer: it is not asked about again, under either first choice.
func TestSessionReadsTurnedOffNotAskedAgain(t *testing.T) {
	for _, all := range []bool{true, false} {
		h := sandboxHome(t)
		copyTree(t, "../sessions/testdata/claude", filepath.Join(h, ".claude"))
		t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(h, ".claude"))
		if err := settings.SetSessionReads(func(*settings.SessionReads) *settings.SessionReads { return &settings.SessionReads{} }); err != nil {
			t.Fatal(err)
		}
		on := true
		if err := sessions.SetReads(&all, map[string]bool{"claude": on}); err != nil {
			t.Fatal(err)
		}
		if err := sessions.SetReads(nil, map[string]bool{"claude": false}); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, f := range readsNow().Found {
			if f.Agent != "claude" {
				continue
			}
			found = true
			if f.On || f.New {
				t.Fatalf("all %v, turned off in Settings: %+v", all, f)
			}
		}
		if !found {
			t.Fatalf("all %v: claude not found", all)
		}
	}
}
