package iconproxy

import (
	"net/url"
	"testing"
)

func TestURLUsesOnlyTheConfiguredProxy(t *testing.T) {
	pic := "https://github.com/magpie-community.png?size=96&x=a b#c"
	t.Setenv("EIPGAM_ICON_PROXY", "")
	if got := URL(pic); got != "" {
		t.Fatalf("unconfigured proxy returned %q", got)
	}
	const base = "https://proxy.example/api/icon"
	t.Setenv("EIPGAM_ICON_PROXY", base)
	if want, got := base+"?url="+url.QueryEscape(pic), URL(pic); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
