package library

import (
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// hermesConfig is a Hermes config.yaml as the user keeps it: their model,
// a comment, a server of their own with settings eipgam doesn't write, and
// an SSE one.
const hermesConfig = `# my Hermes settings
model:
  default: anthropic/claude-sonnet-4.6 # the one I like
  provider: openrouter
mcp_servers:
  mine:
    command: uvx
    args: [mine-mcp]
    timeout: 30
  remote:
    url: https://r.example/sse
    transport: sse
toolsets:
  - hermes-cli
`

// EUPH on Discord: the Library's MCP servers couldn't be given to Hermes.
// Hermes reads mcp_servers from its config.yaml (tools/mcp_tool.py):
// command/args/env, url/headers, transport: sse for SSE. Hermes is an
// agent a plugin adds now, naming this format (pluginMCP's "hermes"), so
// the format is tried on the file itself: each server is written there
// beside the user's settings, comments and servers, read back, and only
// eipgam's taken out again.
func TestHermesMCPFormat(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	write(t, p, hermesConfig)
	f := &mcpFile{Path: p, Format: fmtHermes}
	if pluginMCP["hermes"] != fmtHermes {
		t.Fatal("a plugin can't name Hermes's format")
	}
	// as SaveServer writes one: in place of the entry by its name, what
	// the user set on it kept
	put := func(s Server) {
		t.Helper()
		if err := f.supports(&s); err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		es, err := f.entries()
		if err != nil {
			t.Fatal(err)
		}
		if err := f.put(&s, es[s.Name]); err != nil {
			t.Fatal(err)
		}
	}

	// the user's servers are read as they are
	have, err := f.read()
	if err != nil {
		t.Fatal(err)
	}
	if s := have["mine"]; s == nil || s.Command != "uvx" {
		t.Errorf("mine: %+v", s)
	}
	if s := have["remote"]; s == nil || s.Transport != "sse" || s.URL != "https://r.example/sse" {
		t.Errorf("remote: %+v", s)
	}

	stdio := Server{Name: "fs", Transport: "stdio", Command: "npx", Args: []string{"-y", "@mcp/fs"}, Env: map[string]string{"K": "V"}}
	web := Server{Name: "web", Transport: "http", URL: "https://example.com/mcp", Headers: map[string]string{"Authorization": "Bearer x"}}
	sse := Server{Name: "old", Transport: "sse", URL: "https://example.com/sse"}
	for _, s := range []Server{stdio, web, sse} {
		put(s)
	}

	got := read(t, p)
	for _, want := range []string{"# my Hermes settings", "# the one I like", "provider: openrouter", "- hermes-cli"} {
		if !strings.Contains(got, want) {
			t.Errorf("lost %q:\n%s", want, got)
		}
	}
	var doc struct {
		Servers map[string]map[string]any `yaml:"mcp_servers"`
	}
	if err := yaml.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("%v:\n%s", err, got)
	}
	if fs := doc.Servers["fs"]; fs["command"] != "npx" || fs["url"] != nil || fs["transport"] != nil {
		t.Errorf("fs: %v", fs)
	} else if e, _ := fs["env"].(map[string]any); e["K"] != "V" {
		t.Errorf("fs env: %v", fs)
	}
	if w := doc.Servers["web"]; w["url"] != "https://example.com/mcp" || w["transport"] != nil || w["command"] != nil {
		t.Errorf("web: %v", w)
	} else if hd, _ := w["headers"].(map[string]any); hd["Authorization"] != "Bearer x" {
		t.Errorf("web headers: %v", w)
	}
	if o := doc.Servers["old"]; o["url"] != "https://example.com/sse" || o["transport"] != "sse" {
		t.Errorf("sse: %v", o)
	}
	if m := doc.Servers["mine"]; m["command"] != "uvx" || m["timeout"] != 30 {
		t.Errorf("the user's server: %v", m)
	}
	// written in Hermes' own order, command before args
	if i, j := strings.Index(got, "command: npx"), strings.Index(got, "- '@mcp/fs'"); i < 0 || j < i {
		t.Errorf("fs isn't written as Hermes writes it:\n%s", got)
	}

	back, err := f.read()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []Server{stdio, web, sse} {
		if s := back[want.Name]; s == nil || !s.same(&want) {
			t.Errorf("%s read back as %+v", want.Name, s)
		}
	}

	// a timeout or enabled the user sets on eipgam's server stays when it
	// is written again
	got = strings.Replace(read(t, p), "    url: https://example.com/mcp\n", "    url: https://example.com/mcp\n    enabled: false\n    timeout: 90\n", 1)
	write(t, p, got)
	put(Server{Name: "web", Transport: "http", URL: "https://example.com/v2"})
	yaml.Unmarshal([]byte(read(t, p)), &doc)
	if w := doc.Servers["web"]; w["url"] != "https://example.com/v2" || w["enabled"] != false || w["timeout"] != 90 || w["headers"] != nil {
		t.Errorf("web after edit: %v\n%s", w, read(t, p))
	}

	for _, s := range []string{"fs", "web", "old"} {
		if err := f.del(s); err != nil {
			t.Fatal(err)
		}
	}
	after, _ := f.read()
	if len(after) != 2 || after["mine"] == nil || after["mine"].Command != "uvx" || after["remote"] == nil {
		t.Errorf("after: %+v\n%s", after, read(t, p))
	}
	got = read(t, p)
	for _, want := range []string{"# my Hermes settings", "# the one I like", "timeout: 30"} {
		if !strings.Contains(got, want) {
			t.Errorf("lost %q:\n%s", want, got)
		}
	}
}
