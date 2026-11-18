package library

import (
	"path/filepath"
	"strings"
	"testing"
)

// Only one "---" at the left edge, before anything but comments, opens the
// list (#2667). One anywhere else, or a second document, still fails the
// file instead of being kept as if it were the list's.
func TestDshDocStartOnlyFirst(t *testing.T) {
	h := sandbox(t)
	p := filepath.Join(h, "cordis.patch.yml")
	for _, raw := range []string{
		"---\n- id: a\n",
		"--- \n- id: a\n",
		"--- # patches\n- id: a\n",
		"# patches\n---\n- id: a\n",
		"---\n# patches\n\n- id: a\n",
		"---\n[ { id: a } ]\n",
	} {
		write(t, p, raw)
		d, err := dshRead(p)
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if len(d.items) != 1 || d.items[0][0] != "- id: a" || !strings.Contains(strings.Join(d.head, "\n"), "---") {
			t.Fatalf("%q: head %q items %q", raw, d.head, d.items)
		}
	}
	for _, raw := range []string{
		"---\n---\n- id: a\n",
		"- id: a\n---\n- id: b\n",
		"---\n- id: a\n---\n- id: b\n",
		"  ---\n- id: a\n",
		"----\n- id: a\n",
		"--- x\n- id: a\n",
		"[]\n---\n- id: a\n",
	} {
		write(t, p, raw)
		if _, err := dshRead(p); err == nil || !strings.Contains(err.Error(), "not a list of entries magpie can edit") {
			t.Fatalf("%q: want the file refused, got %v", raw, err)
		}
	}
}
