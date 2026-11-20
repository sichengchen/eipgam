package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// RemoteLibraryPath is where a remote eipgam tells its Library's prompts
// and skills (KoiLx0 on X: managed on one computer, not on every client).
// It is answered to the computer itself and to a full-access gateway key
// of a eipgam shared on the local network, as the quotas are.
const RemoteLibraryPath = "/v1/eipgam/library"

// RemoteLibraryWait is how long a remote eipgam has to send its skills.
const RemoteLibraryWait = 90 * time.Second

// maxRemoteLibrary is the most a remote's Library answer is read to: its
// skills' files, base64 in JSON (library.Shared keeps them under 24 MiB).
const maxRemoteLibrary = 48 << 20

const errRemoteNoLibrary = "remote eipgam doesn't share its Library; update eipgam on that computer"

// RemoteLibrary is the body of the Library the remote eipgam id shares,
// and the remote's name.
func RemoteLibrary(ctx context.Context, id string) ([]byte, string, error) {
	p, err := Find(id)
	if err != nil {
		return nil, "", err
	}
	if !p.IsRemoteMagpie() {
		return nil, "", fmt.Errorf("%s is no remote eipgam", id)
	}
	if p.Key == "" || p.Anthropic == "" {
		return nil, "", errors.New("the remote eipgam's address and key are needed")
	}
	b, err := remoteBodyAs(ctx, *p, http.MethodGet, RemoteLibraryPath, RemoteLibraryWait, maxRemoteLibrary, errRemoteNoLibrary)
	return b, remoteName(*p), err
}
