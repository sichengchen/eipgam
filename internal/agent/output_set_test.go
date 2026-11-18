package agent

import (
	"os"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/provider"
)

// #2064: DeepSeek V4.1 Flash with Max output 384k set on its provider
// reached VS Code as 250000. The
// agents' own trims of a vendor's figure stay for a model the user set
// nothing on; the user's number is handed on as it is.
func TestAgentsTakeTheUsersMaxOutput(t *testing.T) {
	syncHome(t)
	os.WriteFile(catalog.CachePath(), []byte(`{"zai":{"models":{"glm-4.6":{"id":"glm-4.6","name":"GLM-4.6","limit":{"context":1000000,"output":384000}}}}}`), 0o644)
	catalog.Reset()

	outputs := func() map[string]any {
		got := map[string]any{}
		vs := vscodeGroupJSON(vscodeStable, vscodeEndpoint)["models"].([]any)
		got["vscode"] = vs[0].(map[string]any)["maxOutputTokens"]
		oai := vscodeGroupJSON(vscodeStable, vscodeOAI)["models"].([]any)[0].(map[string]any)
		got["vscode-oai"] = oai["maxOutputTokens"]
		got["vscode-oai-in"] = oai["maxInputTokens"]
		return got
	}

	// nothing set: each agent's own trim of the vendor's 384000
	got := outputs()
	want := map[string]any{"vscode": 250000, "vscode-oai": 250000, "vscode-oai-in": 750000}
	for k, w := range want {
		if got[k] != w {
			t.Fatalf("nothing set, %s: %v (%T), want %v; all %v", k, got[k], got[k], w, got)
		}
	}

	// Max output 384k on the provider, as the reporter set it
	if err := provider.SetModelOutputs("relay", map[string]int{"*": 384000}); err != nil {
		t.Fatal(err)
	}
	got = outputs()
	want = map[string]any{"vscode": 384000, "vscode-oai": 384000, "vscode-oai-in": 616000}
	for k, w := range want {
		if got[k] != w {
			t.Fatalf("384k set, %s: %v (%T), want %v; all %v", k, got[k], got[k], w, got)
		}
	}
}
