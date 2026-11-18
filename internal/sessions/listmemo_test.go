package sessions

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// keyOf is the key a listing gives the file at path.
func keyOf(t *testing.T, fs []file, path string) string {
	t.Helper()
	for _, f := range fs {
		if f.path == path {
			return f.key
		}
	}
	t.Fatalf("%s not listed", path)
	return ""
}

// readCounter counts reads of each file a hook is told of.
type readCounter struct {
	mu sync.Mutex
	n  map[string]int
}

func (c *readCounter) saw(p string) {
	c.mu.Lock()
	c.n[p]++
	c.mu.Unlock()
}

func (c *readCounter) take() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := c.n
	c.n = map[string]int{}
	return n
}

// Grok Build's listing reads a session's summary.json, for whether it is
// a subagent's, only when the summary changed.
func TestGrokListingReadsOnlyChangedSummaries(t *testing.T) {
	inZone(t, 0)
	home := setupAgents(t)
	id := "0190aaaa-0000-7000-8000-0000000000dd"
	dir := filepath.Join(home, ".grok", "sessions", "%2Fwork%2Fgrok", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "updates.jsonl")
	if err := os.WriteFile(p, []byte(`{"timestamp":1790496001}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	summary := filepath.Join(dir, "summary.json")
	if err := os.WriteFile(summary, []byte(`{"info":{"id":"`+id+`"},"session_kind":"subagent","parent_session_id":"`+grokMain+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &readCounter{n: map[string]int{}}
	grokSummaryHook = c.saw
	t.Cleanup(func() { grokSummaryHook = nil })

	fs := grokFiles()
	if n := c.take(); n[summary] != 1 || len(n) < 2 {
		t.Fatalf("first listing read %v", n)
	}
	if k := keyOf(t, fs, p); k != "grok:"+grokMain {
		t.Fatalf("subagent keyed %q", k)
	}
	fs = grokFiles()
	if n := c.take(); len(n) != 0 {
		t.Fatalf("nothing changed: read %v", n)
	}
	if k := keyOf(t, fs, p); k != "grok:"+grokMain {
		t.Fatalf("subagent keyed %q from what was kept", k)
	}
	// the updates grew: the summary is not read for that
	appendText(t, p, `{"timestamp":1790496002}`+"\n")
	grokFiles()
	if n := c.take(); len(n) != 0 {
		t.Fatalf("updates grew: read %v", n)
	}
	// the summary written over, no longer a subagent's: read, its own now
	if err := os.WriteFile(summary, []byte(`{"info":{"id":"`+id+`"},"session_kind":"main","generated_title":"Own"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fs = grokFiles()
	if n := c.take(); n[summary] != 1 || len(n) != 1 {
		t.Fatalf("summary changed: read %v", n)
	}
	if k := keyOf(t, fs, p); k != "grok:"+id {
		t.Fatalf("written over: keyed %q", k)
	}
}
