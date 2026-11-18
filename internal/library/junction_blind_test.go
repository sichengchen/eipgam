package library

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// onWindowsLinks makes the sync judge links as it does on Windows, where
// magpie's link may be a junction (skipsJunctions).
func onWindowsLinks(t *testing.T) {
	t.Helper()
	old := linkOS
	linkOS = "windows"
	t.Cleanup(func() { linkOS = old })
}

// #6610, #5107: on Windows, Antigravity skips a junction in its skills
// folder, so it gets marked copies, made again when the library's
// changes; an edit made in one goes to the library and to the agents
// given links. Elsewhere it gets links.
func TestJunctionBlindAgentsGetCopies(t *testing.T) {
	h, cfg := antigravity(t)
	if tg := targetByID("agy"); tg == nil || tg.Copy != (runtime.GOOS == "windows") {
		t.Fatalf("agy here: %+v", tg)
	}
	onWindowsLinks(t)
	if tg := targetByID("agy"); tg == nil || !tg.Copy {
		t.Fatalf("agy on Windows: %+v", tg)
	}
	if tg := targetByID("claude"); tg == nil || tg.Copy {
		t.Fatalf("claude on Windows: %+v", tg)
	}

	src := filepath.Join(h, "src/skills")
	skill(t, filepath.Join(src, "pdf"), "pdf", "Read PDFs")
	ok(t)(InstallSkills(src, []string{"pdf"}, []string{"agy", "claude"}))
	ag, cl := filepath.Join(cfg, "skills/pdf"), filepath.Join(h, ".claude/skills/pdf")
	if isLink(t, ag) || !ours(ag, "pdf") {
		t.Fatalf("%s isn't magpie's copy", ag)
	}
	if runtime.GOOS != "windows" && !isLink(t, cl) {
		t.Fatal("Claude Code's isn't a link")
	}
	v, err := Read(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range v.Agents {
		if a.ID == "agy" && (!a.MustCopy || a.How != HowCopy) {
			t.Errorf("%s: must copy %v, how %q", a.ID, a.MustCopy, a.How)
		}
	}

	// the user edits Antigravity's copy, then syncs (twice: the next start)
	time.Sleep(20 * time.Millisecond)
	write(t, filepath.Join(ag, "SKILL.md"), "---\nname: pdf\ndescription: Edited in Antigravity\n---\n")
	for range 2 {
		ok(t)(Sync())
		for _, p := range []string{ag, cl, filepath.Join(src, "pdf")} {
			if s := read(t, filepath.Join(p, "SKILL.md")); !strings.Contains(s, "Edited in Antigravity") {
				t.Fatalf("%s after a sync:\n%s", p, s)
			}
		}
		if isLink(t, ag) || !ours(ag, "pdf") {
			t.Fatal("Antigravity's isn't magpie's copy any more")
		}
	}

	// the library's changes: the copy is behind till a sync
	time.Sleep(20 * time.Millisecond)
	write(t, filepath.Join(src, "pdf/SKILL.md"), "---\nname: pdf\ndescription: Changed\n---\n")
	v, _ = Read(nil)
	if b := v.Skills[0].Behind; !slices.Equal(b, []string{"agy"}) {
		t.Fatalf("behind: %v", b)
	}
	ok(t)(Sync())
	if s := read(t, filepath.Join(ag, "SKILL.md")); !strings.Contains(s, "Changed") {
		t.Errorf("%s wasn't copied again:\n%s", ag, s)
	}
}

// A project's .agents/skills entry that Antigravity reads is a copy on
// Windows, and an edit made in it goes to the library; one no such agent
// reads is a link again.
func TestJunctionBlindProjectCopies(t *testing.T) {
	h, proj := projectLib(t)
	onWindowsLinks(t)
	ok(t)(ProjectSkill(proj, "pdf", []string{"agy", "claude"}))
	sh, cl := filepath.Join(proj, ".agents/skills/pdf"), filepath.Join(proj, ".claude/skills/pdf")
	if isLink(t, sh) || !ours(sh, "pdf") {
		t.Fatal(".agents/skills/pdf isn't magpie's copy")
	}
	if runtime.GOOS != "windows" && !isLink(t, cl) {
		t.Fatal(".claude/skills/pdf isn't a link")
	}

	// the user edits the project's copy, then syncs
	time.Sleep(20 * time.Millisecond)
	write(t, filepath.Join(sh, "SKILL.md"), "---\nname: pdf\ndescription: Edited in the project\n---\n")
	for range 2 {
		ok(t)(Sync())
		for _, p := range []string{sh, cl, filepath.Join(h, "src/pdf")} {
			if s := read(t, filepath.Join(p, "SKILL.md")); !strings.Contains(s, "Edited in the project") {
				t.Fatalf("%s after a sync:\n%s", p, s)
			}
		}
	}
	if isLink(t, sh) || !ours(sh, "pdf") {
		t.Fatal("the project's copy isn't magpie's any more")
	}

	// Antigravity unticked: Codex's entry is a link again (on Windows a
	// copy as fresh as the library's stands for one)
	ok(t)(ProjectSkill(proj, "pdf", []string{"codex"}))
	if runtime.GOOS != "windows" && !isLink(t, sh) {
		t.Error(".agents/skills/pdf is still a copy")
	}
}
