package update

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// The fixture is the answer api.github.com gave on 2026-11-12 for the
// SHA256SUMS of v0.1.3925 in yetone/magpie-releases, byte for byte.
const attestedSums = "9fbb3e07c679a0a9c391305fde380d27eb61726bf28f4b078b2100f28379ce14"

func attestFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "attest", "sums-v0.1.3925.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// attestServer answers the attestation API with body (status code), counts
// the asks and points the updater at it.
func attestServer(t *testing.T, code int, body []byte) *int32 {
	t.Helper()
	var asks int32
	wantPath := "/repos/" + attestRepo + "/attestations/sha256:"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&asks, 1)
		if !strings.HasPrefix(r.URL.Path, wantPath) {
			t.Errorf("asked %s", r.URL.Path)
		}
		w.WriteHeader(code)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	old := attestAPI
	attestAPI = srv.URL
	t.Cleanup(func() { attestAPI = old })
	t.Setenv("EIPGAM_UPDATE_FEED", "")
	return &asks
}

func TestAttestationOfARealBundleHolds(t *testing.T) {
	old := attestRepo
	attestRepo = "yetone/magpie-releases"
	t.Cleanup(func() { attestRepo = old })
	attestServer(t, 200, attestFixture(t))
	raws, err := fetchAttestations(context.Background(), attestedSums)
	if err != nil {
		t.Fatal(err)
	}
	sum, _ := hex.DecodeString(attestedSums)
	if err := verifyBundle(raws[0], sum, "release.yml", "refs/heads/main", "SHA256SUMS"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ ref, name string }{
		{"refs/tags/v0.1.3925", "SHA256SUMS"},
		{"refs/heads/main", "other.zip"},
	} {
		if err := verifyBundle(raws[0], sum, "release.yml", c.ref, c.name); err == nil {
			t.Fatalf("accepted unrelated provenance for %s %s", c.ref, c.name)
		}
	}
	attestRepo = releaseRepo
	if err := verifyBundle(raws[0], sum, "release.yml", "refs/heads/main", "SHA256SUMS"); err == nil {
		t.Fatal("accepted Magpie's attestation for eipgam")
	}
}

func TestDownloadRequiresAttestationBeforeFetchingBytes(t *testing.T) {
	for _, code := range []int{404, 429, 502, 200} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			asks := attestServer(t, code, []byte(`{"attestations":[]}`))
			var fetched atomic.Bool
			file := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				fetched.Store(true)
			}))
			t.Cleanup(file.Close)
			a := Asset{URL: file.URL + "/eipgam.zip", SHA256: attestedSums, releaseTag: "v0.2.1", releaseWorkflow: "release.yml"}
			if err := download(context.Background(), a, filepath.Join(t.TempDir(), "eipgam.zip")); err == nil {
				t.Fatal("download accepted an asset with no verified provenance")
			}
			if fetched.Load() || atomic.LoadInt32(asks) != 1 {
				t.Fatal("must ask for provenance before requesting the download")
			}
		})
	}
}
