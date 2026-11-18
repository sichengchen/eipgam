package agent

import (
	"testing"

	"github.com/yetone/magpie/internal/usage"
)

// Every agent's requests are told by what its own description says.
func TestAgentOf(t *testing.T) {
	cases := map[string]string{
		"claude-cli/2.1.0 (external, cli)":         "claude",
		"codex_cli_rs/0.40.0 (Mac OS 26.0; arm64)": "codex",
		"GeminiCLI/0.9.0 (darwin; arm64)":          "gemini",
		"opencode/1.2.3":                           "opencode",
		"curl/8.4.0":                               "curl",
		"pi":                                       "pi",
		"grok-pager/0.2.1":                         "grok",
		"":                                         "other",
		// Kimi Code 2.x and its desktop app, as jiabubu's requests named
		// them; the old Python kimi-cli's KimiCLI/
		"kimi-code-cli/2.4.1":     "kimi",
		"kimi-code-desktop/1.2.1": "kimi-code-desktop",
		"KimiCLI/1.4.0":           "kimi",
	}
	for ua, want := range cases {
		if got := usage.AgentOf(ua); got != want {
			t.Errorf("%q: got %q want %q", ua, got, want)
		}
	}
}

// Every agent's own gateway key (gateway.TokenFor) names it in usage: one
// that sends no User-Agent of its own is told by that key alone (#1605's
// check of the class).
func TestEveryAgentNamedByItsKey(t *testing.T) {
	for _, a := range Clients() {
		if got := usage.AgentOf(a.ID); got != a.ID {
			t.Errorf("%s's key names %q", a.ID, got)
		}
	}
}
