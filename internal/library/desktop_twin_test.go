package library

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sichengchen/eipgam/internal/agent"
	"github.com/sichengchen/eipgam/internal/desktopdir"
	"github.com/sichengchen/eipgam/internal/settings"
)

// twinSandbox is the sandbox with Claude Desktop and Claude eipgam, the
// isolated Desktop, on: its folder Claude-eipgam-3p and its Code tab's
// Claude Code folder ~/.claude-eipgam-desktop there. Only on a Mac.
func twinSandbox(t *testing.T) (h, desk, twinDir string) {
	t.Helper()
	if desktopdir.OS != "darwin" {
		t.Skip("Claude eipgam is only on a Mac")
	}
	h = sandbox(t)
	desk, _ = desktopFiles(t, h)
	write(t, desk, `{}`)
	twinDir = desktopdir.Isolated(desktopdir.OS, h)
	write(t, filepath.Join(twinDir, "claude_desktop_config.json"), `{"deploymentMode": "3p"}`)
	write(t, filepath.Join(h, ".claude-eipgam-desktop", "settings.json"), `{}`)
	s, err := settings.LoadChecked()
	if err != nil {
		t.Fatal(err)
	}
	s.DesktopIsolated, s.DesktopIsolatedClaude = true, true
	if err := settings.Save(s); err != nil {
		t.Fatal(err)
	}
	return h, desk, twinDir
}

// xMopf7RF on X: Claude eipgam is an agent of its own in the library. Its
// Desktop takes MCP servers and Cowork skills in its own folder, its Code
// tab's Claude Code (claude#eipgam-desktop) instructions, skills and
// servers in ~/.claude-eipgam-desktop; what is given to one reaches none
// of Claude Desktop's or ~/.claude's files, and the other way round.
func TestClaudeMagpieIsItsOwnLibraryAgent(t *testing.T) {
	h, desk, twinDir := twinSandbox(t)
	twinCfg := filepath.Join(twinDir, "claude_desktop_config.json")
	code := filepath.Join(h, ".claude-eipgam-desktop")
	deskRoot := desktopAccount(t, filepath.Dir(desk), "org", "acct", `{"skills":[]}`)
	twinRoot := desktopAccount(t, twinDir, "org2", "acct2", `{"skills":[]}`)

	tw := targetByID(agent.DesktopTwinID)
	if tw == nil || tw.MCP == nil || tw.MCP.Path != twinCfg || len(tw.MCP.Also) != 0 || !slices.Equal(tw.Desktop, []string{twinRoot}) || tw.Instructions != "" {
		t.Fatalf("Claude eipgam's target: %+v", tw)
	}
	if tw.Agent.Name != "Claude eipgam" {
		t.Errorf("named %q", tw.Agent.Name)
	}
	cd := targetByID("claude-desktop")
	if cd == nil || slices.Contains(cd.MCP.files(), twinCfg) || slices.Contains(cd.Desktop, twinRoot) {
		t.Fatalf("Claude Desktop's target reaches Claude eipgam's folder: %+v %v", cd.MCP, cd.Desktop)
	}
	cc := targetByID("claude#eipgam-desktop")
	if cc == nil || cc.Instructions != filepath.Join(code, "CLAUDE.md") || cc.Skills != filepath.Join(code, "skills") || cc.MCP == nil || cc.MCP.Path != filepath.Join(code, ".claude.json") {
		t.Fatalf("its Code tab's target: %+v", cc)
	}

	keep := map[string]string{}
	for _, p := range []string{desk, filepath.Join(deskRoot, desktopManifest), filepath.Join(h, ".claude", "settings.json")} {
		keep[p] = read(t, p)
	}
	src := filepath.Join(h, "src/skills")
	skill(t, filepath.Join(src, "pdf"), "pdf", "Read PDFs")
	ok(t)(InstallSkills(src, []string{"pdf"}, []string{agent.DesktopTwinID, "claude#eipgam-desktop"}))
	ok(t)(SaveServer("", Server{Name: "fs", Transport: "stdio", Command: "npx", Args: []string{"-y", "fs"}, Agents: []string{agent.DesktopTwinID, "claude#eipgam-desktop"}}))
	brief := "Be brief."
	ok(t)(SaveInstructions(InstructionsChange{Shared: &brief, Agents: []string{"claude#eipgam-desktop"}}))

	if s := desktopServers(t, twinCfg)["fs"]; s["command"] != "npx" {
		t.Errorf("Claude eipgam's servers: %v", desktopServers(t, twinCfg))
	}
	if _, err := os.Stat(filepath.Join(twinRoot, "skills", "pdf", "SKILL.md")); err != nil {
		t.Errorf("Claude eipgam's Cowork skill: %v", err)
	}
	if _, has := desktopEntries(t, twinRoot)["pdf"]; !has {
		t.Errorf("Claude eipgam's manifest: %s", read(t, filepath.Join(twinRoot, desktopManifest)))
	}
	if got := read(t, filepath.Join(code, ".claude.json")); !strings.Contains(got, `"fs"`) {
		t.Errorf("its Code tab's .claude.json: %s", got)
	}
	if _, err := os.Stat(filepath.Join(code, "skills", "pdf", "SKILL.md")); err != nil {
		t.Errorf("its Code tab's skill: %v", err)
	}
	if got := read(t, filepath.Join(code, "CLAUDE.md")); !strings.Contains(got, brief) {
		t.Errorf("its Code tab's CLAUDE.md: %q", got)
	}
	// none of the user's own Desktop, ~/.claude or ~/.claude.json
	for p, was := range keep {
		if got := read(t, p); got != was {
			t.Errorf("%s changed:\n%s\nwas:\n%s", p, got, was)
		}
	}
	for _, p := range []string{filepath.Join(h, ".claude.json"), filepath.Join(h, ".claude", "CLAUDE.md"), filepath.Join(h, ".claude", "skills", "pdf"), filepath.Join(deskRoot, "skills", "pdf")} {
		if _, err := os.Lstat(p); err == nil {
			t.Errorf("%s written", p)
		}
	}

	// given to Claude Desktop alone, Claude eipgam's file keeps what it had
	ok(t)(SaveServer("", Server{Name: "gh", Transport: "stdio", Command: "gh", Agents: []string{"claude-desktop"}}))
	if s := desktopServers(t, desk); s["gh"] == nil || s["fs"] != nil {
		t.Errorf("Claude Desktop's servers: %v", s)
	}
	if s := desktopServers(t, twinCfg); s["gh"] != nil || s["fs"] == nil {
		t.Errorf("Claude eipgam's servers: %v", s)
	}
	// and taken from Claude eipgam, it goes from there alone
	ok(t)(SaveServer("fs", Server{Name: "fs", Transport: "stdio", Command: "npx", Args: []string{"-y", "fs"}, Agents: []string{"claude#eipgam-desktop"}}))
	if s := desktopServers(t, twinCfg); s["fs"] != nil {
		t.Errorf("fs still in Claude eipgam's: %v", s)
	}
	if got := read(t, filepath.Join(code, ".claude.json")); !strings.Contains(got, `"fs"`) {
		t.Errorf("fs gone from its Code tab's too: %s", got)
	}
}

// What the library gave Claude Desktop before Claude eipgam was an app of
// its own was written into Claude eipgam's folder too: it stays there,
// now ticked for Claude eipgam, so the user loses nothing and can take it
// from either app alone.
func TestClaudeMagpieKeepsWhatDesktopWasGiven(t *testing.T) {
	_, desk, twinDir := twinSandbox(t)
	twinCfg := filepath.Join(twinDir, "claude_desktop_config.json")
	twinRoot := desktopAccount(t, twinDir, "org2", "acct2", `{"skills":[]}`)
	// as the release before wrote it: Claude Desktop's server and skill in
	// both folders, recorded for claude-desktop alone
	write(t, desk, `{"mcpServers": {"fs": {"command": "npx"}}}`)
	write(t, twinCfg, `{"deploymentMode": "3p", "mcpServers": {"fs": {"command": "npx"}, "mine": {"command": "x"}}}`)
	skill(t, filepath.Join(twinRoot, "skills", "pdf"), "pdf", "Read PDFs")
	skill(t, filepath.Join(Dir(), "skills", "pdf"), "pdf", "Read PDFs")
	l := &Library{
		MCP:     []*Server{{Name: "fs", Transport: "stdio", Command: "npx", Agents: []string{"claude-desktop"}}, {Name: "other", Transport: "stdio", Command: "o", Agents: []string{"claude-desktop"}}},
		Skills:  []*Skill{{Name: "pdf", Agents: []string{"claude-desktop"}}},
		Applied: map[string]*Applied{"claude-desktop": {MCP: []string{"fs", "other"}, Skills: []string{"pdf"}}},
	}
	if err := l.save(); err != nil {
		t.Fatal(err)
	}
	l, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if got := l.server("fs").Agents; !slices.Equal(got, []string{"claude-desktop", agent.DesktopTwinID}) {
		t.Errorf("fs for %v", got)
	}
	// one that never reached Claude eipgam's file isn't ticked for it
	if got := l.server("other").Agents; !slices.Equal(got, []string{"claude-desktop"}) {
		t.Errorf("other for %v", got)
	}
	if got := l.skill("pdf").Agents; !slices.Equal(got, []string{"claude-desktop", agent.DesktopTwinID}) {
		t.Errorf("pdf for %v", got)
	}
	if a := l.Applied[agent.DesktopTwinID]; a == nil || !slices.Equal(a.MCP, []string{"fs"}) || !slices.Equal(a.Skills, []string{"pdf"}) {
		t.Errorf("recorded for Claude eipgam: %+v", a)
	}
	// once: unticked for Claude eipgam, it stays unticked
	if err := l.save(); err != nil {
		t.Fatal(err)
	}
	l.server("fs").Agents = []string{"claude-desktop"}
	l.Applied[agent.DesktopTwinID].MCP = []string{"fs"}
	if err := l.save(); err != nil {
		t.Fatal(err)
	}
	if l, _ = load(); !slices.Equal(l.server("fs").Agents, []string{"claude-desktop"}) {
		t.Errorf("ticked again: %v", l.server("fs").Agents)
	}
}
