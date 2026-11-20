package gui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sichengchen/eipgam/internal/stats"
)

// Discover's counts, and the Library market's servers', are asked once
// an hour, and a failed ask keeps what was held. Nothing is sent back:
// there is no like route.
func TestPluginCounts(t *testing.T) {
	h := builtinHome(t)
	asked := 0
	wasAsk := pluginCountsAsk
	t.Cleanup(func() {
		pluginCountsAsk = wasAsk
		countsGot, countsAt = nil, countsAt.AddDate(-1, 0, 0)
	})
	countsGot = nil
	fail := false
	pluginCountsAsk = func(context.Context) (stats.Board, error) {
		asked++
		if fail {
			return stats.Board{}, errors.New("offline")
		}
		return stats.Board{
			Plugins: map[string]stats.Counts{"opencode-a": {Users: 12, Likes: 3}},
			Servers: map[string]stats.Counts{"context7": {Users: 40}},
		}, nil
	}

	get := func() (out struct {
		Plugins map[string]stats.Counts `json:"plugins"`
		Servers map[string]stats.Counts `json:"servers"`
		Error   string                  `json:"error"`
	}) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/plugins/counts", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("counts: %d %s", rec.Code, rec.Body)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return
	}

	if c := get(); c.Plugins["opencode-a"] != (stats.Counts{Users: 12, Likes: 3}) || c.Servers["context7"].Users != 40 {
		t.Fatalf("first: %+v", c)
	}
	get()
	if asked != 1 {
		t.Fatalf("asked %d times within the hour", asked)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/plugins/like", nil))
	if rec.Code == http.StatusOK || rec.Code == http.StatusNoContent {
		t.Fatalf("a like was taken: %d %s", rec.Code, rec.Body)
	}
	// an hour on, a failed ask keeps what was held
	countsAt = countsAt.Add(-2 * 60 * 60 * 1e9)
	fail = true
	if c := get(); c.Plugins["opencode-a"].Users != 12 || c.Servers["context7"].Users != 40 || c.Error == "" {
		t.Fatalf("a failed ask: %+v", c)
	}
}
