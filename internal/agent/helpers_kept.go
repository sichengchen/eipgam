package agent

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sichengchen/eipgam/internal/appdir"
)

// Helpers the kept agents share, moved here from the files of agents that
// were removed.

// onAnotherMachine says base is an http(s) address whose host isn't this
// machine's loopback.
func onAnotherMachine(base string) bool {
	if loopbackAuthority(base) {
		return false
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return false
	}
	h := u.Hostname()
	if h == "localhost" {
		return false
	}
	ip := net.ParseIP(h)
	return ip == nil || !ip.IsLoopback() && !ip.IsUnspecified()
}

// loopbackAuthority is the common case of onAnotherMachine without
// url.Parse: an http(s) base on localhost, 127.0.0.1 or [::1], no user
// info, which a state poll asks of each agent's gateway base several
// hundred times. Anything else is left to url.Parse.
func loopbackAuthority(base string) bool {
	rest, ok := strings.CutPrefix(base, "http://")
	if !ok {
		if rest, ok = strings.CutPrefix(base, "https://"); !ok {
			return false
		}
	}
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		rest = rest[:i]
	}
	host := rest
	if i := strings.LastIndexByte(rest, ':'); i >= 0 && !strings.HasSuffix(rest, "]") {
		host = rest[:i]
		for _, c := range rest[i+1:] {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	switch host {
	case "localhost", "127.0.0.1", "[::1]":
		return true
	}
	return false
}

// unstashPeek is a stashed value, left in the stash.
func unstashPeek(k string) string {
	stashMu.Lock()
	defer stashMu.Unlock()
	return stashShared()[k]
}

// xdgDataDirs are the folders whose applications/ hold desktop entries,
// the user's first, as xdg-open reads them.
func xdgDataDirs() []string {
	data := appdir.Getenv("XDG_DATA_HOME")
	if data == "" {
		if home, err := os.UserHomeDir(); err == nil {
			data = filepath.Join(home, ".local", "share")
		}
	}
	dirs := []string{data}
	sys := os.Getenv("XDG_DATA_DIRS")
	if sys == "" {
		sys = "/usr/local/share:/usr/share"
	}
	for _, d := range filepath.SplitList(sys) {
		if filepath.IsAbs(d) {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// desktopArgs is the command line a desktop entry's Exec gives for link,
// its program not looked for yet; nil with no Exec.
func desktopArgs(entry, link string) []string {
	line := ""
	in := false
	for _, l := range strings.Split(entry, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "[") {
			in = l == "[Desktop Entry]"
			continue
		}
		if k, v, ok := strings.Cut(l, "="); in && ok && strings.TrimSpace(k) == "Exec" {
			line = strings.TrimSpace(v)
			break
		}
	}
	var argv []string
	var arg strings.Builder
	has, quoted, gaveURL := false, false, false
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case quoted && c == '\\' && i+1 < len(line):
			i++
			arg.WriteByte(line[i])
		case c == '"':
			quoted, has = !quoted, true
		case !quoted && (c == ' ' || c == '\t'):
			if has {
				argv = append(argv, arg.String())
			}
			arg.Reset()
			has = false
		case c == '%' && i+1 < len(line):
			i++
			switch line[i] {
			case '%':
				arg.WriteByte('%')
				has = true
			case 'u', 'U':
				arg.WriteString(link)
				has, gaveURL = true, true
			}
		default:
			arg.WriteByte(c)
			has = true
		}
	}
	if has {
		argv = append(argv, arg.String())
	}
	if len(argv) == 0 {
		return nil
	}
	if !gaveURL {
		argv = append(argv, link)
	}
	return argv
}

// keptEffort is the default effort a sync leaves on one of eipgam's models:
// the one the user set (in the agent's own settings, or by hand) while the
// model still offers it, else eipgam's def. eipgam's default never
// overrides the user's own.
func keptEffort(was string, levels []string, def string) string {
	if was != "" && slices.Contains(levels, was) {
		return was
	}
	return def
}

// dshAPIs are the wire protocols of llm-pi-ai's (pi-ai's api names) a route
// of eipgam's may speak, the gateway serving each: openai-completions at
// /v1/chat/completions, openai-responses at /v1/responses and
// anthropic-messages at /v1/messages. dsh's Models page offers them as
// OpenAI Chat Completions, OpenAI Responses and Anthropic Messages, set for
// a route as a whole (llm-pi-ai has no api of a model's own); the gateway
// takes every model on each.
var dshAPIs = []string{"openai-completions", "openai-responses", "anthropic-messages"}

// zcodeDefaultLevel is medium where the model has it, else its middle level
// that thinks.
func zcodeDefaultLevel(levels []string) string {
	if slices.Contains(levels, "medium") {
		return "medium"
	}
	on := slices.DeleteFunc(slices.Clone(levels), func(l string) bool { return l == "disabled" })
	if len(on) == 0 {
		return levels[0]
	}
	return on[len(on)/2]
}
