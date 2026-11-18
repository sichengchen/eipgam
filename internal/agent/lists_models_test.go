package agent

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// listsSandbox is a HOME of its own with one provider of two models.
func listsSandbox(t *testing.T) (home, cfg string) {
	t.Helper()
	home = t.TempDir()
	cfg = filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	if err := provider.Save(provider.Provider{ID: "deepseek", Name: "DeepSeek", Chat: "https://api.deepseek.com/v1", Key: "k", Models: []string{"pro", "flash"}}); err != nil {
		t.Fatal(err)
	}
	return home, cfg
}

// An agent whose switch puts magpie's models in a picker of its own, with
// no field of its picking among them, lists only the ones picked for it on
// the Agents page, as every other agent does: ZCode was given every model,
// with no way to hide one (#1508, NovaTrailX). ListsModels is what gives
// its row that list.
func TestEveryPickerOfMagpiesModelsCanBeNarrowed(t *testing.T) {
	home, cfg := listsSandbox(t)
	var bare, menus []string
	for _, a := range builtins(home, cfg) {
		vals := a.Values()
		refs, magpie := false, false
		for _, f := range a.Fields {
			if f.Options == nil {
				continue
			}
			for _, o := range f.Options(vals) {
				refs = refs || o.Ref != ""
				magpie = magpie || o.Value == magpieID
			}
		}
		switch {
		// Claude Desktop's tier pickers list the catalog once it is
		// connected (desktopTierFields), and the sandbox has no Desktop
		case refs, a.ID == "claude-desktop":
		case a.ListsModels:
			menus = append(menus, a.ID)
		case magpie:
			bare = append(bare, a.ID)
		}
	}
	if len(bare) > 0 {
		t.Errorf("magpie's models are put in these agents' pickers with no way to pick which: %v", bare)
	}
	for _, id := range []string{"t3code", CursorLocalID} {
		if !slices.Contains(menus, id) {
			t.Errorf("%s's row has no model list to pick in (ListsModels): %v", id, menus)
		}
	}
}
