package gui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sichengchen/eipgam/internal/update"
)

// The AUR's eipgam-bin installs /usr/bin/eipgam, and with pkexec there
// eipgam put its own download over the package's file on a restart, so
// pacman's database named one version and the disk had another, and the
// next yay upgrade wrote over it again (shuhai on Discord). A binary a
// package manager installed now says a version is out and the command
// that updates it, and never downloads or replaces anything, even where
// it may write or ask for the password.
func TestUpdatePackagedLeavesItToTheManager(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/asset" {
			t.Errorf("a packaged binary downloaded the release")
		}
		json.NewEncoder(w).Encode(update.Release{Version: "0.1.11", URL: "https://github.com/yetone/magpie-releases/releases/tag/v0.1.11", Assets: map[string]update.Asset{
			update.BinaryAsset(): {URL: "http://" + r.Host + "/asset"},
		}})
	}))
	defer srv.Close()
	t.Setenv("EIPGAM_UPDATE_FEED", srv.URL)
	old := Version
	Version = "0.1.10"
	defer func() { Version = old }()
	wasE, wasP := canElevate, packagedBy
	defer func() { canElevate, packagedBy = wasE, wasP }()
	canElevate = func() bool { return true }

	dir := t.TempDir() // writable: ownership alone decides
	exe := filepath.Join(dir, "eipgam")
	if err := os.WriteFile(exe, []byte("eipgam"), 0o755); err != nil {
		t.Fatal(err)
	}
	packagedBy = func(string) *update.Packaged { return &update.Packaged{Manager: "pacman", Package: "eipgam-bin"} }
	u := &updater{}
	u.placeExe(exe)
	if u.exe != "" {
		t.Fatalf("a packaged binary is to be replaced: %q", u.exe)
	}
	u.check()
	j := u.json()
	if j.State != "available" || j.Stuck != "package" || j.Packaged == nil || j.Packaged.Manager != "pacman" || j.Packaged.Package != "eipgam-bin" || j.Packaged.Command != "yay -S eipgam-bin" || j.Latest != "0.1.11" {
		t.Fatalf("%+v %+v", j, j.Packaged)
	}
	if u.install(true) {
		t.Fatal("install put something in")
	}
	if b, _ := os.ReadFile(exe); string(b) != "eipgam" {
		t.Fatalf("the package's file was changed: %q", b)
	}

	// a binary no package owns updates itself as before
	packagedBy = func(string) *update.Packaged { return nil }
	u = &updater{}
	u.placeExe(exe)
	if u.exe != exe || u.stuck != "" {
		t.Fatalf("not packaged: exe %q, stuck %q", u.exe, u.stuck)
	}
}

func TestMacUpdateLeavesInstallationToHomebrew(t *testing.T) {
	t.Setenv("EIPGAM_UPDATE_FEED", "")
	u := &updater{}
	u.place("darwin", true)
	j := u.json()
	const command = "brew upgrade --cask sichengchen/tap/eipgam"
	if u.bundle != "" || u.exe != "" || j.Packaged == nil || !j.Packaged.Cask || j.Packaged.Command != command {
		t.Fatalf("macOS update targets: bundle %q, exe %q, package %+v", u.bundle, u.exe, j.Packaged)
	}
	if u.install(true) {
		t.Fatal("the app updater took over a Homebrew installation")
	}
	old := Version
	Version = "0.2.0"
	t.Cleanup(func() { Version = old })
	remote := u.updateAsked("0.2.1")
	if remote.State != "unsupported" || remote.Why != "package" || remote.Command != command {
		t.Fatalf("remote update: %+v", remote)
	}
}
