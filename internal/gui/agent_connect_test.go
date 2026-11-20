package gui

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sichengchen/eipgam/internal/provider"
)

// The Agents page's switch: connecting Codex puts it on eipgam with the
// whole catalog in its own model list — the model it was on, served by
// eipgam, when eipgam has it — and switching it off puts back what it had.
func TestAgentConnectAPI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
	cfg := filepath.Join(home, ".codex", "config.toml")
	os.WriteFile(cfg, []byte("model = \"m2\"\n"), 0o600)
	if err := provider.Save(provider.Provider{ID: "relay", Name: "Relay", Key: "k", Chat: "http://127.0.0.1:1/v1", Models: []string{"m1", "m2"}}); err != nil {
		t.Fatal(err)
	}
	h := Handler(nil, nil)
	call := func(path, body string) stateJSON {
		t.Helper()
		method := "POST"
		if body == "" {
			method = "GET"
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		if rec.Code != 200 {
			t.Fatalf("%s %d %s", path, rec.Code, rec.Body)
		}
		var s stateJSON
		json.Unmarshal(rec.Body.Bytes(), &s)
		return s
	}
	codex := func(s stateJSON) agentJSON {
		for _, a := range s.Agents {
			if a.ID == "codex" {
				return a
			}
		}
		t.Fatalf("no codex in %+v", s.Agents)
		return agentJSON{}
	}
	a := codex(call("/api/agents/connect/codex", "{}"))
	if !a.Wired || a.Fields[0].Value != "relay/m2" {
		t.Fatalf("after connect: wired %v, model %q", a.Wired, a.Fields[0].Value)
	}
	cat, err := os.ReadFile(filepath.Join(home, ".codex", "eipgam-models.json"))
	if err != nil || !strings.Contains(string(cat), "relay/m1") || !strings.Contains(string(cat), "relay/m2") {
		t.Fatalf("Codex's model list: %s %v", cat, err)
	}
	// connecting again changes nothing: the user's pick in the agent stays
	call("/api/set", `{"agent":"codex","field":"model","value":"relay/m1"}`)
	if a := codex(call("/api/agents/connect/codex", "{}")); a.Fields[0].Value != "relay/m1" {
		t.Fatalf("connect again moved the model to %q", a.Fields[0].Value)
	}
	a = codex(call("/api/agents/disconnect/codex", "{}"))
	if a.Wired || a.Fields[0].Value != "m2" {
		t.Fatalf("after disconnect: wired %v, model %q", a.Wired, a.Fields[0].Value)
	}
}

// A Claude Code config folder beside ~/.claude is a row of its own,
// claude#<name> (as omp's profiles were omp#<name>), and the Agents page asks
// for it with the # encoded: left raw, a browser reads #work as the
// fragment and the request reaches the claude row, whose settings.json then
// changes (#1187). Connecting and disconnecting the encoded id touches
// that folder's files alone.
func TestClaudeDirConnectURL(t *testing.T) {
	home := sandboxHome(t)
	def := filepath.Join(home, ".claude", "settings.json")
	os.MkdirAll(filepath.Dir(def), 0o755)
	os.WriteFile(def, []byte(`{"model":"opus"}`), 0o600)
	work := filepath.Join(home, ".claude-work", "settings.json")
	os.MkdirAll(filepath.Dir(work), 0o755)
	os.WriteFile(work, []byte(`{"model":"sonnet"}`), 0o600)
	if err := provider.Save(provider.Provider{ID: "relay", Name: "Relay", Key: "k", Chat: "http://127.0.0.1:1/v1", Models: []string{"m1"}}); err != nil {
		t.Fatal(err)
	}
	h := Handler(nil, nil)
	call := func(path string) stateJSON {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("POST", path, strings.NewReader("{}")))
		if rec.Code != 200 {
			t.Fatalf("%s %d %s", path, rec.Code, rec.Body)
		}
		var s stateJSON
		json.Unmarshal(rec.Body.Bytes(), &s)
		return s
	}
	row := func(s stateJSON, id string) agentJSON {
		for _, a := range s.Agents {
			if a.ID == id {
				return a
			}
		}
		t.Fatalf("no %s", id)
		return agentJSON{}
	}
	before, err := os.ReadFile(def)
	if err != nil {
		t.Fatal(err)
	}
	a := row(call("/api/agents/connect/claude%23work"), "claude#work")
	if !a.Wired {
		t.Fatal("claude#work isn't connected")
	}
	got, err := os.ReadFile(def)
	if err != nil || string(got) != string(before) {
		t.Fatalf("connecting the folder's row wrote the claude row's settings.json:\n%s", got)
	}
	folder, err := os.ReadFile(work)
	if err != nil || !strings.Contains(string(folder), "relay/m1") {
		t.Fatalf("the folder's settings.json wasn't connected:\n%s", folder)
	}
	s := call("/api/agents/disconnect/claude%23work")
	if row(s, "claude#work").Wired || row(s, "claude").Wired {
		t.Fatal("a row stayed connected")
	}
	if got, err = os.ReadFile(def); err != nil || string(got) != string(before) {
		t.Fatalf("disconnecting the folder's row wrote the claude row's settings.json:\n%s", got)
	}
}

// Claude Code's /model lists modelPicker's rows: connected, they are
// eipgam's models; switched off, they go, and a modelPicker the user wrote
// is never touched.
func TestAgentConnectClaudePicker(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	cfg := filepath.Join(home, ".claude", "settings.json")
	if err := provider.Save(provider.Provider{ID: "relay", Name: "Relay", Key: "k", Chat: "http://127.0.0.1:1/v1", Models: []string{"m1", "m2"}}); err != nil {
		t.Fatal(err)
	}
	h := Handler(nil, nil)
	post := func(path string) {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("POST", path, strings.NewReader("{}")))
		if rec.Code != 200 {
			t.Fatalf("%s %d %s", path, rec.Code, rec.Body)
		}
	}
	picker := func() []string {
		var s struct {
			ModelPicker *struct{ Options []struct{ Model string } } `json:"modelPicker"`
		}
		b, _ := os.ReadFile(cfg)
		json.Unmarshal(b, &s)
		if s.ModelPicker == nil {
			return nil
		}
		var out []string
		for _, o := range s.ModelPicker.Options {
			out = append(out, o.Model)
		}
		return out
	}
	post("/api/agents/connect/claude")
	if got := strings.Join(picker(), " "); got != "relay/m1 relay/m2" {
		t.Fatalf("connected, /model lists %q", got)
	}
	post("/api/agents/disconnect/claude")
	if got := picker(); got != nil {
		t.Fatalf("switched off, /model still lists %q", got)
	}
	own := `{"modelPicker":{"options":[{"model":"mine"}]}}`
	os.WriteFile(cfg, []byte(own), 0o600)
	post("/api/agents/connect/claude")
	post("/api/agents/disconnect/claude")
	if got := strings.Join(picker(), " "); got != "mine" {
		t.Fatalf("the user's own modelPicker became %q", got)
	}
}
