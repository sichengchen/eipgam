package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// writeMode writes s to path with mode, making its folder first.
func writeMode(t *testing.T, path, s string, mode os.FileMode) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(s), mode); err != nil {
		t.Fatal(err)
	}
}
