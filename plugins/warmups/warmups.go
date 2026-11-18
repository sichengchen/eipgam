// Package warmups is warm-up: start a subscription's usage window at a
// time the user picks. While one is set, it stays on.
package warmups

import (
	"github.com/yetone/magpie/internal/pluginrt"
	"github.com/yetone/magpie/internal/settings"
)

func init() {
	pluginrt.Register(pluginrt.Manifest{
		ID: "warmups", API: "^1", Requires: []string{"window", "settings"},
		Name: "Warm-ups", Sub: "Start a subscription's usage window at a time you pick",
		Opens: "providers",
		One:   "1 warm-up on", Many: "{n} warm-ups on",
		InUse: func() (int, error) {
			s, err := settings.LoadChecked()
			if err != nil {
				return 0, err
			}
			n := count(s.CodexWarmup != "", s.ClaudeWarmup != "", s.CodexWarmAt != "" || len(s.CodexWarmAts) > 0,
				s.ClaudeWarmAt != "" || len(s.ClaudeWarmAts) > 0)
			n += len(s.CodexWarmAtOf)
			return n, nil
		},
		Parts: []pluginrt.Part{{Slot: "part", Key: "warmups"}},
	})
}

func count(on ...bool) int {
	n := 0
	for _, b := range on {
		if b {
			n++
		}
	}
	return n
}
