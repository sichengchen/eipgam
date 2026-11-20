package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sichengchen/eipgam/internal/settings"
)

func TestGitHubReleaseIsTheDefaultSource(t *testing.T) {
	t.Setenv("EIPGAM_UPDATE_FEED", "")
	t.Setenv("EIPGAM_NOTES_FEED", "")
	digest := strings.Repeat("02", 32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const base = "/repos/sichengchen/eipgam/releases"
		rel := map[string]any{
			"tag_name": "v0.2.7", "body": "eipgam notes", "published_at": time.Now().Add(-14 * 24 * time.Hour),
			"assets": []map[string]any{
				{"id": 10, "name": "eipgam-darwin-arm64.zip", "size": 42, "digest": "sha256:" + digest},
				{"id": 11, "name": "missing-checksum.zip"},
			},
		}
		switch r.URL.Path {
		case base + "/latest":
			json.NewEncoder(w).Encode(rel)
		case base:
			json.NewEncoder(w).Encode([]any{
				map[string]any{"tag_name": "pf-v4"}, map[string]any{"tag_name": "v0.2.8", "draft": true},
				rel,
			})
		default:
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
		}
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
		a := rel.Assets["eipgam-darwin-arm64.zip"]
		if rel.Version != "0.2.7" || rel.URL != "https://github.com/sichengchen/eipgam/releases/tag/v0.2.7" ||
			len(rel.Assets) != 1 || a.SHA256 != digest || a.Size != 42 || a.releaseTag != "v0.2.7" ||
			a.URL != "https://github.com/sichengchen/eipgam/releases/download/v0.2.7/eipgam-darwin-arm64.zip" ||
			a.API != srv.URL+"/repos/sichengchen/eipgam/releases/assets/10" {
			t.Fatalf("%q: got %+v and asset %+v", channel, rel, a)
		}
	}
	notes, err := NotesBetween(context.Background(), "", "0.2.7", "en")
	if err != nil || len(notes) != 1 || notes[0].Notes != "eipgam notes" {
		t.Fatalf("release notes: %+v, %v", notes, err)
	}
}
