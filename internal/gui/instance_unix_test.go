//go:build !windows

package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A second eipgam on the same config finds the first one's lock held, a
// copy elsewhere (eipgam-dev) has its own, and a lock whose holder ended,
// as when the Mac shuts down with eipgam running, is free again (Crispin
// on Discord: two magpies after each boot).
func TestOneMagpieAtATime(t *testing.T) {
	dir := t.TempDir()
	if os.Getenv("EIPGAM_HOLD_LOCK") != "" {
		if held, err := holdInstance(os.Getenv("EIPGAM_HOLD_LOCK")); err != nil || !held {
			os.Exit(3)
		}
		os.Stdout.WriteString("held\n")
		select {}
	}
	lock := instanceLock(dir, "/Applications/eipgam.app/Contents/MacOS/eipgam")
	if dev := instanceLock(dir, "/src/eipgam/eipgam-dev"); dev == lock {
		t.Fatal("eipgam-dev shares the app's lock")
	}
	// every installed copy shares the lock: another eipgam.app (one the
	// Mac reopens, one run translocated from Downloads) and Homebrew's of
	// any version (inaction on Discord: two birds after a restart)
	for _, other := range []string{
		"/Users/u/Downloads/eipgam.app/Contents/MacOS/eipgam",
		"/private/var/folders/x/T/AppTranslocation/1F2E/d/eipgam.app/Contents/MacOS/eipgam",
		"/opt/homebrew/Cellar/eipgam/0.1.900/bin/eipgam",
	} {
		if instanceLock(dir, other) != lock {
			t.Errorf("%s has a lock of its own", other)
		}
	}
	if filepath.Dir(lock) != dir {
		t.Fatalf("lock %s not in the config dir", lock)
	}

	// the first eipgam: another process, killed as a shutdown would
	cmd := exec.Command(os.Args[0], "-test.run=^TestOneMagpieAtATime$")
	cmd.Env = append(os.Environ(), "EIPGAM_HOLD_LOCK="+lock)
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	if _, err := out.Read(buf); err != nil || string(buf[:4]) != "held" {
		cmd.Process.Kill()
		t.Fatalf("the first eipgam didn't take the lock: %q %v", buf, err)
	}
	if held, err := holdInstance(lock); err != nil || held {
		cmd.Process.Kill()
		t.Fatalf("a second eipgam took the held lock: %v %v", held, err)
	}
	cmd.Process.Kill()
	cmd.Wait()
	if held, err := holdInstance(lock); err != nil || !held {
		t.Fatalf("the lock of a eipgam that was killed stayed held: %v %v", held, err)
	}
	heldInstance.Close()
	heldInstance = nil
}
