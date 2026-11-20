package update

import (
	"path/filepath"
	"testing"

	"github.com/sichengchen/eipgam/internal/proc"
)

// Scoop runs an app from apps\<name>\current, a junction to the version
// folder, and Go's EvalSymlinks fails on it. The binary there is still
// judged as Scoop's, so `eipgam update` names `scoop update` rather than
// failing with "The system cannot find the path specified".
func TestScoopJunctionStillPackaged(t *testing.T) {
	root := filepath.Join(t.TempDir(), "scoop")
	ver := filepath.Join(root, "apps", "eipgam", "0.1.4478")
	writeFile(t, filepath.Join(ver, "eipgam.exe"), "MZ")
	writeFile(t, filepath.Join(ver, "scoop-install.json"), `{"bucket":"eipgam"}`)
	writeFile(t, filepath.Join(ver, "scoop-manifest.json"), `{"version":"0.1.4478"}`)
	cur := filepath.Join(root, "apps", "eipgam", "current")
	if out, err := proc.Command("cmd", "/c", "mklink", "/J", cur, ver).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J: %v %s", err, out)
	}
	exe := resolved(filepath.Join(cur, "eipgam.exe"))
	p := PackagedBy(exe)
	if p == nil || p.Manager != "scoop" || p.Package != "eipgam" {
		t.Fatalf("PackagedBy(%q) = %+v", exe, p)
	}
}
