package shortcut

import "testing"

func TestTarget(t *testing.T) {
	for in, want := range map[string]string{
		`C:\Tools\eipgam-windows-amd64.exe`:          `C:\Tools\eipgam-windows-amd64.exe`,
		`C:\Tools\eipgam.exe.old`:                    `C:\Tools\eipgam.exe`,
		`C:\Tools\eipgam.exe.old-3`:                  `C:\Tools\eipgam.exe`,
		`C:\Tools\eipgam.exe.new`:                    `C:\Tools\eipgam.exe`,
		`C:\Tools\EIPGAM.EXE.OLD`:                    `C:\Tools\EIPGAM.EXE`,
		`C:\old\eipgam.exe`:                          `C:\old\eipgam.exe`,
		`C:\Tools\eipgam.exe.older`:                  `C:\Tools\eipgam.exe.older`,
		`/usr/local/bin/eipgam`:                      `/usr/local/bin/eipgam`,
		`C:\Users\me\Downloads\eipgam (1).exe.old-2`: `C:\Users\me\Downloads\eipgam (1).exe`,
	} {
		if got := Target(in); got != want {
			t.Errorf("Target(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStale(t *testing.T) {
	exe := "/tools/eipgam.exe"
	for _, c := range []struct {
		target, dir string
		stale       bool
	}{
		{"", "", true},                            // no shortcut yet
		{"/tools/eipgam.exe", "/tools", false},    // already this one
		{"/TOOLS/eipgam.EXE", "/Tools/", false},   // the same, in another case
		{"/downloads/eipgam.exe", "/tools", true}, // the exe was moved
		{"/tools/eipgam.exe", "/downloads", true}, // starts in another folder
		{"/tools/eipgam.exe", "", true},           // in no folder
	} {
		if got := Stale(c.target, c.dir, exe); got != c.stale {
			t.Errorf("Stale(%q, %q) = %v, want %v", c.target, c.dir, got, c.stale)
		}
	}
}

// Installed apps runs the exe it names, quoted, with the arguments the
// CLI opens the window on (the path has spaces under a user's name).
func TestUninstallCommand(t *testing.T) {
	got := UninstallCommand(`C:\Users\Ann Lee\AppData\Local\Programs\eipgam\eipgam.exe`)
	want := `"C:\Users\Ann Lee\AppData\Local\Programs\eipgam\eipgam.exe" uninstall --window`
	if got != want {
		t.Fatalf("UninstallCommand = %s, want %s", got, want)
	}
}
