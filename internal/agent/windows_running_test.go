package agent

import (
	"runtime"
	"testing"
)

// These are named TestDsh… on purpose: the Windows job in
// .github/workflows/test.yml compiles the whole suite but runs only
// `-run '^Test(Dsh|KeepDsh)'` on a real Windows, so a Windows-only check
// under any other name would be compiled on every platform and executed on
// none — it skips on Linux and macOS (GOOS), and Windows never selects it.

// Running reports whether a process whose command line matches a pattern is
// alive, by asking pgrep -f. Windows has no pgrep, so it answered no for
// every pattern — and every one of its 25 call sites is the advice an
// agent's own lists need after eipgam changed what that agent reads at
// start ("Codex builds its model list at start-up — restart the Codex app",
// "New dsh sessions start on this; one already open keeps its model until
// you pick another in it"), each inside a Notice closure. So on Windows the
// Agents page wrote the file, said nothing, and a model picked in eipgam
// looked like it had done nothing at all.
func TestDshRunningOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the Windows answer is only given there")
	}
	if !Running(`(^|/)codex( |$)`) {
		t.Fatal("a pattern that may match was answered no: Windows can't tell")
	}
	if !Running(`(^|/)dsh( |$)`, `(^|/)codex( |$)`) {
		t.Fatal("the second of two patterns was answered no")
	}
	// no pattern is no question: a caller that asked nothing isn't told a
	// process is there
	if Running() {
		t.Fatal("no pattern was answered yes")
	}
}
