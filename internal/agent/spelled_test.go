package agent

import (
	"path/filepath"
	"testing"
)

// An agent that spells a model provider/model, eipgam's as eipgam/<ref>:
// one of its own providers named as one of eipgam's (its own relay, and
// eipgam's relay) is its own model, not eipgam's. The row isn't connected
// on it, and Disconnect from eipgam puts it back and is done, rather than
// leaving the row connected on the model it went back to (#835, an agent's
// own deepseek/deepseek-v4-pro).
func TestOwnProviderNamedAsMagpies(t *testing.T) {
	ids := []string{"agy", "opencode", "pi", "zed", "vscode", "kimi", "grok"}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			syncHome(t)
			a, err := Find(id)
			if err != nil {
				t.Fatal(err)
			}
			f := &a.Fields[0]
			if err := f.Set("relay/glm-4.6"); err != nil {
				t.Fatal(err)
			}
			if v := f.Get(); v != "relay/glm-4.6" {
				t.Fatalf("own model reads %q", v)
			}
			if a.Wired() {
				t.Fatal("its own relay/glm-4.6 reads as eipgam's")
			}
			if err := a.Connect(); err != nil {
				t.Fatal(err)
			}
			if !a.Wired() {
				t.Fatalf("not connected after Connect: %v", a.Values())
			}
			if err := a.Disconnect(); err != nil {
				t.Fatal(err)
			}
			if a.Wired() {
				t.Fatalf("still connected after Disconnect: %v", a.Values())
			}
			if v := f.Get(); v != "relay/glm-4.6" {
				t.Errorf("Disconnect left %q, not its own relay/glm-4.6", v)
			}
		})
	}
}

// OpenCode's own provider that sends to eipgam's gateway is eipgam's all
// the same (openCodeRef names a model there by it), once eipgam set
// OpenCode up: before that, it is a provider the user wrote by hand and
// theirs (handWired, JFunfunfun on X).
func TestOpenCodeGatewayProviderIsMagpies(t *testing.T) {
	syncHome(t)
	a, err := Find("opencode")
	if err != nil {
		t.Fatal(err)
	}
	writeMode(t, a.Path, `{"provider":{"relay":{"options":{"baseURL":"`+gatewayV1()+`"},"models":{"relay/glm-4.6":{}}}},"model":"relay/relay/glm-4.6"}`, 0o644)
	if a.Wired() {
		t.Fatalf("its own provider at the gateway, written by hand, reads as connected: %v", a.Values())
	}
	// as eipgam opencode relay/glm-4.6 sets it (9b1641a49)
	if err := a.Apply("model", "relay/relay/glm-4.6"); err != nil {
		t.Fatal(err)
	}
	if !a.Wired() {
		t.Fatalf("a model through its own provider at the gateway isn't eipgam's: %v (%s)", a.Values(), filepath.Base(a.Path))
	}
	writeMode(t, a.Path, `{"provider":{"relay":{"options":{"baseURL":"https://x/v1"},"models":{"glm-4.6":{}}}},"model":"relay/glm-4.6"}`, 0o644)
	if a.Wired() {
		t.Fatalf("its own relay elsewhere reads as eipgam's: %v", a.Values())
	}
}
