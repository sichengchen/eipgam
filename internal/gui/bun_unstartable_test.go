package gui

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/sichengchen/eipgam/internal/plugin"
)

// A plugin install on a bun this Windows or processor can't run answers
// with a code and the reason, which the page says in the reader's
// language (zgcwkj on Discord), wrapped however deep the install put it.
func TestFailSaysBunUnstartable(t *testing.T) {
	bun := `C:\eipgam\cache\bun\1.4.2\eipgam-bun.exe`
	ue := &plugin.BunUnstartableError{Bun: bun, Why: "windows", Err: fmt.Errorf("exit status 0xc0000139")}
	rw := httptest.NewRecorder()
	fail(rw, fmt.Errorf("bun add @magpie-community/opencode-raccoon-auth@latest: %w", ue))
	var out map[string]string
	if err := json.Unmarshal(rw.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if rw.Code != 400 || out["code"] != "bun_unstartable" || out["bunWhy"] != "windows" || out["bun"] != bun || out["error"] == "" || out["said"] != bunSaid["windows"] {
		t.Fatalf("fail answered %d %v", rw.Code, out)
	}
	rw = httptest.NewRecorder()
	fail(rw, fmt.Errorf("bun add x: exit status 1"))
	out = nil
	if err := json.Unmarshal(rw.Body.Bytes(), &out); err != nil || out["code"] != "" || out["said"] != "" {
		t.Fatalf("an ordinary failure answered %v (%v)", out, err)
	}
}
