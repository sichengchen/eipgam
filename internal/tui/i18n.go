package tui

// The TUI speaks English. tr and trf look a word up in words by language,
// the English itself the key, and fall back to it; this fork keeps no
// other language's table.

import "fmt"

// lang is the language the TUI speaks, set once as Run starts from the
// GUI's setting (settings.Lang, through uilang.Pick). Tests leave it "en".
var lang = "en"

// speak is the table lang reads, English when it has none.
func speak(l string) string {
	if l == "zh-TW" {
		return "zh"
	}
	if _, ok := words[l]; ok {
		return l
	}
	return "en"
}

// tr is s in the TUI's language: s itself, the English, when the table
// has no word for it.
func tr(s string) string {
	if w, ok := words[lang][s]; ok {
		return w
	}
	return s
}

// trf is tr's format, filled in: the translated format keeps the
// English one's verbs (%[n]s to reorder them).
func trf(format string, args ...any) string {
	return fmt.Sprintf(tr(format), args...)
}

// words is each language's table, keyed by the English; empty, as the
// fork keeps English only.
var words = map[string]map[string]string{}
