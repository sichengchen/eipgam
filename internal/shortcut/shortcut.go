// Package shortcut puts eipgam in the Start menu on Windows. There it is
// one exe with no installer, so nothing else would: the first start of the
// desktop app makes a eipgam shortcut in the user's Start menu opening the
// exe it runs as, and Windows search finds it by that. It is made once:
// eipgam remembers it did, and a shortcut the user deleted stays deleted
// (#508); one still there is pointed at the exe again when that was moved.
// A portable eipgam (a data folder beside it) makes none and registers
// nothing, leaving no trace on the machine. Elsewhere it does nothing —
// the Mac has the app in Applications, Linux its own ways.
package shortcut

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/sichengchen/eipgam/internal/appdir"
	"github.com/sichengchen/eipgam/internal/lazyre"
)

// Name is what the shortcut is called, and what Windows search finds.
const Name = "eipgam"

// StateName is the file in eipgam's folder recording that the shortcut was
// made, and for which exe.
const StateName = "start-menu-shortcut"

// what Ensure asks of the system; the tests put fakes in
var (
	// startMenu is where the shortcut goes (…\Start Menu\Programs\eipgam.lnk)
	startMenu = startMenuLink
	// writeLink has the shortcut at lnk open exe, and reports whether it had to
	writeLink = writeShortcut
	// register has Win+R and `start eipgam` find exe
	register = registerAppPath
	// registered reports whether a eipgam registered itself there before
	registered = registeredAppPath
)

// Ensure makes the shortcut on the first start, keeps it pointed at this
// eipgam after that while it is there, and never makes it again once the
// user deleted it. A portable eipgam makes none. It fails quietly — eipgam
// runs the same without one — and is skipped with EIPGAM_NO_SHORTCUT=1
// (tests, sandboxes).
func Ensure() {
	if os.Getenv("EIPGAM_NO_SHORTCUT") == "1" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	if err := ensure(Target(exe), appdir.Portable(), filepath.Join(appdir.Config(), StateName)); err != nil {
		log.Println("start menu shortcut:", err)
	}
}

// errNoStartMenu is startMenu's answer where there is none (off Windows).
var errNoStartMenu = errors.New("no Start menu here")

// ensure is Ensure for exe, portable being the data folder of a portable
// eipgam ("" installed) and state the file remembering the shortcut was
// made.
func ensure(exe, portable, state string) error {
	if portable != "" {
		return nil
	}
	lnk, err := startMenu()
	if errors.Is(err, errNoStartMenu) {
		return nil
	}
	// a eipgam from before the state file registered itself at every start
	before := registered()
	register(exe)
	if err != nil {
		return err
	}
	_, gone := os.Stat(lnk)
	if gone != nil {
		if _, err := os.Stat(state); err == nil {
			// made once already: the user deleted it, and it stays deleted
			return nil
		}
		if before {
			// an older eipgam made it at its every start, and it is gone:
			// the user deleted it, so it isn't made again now either
			return remember(state, exe)
		}
	}
	if _, err := writeLink(lnk, exe); err != nil {
		return err
	}
	return remember(state, exe)
}

// remember records in state that the shortcut was made, for exe.
func remember(state, exe string) error {
	if err := os.MkdirAll(filepath.Dir(state), 0o755); err != nil {
		return err
	}
	return os.WriteFile(state, []byte(exe+"\n"), 0o644)
}

// aside is what an update moves the exe it replaces to, or stages the new
// one as: eipgam.exe.old, eipgam.exe.old-2…, eipgam.exe.new
var aside = lazyre.New(`(?i)(\.exe)\.(old(-\d+)?|new)$`)

// Target is what the shortcut opens for exe: exe itself, but for a eipgam
// left running from where an update moved it aside, the exe in its place,
// which is what the next start runs. An update replaces the exe where it
// is, so the shortcut stays good through it.
func Target(exe string) string {
	return aside.ReplaceAllString(exe, "$1")
}

// Stale reports whether a shortcut opening target in dir is not one for
// exe: there is none yet, or the exe was moved. Windows paths are the same
// in any case.
func Stale(target, dir, exe string) bool {
	same := func(a, b string) bool {
		return a != "" && strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return !same(target, exe) || !same(dir, filepath.Dir(exe))
}

// UninstallArgs are what Windows' Installed apps runs eipgam with for its
// Uninstall: the app's window on Settings › Uninstall eipgam, which asks
// first and puts every agent back before eipgam's files go. It is started
// with no console, so the terminal's `eipgam uninstall` couldn't ask.
var UninstallArgs = []string{"uninstall", "--window"}

// UninstallCommand is the command line Installed apps runs for exe.
func UninstallCommand(exe string) string {
	return `"` + exe + `" ` + strings.Join(UninstallArgs, " ")
}

// what Remove asks of the system besides startMenu; the tests put fakes in
var unregister = unregisterAppPath

// Remove takes out what Ensure made, for eipgam uninstall: the Start menu
// shortcut, the App Paths and Installed apps entries while they name exe
// (or an exe gone), and
// the record of having made them. A portable eipgam made none, and an
// entry another copy of eipgam registered stays that copy's.
func Remove(exe string) error {
	var errs []error
	if lnk, err := startMenu(); err == nil {
		if err := os.Remove(lnk); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	} else if !errors.Is(err, errNoStartMenu) {
		errs = append(errs, err)
	}
	if err := unregister(Target(exe)); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
