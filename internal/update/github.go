package update

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sichengchen/eipgam/internal/settings"
	"github.com/sichengchen/eipgam/internal/source"
)

const releaseRepo = "sichengchen/eipgam"
const macReleaseRepo = "sichengchen/macos-release"

type githubRelease struct {
	Tag         string    `json:"tag_name"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
	} `json:"assets"`
}

func (r githubRelease) version() string { return strings.TrimPrefix(r.Tag, "eipgam-v") }

func (r githubRelease) published() bool {
	v := r.version()
	if r.Draft || r.Prerelease || r.Tag != "eipgam-v"+v || strings.ContainsAny(v, "-+") || parse(v) == nil {
		return false
	}
	for _, asset := range r.Assets {
		if asset.Name == "eipgam-"+v+"-universal.zip" && asset.Size > 0 {
			return true
		}
	}
	return false
}

func (r githubRelease) release() *Release {
	// Homebrew owns the app download and its checksum. Use GitHub for metadata only.
	return &Release{Version: r.version(), Notes: r.Body, URL: ReleasePage(r.version())}
}

func (r githubRelease) assets(workflow string) map[string]Asset {
	assets := make(map[string]Asset)
	for _, a := range r.Assets {
		sum, ok := strings.CutPrefix(a.Digest, "sha256:")
		digest, err := hex.DecodeString(sum)
		if !ok || err != nil || len(digest) != 32 || a.Name == "" || strings.ContainsAny(a.Name, `/\`) {
			continue
		}
		asset := Asset{
			URL:  githubWeb + "/" + releaseRepo + "/releases/download/" + url.PathEscape(r.Tag) + "/" + url.PathEscape(a.Name),
			Size: a.Size, SHA256: strings.ToLower(sum), releaseTag: r.Tag, releaseWorkflow: workflow,
		}
		if a.ID > 0 {
			asset.API = githubAPI + "/repos/" + releaseRepo + "/releases/assets/" + strconv.FormatInt(a.ID, 10)
		}
		assets[a.Name] = asset
	}
	return assets
}

// DownloadPrivacyHelper fetches a versioned eipgam privacy-filter helper.
func DownloadPrivacyHelper(ctx context.Context, tag, platform, dest string) error {
	var rel githubRelease
	if err := readGitHub(ctx, releaseRepo, "/tags/"+url.PathEscape(tag), &rel); err != nil {
		return err
	}
	if rel.Tag != tag || rel.Draft {
		return fmt.Errorf("eipgam privacy-filter release %q is unavailable", tag)
	}
	name := "pf-serve-" + platform + ".zip"
	asset, ok := rel.assets("privacy-filter.yml")[name]
	if !ok {
		return fmt.Errorf("eipgam %s has no checksummed %s", tag, name)
	}
	return download(ctx, asset, dest)
}

func githubLatest(ctx context.Context, channel string) (*Release, error) {
	now := time.Now().UTC()
	cutoff := now.Truncate(24 * time.Hour).Add(20 * time.Hour)
	if now.Before(cutoff) {
		cutoff = cutoff.AddDate(0, 0, -1)
	}
	if channel == settings.UpdateStable {
		daysSinceMonday := (int(now.Weekday()) + 6) % 7
		cutoff = now.Truncate(24*time.Hour).AddDate(0, 0, -daysSinceMonday-3)
	}
	var chosen githubRelease
	err := githubHistory(ctx, func(r githubRelease) {
		urgent := strings.HasPrefix(r.Body, "**Urgent release:**") || strings.Contains(r.Body, "\n**Urgent release:**")
		eligible := channel == "" || (!r.PublishedAt.IsZero() && (!r.PublishedAt.After(cutoff) || urgent))
		if eligible && (chosen.Tag == "" || Newer(r.version(), chosen.version())) {
			chosen = r
		}
	})
	if err != nil {
		return nil, err
	}
	if chosen.Tag == "" {
		if channel == "" {
			return nil, fmt.Errorf("no eipgam release has been published for Homebrew yet")
		}
		return nil, fmt.Errorf("no eipgam release is available on the %s channel yet", channel)
	}
	return chosen.release(), nil
}

func githubNotes(ctx context.Context, after, upto string) ([]Note, error) {
	var notes []Note
	err := githubHistory(ctx, func(r githubRelease) {
		v := r.version()
		if Newer(v, upto) || (after != "" && !Newer(v, after)) {
			return
		}
		notes = append(notes, Note{Version: v, Notes: r.Body, URL: ReleasePage(v)})
	})
	return notes, err
}

// githubHistory walks the signing repository's public releases for eipgam.
func githubHistory(ctx context.Context, visit func(githubRelease)) error {
	const perPage = 100
	for page := 1; ; page++ {
		var releases []githubRelease
		if err := readGitHub(ctx, macReleaseRepo, fmt.Sprintf("?per_page=%d&page=%d", perPage, page), &releases); err != nil {
			return err
		}
		for _, r := range releases {
			if r.published() {
				visit(r)
			}
		}
		if len(releases) < perPage {
			return nil
		}
	}
}

func readGitHub(ctx context.Context, repo, suffix string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPI+"/repos/"+repo+"/releases"+suffix, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := source.Do(client, req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("eipgam releases on GitHub: %s", res.Status)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(dst); err != nil {
		return fmt.Errorf("eipgam releases on GitHub: %w", err)
	}
	return nil
}
