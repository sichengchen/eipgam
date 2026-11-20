//go:build darwin && cgo

package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestDockFlapFrames writes the Dock tile's frames, cut from the icon of
// EIPGAM_DOCK_APP (an installed eipgam.app), into EIPGAM_DOCK_FRAMES, to
// look at by eye.
func TestDockFlapFrames(t *testing.T) {
	app, dir := os.Getenv("EIPGAM_DOCK_APP"), os.Getenv("EIPGAM_DOCK_FRAMES")
	if app == "" || dir == "" {
		t.Skip("set EIPGAM_DOCK_APP and EIPGAM_DOCK_FRAMES")
	}
	for f := 0; f <= 30; f++ {
		if !dockFramePNG(app, filepath.Join(dir, fmt.Sprintf("f%02d.png", f)), float64(f)/30) {
			t.Fatalf("frame %d not written", f)
		}
	}
}
