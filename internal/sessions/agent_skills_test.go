package sessions

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The sessions under testdata/opencode-skills, cline-skills and dsh-skills
// are real ones, run in a sandbox home through magpie's gateway, each
// with a skill greet-user: one where the model runs a command and then
// calls the skill tool, one where the user picks or types the skill, and
// one with /nonexistentzz, which no skill has. They are byte for byte as
// each agent wrote them (OpenCode's as its database's own .dump of
// session_v2 and session_message; dsh's working-directory folder is named
// shorter). MaxMooreMy on X asked whether every agent's skills count;
// these three counted none.

// agentSessions are the Sessions statistics' summaries of an agent's
// sessions on 2026-10-30 (UTC), by id.
func agentSessions(t *testing.T, agent string) (map[string]Summary, Stats) {
	t.Helper()
	s := statsAt(0, time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC))
	by := map[string]Summary{}
	for _, x := range s.Sessions {
		if x.Agent == agent {
			by[x.ID] = x
		}
	}
	return by, s
}

// OpenCode 2.0.25: a reply's content has {"type":"tool","name":"skill",
// "state":{"input":{"id":"greet-user"}}}; a prompt with the skill picked
// (@greet-user in the TUI, sent here as the TUI sends it) lists it in
// skills; a skill row is one brought in by itself.
func TestOpenCodeToolsAndSkills(t *testing.T) {
	inZone(t, 0)
	setupAgents(t)
	if err := os.MkdirAll(OpenCodeDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	dump, err := os.ReadFile("testdata/opencode-skills/opencode.sql")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(OpenCodeDir(), "opencode.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(dump)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	Reset()

	by, s := agentSessions(t, "opencode")
	if len(by) != 6 {
		t.Fatalf("want the 6 OpenCode sessions, got %d", len(by))
	}
	called := by["ses_e753a44afffeaDlZRyBkvVR0wE"] // cat notes.txt, then the skill
	if called.ToolCalls != 2 || called.tools.of("read") != 1 || called.tools.of("skill") != 1 {
		t.Fatalf("tools %d %v", called.ToolCalls, called.tools)
	}
	if called.skills.of("greet-user") != 1 || len(called.skills) != 1 {
		t.Fatalf("the skill tool's skill: %v", called.skills)
	}
	// /greet-user typed in opencode run: sent as typed, the model calls the tool
	if x := by["ses_e7539d2a5ffePUVJFMb1gKVccL"]; x.tools.of("skill") != 1 || x.skills.of("greet-user") != 1 {
		t.Fatalf("/greet-user: %v %v", x.tools, x.skills)
	}
	// @greet-user picked: no tool call, the skill in the prompt's skills
	if x := by["ses_e75393cbaffe9qtPD0p9PI6xYK"]; x.ToolCalls != 0 || x.skills.of("greet-user") != 1 || len(x.skills) != 1 {
		t.Fatalf("@greet-user picked: %d %v", x.ToolCalls, x.skills)
	}
	// a skill row
	if x := by["ses_e7538abfbffe2oKYxDK1OHgsrO"]; x.ToolCalls != 0 || x.skills.of("greet-user") != 1 {
		t.Fatalf("a skill row: %d %v", x.ToolCalls, x.skills)
	}
	// @greet-user only as text, and /nonexistentzz: no skill
	for _, id := range []string{"ses_e753a0f28ffe4fFER8Ll41K3GY", "ses_e75386b77ffeTIwbQnkzH3CYw8"} {
		if x := by[id]; x.ToolCalls != 0 || len(x.skills) != 0 {
			t.Fatalf("%s: %d %v", id, x.ToolCalls, x.skills)
		}
	}
	o := s.Overview("", "", "", 10)
	if o.Skills.Top[0].Name != "greet-user" || o.Skills.Top[0].Agents["opencode"] != 4 {
		t.Fatalf("overview skills %+v", o.Skills)
	}
}
