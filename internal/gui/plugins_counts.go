package gui

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/yetone/magpie/internal/stats"
)

// Discover's counts, read from usemagpie.ai (stats): each plugin's users
// and likes, and each MCP server's users for the Library's market, asked
// at most every hour. Nothing is sent back.

var (
	countsMu  sync.Mutex
	countsAt  time.Time
	countsGot *stats.Board
)

// pluginCountsAsk is stats', for tests to stand in for.
var pluginCountsAsk = stats.PluginCounts

// pluginCounts is the counts held, asked again when an hour old; a failed
// ask keeps what was held, tried again in a minute.
func pluginCounts(ctx context.Context) (stats.Board, error) {
	countsMu.Lock()
	defer countsMu.Unlock()
	if countsGot != nil && time.Since(countsAt) < time.Hour {
		return *countsGot, nil
	}
	got, err := pluginCountsAsk(ctx)
	if err != nil {
		countsAt = time.Now().Add(-time.Hour + time.Minute)
		if countsGot != nil {
			return *countsGot, err
		}
		return stats.Board{}, err
	}
	countsGot, countsAt = &got, time.Now()
	return got, nil
}

func pluginCountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/plugins/counts", func(rw http.ResponseWriter, r *http.Request) {
		got, err := pluginCounts(r.Context())
		if got.Plugins == nil {
			got.Plugins = map[string]stats.Counts{}
		}
		if got.Servers == nil {
			got.Servers = map[string]stats.Counts{}
		}
		out := map[string]any{"plugins": got.Plugins, "servers": got.Servers}
		if err != nil {
			out["error"] = err.Error()
		}
		writeJSON(rw, out)
	})
}
