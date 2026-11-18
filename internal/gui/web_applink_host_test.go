package gui

import (
	"testing"
)

func TestOpenOnHostLeavesWebLinksToThePage(t *testing.T) {
	if err := webHost.OpenURL(webHost{}, "https://example.invalid/x"); err != nil {
		t.Fatal(err)
	}
}
