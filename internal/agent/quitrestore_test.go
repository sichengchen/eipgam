package agent

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const quitUserSettings = "{\n  \"theme\": \"dark\",\n  \"defaultProvider\": \"xai\",\n  \"defaultModel\": \"grok-4.6\"\n}\n"
const quitUserModels = "{\n  \"providers\": {\n    \"mine\": {\n      \"baseUrl\": \"http://127.0.0.1:9/v1\",\n      \"api\": \"openai-completions\",\n      \"apiKey\": \"MINE_KEY\",\n      \"models\": [{\"id\": \"m1\"}]\n    }\n  }\n}\n"

const quitUserKimi = "theme = \"dark\"\n"

// quitHome is a sandbox with Pi, its own settings.json and models.json, and
// Kimi Code, its own config.toml, both connected. Each puts its files back
// byte for byte.
func quitHome(t *testing.T) (piA, km *Agent, files map[string][]byte) {
	t.Helper()
	home := syncHome(t)
	piA, km = pi(home), kimi(home)
	files = map[string][]byte{}
	writeFile(t, piA.Path, quitUserSettings)
	writeFile(t, filepath.Join(piA.Dir, "models.json"), quitUserModels)
	files[piA.Path] = []byte(quitUserSettings)
	files[filepath.Join(piA.Dir, "models.json")] = []byte(quitUserModels)
	writeFile(t, km.Path, quitUserKimi)
	files[km.Path] = []byte(quitUserKimi)
	for _, a := range []*Agent{piA, km} {
		if err := a.Connect(); err != nil || !a.Wired() {
			t.Fatalf("%s: connect: %v", a.ID, err)
		}
	}
	return
}

func sameBytes(t *testing.T, files map[string][]byte) {
	t.Helper()
	for p, want := range files {
		if b, _ := os.ReadFile(p); !bytes.Equal(b, want) {
			t.Fatalf("%s not as it was:\n%s", p, b)
		}
	}
}

func stashMark(a *Agent) { stash(map[string]string{a.ID + disconnectedKey: "1"}) }

// Quitting with the setting on puts every connected agent's files back as
// they were before eipgam, byte for byte, leaves no disconnected mark, and
// the next start connects them again.
func TestRestoreOnQuitAndRewireOnStart(t *testing.T) {
	piA, km, files := quitHome(t)
	all := []*Agent{piA, km}
	ids, err := RestoreOnQuit(all)
	if err != nil || len(ids) != 2 {
		t.Fatalf("restored %v: %v", ids, err)
	}
	for _, a := range all {
		if a.Wired() || a.Disconnected() {
			t.Fatalf("%s: wired %v, disconnected mark %v", a.ID, a.Wired(), a.Disconnected())
		}
	}
	sameBytes(t, files)
	got, err := RewireOnStart(all)
	if err != nil || len(got) != 2 {
		t.Fatalf("rewired %v: %v", got, err)
	}
	for _, a := range all {
		if !a.Wired() || a.Check() != "" {
			t.Fatalf("%s not connected again:\n%s", a.ID, readFile(a.Path))
		}
	}
	// the record is used once
	if again, _ := RewireOnStart(all); len(again) != 0 {
		t.Fatalf("rewired twice: %v", again)
	}
}

// An agent the user disconnected stays disconnected: a quit doesn't touch
// it, and a start doesn't connect it, nor one they disconnected after the
// quit.
func TestRestoreOnQuitLeavesUserDisconnected(t *testing.T) {
	piA, km, files := quitHome(t)
	all := []*Agent{piA, km}
	if err := km.Disconnect(); err != nil {
		t.Fatal(err)
	}
	ids, err := RestoreOnQuit(all)
	if err != nil || len(ids) != 1 || ids[0] != "pi" {
		t.Fatalf("restored %v: %v", ids, err)
	}
	sameBytes(t, files)
	if got, _ := RewireOnStart(all); len(got) != 1 || got[0] != "pi" {
		t.Fatalf("rewired %v", got)
	}
	if km.Wired() || !km.Disconnected() {
		t.Fatalf("Kimi Code came back: wired %v, mark %v", km.Wired(), km.Disconnected())
	}
	// disconnected by hand while eipgam was closed
	if _, err := RestoreOnQuit(all); err != nil {
		t.Fatal(err)
	}
	stashMark(piA)
	if got, _ := RewireOnStart(all); len(got) != 0 || piA.Wired() {
		t.Fatalf("rewired %v, wired %v", got, piA.Wired())
	}
}

// With nothing restored on quit, a start connects nothing: the setting's
// off behaviour is that a connected agent keeps eipgam.
func TestRewireOnStartWithoutRestore(t *testing.T) {
	piA, km, _ := quitHome(t)
	if err := km.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if got, _ := RewireOnStart([]*Agent{piA, km}); len(got) != 0 || km.Wired() || !piA.Wired() {
		t.Fatalf("rewired %v", got)
	}
}
