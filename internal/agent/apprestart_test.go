package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sichengchen/eipgam/internal/provider"
)

// restartSandbox is a home with a provider in it; the system is said to be
// neither macOS nor Windows until a test says which.
func restartSandbox(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	if err := provider.Save(provider.Provider{ID: "deepseek", Name: "DeepSeek", Chat: "https://api.deepseek.com/v1", Key: "k", Models: []string{"pro"}}); err != nil {
		t.Fatal(err)
	}
	wasWin, wasMac, wasLinux, wasFolders, wasRunning := restartOnWindows, restartOnMac, restartOnLinux, appFolders, running
	wasQuit, wasOpen, wasQuitWait := appQuit, appOpen, appQuitWait
	wasProcs, wasClose, wasEnd, wasLaunch, wasCloseWait, wasBackWait, wasPoll := winProcs, winClose, winEnd, winLaunch, appCloseWait, appBackWait, appPoll
	wasShown, wasGrace := winShown, appTrayGrace
	t.Cleanup(func() {
		restartOnWindows, restartOnMac, restartOnLinux, appFolders, running = wasWin, wasMac, wasLinux, wasFolders, wasRunning
		appQuit, appOpen, appQuitWait = wasQuit, wasOpen, wasQuitWait
		winProcs, winClose, winEnd, winLaunch, appCloseWait, appBackWait, appPoll = wasProcs, wasClose, wasEnd, wasLaunch, wasCloseWait, wasBackWait, wasPoll
		winShown, appTrayGrace = wasShown, wasGrace
	})
	restartOnWindows, restartOnMac, restartOnLinux = false, false, false
	appPoll = 5 * time.Millisecond
	winShown = func([]int) bool { return true }
	return home
}

// macApp puts a bundle with id and executable exe at dir/name, as an
// installer leaves it.
func macApp(t *testing.T, dir, name, id, exe string) string {
	t.Helper()
	b := filepath.Join(dir, name)
	os.MkdirAll(filepath.Join(b, "Contents", "MacOS"), 0o755)
	os.WriteFile(filepath.Join(b, "Contents", "Info.plist"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
	<key>CFBundleExecutable</key>
	<string>`+exe+`</string>
	<key>CFBundleIdentifier</key>
	<string>`+id+`</string>
</dict></plist>`), 0o644)
	return b
}

// Which Windows processes are each app's: Claude Desktop's claude.exe, not
// Claude Code's.
func TestRestartAppsOnWindows(t *testing.T) {
	cases := []struct {
		id, image string
		want      bool
	}{
		{"claude-desktop", `C:\Users\u\AppData\Local\AnthropicClaude\app-2.9939.2\claude.exe`, true},
		{"claude-desktop", `C:\Users\u\AppData\Local\AnthropicClaude\claude.exe`, true},
		{"claude-desktop", `C:\Program Files\WindowsApps\Claude_1.0.0.0_x64__pzs8sxrjxfjjc\app\claude.exe`, true},
		{"claude-desktop", `C:\Program Files\WindowsApps\AnthropicPBC.Claude_1.0.0.0_x64__fnn82j28hfe8t\app\Claude.exe`, true},
		{"claude-desktop", `C:\Users\u\.local\bin\claude.exe`, false},
		{"claude-desktop", `C:\Users\u\AppData\Roaming\npm\node_modules\@anthropic-ai\claude-code\bin\claude.exe`, false},
		{"claude-desktop", `C:\Users\u\AppData\Local\AnthropicClaude\Update.exe`, false},
	}
	for _, c := range cases {
		if got := restartApps[c.id].win(c.image); got != c.want {
			t.Errorf("%s %s: %v, want %v", c.id, c.image, got, c.want)
		}
	}
	for _, gone := range []string{"workbuddy", "workbuddy-ai", "openmausbot", "zcode"} {
		if _, ok := restartApps[gone]; ok {
			t.Errorf("%s is no agent of eipgam's, yet it has a Restart", gone)
		}
	}
	if _, ok := restartApps["codex"]; ok {
		t.Error("the Codex app has its own Restart (codex_app.go)")
	}
}

// Which command lines are a script run on an app's executable: the real
// ones of WorkBuddy 5.6 on Windows, its sidecar and prewarmed CLI as
// 5.5.6's app.asar spawns them, and Electron's own processes.
func TestRunsScript(t *testing.T) {
	const win = `C:\Program Files\WorkBuddy\WorkBuddy.exe`
	const mac = `/Applications/WorkBuddy AI.app/Contents/MacOS/Electron`
	cases := []struct {
		cmd, exe string
		want     bool
	}{
		{`"` + win + `"`, win, false},
		{`"` + win + `" --type=gpu-process --no-sandbox --user-data-dir="C:\Users\yeton\.workbuddy\app" --mojo-platform-channel-handle=1272 /prefetch:2`, win, false},
		{`"` + win + `" --type=renderer --user-data-dir="C:\Users\John Smith\.workbuddy\app" --cors-schemes=vscode-file,wb-extension,buddy-app`, win, false},
		{`"` + win + `" --type=crashpad-handler --user-data-dir=C:\Users\yeton\.workbuddy\app /prefetch:4 --no-rate-limit`, win, false},
		{`"` + win + `" "workbuddy://open?task=1"`, win, false},
		{`"` + win + `" --disable-warning=ExperimentalWarning "C:\Program Files\WorkBuddy\resources\app.asar\main\daemon-app-server-entry.js" --stdio`, win, true},
		{`"` + win + `" "C:\Program Files\WorkBuddy\resources\app.asar.unpacked\cli\bin\codebuddy" --prewarm --prewarm-id wb-pool-1`, win, true},
		{`"` + win + `" "C:\Program Files\WorkBuddy\resources\app.asar.unpacked\cli\bin\codebuddy" --prewarm`, "", true},
		{mac, mac, false},
		{mac + ` -psn_0_123456`, mac, false},
		{mac + ` /Applications/WorkBuddy AI.app/Contents/Resources/app.asar/main/sidecar-entry.js --token t`, mac, true},
		{mac + ` /Applications/WorkBuddy AI.app/Contents/Resources/app.asar.unpacked/cli/bin/codebuddy --prewarm --prewarm-id wb-pool-1`, mac, true},
		// #3700's (Raters0's) ps: the sidecar's codebuddy --serve child.
		{mac + ` /Applications/WorkBuddy AI.app/Contents/Resources/app.asar.unpacked/cli/bin/codebuddy --serve --no-session-persistence`, mac, true},
		{`/Applications/WorkBuddy AI.app/Contents/Frameworks/WorkBuddy Helper (Renderer).app/Contents/MacOS/WorkBuddy Helper (Renderer) --type=renderer --user-data-dir=/Users/u/Library/Application Support/WorkBuddy`, mac, false},
		{`/Applications/Claude.app/Contents/MacOS/Claude`, "/Applications/Claude.app/Contents/MacOS/Claude", false},
		{`/Applications/X.app/Contents/MacOS/X /tmp/server.MJS`, "", true},
		{``, "", false},
	}
	for _, c := range cases {
		if got := runsScript(c.cmd, c.exe); got != c.want {
			t.Errorf("%s: %v, want %v", c.cmd, got, c.want)
		}
	}
}

// A change no file's time shows (Claude Desktop's models, from the
// gateway's catalog) is put at when it is first seen, not at the files'
// last write: a Desktop started between the two still read the old list.
func TestChangedAtWithoutAWrite(t *testing.T) {
	restartSandbox(t)
	f := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(f, []byte("{}"), 0o600)
	old := time.Now().Add(-2 * time.Hour)
	os.Chtimes(f, old, old)
	a := &Agent{ID: "changed-at-test"}
	list := "a"
	reads := func() string { return list }
	if got := a.changedAt([]string{f}, reads); !got.Equal(old) {
		t.Fatalf("first seen: %v, want the file's %v", got, old)
	}
	if got := a.changedAt([]string{f}, reads); !got.Equal(old) {
		t.Fatalf("again: %v, want %v", got, old)
	}
	list = "a b"
	before := time.Now()
	got := a.changedAt([]string{f}, reads)
	if got.Before(before) {
		t.Fatalf("changed with no write: %v, want now (after %v)", got, before)
	}
	if again := a.changedAt([]string{f}, reads); !again.Equal(got) {
		t.Fatalf("moved on a second read: %v, then %v", got, again)
	}
	// a change with a write is at the write, as before
	list = "a b c"
	later := time.Now().Add(time.Hour)
	os.Chtimes(f, later, later)
	if got := a.changedAt([]string{f}, reads); !got.Equal(later) {
		t.Fatalf("with a write: %v, want %v", got, later)
	}
}
