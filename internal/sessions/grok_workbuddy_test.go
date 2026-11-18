package sessions

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const (
	grokMain = "0190aaaa-0000-7000-8000-00000000000a"
	grokFork = "0190aaaa-0000-7000-8000-00000000000c"
	wbID     = "22222222-aaaa-4bbb-8ccc-000000000001"
	wbOldID  = "0123456789abcdef0123456789abcdef"
)

func TestGrok(t *testing.T) {
	inZone(t, 0)
	home := setupAgents(t)
	ss := List(0)
	if n := count(ss, "grok"); n != 2 {
		t.Fatalf("want 2 Grok Build sessions (the subagent in its session, the one of hooks only left out), got %d", n)
	}
	g := find(t, ss, "grok", grokMain)
	// the first prompt typed, its chunks put together, before the title Grok made
	if g.Cwd != "/work/grok" || g.Title != "Parser port" {
		t.Fatalf("grok: %+v", g)
	}
	// the cache out of the input, the subagent's turn in once
	if m := model(g, "grok-4.7-build"); m.Tokens != (Tokens{700, 100, 2800, 0, 0}) {
		t.Fatalf("build: %+v", m)
	}
	if m := model(g, "grok-4.7-mini"); m.Tokens != (Tokens{300, 30, 100, 0, 0}) {
		t.Fatalf("mini, the subagent's in: %+v", m)
	}
	// the hook that ran as it was closed, a day on, isn't when it was at work
	if !g.Start.Equal(at("2026-09-27T08:00:00Z")) || !g.Last.Equal(at("2026-09-27T08:02:10Z")) {
		t.Fatalf("grok times %s %s", g.Start, g.Last)
	}
	want := "cd '/work/grok' && grok --resume " + grokMain
	if runtime.GOOS == "windows" {
		want = "Set-Location -LiteralPath '/work/grok'; grok --resume " + grokMain
	}
	if g.Resume != want {
		t.Fatalf("resume %q", g.Resume)
	}
	// the fork: what it copied counted in the session it came from
	f := find(t, ss, "grok", grokFork)
	if f.Tokens != (Tokens{100, 30, 300, 0, 0}) || f.Title != "Parser port, another way" {
		t.Fatalf("fork: %+v", f)
	}
	if !f.Start.Equal(at("2026-09-27T09:00:00.25Z")) || !f.Last.Equal(at("2026-09-27T09:00:30Z")) {
		t.Fatalf("fork times %s %s", f.Start, f.Last)
	}
	var active int64
	for _, d := range StatsFor(0).Days {
		for _, a := range d.Active {
			if a.Agent == "grok" {
				active += a.Seconds
			}
		}
	}
	if active < 150 || active > 170 {
		t.Fatalf("active %ds, want the main session's 130 and the fork's 30", active)
	}

	// $GROK_HOME
	moved := filepath.Join(filepath.Dir(home), "grok-home")
	if err := os.Rename(filepath.Join(home, ".grok"), moved); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GROK_HOME", moved)
	Reset()
	find(t, List(0), "grok", grokMain)
}
