package library

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Every agent on the Agents page that reads skills has a place for them in
// the library (Kimi Code had none, so a skill never reached it): each its
// own folder, but Kimi Code, whose own would hide Claude Code's from it,
// the shared ~/.agents/skills.
func TestAgentsSkillsFolders(t *testing.T) {
	h := sandbox(t)
	for _, f := range []string{".kimi/config.toml", ".grok/config.toml", ".config/devin/config.json"} {
		write(t, filepath.Join(h, f), "")
	}
	want := map[string]string{
		"kimi":  ".agents/skills",
		"grok":  ".grok/skills",
		"devin": ".config/devin/skills",
	}
	for id, d := range want {
		tg := targetByID(id)
		if tg == nil || tg.Skills != filepath.Join(h, d) {
			t.Errorf("%s: %+v", id, tg)
		}
	}
	// what Kimi Code reads first of the shared ones, when it's there
	os.MkdirAll(filepath.Join(h, ".config/agents/skills"), 0o755)
	if tg := targetByID("kimi"); tg == nil || tg.Skills != filepath.Join(h, ".config/agents/skills") {
		t.Errorf("kimi with ~/.config/agents/skills: %+v", tg)
	}
	if ProjectSkillsDir("kimi") != ".agents/skills" {
		t.Error("kimi reads a project's .agents/skills")
	}
}

// A skill given to Kimi Code is one link in ~/.agents/skills; Codex and
// Gemini CLI, which read that folder as well, get no second link in their
// own, and get one again once it's not there.
func TestSharedSkillsWrittenOnce(t *testing.T) {
	h := sandbox(t)
	write(t, filepath.Join(h, ".kimi/config.toml"), "")
	src := filepath.Join(h, "src/skills")
	skill(t, filepath.Join(src, "pdf"), "pdf", "Read PDFs")
	shared := filepath.Join(h, ".agents/skills/pdf")
	cx, cl := filepath.Join(h, ".codex/skills/pdf"), filepath.Join(h, ".claude/skills/pdf")

	ok(t)(InstallSkills(src, []string{"pdf"}, []string{"claude", "codex", "kimi"}))
	if !ours(shared, "pdf") {
		t.Error("~/.agents/skills/pdf isn't the library's")
	}
	if !ours(cl, "pdf") {
		t.Error("claude hasn't it")
	}
	if _, err := os.Lstat(cx); !os.IsNotExist(err) {
		t.Error("codex has a second link to it")
	}
	v, err := Read(nil)
	if err != nil {
		t.Fatal(err)
	}
	also := map[string][]string{}
	for _, a := range v.Agents {
		also[a.ID] = a.SkillsAlso
	}
	for id, w := range map[string]string{"codex": "kimi", "gemini": "kimi"} {
		if !slices.Contains(also[id], w) {
			t.Errorf("%s reads %s's skills too: %v", id, w, also[id])
		}
	}
	if slices.Contains(also["claude"], "kimi") {
		t.Errorf("claude reads no ~/.agents/skills: %v", also["claude"])
	}

	// Kimi Code off: Codex gets its own again, the shared goes
	ok(t)(SkillAgents("pdf", []string{"claude", "codex"}))
	if !ours(cx, "pdf") {
		t.Error("codex hasn't it")
	}
	if _, err := os.Lstat(shared); !os.IsNotExist(err) {
		t.Error("~/.agents/skills still has it")
	}
	// on again: codex's own goes, not the skill
	ok(t)(SkillAgents("pdf", []string{"claude", "codex", "kimi"}))
	if _, err := os.Lstat(cx); !os.IsNotExist(err) || !ours(shared, "pdf") {
		t.Error("not once in ~/.agents/skills")
	}
	if _, err := os.Stat(filepath.Join(src, "pdf/SKILL.md")); err != nil {
		t.Error("the user's folder went")
	}
}

// A skill of the user's own in ~/.agents/skills brought into the library
// stays there as it is, and Kimi Code, whose folder it is, has it without
// a word of its "own skill by that name".
func TestImportSharedSkillForKimi(t *testing.T) {
	h := sandbox(t)
	write(t, filepath.Join(h, ".kimi/config.toml"), "")
	shared := filepath.Join(h, ".agents/skills/orca")
	skill(t, shared, "orca", "Orca")
	ok(t)(ImportSkill("orca"))
	if fi, err := os.Lstat(shared); err != nil || !fi.IsDir() {
		t.Errorf("the shared folder's skill: %v", err)
	}
	v, _ := Read(nil)
	if len(v.Skills) != 1 || !slices.Equal(v.Skills[0].Agents, []string{"kimi"}) || len(v.Skills[0].Problems) != 0 {
		t.Errorf("skills: %+v", v.Skills)
	}
	ok(t)(Sync())
}

// Grok Build reads a user-wide AGENTS.md in its folder ($GROK_HOME else
// ~/.grok): the library's instructions reach it there, after what the user
// wrote, which stays.
func TestGrokInstructions(t *testing.T) {
	h := sandbox(t)
	g := filepath.Join(h, "grok-home")
	t.Setenv("GROK_HOME", g)
	write(t, filepath.Join(g, "config.toml"), "")
	f := filepath.Join(g, "AGENTS.md")
	if tg := targetByID("grok"); tg == nil || tg.Instructions != f {
		t.Fatalf("grok: %+v", tg)
	}
	write(t, f, "# Mine\n")
	shared := "Use tabs."
	ok(t)(SaveInstructions(InstructionsChange{Shared: &shared, Agents: []string{"grok"}}))
	if s := read(t, f); s != "# Mine\n\n"+blockBegin+"\nUse tabs.\n"+blockEnd+"\n" {
		t.Errorf("grok's AGENTS.md:\n%s", s)
	}
}

// Devin CLI reads its global rules from AGENTS.md in its config folder
// (~/.config/devin, %APPDATA%\devin on Windows), else the singular AGENT.md
// there (#3151): the library's instructions reach it, after what the user
// wrote, which stays, and Devin is offered them rather than listed as having
// no user-wide file.
func TestDevinInstructions(t *testing.T) {
	h := sandbox(t)
	write(t, filepath.Join(h, ".config/devin/config.json"), "{}")
	tg := targetByID("devin")
	if tg == nil {
		t.Fatal("Devin isn't a target")
	}
	f := filepath.Join(tg.Agent.Dir, "AGENTS.md")
	if tg.Instructions != f {
		t.Fatalf("Devin's instructions: %q, want %q before the file is there", tg.Instructions, f)
	}
	write(t, f, "# Mine\n")
	shared := "Use tabs."
	ok(t)(SaveInstructions(InstructionsChange{Shared: &shared, Agents: []string{"devin"}}))
	if s := read(t, f); s != "# Mine\n\n"+blockBegin+"\nUse tabs.\n"+blockEnd+"\n" {
		t.Fatalf("Devin's AGENTS.md:\n%s", s)
	}

	// a user who keeps theirs as AGENT.md has it written there instead
	h = sandbox(t)
	write(t, filepath.Join(h, ".config/devin/config.json"), "{}")
	dir := targetByID("devin").Agent.Dir
	write(t, filepath.Join(dir, "AGENT.md"), "# Mine\n")
	if tg := targetByID("devin"); tg.Instructions != filepath.Join(dir, "AGENT.md") {
		t.Fatalf("Devin with AGENT.md: %q", tg.Instructions)
	}
}

// Kimi Code 2.x reads one user-wide instructions file of its own, which the
// library writes into; the old kimi-cli reads none.
func TestKimiCodeInstructions(t *testing.T) {
	h := sandbox(t)
	write(t, filepath.Join(h, ".kimi-code", "config.toml"), "default_model = \"k3\"\n")
	inst := filepath.Join(h, ".kimi-code", "AGENTS.md")
	if tg := targetByID("kimi"); tg == nil || tg.Instructions != inst {
		t.Fatalf("kimi's instructions: %+v, want %s", tg, inst)
	}
	if got, err := Takes("kimi", "instructions"); err != nil || got != "kimi" {
		t.Fatalf("Takes(kimi): %q %v", got, err)
	}
	shared := "Use tabs."
	ok(t)(SaveInstructions(InstructionsChange{Shared: &shared, Agents: []string{"kimi"}}))
	if s := read(t, inst); s != blockBegin+"\nUse tabs.\n"+blockEnd+"\n" {
		t.Fatalf("kimi's AGENTS.md:\n%s", s)
	}

	// the old kimi-cli reads no user-wide instructions
	t.Setenv("KIMI_CODE_HOME", "")
	if err := os.RemoveAll(filepath.Join(h, ".kimi-code")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(h, ".kimi", "config.toml"), "")
	if tg := targetByID("kimi"); tg == nil || tg.Instructions != "" {
		t.Fatalf("kimi-cli's instructions: %+v", tg)
	}
}

// Grok App's independent home (Use on a custom provider) reads AGENTS.md
// and skills beside agent-home/config.toml, not Grok Build's ~/.grok.
func TestGrokAppInstructions(t *testing.T) {
	h := sandbox(t)
	root := filepath.Join(h, "grok-app-data")
	t.Setenv("GROK_APP_HOME", root)
	home := filepath.Join(root, "agent-home")
	write(t, filepath.Join(home, "config.toml"), "[models]\ndefault = \"deepseek\"\n")
	inst := filepath.Join(home, "AGENTS.md")
	write(t, inst, "# Mine\n")
	tg := targetByID("grok-app")
	if tg == nil || tg.Instructions != inst || tg.Skills != filepath.Join(home, "skills") || tg.MCP == nil || tg.MCP.Path != filepath.Join(home, "config.toml") {
		t.Fatalf("Grok App library places: %+v", tg)
	}
	if !slices.Contains(readsShared, "grok-app") {
		t.Fatal("Grok App doesn't read ~/.agents/skills")
	}
	shared := "Use tabs."
	ok(t)(SaveInstructions(InstructionsChange{Shared: &shared, Agents: []string{"grok-app"}}))
	if s := read(t, inst); s != "# Mine\n\n"+blockBegin+"\nUse tabs.\n"+blockEnd+"\n" {
		t.Fatalf("AGENTS.md:\n%s", s)
	}
	if cfg := read(t, filepath.Join(home, "config.toml")); !strings.Contains(cfg, `default = "deepseek"`) {
		t.Fatalf("instructions rewrote the route:\n%s", cfg)
	}
}

// Pi reads AGENTS.override.md in its agent folder when it is there, else
// the first there is of AGENTS.md, AGENTS.MD, CLAUDE.md and CLAUDE.MD, so a
// user's own CLAUDE.md is written into rather than hidden by a new
// AGENTS.md.
func TestPiInstructionsFirstThere(t *testing.T) {
	h := sandbox(t)
	pi := filepath.Join(h, ".pi", "agent")
	if tg := targetByID("pi"); tg == nil ||
		tg.Instructions != filepath.Join(pi, "AGENTS.md") || tg.Override != filepath.Join(pi, "AGENTS.override.md") {
		t.Fatalf("with no file there: %+v", tg)
	}
	inst := filepath.Join(pi, "CLAUDE.md")
	write(t, inst, "# Mine\n")
	if tg := targetByID("pi"); tg == nil || tg.Instructions != inst {
		t.Fatalf("pi's instructions: %+v, want %s", tg, inst)
	}
	shared := "Use tabs."
	ok(t)(SaveInstructions(InstructionsChange{Shared: &shared, Agents: []string{"pi"}}))
	if s := read(t, inst); s != "# Mine\n\n"+blockBegin+"\nUse tabs.\n"+blockEnd+"\n" {
		t.Fatalf("pi's CLAUDE.md:\n%s", s)
	}
	if _, err := os.Stat(filepath.Join(pi, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("pi got an AGENTS.md that hides CLAUDE.md: %v", err)
	}
	// an AGENTS.md there comes first again
	write(t, filepath.Join(pi, "AGENTS.md"), "")
	if tg := targetByID("pi"); tg.Instructions != filepath.Join(pi, "AGENTS.md") {
		t.Fatalf("AGENTS.md comes before CLAUDE.md: %q", tg.Instructions)
	}
}
