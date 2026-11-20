package update

import (
	"context"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"

	"github.com/sichengchen/eipgam/internal/source"
)

// GitHub release assets require provenance from this repository's release
// workflow on the matching tag. The downloaded file must also match the
// attested SHA-256, including when a mirror serves the bytes.
//
//go:embed attest_root.json
var attestRootJSON []byte

var (
	attestAPI  = "https://api.github.com"
	attestRepo = releaseRepo
)

var errUnattested = errors.New("not attested")

func attestedFile(ctx context.Context, asset Asset, name string) error {
	if asset.releaseTag == "" {
		return nil
	}
	sum, err := hex.DecodeString(asset.SHA256)
	if err != nil || len(sum) != 32 {
		return fmt.Errorf("%s: %w: invalid SHA-256", name, errUnattested)
	}
	raws, err := fetchAttestations(ctx, strings.ToLower(asset.SHA256))
	if err != nil {
		return fmt.Errorf("%s: %w; not installed", name, err)
	}
	assetName, _ := url.PathUnescape(path.Base(asset.URL))
	var last error
	for _, raw := range raws {
		last = verifyBundle(raw, sum, asset.releaseWorkflow, "refs/tags/"+asset.releaseTag, assetName)
		if last == nil {
			return nil
		}
	}
	return fmt.Errorf("%s: %w: %v; not installed", name, errUnattested, last)
}

// fetchAttestations asks GitHub for the bundles attesting digest. The
// bundles check themselves, so whoever answers can't make a bad one pass.
func fetchAttestations(ctx context.Context, digest string) ([]json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", attestAPI+"/repos/"+attestRepo+"/attestations/sha256:"+digest, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := source.Do(client, req)
	if err != nil {
		return nil, fmt.Errorf("GitHub's attestation API: %w", err)
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("%w: GitHub has no build attestation for sha256:%s in %s", errUnattested, digest, attestRepo)
	case res.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("GitHub's attestation API answered %s", res.Status)
	}
	var out struct {
		Attestations []struct {
			Bundle json.RawMessage `json:"bundle"`
		} `json:"attestations"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("GitHub's attestation API: %w", err)
	}
	var raws []json.RawMessage
	for _, x := range out.Attestations {
		if len(x.Bundle) > 0 {
			raws = append(raws, x.Bundle)
		}
	}
	if len(raws) == 0 {
		return nil, fmt.Errorf("%w: GitHub lists no build attestation for sha256:%s", errUnattested, digest)
	}
	return raws, nil
}

// verifyBundle checks one Sigstore bundle against the embedded trusted root
// and the identity above, for an artifact of digest sum.
func verifyBundle(raw []byte, sum []byte, workflow, ref, name string) error {
	tr, err := root.NewTrustedRootFromJSON(attestRootJSON)
	if err != nil {
		return fmt.Errorf("the embedded Sigstore root: %w", err)
	}
	var b bundle.Bundle
	if err := b.UnmarshalJSON(raw); err != nil {
		return fmt.Errorf("the bundle: %w", err)
	}
	v, err := verify.NewVerifier(tr,
		verify.WithSignedCertificateTimestamps(1),
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1))
	if err != nil {
		return err
	}
	repo := "https://github.com/" + attestRepo
	san, err := verify.NewSANMatcher(repo+"/.github/workflows/"+workflow+"@"+ref, "")
	if err != nil {
		return err
	}
	iss, err := verify.NewIssuerMatcher("https://token.actions.githubusercontent.com", "")
	if err != nil {
		return err
	}
	id, err := verify.NewCertificateIdentity(san, iss, certificate.Extensions{SourceRepositoryURI: repo})
	if err != nil {
		return err
	}
	result, err := v.Verify(&b, verify.NewPolicy(verify.WithArtifactDigest("sha256", sum), verify.WithCertificateIdentity(id)))
	if err != nil {
		return err
	}
	for _, subject := range result.Statement.GetSubject() {
		if path.Base(subject.GetName()) == name && strings.EqualFold(subject.GetDigest()["sha256"], hex.EncodeToString(sum)) {
			return nil
		}
	}
	return fmt.Errorf("the attestation names no asset %q with this SHA-256", name)
}
