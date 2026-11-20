package gui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sichengchen/eipgam/internal/autostart"
)

// /api/open opens web pages and System Settings' Login Items, and no
// other link a page names: no mailto:, no other scheme.
func TestOpenOpensWebPagesAndLoginItemsOnly(t *testing.T) {
	sandboxHome(t)
	w := &importWindows{}
	mux := http.NewServeMux()
	providerRoutes(mux, w)
	for link, want := range map[string]bool{
		"https://example.com/docs":                               true,
		"http://localhost:1234":                                  true,
		autostart.LoginItemsURL:                                  true,
		"mailto:yetoneful@gmail.com?subject=Sponsoring%20magpie": false,
		"mailto:someone@example.com":                             false,
		"x-apple.systempreferences:com.apple.Other":              false,
		"file:///etc/passwd":                                     false,
	} {
		w.opened = ""
		body, _ := json.Marshal(map[string]string{"url": link})
		mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/open", strings.NewReader(string(body))))
		if (w.opened == link) != want {
			t.Errorf("%s: opened %q, want opened %v", link, w.opened, want)
		}
	}
}

type importWindows struct {
	Windows
	opened string
}

func (w *importWindows) OpenURL(u string) error { w.opened = u; return nil }
