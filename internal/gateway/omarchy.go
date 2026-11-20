package gateway

import (
	"net/http"
	"os"

	"github.com/sichengchen/eipgam/internal/omarchy"
	"github.com/sichengchen/eipgam/internal/provider"
)

// A development site may opt in to reading the local Omarchy theme.
func siteOrigin(o string) bool {
	return o != "" && o == os.Getenv("EIPGAM_SITE_ORIGIN")
}

// onOmarchy is whether this eipgam runs on Omarchy, asked once.
var onOmarchy = omarchy.Detect()

// omarchyTheme is Omarchy's current theme as eipgam's own windows draw it
// (name, mode, stamp, CSS variables: palette, font, corners, border), for
// trusted local clients. A development site can use EIPGAM_SITE_ORIGIN.
func (s *Server) omarchyTheme(w http.ResponseWriter, r *http.Request) {
	if o := r.Header.Get("Origin"); siteOrigin(o) {
		w.Header().Set("Access-Control-Allow-Origin", o)
		w.Header().Set("Vary", "Origin")
		if r.Header.Get("Access-Control-Request-Private-Network") == "true" {
			w.Header().Set("Access-Control-Allow-Private-Network", "true")
		}
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !local(r) {
		writeError(w, provider.Chat, http.StatusForbidden, "eipgam tells its Omarchy theme to this machine only")
		return
	}
	th, ok := omarchy.Theme{}, false
	if onOmarchy {
		th, ok = omarchy.Current()
	}
	if !ok {
		writeError(w, provider.Chat, http.StatusNotFound, "eipgam isn't running on Omarchy")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, th)
}
