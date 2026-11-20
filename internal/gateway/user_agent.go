package gateway

import (
	"context"
	"net/http"
	"strings"

	"github.com/sichengchen/eipgam/internal/provider"
)

// magpieUA is the User-Agent eipgam's requests go to a vendor with.
func magpieUA() string { return "eipgam/" + Version }

// userAgentFor is the User-Agent a request to p goes with, ua being the
// one the request came to eipgam with: the agent's own, as it sent it,
// where p passes it on (provider.PassesUserAgent: a key+URL provider not
// set to go as eipgam, #1851), and eipgam's where it doesn't, where the
// agent sent none, and on eipgam's own
// requests — its classifier's, vision's, search's, the image tool's
// (magpiesOwn) — which no agent sent. A User-Agent the user set among the
// provider's headers is put on after, when the request is signed, and
// stands over either.
func userAgentFor(p provider.Provider, ua string) string {
	if ua != "" && p.PassesUserAgent() && !magpiesOwn(ua) {
		return ua
	}
	return magpieUA()
}

// magpiesOwn is a User-Agent eipgam's own requests carry: another eipgam's
// (eipgam/…), the classifier's, vision's and search's (eipgam-router/1,
// eipgam-vision/1, eipgam-search/1), and the image tool's, which names the
// agent it draws for before its own (claude-code/2.1.0 eipgam-image/1)
// from what the agent told it, not from a User-Agent the agent sent.
func magpiesOwn(ua string) bool {
	return strings.HasPrefix(ua, "eipgam/") || strings.HasPrefix(ua, "eipgam-") || strings.HasSuffix(ua, " "+DrawAgent)
}

// callerUA is the User-Agent the request ctx is for came with (withCaller),
// or the agent's another eipgam passed it on for; "" when there is none.
func callerUA(ctx context.Context) string {
	c, _ := ctx.Value(callerCtx{}).(caller)
	return c.ua
}

// stampUA puts on req, a request to p that isn't eipgam's own forwarded
// one (an embedding, a drawing, a video), the User-Agent forwardOnce's
// carry: eipgam's, or the agent's where p passes it on. A signed-in
// account's is left to its signing, as before, and another eipgam's is
// set where it is asked.
func stampUA(ctx context.Context, p provider.Provider, h http.Header) {
	if p.Account != nil || p.IsRemoteMagpie() {
		return
	}
	h.Set("User-Agent", userAgentFor(p, callerUA(ctx)))
}

// agentUA is the User-Agent of the agent a request with headers in is
// for: the one it came with, or, from another eipgam (eipgam/…), the
// agent's that eipgam named (ClientUAHeader). eipgam's own requests made
// here (eipgam-router/1, …) keep theirs.
func agentUA(ctx context.Context, in http.Header) string {
	ua := in.Get("User-Agent")
	if c, ok := ctx.Value(callerCtx{}).(caller); ok && c.relayed && strings.HasPrefix(ua, "eipgam/") {
		return c.ua
	}
	return ua
}

// passOnUA names, on a request to a remote eipgam, the agent's User-Agent
// ua, for that eipgam to send on as this one would; eipgam's own is
// never named, and a vendor is never sent the header (withCaller takes
// it off what comes in).
func passOnUA(req *http.Request, ua string) {
	req.Header.Del(ClientUAHeader)
	if ua != "" && !magpiesOwn(ua) {
		req.Header.Set(ClientUAHeader, ua)
	}
}
