package library

import (
	"os"
	"path/filepath"
	"testing"
)

// A file where a part of an agent that is there would be (Gemini CLI's
// skills folder, another tool's) leaves that part out, the rest given, and
// nothing is said to have gone wrong with it. Before, every change said
// "open …: not a directory".
func TestAgentHomeTakenByAFile(t *testing.T) {
	h := sandbox(t)
	skills := filepath.Join(h, ".gemini/skills")
	write(t, skills, "ls -la\n")
	g := targetByID("gemini")
	if g == nil || g.Skills != "" || g.Instructions == "" || g.MCP == nil {
		t.Errorf("gemini target: %+v", g)
	}

	shared := "Use tabs."
	ok(t)(SaveInstructions(InstructionsChange{Shared: &shared, Agents: []string{"claude", "gemini"}}))
	ok(t)(SaveServer("", Server{Name: "fs", Transport: "stdio", Command: "npx", Agents: []string{"claude", "gemini"}}))
	ok(t)(Sync())
	if b, err := os.ReadFile(skills); err != nil || string(b) != "ls -la\n" {
		t.Errorf("~/.gemini/skills: %q %v", b, err)
	}
}
