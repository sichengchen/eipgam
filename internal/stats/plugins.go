// Package stats reads counts from a configured plugin statistics service.
package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Site serves the counts; EIPGAM_SITE_HOST points them elsewhere, for
// testing.
func Site() string {
	if h := os.Getenv("EIPGAM_SITE_HOST"); h != "" {
		return strings.TrimRight(h, "/")
	}
	return ""
}

// Counts is the reported numbers for a plugin: the computers that
// had it in the last seven days, and its likes.
type Counts struct {
	Users int `json:"users,omitempty"`
	Likes int `json:"likes,omitempty"`
}

// Board is plugin and server counts: each plugin's, by Discover's key,
// and each MCP server's, by its market id.
type Board struct {
	Plugins map[string]Counts `json:"plugins"`
	Servers map[string]Counts `json:"servers"`
}

// PluginCounts reads the configured plugin and server counts.
// The request carries no id.
func PluginCounts(ctx context.Context) (Board, error) {
	site := Site()
	if site == "" {
		return Board{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", site+"/api/plugins/stats", nil)
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
