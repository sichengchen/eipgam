package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sichengchen/eipgam/internal/settings"
)

func TestGitHubReleaseIsTheDefaultSource(t *testing.T) {
	t.Setenv("EIPGAM_UPDATE_FEED", "")
	t.Setenv("EIPGAM_NOTES_FEED", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/sichengchen/macos-release/releases" {
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
			return
		}
		old := time.Now().Add(-14 * 24 * time.Hour)
		app := func(tag, asset string, draft, prerelease bool) map[string]any {
			return map[string]any{
				"tag_name": tag, "body": "eipgam notes", "published_at": old,
				"draft": draft, "prerelease": prerelease, "assets": []map[string]any{{"name": asset, "size": 42}},
			}
		}
		json.NewEncoder(w).Encode([]any{
			app("eau-v9.0.0", "Eau-9.0.0-arm64.pkg", false, false),
			app("eipgam-v0.8.0", "eipgam-0.8.0-universal.zip", true, false),
			app("eipgam-v0.7.0", "eipgam-0.7.0-universal.zip", false, true),
			app("eipgam-v0.6.0", "eipgam-0.6.0.txt", false, false),
			app("eipgam-v0.2.7", "eipgam-0.2.7-universal.zip", false, false),
			app("eipgam-v0.2.5", "eipgam-0.2.5-universal.zip", false, false),
		})
	}))
	t.Cleanup(srv.Close)
	oldAPI := githubAPI
	githubAPI = srv.URL
	t.Cleanup(func() { githubAPI = oldAPI })
	for _, channel := range []string{"", settings.UpdateDaily, settings.UpdateStable} {
		stableFor(t, channel)
		rel, err := Latest(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if rel.Version != "0.2.7" || rel.URL != "https://github.com/sichengchen/macos-release/releases/tag/eipgam-v0.2.7" ||
			len(rel.Assets) != 0 {
			t.Fatalf("%q: got %+v", channel, rel)
		}
	}
	notes, err := NotesBetween(context.Background(), "", "0.2.7", "en")
	if err != nil || len(notes) != 1 || notes[0].Notes != "eipgam notes" {
		t.Fatalf("release notes: %+v, %v", notes, err)
	}
}
