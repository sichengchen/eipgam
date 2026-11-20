package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sichengchen/eipgam/internal/settings"
	"github.com/sichengchen/eipgam/internal/update"
)

// nnlrl_75403 on Discord: a `eipgam serve` that another eipgam uses as its
// Remote eipgam is asked to update when that one has. It puts the latest
// in where it runs from, once at a time, and never asks for a password;
// in the Docker image, a package manager's copy or a folder it can't
// write, it says so instead.
func TestServeUpdateAsked(t *testing.T) {
	t.Setenv("EIPGAM_UPDATE_FEED", "http://127.0.0.1:1")
	dir := t.TempDir()
	exe := filepath.Join(dir, "eipgam")
	if err := os.WriteFile(exe, []byte("eipgam"), 0o755); err != nil {
		t.Fatal(err)
	}
	wasV, wasX, wasC, wasP, wasPut, wasL := version, updateExecutable, updateInContainer, updatePackagedBy, servePutIn, serveLatest
	t.Cleanup(func() {
		version, updateExecutable, updateInContainer, updatePackagedBy, servePutIn, serveLatest = wasV, wasX, wasC, wasP, wasPut, wasL
	})
	version = "v0.1.100"
	updateExecutable = func() (string, error) { return exe, nil }
	updateInContainer = func() bool { return false }
	updatePackagedBy = func(string) *update.Packaged { return nil }
	release := make(chan struct{})
	var puts atomic.Int32
	var mu sync.Mutex
	var putAt string
	serveLatest = func(context.Context) (*update.Release, error) {
		return &update.Release{Version: "0.1.200"}, nil
	}
	servePutIn = func(_ context.Context, rel *update.Release, app, at string) error {
		puts.Add(1)
		mu.Lock()
		putAt = at
		mu.Unlock()
		<-release
		return nil
	}
	ask := serveUpdateAsked(&serveFollow{exe: exe})

	if got := ask("v0.1.100"); got.State != "current" {
		t.Fatalf("same version: %+v", got)
	}
	if err := settings.Save(settings.Settings{NoAutoUpdate: true}); err != nil {
		t.Fatal(err)
	}
	if got := ask("v0.1.200"); got.State != "off" {
		t.Fatalf("updates off: %+v", got)
	}
	if err := settings.Save(settings.Settings{}); err != nil {
		t.Fatal(err)
	}
	if got := serveUpdateAsked(nil)("v0.1.200"); got.State != "unsupported" || got.Why != "not-writable" {
		t.Fatalf("no program to follow: %+v", got)
	}
	updatePackagedBy = func(string) *update.Packaged { return &update.Packaged{Manager: "pacman", Package: "eipgam-bin"} }
	if got := ask("v0.1.200"); got.State != "unsupported" || got.Why != "package" || got.Command != "yay -S eipgam-bin" {
		t.Fatalf("a package: %+v", got)
	}
	updatePackagedBy = func(string) *update.Packaged { return nil }
	if puts.Load() != 0 {
		t.Fatal("put in when it said it wouldn't")
	}

	// asked twice while the first is under way: one download
	if got := ask("v0.1.200"); got.State != "updating" {
		t.Fatalf("newer asker: %+v", got)
	}
	if got := ask("v0.1.200"); got.State != "updating" {
		t.Fatalf("asked again: %+v", got)
	}
	for deadline := time.Now().Add(3 * time.Second); puts.Load() == 0 && time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
	}
	time.Sleep(50 * time.Millisecond)
	if n := puts.Load(); n != 1 {
		t.Fatalf("%d downloads, want 1", n)
	}
	mu.Lock()
	at := putAt
	mu.Unlock()
	if at != exe {
		t.Fatalf("put in at %q, want %q", at, exe)
	}
	close(release)
	for deadline := time.Now().Add(3 * time.Second); serveUpdating.Load() && time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
	}
	if serveUpdating.Load() {
		t.Fatal("still updating after the download ended")
	}

	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return // a folder this user can't write takes a Unix non-root user
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	if got := ask("v0.1.200"); got.State != "unsupported" || got.Why != "not-writable" {
		t.Fatalf("a folder it can't write: %+v", got)
	}
	updateInContainer = func() bool { return true }
	if got := ask("v0.1.200"); got.State != "unsupported" || got.Why != "container" {
		t.Fatalf("in a container: %+v", got)
	}
	if n := puts.Load(); n != 1 {
		t.Fatalf("%d downloads, want 1", n)
	}
}
