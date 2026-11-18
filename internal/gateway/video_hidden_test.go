package gateway

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/provider"
)

// axelburks on Discord: a provider's video models are ticked in its
// editor as its image ones are. One unticked there (HiddenVideomakers)
// leaves generate_video's list, another magpie's list and Automatic, and
// still makes its video when named; its image models are left as they
// were.
func TestHiddenVideomakerLeavesListsButStillFilms(t *testing.T) {
	fresh(t)
	if err := provider.Save(provider.Provider{ID: "vendor", Name: "Vendor", Chat: "https://vendor.example/v1", Key: "key",
		Models: []string{"chat-1"}}); err != nil {
		t.Fatal(err)
	}
	// the list as the vendor gives it: two video models, marked so, and
	// an image model
	if err := catalog.SaveLive("vendor", "https://vendor.example/v1", []catalog.Model{
		{ID: "chat-1"},
		{ID: "video-1", Films: true},
		{ID: "video-1-fast", Films: true},
		{ID: "image-1", Draws: true},
	}); err != nil {
		t.Fatal(err)
	}
	set := func(hidden []string) provider.Provider {
		t.Helper()
		found, err := provider.Find("vendor")
		if err != nil {
			t.Fatal(err)
		}
		p := *found
		p.HiddenVideomakers = hidden
		if err := provider.Save(p); err != nil {
			t.Fatal(err)
		}
		found, _ = provider.Find("vendor")
		return *found
	}
	ids := func(ms []catalog.Model) (out []string) {
		for _, m := range ms {
			out = append(out, m.ID)
		}
		return
	}
	p := set(nil)
	if got := ids(OfferedVideomakers(p)); !slices.Equal(got, []string{"video-1", "video-1-fast"}) {
		t.Fatalf("offered with none hidden %v", got)
	}

	p = set([]string{"video-1"})
	if !slices.Equal(p.HiddenVideomakers, []string{"video-1"}) {
		t.Fatalf("saved %v", p.HiddenVideomakers)
	}
	if got := ids(OfferedVideomakers(p)); !slices.Equal(got, []string{"video-1-fast"}) {
		t.Fatalf("offered %v", got)
	}
	if got := ids(Videomakers(p)); !slices.Contains(got, "video-1") {
		t.Fatalf("videomakers lost the hidden one: %v", got)
	}
	if got := ids(OfferedDrawers(p)); !slices.Equal(got, []string{"image-1"}) {
		t.Fatalf("a hidden video model changed the image models: %v", got)
	}
	var listed []string
	for _, o := range videomakerObjects(httptest.NewRequest(http.MethodGet, "/v1/models", nil)) {
		listed = append(listed, o["id"].(string))
	}
	if !slices.Equal(listed, []string{"vendor/video-1-fast"}) {
		t.Fatalf("video objects %v", listed)
	}
	if a := AutoVideomaker(); a != "vendor/video-1-fast" {
		t.Fatalf("automatic %q", a)
	}
	if all, def := mediaModels(true); !slices.Equal(all, []string{"vendor/video-1-fast"}) || def != all[0] {
		t.Fatalf("generate_video's list %v %q", all, def)
	}
	// named, it still makes its video, at the vendor's videos API
	if q, id, ok := bareVideomaker("video-1"); !ok || q.ID != "vendor" || id != "video-1" {
		t.Fatalf("bare hidden: %v %q %v", q.ID, id, ok)
	}
	if !filmsCustom(p, "video-1") {
		t.Fatal("a hidden video model isn't made at the vendor's videos API")
	}

	p = set([]string{"video-1", "video-1-fast"})
	if a := AutoVideomaker(); a != "" {
		t.Fatalf("automatic with every model hidden: %q", a)
	}
}
