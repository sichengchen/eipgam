package gateway

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync/atomic"

	"github.com/sichengchen/eipgam/internal/provider"
)

// RestartPath is where `eipgam update`, once it has put a new version in,
// asks the eipgam serving the gateway to restart into it
// (gaubeebangeel on X: an agent served by eipgam runs the update at 4am,
// and the app stayed on the old version until someone quit and reopened
// it, which cut whatever the agents had in flight). The app restarts the
// way its own update does: on a Mac or Linux it hands the gateway to the
// new version, so the request that ran the update, and every other,
// goes on. Loopback only, from a program, never a page.
const RestartPath = "/v1/eipgam/restart"

// RestartReply is what RestartPath answers.
type RestartReply struct {
	// restarting: at once; waiting: once what a restart would cut short
	// (Busy) is done; current: this eipgam wasn't the one updated (another
	// copy, or nothing put in); unsupported: this eipgam (eipgam serve)
	// doesn't restart itself, and is restarted the way it was started
	State string `json:"state"`
	// Version is this eipgam's, the one restarting
	Version string `json:"version"`
	// Handover: the new version takes the gateway over while this one
	// finishes its requests (Busy is then the Claude subscription's turns)
	Handover bool  `json:"handover,omitempty"`
	Busy     *Busy `json:"busy,omitempty"`
	// Forced: asked with force, restarting at once whatever is in flight
	Forced bool `json:"forced,omitempty"`
	// Drain is the seconds a waiting restart waits at most; past it the
	// restart goes ahead and cuts what is still in flight (ljlwlcz on X: a
	// Windows eipgam with long streams waited on "N requests" for good).
	// 0 from a eipgam that waits without a bound.
	Drain int `json:"drain,omitempty"`
}

var restarter atomic.Pointer[func(version string) RestartReply]

// SetRestarter has RestartPath call f with the version `eipgam update`
// put in; the gui's updater sets it. Without one the answer is
// "unsupported".
func SetRestarter(f func(version string) RestartReply) {
	if f == nil {
		restarter.Store(nil)
		return
	}
	restarter.Store(&f)
}

var forceRestarter atomic.Pointer[func(version string) RestartReply]

// SetForceRestarter is SetRestarter's for `eipgam update --force`: f
// restarts at once, whatever is in flight. Without one a forced ask gets
// the ordinary answer, with Forced unset.
func SetForceRestarter(f func(version string) RestartReply) {
	if f == nil {
		forceRestarter.Store(nil)
		return
	}
	forceRestarter.Store(&f)
}

// restartRefused says why r may not restart this eipgam, "" when it may:
// as heapRefused, only a program on this computer calling localhost or a
// loopback address, never another machine, a page, or another Host.
func restartRefused(r *http.Request) string {
	switch {
	case !local(r):
		return "eipgam restarts to update only when a program on this computer asks, over loopback: run eipgam update there"
	case fromBrowser(r):
		return "eipgam doesn't restart when a web page asks: run eipgam update in a terminal"
	case !loopbackHost(r.Host):
		return "eipgam restarts to update only on a call to localhost or a loopback address, not " + r.Host + ": run eipgam update"
	}
	return ""
}

func (s *Server) restartToUpdate(w http.ResponseWriter, r *http.Request) {
	if why := restartRefused(r); why != "" {
		log.Printf("refused %s %s from %s: %s", r.Method, r.URL.Path, r.RemoteAddr, why)
		writeError(w, provider.Chat, http.StatusForbidden, why)
		return
	}
	var in struct {
		Version string `json:"version"`
		Force   bool   `json:"force"`
	}
	json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in)
	reply := RestartReply{State: "unsupported", Version: Version}
	f := restarter.Load()
	if in.Force {
		if ff := forceRestarter.Load(); ff != nil {
			f = ff
		}
	}
	if f != nil {
		reply = (*f)(in.Version)
		reply.Version = Version
	}
	log.Printf("restart to update to %q asked from %s: %s", in.Version, r.RemoteAddr, reply.State)
	writeJSON(w, http.StatusOK, reply)
}
