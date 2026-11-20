package agent

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sichengchen/eipgam/internal/access"
	"github.com/sichengchen/eipgam/internal/gateway"
)

// T3 Code's eipgam instance is Claude Code routed with the gateway's
// address and the token Claude Code is sent with: beyond loopback that is
// the LAN sharing key, as at.gwKey() routes Claude Code itself.
// gateway.Token is turned away there, so T3's every call was answered 401
// (#1647, ggbdpq).
func TestT3CodeKeyBeyondLoopback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	path := filepath.Join(home, ".t3", "userdata", "settings.json")
	wsl := "http://172.23.80.1:3425"

	env := func(gw string) map[string]string {
		t.Helper()
		b, err := json.Marshal(t3InstanceJSON(path, gw))
		if err != nil {
			t.Fatal(err)
		}
		e := t3Format(t, b)
		out := map[string]string{}
		for _, v := range e.Environment {
			out[v.Name] = v.Value
		}
		return out
	}

	if err := access.ConfigureLAN(true, false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { access.ConfigureLAN(false, false) })
	lan := access.LANSecret()
	if !strings.HasPrefix(lan, access.Prefix) {
		t.Fatalf("LAN key %q", lan)
	}
	if e := env(wsl); e["ANTHROPIC_AUTH_TOKEN"] != lan {
		t.Errorf("T3 Code at %s: token %q, want the LAN sharing key", wsl, e["ANTHROPIC_AUTH_TOKEN"])
	}
	if e := env("http://127.0.0.1:3425"); e["ANTHROPIC_AUTH_TOKEN"] != gateway.Token {
		t.Errorf("T3 Code on loopback: token %q, want %q", e["ANTHROPIC_AUTH_TOKEN"], gateway.Token)
	}
	if err := access.ConfigureLAN(false, false); err != nil {
		t.Fatal(err)
	}
	if e := env(wsl); e["ANTHROPIC_AUTH_TOKEN"] != gateway.Token {
		t.Errorf("T3 Code at %s with nothing shared: token %q, want %q", wsl, e["ANTHROPIC_AUTH_TOKEN"], gateway.Token)
	}
}

// An agent in a WSL distro under NAT reaches the gateway at Windows'
// address, where gateway.Token is turned away (whqtian on Discord: OpenCode
// in WSL was written "apiKey": "eipgam" and refused). Every agent's config
// carries the LAN sharing key there while sharing is on, and gateway.Token on
// loopback, or while nothing is shared.
func TestGatewayKeyBeyondLoopback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	wsl := "http://172.23.80.1:3425"
	local := "http://127.0.0.1:3425"

	keys := func(gw string) map[string]string {
		oc := magpieProviderJSONAt("opencode", "opencode", gw).(map[string]any)["options"].(map[string]any)
		pi := magpieProviderJSONAt("pi", "pi", gw).(map[string]any)
		out := map[string]string{
			"opencode": oc["apiKey"].(string),
			"pi":       pi["apiKey"].(string),
		}
		return out
	}
	want := func(gw, key string) {
		t.Helper()
		for agent, k := range keys(gw) {
			if k != key {
				t.Errorf("%s at %s: key %q, want %q", agent, gw, k, key)
			}
		}
	}

	want(wsl, gateway.Token) // nothing shared: nothing else to give
	if err := access.ConfigureLAN(true, false); err != nil {
		t.Fatal(err)
	}
	lan := access.LANSecret()
	if !strings.HasPrefix(lan, access.Prefix) {
		t.Fatalf("LAN key %q", lan)
	}
	want(wsl, lan)
	want(local, gateway.Token)
	want("http://localhost:3425", gateway.Token)
	if !ourKey(lan) || !ourKey(gateway.Token) || ourKey("sk-someone-else") || ourKey("") {
		t.Error("ourKey tells eipgam's keys from others wrongly")
	}

	if err := access.ConfigureLAN(false, false); err != nil {
		t.Fatal(err)
	}
	want(wsl, gateway.Token)
}

// Every agent given the gateway's own address, gateway.URL(), with the key
// that names it (gateway.TokenFor) or gateway.Token, is given the LAN
// sharing key instead while EIPGAM_ADDR pins that address beyond loopback
// and sharing is on: the gateway takes neither from there (#1647's class).
func TestPinnedGatewayKeyForEveryWriter(t *testing.T) {
	codexHome(t, "", "")
	t.Setenv("EIPGAM_ADDR", "172.23.80.1:3425")
	gw := gateway.URL()
	if gw != "http://172.23.80.1:3425" {
		t.Fatalf("gateway URL %q", gw)
	}
	keys := func() map[string]string {
		agy := ""
		if m := regexp.MustCompile(`GEMINI_API_KEY="?([^" ;]+)`).FindStringSubmatch(AgyLaunch("m")); m != nil {
			agy = m[1]
		}
		return map[string]string{
			"agy":      agy,
			"cursor":   cursorLocalEnv(gw)[cursorLocalVars[1]],
			"vscode":   vscodeStable.token(),
			"vscodium": vscodiumKind.token(),
		}
	}
	plain := keys()
	if err := access.ConfigureLAN(true, false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { access.ConfigureLAN(false, false) })
	lan := access.LANSecret()
	if !strings.HasPrefix(lan, access.Prefix) {
		t.Fatalf("LAN key %q", lan)
	}
	for agent, k := range keys() {
		if k != lan {
			t.Errorf("%s at %s: key %q, want the LAN sharing key", agent, gw, k)
		}
		if plain[agent] == lan || !strings.HasPrefix(plain[agent], gateway.Token) {
			t.Errorf("%s with nothing shared: key %q, want eipgam's own", agent, plain[agent])
		}
	}
}
