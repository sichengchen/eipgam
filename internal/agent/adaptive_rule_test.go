package agent

import (
	"testing"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/provider"
)

// #6778 (larryhaonlp): DSH's Xhigh reached agy-cli/gemini-4-hydrogen, on
// magpie's Anthropic route, as high. pi-ai asks a model on Messages for a
// budget unless its compat says forceAdaptiveThinking, and the budget path
// clamps xhigh and max to high's 16384, which agy-cli reads as high. A
// Gemini 3 or later is now told to think adaptively there, by the one rule
// dsh and Pi (both pi-ai) are written by; a Gemini 2.x, another route and
// a model that takes a budget are not.
func TestPiAgentsAskGeminiAdaptively(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{"agy-cli/gemini-4-hydrogen", true},
		{"relay/gemini-3.8-flash", true},
		{"anth/claude-sonnet-5", true},
		{"zcode/GLM-5.3", true},
		{"relay/gemini-2.5-pro", false},
		{"kimi/kimi-k3", false},
		{"minimax/MiniMax-M3.1-Flash", false},
	} {
		m := catalog.Model{ID: tc.id, Context: 1048576, Output: 65536, APIs: []string{string(provider.Anthropic)}, Efforts: []string{"low", "medium", "high", "xhigh"}}
		for _, on := range []string{"", "anthropic-messages", "openai-completions"} {
			e := piModelOn(m, "http://127.0.0.1:3425", true, on)
			c, _ := e["compat"].(map[string]any)
			if want := tc.want && on != "openai-completions"; (c["forceAdaptiveThinking"] == true) != want {
				t.Errorf("Pi %s on %q: compat %v, want forceAdaptiveThinking %v", tc.id, on, c, want)
			}
		}
	}
}
