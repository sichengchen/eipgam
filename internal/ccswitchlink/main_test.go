package ccswitchlink

import (
	"errors"
	"testing"

	"github.com/sichengchen/eipgam/internal/testenv"
)

// The tests run in a home of their own, never the user's real files.
func TestMain(m *testing.M) { testenv.Main(m) }

func TestIs(t *testing.T) {
	for l, want := range map[string]bool{
		"ccswitch://v1/import?resource=provider&app=claude&name=x": true,
		"CCSwitch://v1/import":     true,
		"eipgam://import?preset=x": false,
		"https://ccswitch.io":      false,
		"":                         false,
	} {
		if Is(l) != want {
			t.Errorf("Is(%q) = %v", l, !want)
		}
	}
	if err := Forward("eipgam://import?preset=x"); err == nil || errors.Is(err, ErrNotInstalled) {
		t.Fatalf("a eipgam:// link was taken: %v", err)
	}
}
