package sessions

import (
	"testing"
)

// A database in a distro is read through Windows' share of it, where an
// open in place answers SQLITE_BUSY (openAppStore), so it is read as
// OpenUntouched does; this computer's own, and a distro's folder that
// leads to Windows' drive, are read in place.
func TestOnWSLShare(t *testing.T) {
	for p, want := range map[string]bool{
		`\\wsl.localhost\Ubuntu\home\me\.gemini\antigravity\conversation_summaries.db`: true,
		`\\WSL.LOCALHOST\Ubuntu-24.04\home\me\x.db`:                                    true,
		`\\wsl$\Ubuntu\home\me\x.db`:                                                   true,
		`//wsl.localhost/Ubuntu/home/me/x.db`:                                          true,
		`C:\Users\me\.gemini\antigravity\conversation_summaries.db`:                    false,
		`\\server\wsl.localhost\x.db`:                                                  false,
		`/home/me/.gemini/antigravity/conversation_summaries.db`:                       false,
	} {
		if got := onWSLShare(p); got != want {
			t.Errorf("onWSLShare(%s) = %v, want %v", p, got, want)
		}
	}
}
