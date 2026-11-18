// Package stats reads Discover's public counts from usemagpie.ai: how many
// computers use each plugin, and each of the Library market's MCP servers.
// It only reads them. magpie sends no telemetry: no count of its users,
// no id, and nothing about what it is used with.
package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yetone/magpie/internal/update"
)

// Site serves the counts; MAGPIE_SITE_HOST points them elsewhere, for
// testing.
func Site() string {
	if h := os.Getenv("MAGPIE_SITE_HOST"); h != "" {
		return strings.TrimRight(h, "/")
	}
	return update.Site
}

// Counts is what usemagpie.ai says of each plugin: the computers that
// had it in the last seven days, and its likes.
type Counts struct {
	Users int `json:"users,omitempty"`
	Likes int `json:"likes,omitempty"`
}

// Board is what usemagpie.ai counts: each plugin's, by Discover's key,
// and each MCP server's, by its market id.
type Board struct {
	Plugins map[string]Counts `json:"plugins"`
	Servers map[string]Counts `json:"servers"`
}

// PluginCounts asks usemagpie.ai each plugin's and each server's counts.
// The request carries no id.
func PluginCounts(ctx context.Context) (Board, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", Site()+"/api/plugins/stats", nil)
	if err != nil {
		return Board{}, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Board{}, err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return Board{}, fmt.Errorf("plugin counts: %s", res.Status)
	}
	var out Board
	if err := json.NewDecoder(http.MaxBytesReader(nil, res.Body, 4<<20)).Decode(&out); err != nil {
		return Board{}, err
	}
	if out.Plugins == nil {
		out.Plugins = map[string]Counts{}
	}
	if out.Servers == nil {
		out.Servers = map[string]Counts{}
	}
	return out, nil
}
