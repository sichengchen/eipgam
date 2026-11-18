package provider

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/yetone/magpie/internal/appdir"
	"github.com/yetone/magpie/internal/edit"
)

// OpenCode's own sign-ins, as `opencode auth login` keeps them: auth.json in
// its data folder, $XDG_DATA_HOME/opencode, else ~/.local/share/opencode
// (xdg-basedir's, on Windows too). It maps a provider id (models.dev's) to
// {"type":"api","key":…}, {"type":"oauth",…} or {"type":"wellknown",…}
// (packages/opencode/src/auth/index.ts). It holds keys only, never where
// they go: an id is imported when magpie has its preset.

func openCodeAuthPath() string {
	if d := appdir.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "opencode", "auth.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "opencode", "auth.json")
}

// openCodePresets are OpenCode's provider ids by the magpie preset at the
// same address. An id at a host the preset doesn't serve (SiliconFlow's
// siliconflow-cn, at .cn beside the preset's .com; tencent-token-plan, at
// China's api.lkeap.cloud.tencent.com beside the preset's Singapore
// TokenHub) is left out: its key would be sent there.
// The ids' hosts are models.dev's api for each, what OpenCode itself sends
// to, and it is the preset's own address, not one of its regions. Where a
// region would be the right target — Z.ai's and StepFun's pay-as-you-go
// ids beside plan-first defaults, MiniMax's coding plan — auth.json says
// nothing to pick a region by, so those ids stay out and are skipped with
// a reason that says why.
var openCodePresets = map[string]string{
	"opencode": "opencode-zen", "opencode-go": "opencode-go",
	"openai": "openai", "anthropic": "anthropic", "google": "google",
	"xai": "xai", "moonshotai": "moonshot", "mistral": "mistral",
	"groq": "groq", "openrouter": "openrouter", "togetherai": "together", "fireworks-ai": "fireworks",
	"siliconflow": "siliconflow", "nvidia": "nvidia",
	"ollama-cloud": "ollama-cloud", "vercel": "vercel",
	"zai": "zai", "alibaba": "qwen",
	"minimax": "minimax", "kilo": "kilo", "cline-pass": "clinepass",
	"kimi-code-plan-global": "kimi-code",
	"stepfun-ai-step-plan":  "stepfun",
	"alibaba-token-plan":    "alibaba-token-plan",
}

func readOpenCodeAuth(path string) ([]AppImport, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(edit.TrimBOM(raw), &all); err != nil {
		return nil, errors.New("OpenCode's auth.json: " + err.Error())
	}
	var out []AppImport
	for _, id := range slices.Sorted(maps.Keys(all)) {
		var e struct {
			Type string `json:"type"`
			Key  string `json:"key"`
		}
		if json.Unmarshal(all[id], &e) != nil {
			continue // OpenCode passes over what it can't read too
		}
		it := AppImport{Ref: id, From: "auth.json"}
		pr := Preset(openCodePresets[id])
		name := id
		if pr != nil {
			name = pr.Name
		}
		skip := ""
		switch {
		case e.Type == "oauth" || e.Type == "wellknown":
			skip = "it is a sign-in, not a key; add the subscription in magpie"
		case e.Type != "api":
			continue
		case pr == nil:
			skip = "it has no base URL" // OpenCode keeps only its key here
		case strings.TrimSpace(e.Key) == "":
			skip = "it has no API key"
		default:
			it.Provider, skip = imported(name, strings.TrimSpace(e.Key), endpoints{pr.Chat, pr.Responses, pr.Anthropic, ""}, nil)
		}
		if skip != "" {
			it.Provider, it.Skip = Provider{Name: name}, skip
		}
		out = append(out, it)
	}
	return out, nil
}
