package agent

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/sichengchen/eipgam/internal/appdir"
	"github.com/sichengchen/eipgam/internal/catalog"
	"github.com/sichengchen/eipgam/internal/edit"
	"github.com/sichengchen/eipgam/internal/gateway"
	"github.com/sichengchen/eipgam/internal/lastgood"
	"github.com/sichengchen/eipgam/internal/proc"
	"github.com/sichengchen/eipgam/internal/provider"
	"github.com/sichengchen/eipgam/internal/sessions"
	"github.com/sichengchen/eipgam/internal/settings"
	"github.com/sichengchen/eipgam/internal/wslrun"
)

// An agent installed in a WSL distro reads its config there, not in
// Windows' home — so does the Codex app's WSL connection. On Windows eipgam
// lists the distros (wsl.exe -l -q), asks each running one once for its
// $HOME and which of wslKinds' agents are there (never starting one that is
// stopped), and edits the files through \\wsl.localhost\<distro>. Each is
// an agent of its own, <id>@wsl:<distro> (codex@wsl:Ubuntu,
// pi@wsl:Ubuntu, claude@wsl:Ubuntu). The gateway it is pointed at is
// 127.0.0.1 when the distro's 127.0.0.1 reaches Windows' (mirrored or
// consomme networking, as the distro's wslinfo says, wslrun.HostLoopback;
// or mirrored in .wslconfig where there is no wslinfo); under NAT it is
// Windows as WSL sees it, which reaches the gateway only while that
// listens beyond loopback.

// place is where an agent lives: its home as eipgam opens it, how a path
// there is spelt in the agent's own config, and the gateway as it reaches
// it. here(home) is this machine's.
type place struct {
	home  string
	id    string              // the agent's id when not its own: its stash keys go under it
	spell func(string) string // a path under home as the agent names it; nil: as is
	sys   func(string) string // a path of the agent's system (/etc/…) as eipgam opens it; nil: as is
	base  func() string       // the gateway's URL from the agent; nil: gateway.URL
	// cold: a stopped distro's, whose files can't be looked at without
	// starting it — an agent built there takes its files at their defaults
	cold bool
	// version is the agent's CLI's there, as the probe found it; "" when
	// not known (one on this machine is asked itself)
	version string
	// reach is a path under home as eipgam can open it (distro.open): one
	// below a link the distro has is opened where the link leads; nil: as
	// is. unreached keeps the first path it couldn't tell the way to.
	reach     func(string) (string, error)
	unreached *error
	// chmod takes the group's and others' bits off a file under home, where
	// eipgam's own mode doesn't hold (a distro's: see ownerOnly); nil: here
	chmod func(string) error
	// dir is the agent's config folder when eipgam keeps one of its own
	// for it (claude-eipgam's launch file, claude_launch.go); "": its own
	dir string
	// wsl is the distro of an agent there, for one eipgam runs there
	// itself (Claude Science's daemon); nil on this machine
	wsl *distro
}

func here(home string) place { return place{home: home} }

// open is path, a path under the place's home, where eipgam can open it:
// in a WSL distro, a folder that is a link there (~/.codex ->
// /mnt/c/Users/me/.codex, #4502) is opened where it leads, as nothing
// below it can be opened through \\wsl.localhost. A link the distro
// couldn't resolve leaves path as it is, and the place unreached.
func (p place) open(path string) string {
	if p.reach == nil {
		return path
	}
	o, err := p.reach(path)
	if err != nil {
		if p.unreached != nil && *p.unreached == nil {
			*p.unreached = err
		}
		return path
	}
	return o
}

// join is filepath.Join(home, elem...), opened where eipgam can (open).
func (p place) join(elem ...string) string {
	return p.open(filepath.Join(append([]string{p.home}, elem...)...))
}

// getenv is this machine's variable for this machine's agent; "" for one
// in a distro, whose variables eipgam can't read.
func (p place) getenv(k string) string {
	if p.spell != nil {
		return ""
	}
	return appdir.Getenv(k)
}

// codexHome is Codex's folder at the place: $CODEX_HOME on this machine
// (appdir.CodexHome), ~/.codex in a distro, whose variables eipgam can't
// read.
func (p place) codexHome() string {
	if p.spell != nil {
		return p.join(".codex")
	}
	return appdir.CodexHomeIn(p.home)
}

// claudeDir is Claude Code's folder at the place: $CLAUDE_CONFIG_DIR on
// this machine (appdir.ClaudeDir), ~/.claude in a distro, whose variables
// eipgam can't read.
func (p place) claudeDir() string {
	if p.dir != "" {
		return p.dir
	}
	if p.spell != nil {
		return p.join(".claude")
	}
	return appdir.ClaudeDirIn(p.home)
}

// exists is whether there is a file or folder at path, as an agent looks
// for the one it reads; false at a cold place.
func (p place) exists(path string) bool {
	if p.cold {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// isDir is isDir at the place; false at a cold one.
func (p place) isDir(path string) bool { return !p.cold && isDir(path) }

func (p place) gw() string {
	if p.base != nil {
		return p.base()
	}
	return gateway.URL()
}

func (p place) v1() string       { return p.gw() + "/v1" }
func (p place) codexURL() string { return p.gw() + gateway.CodexPath }

// host is the gateway's host as the agent reaches it.
func (p place) host() string {
	if p.base == nil {
		return "127.0.0.1"
	}
	h, _, err := net.SplitHostPort(strings.TrimPrefix(p.base(), "http://"))
	if err != nil {
		return "127.0.0.1"
	}
	return h
}

// native is a path under home as the agent names it in its config.
func (p place) native(path string) string {
	if p.spell == nil {
		return path
	}
	return p.spell(path)
}

// key is a stash key of this machine's agent ("codex.model") as this
// place's agent's.
func (p place) key(k string) string {
	if p.id == "" {
		return k
	}
	_, rest, _ := strings.Cut(k, ".")
	return p.id + "." + rest
}

// distro is one WSL distro, as probed.
type distro struct {
	Name    string          `json:"name"`
	Home    string          `json:"home"`              // $HOME inside it, e.g. /home/me
	Root    string          `json:"root"`              // where eipgam opens its / from, e.g. \\wsl.localhost\Ubuntu
	Has     map[string]bool `json:"has"`               // "dir:.codex", "bin:pi": what the probe found
	Probe   int             `json:"probe,omitempty"`   // the wslProbeVersion that found it
	Gateway string          `json:"gateway,omitempty"` // the Windows host as the distro reaches it, when not at 127.0.0.1
	// Was are the addresses it reached Windows at before, newest last: an
	// agent's config still on one is offered the one now (#1013)
	Was    []string          `json:"was,omitempty"`
	Values map[string]string `json:"values,omitempty"` // its agents' fields as last read (wslKind.memo), shown while it is stopped
	// Net is WSL's networking mode as the distro's wslinfo said at the
	// last probe ("mirrored", "consomme", "nat", "bridged", "none", and
	// "virtioproxy" before WSL 2.9); "" when it couldn't say (a WSL without
	// wslinfo), and .wslconfig is read instead
	Net string `json:"net,omitempty"`
	// Versions is what the CLIs of wslKinds that ask (version) said their
	// versions are at the last probe, by the kind's id
	Versions map[string]string `json:"versions,omitempty"`
	// Paths is where the probe found each command in the distro, by name
	// ("opencode": /home/me/.opencode/bin/opencode); a distro probed
	// before this has none until it is probed again
	Paths map[string]string `json:"paths,omitempty"`
	// Links are the agents' folders (and files) under $HOME that lead
	// elsewhere, by their path under it (".codex"): where readlink -f
	// there says they lead, "" where it couldn't say (#4502)
	Links map[string]string `json:"links,omitempty"`
	// Drives are where Windows' drives are mounted in it, by mount point:
	// "/mnt/c" is "C:"
	Drives map[string]string `json:"drives,omitempty"`
	// Mirrored is whether the distro's 127.0.0.1 is Windows' (mirror): in
	// mirrored networking, or in consomme
	Mirrored bool `json:"-"`
	// noRelay: consomme with localhostForwarding=false in .wslconfig, where
	// the distro's 127.0.0.1 is its own after all
	noRelay bool
	Running bool `json:"-"`
	// up asks whether it runs now (wslUp), for one wslDistros listed
	up func() bool
}

// live is whether the distro runs now: asked of wsl.exe again, at most
// wslRunningAge after the last answer, for one wslDistros listed (an
// agent of it may be read after the user stopped it); Running otherwise.
func (d distro) live() bool {
	if d.up == nil {
		return d.Running
	}
	return d.up()
}

// local is a path inside the distro as eipgam opens it.
func (d distro) local(linux string) string {
	return d.Root + strings.ReplaceAll(linux, "/", string(filepath.Separator))
}

// native is a path eipgam opens as the distro spells it: one open moved
// to where a link leads is spelt through the link, as the agent knows it.
func (d distro) native(local string) string {
	best, spelt := "", ""
	for rel, to := range d.Links {
		if to == "" {
			continue
		}
		at := d.where(to)
		if r, ok := below(local, at); ok && len(at) > len(best) {
			best, spelt = at, strings.TrimRight(d.Home, "/")+"/"+rel+strings.ReplaceAll(r, string(filepath.Separator), "/")
		}
	}
	if best != "" {
		return spelt
	}
	rel := local
	if len(local) >= len(d.Root) && strings.EqualFold(local[:len(d.Root)], d.Root) {
		rel = local[len(d.Root):]
	}
	rel = strings.ReplaceAll(rel, `\`, "/")
	if !strings.HasPrefix(rel, "/") {
		rel = "/" + rel
	}
	return rel
}

// open is local, a path in the distro as eipgam opens it, where eipgam can
// open it. Through \\wsl.localhost Windows can't follow a link the
// distro has: a path below ~/.codex, when that leads to
// /mnt/c/Users/me/.codex, can be neither made, written, read nor listed,
// and a mkdir of ~/.codex itself says it exists (linusxiong's #4502). So a
// path at or below one of Links is opened where the link leads: on one of
// Windows' drives, there itself (C:\Users\me\.codex); elsewhere in the
// distro, through \\wsl.localhost again. A link the distro couldn't
// resolve is an error: where it leads isn't guessed.
func (d distro) open(local string) (string, error) {
	h := d.local(d.Home)
	best, rest := "", ""
	for rel := range d.Links {
		at := filepath.Join(h, filepath.FromSlash(rel))
		r, ok := below(local, at)
		if ok && len(rel) > len(best) {
			best, rest = rel, r
		}
	}
	if best == "" {
		return local, nil
	}
	to := d.Links[best]
	if to == "" {
		return "", fmt.Errorf("~/%s in WSL %s is a link eipgam couldn't follow (readlink -f there failed), so nothing is written through it; check where it points from a shell there", best, d.Name)
	}
	return d.where(to) + rest, nil
}

// where is linux, a path in the distro, as eipgam opens it: on one of
// Windows' drives, the drive's own path; anywhere else, through the
// distro's root.
func (d distro) where(linux string) string {
	drives := d.Drives
	if len(drives) == 0 {
		// a probe before Drives: WSL's default mounts, /mnt/<letter>
		if rest, ok := strings.CutPrefix(linux, "/mnt/"); ok && len(rest) >= 1 && (len(rest) == 1 || rest[1] == '/') && isLetter(rest[0]) {
			drives = map[string]string{"/mnt/" + rest[:1]: strings.ToUpper(rest[:1]) + ":"}
		}
	}
	best := ""
	for m := range drives {
		if (linux == m || strings.HasPrefix(linux, m+"/")) && len(m) > len(best) {
			best = m
		}
	}
	if best == "" {
		return d.local(linux)
	}
	return wslDrive(drives[best]) + strings.ReplaceAll(strings.TrimPrefix(linux[len(best):], "/"), "/", string(filepath.Separator))
}

// wslDrive is the root of a Windows drive ("C:") as eipgam opens it; a var
// for tests.
var wslDrive = func(drive string) string { return drive + `\` }

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// below is what path has below dir (with its leading separator, "" for
// dir itself), and whether it is dir or under it; Windows' paths compared
// without case.
func below(path, dir string) (string, bool) {
	if len(path) < len(dir) || !strings.EqualFold(path[:len(dir)], dir) {
		return "", false
	}
	rest := path[len(dir):]
	if rest != "" && rest[0] != filepath.Separator {
		return "", false
	}
	return rest, true
}

// mirror settles Mirrored from what wslinfo said at the probe, or, when it
// couldn't, from cfg (.wslconfig) saying mirrored: a WSL without wslinfo
// predates consomme, so .wslconfig's consomme isn't taken for one. Under
// consomme, cfg's localhostForwarding=false (wsl2.localhostForwarding)
// turns the relay of 127.0.0.1 off; it stays the address, with a notice,
// as Windows' next hop (the default route) is never Windows there.
func (d *distro) mirror(cfg string) {
	d.Mirrored = wslrun.HostLoopback(d.Net) || d.Net == "" && wslMirrored(cfg)
	d.noRelay = d.Net != "mirrored" && wslrun.HostLoopback(d.Net) && wslOff(cfg, "localhostForwarding")
}

// notMirrored is the notice of an agent in a distro not known to be in
// mirrored networking: what WSL said, or that only .wslconfig was read.
func (d distro) notMirrored(agent string) string {
	n := noticeWSLNotMirrored
	if d.Net == "" {
		n = noticeWSLNoMode
	}
	return n.say("distro", d.Name, "net", d.Net, "agent", agent, "url", d.base())
}

// base is the gateway's URL from inside the distro.
func (d distro) base() string {
	if d.Mirrored || d.Gateway == "" {
		return gateway.URL()
	}
	return "http://" + net.JoinHostPort(d.host(), gateway.Port())
}

// host is Windows as the distro reaches it: Gateway, or under NAT the WSL
// via (gateway.WSLVia) where a firewall keeps WSL off Gateway (wslFindVia).
func (d distro) host() string {
	if d.Gateway == "" || d.Net == "bridged" || wslrun.HostLoopback(d.Net) || !gateway.OnWSLNet(d.Gateway) {
		return d.Gateway
	}
	if v := gateway.WSLVia(); len(v) > 0 {
		return v[0].String()
	}
	return d.Gateway
}

func (d distro) place(id string) place {
	kind, _, _ := strings.Cut(id, "@")
	// a profile's row is omp#<name>@wsl:<distro>, whose kind is
	// omp#<name>: the probe keeps the binary version under omp, so the
	// lookup cuts the profile off
	if k, _, ok := strings.Cut(kind, "#"); ok {
		kind = k
	}
	return place{home: d.local(d.Home), id: id, spell: d.native, sys: d.local, base: d.base, cold: !d.Running, version: d.Versions[kind],
		reach: d.open, unreached: new(error), chmod: d.chmod, wsl: &d}
}

// chmod takes the group's and others' bits off local, a file in the distro
// eipgam opens, from inside it: a file written through \\wsl.localhost is
// 0644 there whatever mode eipgam asks for, and a key store so is refused.
func (d distro) chmod(local string) error {
	if _, err := os.Stat(local); err != nil {
		return nil
	}
	// a failure is said in the log, not as the pick's: the route and the key
	// are written, and the next pick or sync tries again
	if out, err := wslRun(30*time.Second, "-d", d.Name, "-e", "chmod", "go-rwx", d.native(local)); err != nil {
		log.Printf("chmod go-rwx %s in %s: %v %s", d.native(local), d.Name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// wslKind is an agent eipgam looks for in a distro: what the probe finds
// of it, and how it lives at a place. Adding one here is all it takes.
type wslKind struct {
	id, name string
	// dir, under $HOME, and bin, on its PATH: either says it is there; ""
	// for one that says nothing (a folder other tools keep too)
	dir, bin string
	in       func(place) *Agent // the agent at a place, as on this machine
	restart  notice             // advice after a change, when mirrored; "" none
	// restartAt, when set, is restart's choice at a place (kimi-cli or Kimi
	// Code 2.x, told by the folder there)
	restartAt func(place) notice
	// version: the probe asks the distro's own bin its version (--version),
	// for an agent whose config depends on it
	version bool
	// accept, when set, is whether what the probe found is the agent: a
	// command another tool's release shares the name of, told by its version
	accept func(d distro) bool
	// asleep, when set, is a stopped distro's options for a field, which
	// must read nothing of its files; nil keeps the live agent's
	asleep func(key string) func(map[string]string) []Option
	// ownNotice keeps the agent's own notice: one that reaches the gateway
	// under NAT by a way of its own (Claude Science's relay)
	ownNotice bool
}

// served is k with its agent the plugin's, where a plugin that says wsl
// serves the built-in now (agentMovers).
func (k wslKind) served() wslKind {
	if p := movedAgent(k.id); p != nil && p.Desc.WSL {
		k.in = func(at place) *Agent { return plugged(at, p) }
	}
	return k
}

var wslKinds = []wslKind{
	// its daemon, which eipgam starts there itself (claudescience_wsl.go):
	// the probe looks in ~/.local/bin, where its installer puts it
	{id: ScienceID, name: scienceName, dir: ".claude-science", bin: "claude-science", in: scienceIn, ownNotice: true},
	{id: "codex", name: "Codex", dir: ".codex", bin: "codex", in: codexIn,
		restart: newNotice("{agent} in WSL {distro} builds its model list at start-up — restart it, the app-server its sessions share ({command} there) and the Codex app's WSL connection to see this."),
		asleep: func(key string) func(map[string]string) []Option {
			switch key {
			case "model", "subagent", "memories":
				return func(map[string]string) []Option {
					// over several accounts eipgam routes, so they are
					// offered via eipgam too, as on a live Codex (#1603)
					if codexFailover() {
						return withVia(group("OpenAI", codexNamed(ownCodex())), "codex", "")
					}
					own := codexInMagpie(group("OpenAI", codexNamed(ownCodex())))
					return append(own, codexViaMagpieFor(own)...)
				}
			case "effort":
				return func(cur map[string]string) []Option {
					return codexEfforts(append(catalog.Codex(), magpieModels("codex")...), cur["model"])
				}
			case "subagent_effort":
				return func(cur map[string]string) []Option {
					m := cur["subagent"]
					if m == "" {
						m = cur["model"]
					}
					if e := catalog.Efforts(append(catalog.Codex(), magpieModels("codex")...), m); len(e) > 0 {
						return static(e...)
					}
					return static("low", "medium", "high", "xhigh")
				}
			}
			return nil
		}},
	// its version says which file its MCP servers go in (Pi 0.99 reads
	// mcp.json itself, library's piMCP)
	{id: "pi", name: "Pi", dir: ".pi", bin: "pi", in: piIn, version: true,
		asleep: func(key string) func(map[string]string) []Option {
			if key == "model" {
				// the providers of its auth.json are the distro's files
				return func(cur map[string]string) []Option {
					return withVia(ownOptions("", cur["model"]), "pi", magpieID+"/")
				}
			}
			return nil
		}},
	// only its settings.json: its sign-in, sessions and prompt history,
	// read on this machine for Claude Code here, aren't read in a distro
	{id: "claude", name: "Claude Code", dir: ".claude", bin: "claude", in: claudeIn,
		asleep: func(key string) func(map[string]string) []Option {
			switch key {
			case "model":
				// whether it has a base URL of its own is the distro's file
				return func(cur map[string]string) []Option {
					return joinedVia(group("Claude Code", claudeOwn(cur["model"], nil, catalog.Provider("anthropic"))), claudeViaShared(false), [2]string{"\x00claude-via", "\x00asleep"})
				}
			case "effort", "ultracode", "statusline":
				// their own options read only what was last seen; eipgam's
				// status line isn't offered in a distro
				return nil
			case "login":
				// its sign-in, while the model last seen is eipgam's
				return func(cur map[string]string) []Option {
					if !isMagpie(cur["model"]) {
						return nil
					}
					return claudeSignIns()
				}
			}
			// a tier's effort or the subagents': the levels of the model it
			// was last seen on, as when running, not a model to pick
			if tier, ok := strings.CutSuffix(key, "_effort"); ok {
				return func(cur map[string]string) []Option {
					if !isMagpie(cur["model"]) {
						return nil
					}
					return static(tierEfforts(cmp.Or(cur[tier], cur["model"]))...)
				}
			}
			// a tier, the subagents or auto mode: eipgam's models while the
			// model last seen is one
			return func(cur map[string]string) []Option {
				if !isMagpie(cur["model"]) {
					return nil
				}
				return claudeViaMagpie(false)
			}
		}},
	// The CLIs below keep a plain file in the distro's home, written as on
	// this machine, at their default folders: the distro's variables that
	// move them (OPENCODE_CONFIG_DIR, KIMI_CODE_HOME, HERMES_HOME, …)
	// aren't read. Each one's restart advice is its own Notice's.
	// its version says whether its config is written in OpenCode 2's words
	// (ocFile)
	{id: "opencode", name: "OpenCode", dir: ".config/opencode", bin: "opencode", in: opencodeIn, version: true,
		restart: newNotice("{agent} in WSL {distro} reads its config at start-up — restart open opencode sessions to use this."),
		asleep:  wslOwnAsleep("opencode", "model", "small")},
	// Kimi Code's ~/.kimi-code, or the old kimi-cli's ~/.kimi where that is
	// all there is; a stopped distro's is looked at once it is started
	{id: "kimi", name: "Kimi Code", dir: ".kimi-code", bin: "kimi", in: kimiIn,
		restart: noticeWSLKimiCode, restartAt: kimiWSLNotice,
		asleep: func(key string) func(map[string]string) []Option {
			if key != "model" {
				return nil
			}
			return func(cur map[string]string) []Option {
				return withVia(kimiOwnOptions("", cur["model"]), "kimi", magpieID+"/")
			}
		}},
	// no bin: grok is also other tools' name, as on this machine
	{id: "grok", name: "Grok Build", dir: ".grok", in: grokIn,
		restart: newNotice("{agent} in WSL {distro} reads its settings at start-up — restart open grok sessions to use this."),
		asleep: func(key string) func(map[string]string) []Option {
			if key != "model" {
				return nil
			}
			// its models_cache.json and config.toml are the distro's files
			return func(cur map[string]string) []Option {
				opts := viaMagpie("grok", magpieID+"/")
				for i := range opts {
					opts[i].Same = opts[i].own
				}
				return append(grokOwnOptions("", "", cur["model"]), opts...)
			}
		}},
	{id: "muse", name: "Muse Code", dir: ".config/muse", bin: "muse", in: museIn,
		restart: newNotice("{agent} in WSL {distro} reads its settings at start-up — restart open muse sessions to use this.")},
	// no dir: Antigravity keeps its folders in ~/.gemini too, so only the
	// command says Gemini CLI is there
	{id: "gemini", name: "Gemini CLI", bin: "gemini", in: geminiIn,
		restart: newNotice("{agent} in WSL {distro} reads its settings at start-up — restart open gemini sessions to see this."),
		asleep: func(key string) func(map[string]string) []Option {
			if key != "provider" {
				return nil
			}
			// which key its .env has is the distro's file
			return func(cur map[string]string) []Option {
				out := []Option{
					{Value: "google", Label: "Google", Icon: "gemini-color", Note: "Google account · OAuth sign-in"},
					{Value: "api-key", Label: "API key", Icon: "gemini-color"},
					{Value: "vertex", Label: "Vertex AI", Icon: "googlecloud-color", Note: "Vertex AI · $GOOGLE_CLOUD_PROJECT"},
				}
				if v := cur["provider"]; v == magpieID || v == "custom" {
					out = append(out, Option{Value: v})
				}
				return out
			}
		}},
	// Antigravity CLI, by its own folder under ~/.gemini or its command
	// (mimi001234 on Discord: eipgam didn't find agy in WSL); it takes the
	// gateway only from how it is started, so the distro's shells get the
	// function agy (agyDistroFunction)
	{id: "agy", name: "Antigravity CLI", dir: ".gemini/antigravity-cli", bin: "agy", in: agyIn,
		restart: newNotice("{agent} in WSL {distro} takes eipgam's gateway and model only from how it is started — type agy in a new terminal there: eipgam put it in the shell's profile."),
		asleep: func(key string) func(map[string]string) []Option {
			if key != "model" {
				return nil
			}
			// its custom models are the distro's settings.json
			return func(cur map[string]string) []Option {
				var own []Option
				if c := cur["model"]; c != "" && !usesMagpie(c) {
					own = append(own, Option{Value: c, Icon: modelIcon("", c), Direct: "Google"})
				}
				return withVia(group("Antigravity CLI", own), "agy", magpieID+"/")
			}
		}},
}

// wslOwnAsleep is a stopped distro's options for an agent's model fields
// (keys) whose live ones read its files: the providers of the value last
// seen, and eipgam's.
func wslOwnAsleep(id string, keys ...string) func(string) func(map[string]string) []Option {
	return func(key string) func(map[string]string) []Option {
		if !slices.Contains(keys, key) {
			return nil
		}
		return func(cur map[string]string) []Option {
			return withVia(ownOptions("", cur[key]), id, magpieID+"/")
		}
	}
}

// found reports whether the probe found the agent in d.
func (k wslKind) found(d distro) bool {
	if k.accept != nil && !k.accept(d) {
		return false
	}
	return k.dir != "" && d.Has["dir:"+k.dir] || k.bin != "" && d.Has["bin:"+k.bin]
}

// memo is the key a field of the agent is kept under in distro.Values:
// Codex's as they are, as wsl.json has always had them.
func (k wslKind) memo(key string) string {
	if k.id == "codex" {
		return key
	}
	return k.id + "." + key
}

// wslFound reports whether any agent eipgam knows is in d.
func wslFound(d distro) bool {
	for _, k := range wslKinds {
		if k.found(d) {
			return true
		}
	}
	return false
}

// wslKept is whether eipgam keeps a distro it probed: one of wslKinds is
// there, or the Copilot CLI's folder, whose sign-in it reads (#723), or
// the Antigravity app's data, whose sessions it reads.
func wslKept(d distro) bool {
	return wslFound(d) || d.Has["dir:"+wslCopilotDir] ||
		slices.ContainsFunc(wslAntigravityDirs, func(dir string) bool { return d.Has["dir:"+dir] })
}

// ListsFor is whose model lists an agent's are (provider.CatalogFor): its
// own id's, or, for one in a WSL distro, its Windows twin's (claude for
// claude@wsl:Ubuntu). A distro's agent is written eipgam's models under
// that id and sends the gateway that id's key, so the models picked for
// its row are the ones it is shown only when kept under that id too (#927).
func (a *Agent) ListsFor() string {
	id, _, _ := strings.Cut(a.ID, "@wsl:")
	// a named omp profile (omp#work) shares omp's lists: it is another
	// omp, not another agent with a catalog of its own
	id, _, _ = strings.Cut(id, "#")
	return id
}

func wslKindOf(id string) wslKind {
	for _, k := range wslKinds {
		if k.id == id {
			return k
		}
	}
	panic("no WSL agent " + id)
}

// wslCodex is Codex in a distro.
func wslCodex(d distro) *Agent { return wslAgent(wslKindOf("codex"), d) }

// wslPi is Pi in a distro.
func wslPi(d distro) *Agent { return wslAgent(wslKindOf("pi"), d) }

// wslAgent is an agent in a distro: its own reading and writing, at the
// distro's home, with the distro's way to the gateway. A distro that isn't
// running is shown as eipgam last saw it, and nothing of it is read: any
// access to its files starts it. Picking a value starts it, as asked.
func wslAgent(k wslKind, d distro) *Agent {
	id := k.id + "@wsl:" + d.Name
	at := d.place(id)
	a := k.in(at)
	if k.restartAt != nil {
		k.restart = k.restartAt(at)
	}
	a.ID, a.Name, a.Aliases, a.Bin, a.UA, a.WSL = id, k.name+" · WSL "+d.Name, nil, "", nil, d.Name
	a.wslVersion = at.version
	a.Gateway = d.base
	a.reachFrom = func(base string) (ok, known bool) {
		if !d.live() {
			return false, false // never started to be asked
		}
		key := "wsl " + d.Name + " " + base
		return probed(key, func() bool {
			if !d.live() {
				return true // stopped since: never started to be asked
			}
			ok, told := wslReach(d.Name, base)
			if told && !ok && d.Gateway != "" && d.Net != "bridged" && gateway.OnWSLNet(base) {
				// shut from WSL: another of Windows' addresses may answer
				wslFindVia(d, base)
			}
			return ok || !told // can't tell: taken as answering
		})
	}
	if d.Running {
		a.Home = d.local(d.Home)
	}
	a.detect = func() bool { return k.found(d) }
	// its requests carry the agent's User-Agent and are counted as its
	// Windows twin's, so a prompt with none of "its" own seen isn't a bypass
	a.LastUsed = nil
	own := a.Notice
	a.Notice = func() string {
		if k.ownNotice && own != nil {
			return own()
		}
		if !d.Mirrored {
			return d.notMirrored(k.name)
		}
		if d.noRelay {
			return noticeWSLNoRelay.say("distro", d.Name, "net", d.Net, "agent", k.name)
		}
		if k.restart == "" {
			return ""
		}
		return k.restart.say("agent", k.name, "distro", d.Name, "command", provider.CodexDaemonRestart)
	}
	if !d.Running {
		return asleep(a, k, d)
	}
	if k.dir != "" {
		at.join(filepath.FromSlash(k.dir))
	}
	if err := *at.unreached; err != nil {
		return unreached(a, err)
	}
	if reached := a.Reached; reached != nil {
		a.Reached = func(since time.Time) (time.Time, string, bool) {
			if !d.live() {
				return time.Time{}, "", false
			}
			at, to, refused := reached(since)
			if sameHost(to, d.base()) {
				to = gateway.URL() // the gateway, however WSL reaches it
			}
			return at, to, refused
		}
	}
	// the agent may be read after the distro stopped (it is made at most
	// wslRunningAge before): what reads its files in the background asks
	// again first, and a stopped one reads as last seen
	for i := range a.Fields {
		f := &a.Fields[i]
		get, key := f.Get, k.memo(f.Key)
		f.Get = func() string {
			if !d.live() {
				return wslLastSeen(d.Name, key)
			}
			v := get()
			wslRemember(d.Name, key, v)
			return v
		}
	}
	if sync := a.Sync; sync != nil {
		a.Sync = func() error {
			if !d.live() {
				return nil // its files get the catalog once it runs
			}
			return sync()
		}
	}
	if check := a.Check; check != nil {
		a.Check = func() string {
			if !d.live() {
				return ""
			}
			return check()
		}
	}
	for _, p := range []*func() bool{&a.Routed, &a.Joined, &a.Beside} {
		if f := *p; f != nil {
			*p = func() bool { return d.live() && f() }
		}
	}
	// a stopped distro's app can't be read: unknown, not "not added"
	if f := a.Added; f != nil {
		a.Added = func() (bool, bool) {
			if !d.live() {
				return false, false
			}
			return f()
		}
	}
	// what a set leaves is kept at once, in case the distro stops before
	// the next look
	for i := range a.Fields {
		f := &a.Fields[i]
		if set := f.Set; set != nil {
			f.Set = func(v string) error {
				err := set(v)
				for _, g := range a.Fields {
					g.Get()
				}
				wslSave()
				return err
			}
		}
	}
	return a
}

// unreached is an agent in a distro whose folder is a link the distro
// couldn't resolve (distro.open): nothing of it is written, and each
// change, and its notice, says why.
func unreached(a *Agent, err error) *Agent {
	a.unreached = err
	a.Notice = func() string { return err.Error() }
	for i := range a.Fields {
		if a.Fields[i].Set != nil {
			a.Fields[i].Set = func(string) error { return err }
		}
	}
	if a.Sync != nil {
		a.Sync = func() error { return err }
	}
	return a
}

// Unreached is why eipgam can't open the folder of a, an agent in a WSL
// distro: a link there the distro couldn't resolve; nil for any other.
func (a *Agent) Unreached() error { return a.unreached }

// WSLOpen is path, under the home of a, an agent in a WSL distro, where
// eipgam can open it (distro.open): below a link of the distro's, where
// the link leads. The error says a link there couldn't be resolved. Any
// other agent's path is as it is.
func WSLOpen(a *Agent, path string) (string, error) {
	if a == nil || a.WSL == "" {
		return path, nil
	}
	wsl.Lock()
	d := wsl.seen[a.WSL]
	var c distro
	if d != nil {
		c = *d
	}
	wsl.Unlock()
	if d == nil || c.Root == "" {
		return path, nil
	}
	return c.open(path)
}

// asleep is a stopped distro's agent: its fields read what eipgam last
// saw, their options come from eipgam alone, and it has no files to check,
// sync or migrate (Path and Dir are empty); setting a field goes to the
// files, which starts the distro.
func asleep(live *Agent, k wslKind, d distro) *Agent {
	started := false
	a := &Agent{ID: live.ID, Name: live.Name, Icon: live.Icon, WSL: d.Name, detect: live.detect, Spelled: live.Spelled, ModelAliases: live.ModelAliases, NoNames: live.NoNames,
		Notice: func() string {
			if started {
				return live.Notice()
			}
			return noticeWSLAsleep.say("distro", d.Name)
		}}
	for _, lf := range live.Fields {
		key := lf.Key
		f := Field{Key: key, Label: lf.Label, Quiet: lf.Quiet, Live: lf.Live, Follows: lf.Follows, Own: lf.Own, Options: lf.Options,
			Get: func() string { return wslLastSeen(d.Name, k.memo(key)) },
			Set: func(v string) error {
				// opening a stopped distro's files is aborted rather than
				// waiting for it to start, so it is started first
				if _, err := wslRun(time.Minute, "-d", d.Name, "-e", "true"); err != nil {
					return fmt.Errorf("start WSL %s: %w", d.Name, err)
				}
				started = true
				// the agent again, now its files can be looked at: which of
				// them it reads (opencode.jsonc or .json, ~/.kimi-code or
				// ~/.kimi) is theirs to say, not the defaults it was built on
				w := d
				w.Running = true
				warm := k.in(w.place(live.ID))
				wf := warm.Field(key)
				if wf == nil || wf.Set == nil {
					return fmt.Errorf("%s has no %s", live.Name, key)
				}
				err := wf.Set(v)
				// a model settles the effort too
				for _, f := range warm.Fields {
					wslRemember(d.Name, k.memo(f.Key), f.Get())
				}
				wslSave()
				return err
			}}
		if k.asleep != nil {
			if o := k.asleep(key); o != nil {
				f.Options = o
			}
		}
		// Claude Code's claude.ai sign-in is kept only where the gateway
		// takes any key, which the distro's address says without its files
		if k.id == "claude" && key == "login" && d.place(live.ID).gwKey() != gateway.Token {
			f.Options = func(map[string]string) []Option { return nil }
		}
		a.Fields = append(a.Fields, f)
	}
	return a
}

// wslAgents are the agents in this machine's WSL distros; none off Windows.
func wslAgents() []*Agent {
	if !wslLooks() {
		return nil
	}
	return wslAgentsOf(wslDistros())
}

func wslAgentsOf(ds []distro) []*Agent {
	var out []*Agent
	for _, d := range ds {
		// only the agents it has: eipgam writes nothing for one that isn't
		// there, and doesn't make it, which reads its files: a stopped
		// distro's files opened start it again (#3896)
		for _, k := range wslKinds {
			if k.found(d) {
				out = append(out, wslAgent(k.served(), d))
			}
		}
	}
	return out
}

// wslHomes are the distros agents (or the Copilot CLI's folder, or the
// Antigravity app's data, wslKept)
// were found in, each user's home as eipgam
// opens it, for internal/sessions to read their sessions in; none off
// Windows. A stopped distro's is its home as last probed.
func wslHomes() []sessions.WSLHome {
	if !wslLooks() {
		return nil
	}
	return wslHomesOf(wslDistros())
}

func wslHomesOf(ds []distro) []sessions.WSLHome {
	var out []sessions.WSLHome
	for _, d := range ds {
		if d.Root == "" || d.Home == "" {
			continue
		}
		open := func(path string) string {
			if to, err := d.open(path); err == nil {
				return to
			}
			return path
		}
		out = append(out, sessions.WSLHome{Distro: d.Name, Home: d.local(d.Home), Running: d.Running, Open: open})
	}
	return out
}

// wslAnswered is whether wsl.exe answered the last list of distros, so an
// empty wslHomes means no distro has an agent (sessions.WSLListed).
func wslAnswered() bool {
	wslDistros()
	wsl.Lock()
	defer wsl.Unlock()
	return wsl.listed
}

// wslCopilotDir is the Copilot CLI's folder under a distro's $HOME: not an
// agent eipgam sets up, but a sign-in it uses (provider.CopilotWSLDirs),
// so a distro that has only it is kept as one agents are found in (#723).
const wslCopilotDir = ".copilot"

// wslAntigravityDirs are the Antigravity app's data folders under a
// distro's $HOME. The Windows app connected to a WSL distro runs its
// language server there (`$HOME/.antigravity-server/bin/<version>/
// language_server`, started through wsl.exe with --app_data_dir
// antigravity, Antigravity 2.x's dist/wsl.js), which keeps the
// conversations in the distro's ~/.gemini/antigravity, not Windows'
// (@guyng35955 on X: no usage for Antigravity connected to WSL Ubuntu).
// Not an agent eipgam sets up, but sessions it reads
// (sessions.wslAgents), so a distro that has only them is kept as one
// agents are found in.
var wslAntigravityDirs = []string{".gemini/antigravity", ".gemini/antigravity-insiders"}

// copilotWSLDirs are the Copilot CLI's folders, as eipgam opens them (where
// a link leads, #4502), in the running distros it was found in, for
// provider to read its sign-in in; none off Windows. A stopped distro is
// left out: reading it would start it. It is asked on a request's path
// (provider's accounts), so it waits for WSL at most wslRunningWait.
func copilotWSLDirs() []string {
	if !wslLooks() {
		return nil
	}
	var out []string
	for _, h := range wslHomesOf(wslDistrosWithin(wslRunningWait)) {
		if h.Running && wslHasCopilot(h.Distro) {
			out = append(out, h.Open(filepath.Join(h.Home, wslCopilotDir)))
		}
	}
	return out
}

func wslHasCopilot(name string) bool {
	wsl.Lock()
	defer wsl.Unlock()
	d := wsl.seen[name]
	return d != nil && d.Has["dir:"+wslCopilotDir]
}

// Only running distros are probed — asking one anything starts it — once
// each in a process, and again only after a probe that failed. Those an
// agent of wslKinds was found in are kept in wsl.json beside eipgam's settings (not the
// stash, which profiles and backups carry), with their fields as last
// read, so a stopped one is listed without being started.
var wsl struct {
	sync.Mutex
	at      time.Time
	names   []string        // every distro installed
	running map[string]bool // those running
	runAt   time.Time       // when running was listed
	runOK   bool            // running is wsl.exe's answer, not a failure
	listed  bool            // names is a real answer, and may forget distros
	seen    map[string]*distro
	probed  map[string]bool
	failed  map[string]time.Time
	dirty   bool
}

const (
	wslListAge  = time.Minute
	wslRetryAge = 10 * time.Minute
	// wslRunningAge is how long which distros run is taken as wsl.exe
	// said: one the user stops (wsl --shutdown or --terminate, to repair
	// WSL) is seen stopped within it, before anything opens its files,
	// which would start it again (TJHHHH on Discord)
	wslRunningAge = 3 * time.Second
	// wslRunningFailedAge is how long wsl.exe failing to say is kept: each
	// ask that times out takes 10s and leaves a wsl.exe behind (#2341)
	wslRunningFailedAge = 30 * time.Second
)

// wslOn is whether there is WSL to look in: on Windows, or in tests.
var wslOn = runtime.GOOS == "windows"

// wslLooks is whether eipgam looks in WSL on its own: there is WSL, and
// Settings' Detect agents in WSL is on (#1264).
func wslLooks() bool { return wslOn && !settings.Shared().NoWSLAgents }

// wslRun runs wsl.exe; a var for tests.
// wslReachScript asks, inside a distro, whether anything answers at $1
// (http://host:port) on host $2, port $3: "ok" when it does, "no" when the
// connection is refused or times out, "?" when the distro has nothing to
// ask with.
const wslReachScript = `u="$1"; h="$2"; p="$3"
if command -v curl >/dev/null 2>&1; then
  curl -s -o /dev/null -m 3 "$u/v1/models"; c=$?
  case $c in 0) echo ok;; 7|28) echo no;; *) echo '?';; esac; exit 0
fi
if command -v wget >/dev/null 2>&1; then
  wget -q -O /dev/null -T 3 -t 1 "$u/v1/models"; c=$?
  case $c in 0|8) echo ok;; 4) echo no;; *) echo '?';; esac; exit 0
fi
if command -v bash >/dev/null 2>&1 && command -v timeout >/dev/null 2>&1; then
  if timeout 3 bash -c 'exec 3<>/dev/tcp/'"$h"'/'"$p" 2>/dev/null; then echo ok; else echo no; fi; exit 0
fi
echo '?'`

// wslReach is whether base answers from inside the running distro name,
// and whether that could be told.
func wslReach(name, base string) (ok, known bool) {
	u, err := url.Parse(base)
	if err != nil || u.Hostname() == "" {
		return false, false
	}
	port := u.Port()
	if port == "" {
		port = map[string]string{"https": "443"}[u.Scheme]
		if port == "" {
			port = "80"
		}
	}
	out, err := wslRun(10*time.Second, "-d", name, "-e", "sh", "-c", wslReachScript, "sh", strings.TrimSuffix(base, "/"), u.Hostname(), port)
	if err != nil {
		return false, false
	}
	switch strings.TrimSpace(string(out)) {
	case "ok":
		return true, true
	case "no":
		return false, true
	}
	return false, false
}

var wslRun = func(timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := proc.CommandContext(ctx, "wsl.exe", args...)
	// wsl.exe speaks UTF-16 unless told otherwise; either is read
	cmd.Env = append(os.Environ(), "WSL_UTF8=1")
	out, err := cmd.Output()
	// one that says WSL must be updated isn't run again for hours
	proc.WSLSaid(out, err)
	return out, err
}

// wslRootWait is how long wslRoot waits for Windows to open a distro's
// /: a stat of WSL's file share can't be cancelled, and hangs while WSL
// does. A var for tests.
var wslRootWait = 5 * time.Second

// wslStat is os.Stat, for wslRoot; a var for tests.
var wslStat = os.Stat

// wslRoot is where eipgam opens a running distro's / from, "" when
// Windows didn't open it within wslRootWait (the probe failed, and is
// tried again in wslRetryAge); a var for tests.
var wslRoot = func(name string) string {
	ok, stat := make(chan bool, 1), wslStat
	go func() {
		_, err := stat(`\\wsl.localhost\` + name + `\`)
		ok <- err == nil
	}()
	t := time.NewTimer(wslRootWait)
	defer t.Stop()
	select {
	case found := <-ok:
		if found {
			return `\\wsl.localhost\` + name
		}
		return `\\wsl$\` + name // before Windows 11 / WSL 0.50
	case <-t.C:
		return ""
	}
}

// wslHungPath, while recent, says a wsl.exe listing timed out: each
// command a hung WSL held for its 10s timeout (20s with the running list),
// `eipgam providers` for Copilot's sign-in in a distro, `eipgam version`
// for start-up's moves.
func wslHungPath() string { return filepath.Join(filepath.Dir(provider.Path()), "wsl-hung") }

// wslHungAfter: a failure this slow is a hang, not no WSL; a var for tests.
var wslHungAfter = 5 * time.Second

const wslHungAge = 10 * time.Minute // how long another process takes it as hung

func wslHungLately() bool {
	fi, err := os.Stat(wslHungPath())
	return err == nil && time.Since(fi.ModTime()) < wslHungAge
}

func wslHung(hung bool) {
	if hung {
		_ = os.WriteFile(wslHungPath(), []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o600)
	} else {
		_ = os.Remove(wslHungPath())
	}
}

func wslStatePath() string { return filepath.Join(filepath.Dir(provider.Path()), "wsl.json") }

// wslRefreshMu is held by the one refresh of what eipgam knows of WSL
// (wslRefresh) at a time. wsl.exe and the distros' files are asked while
// it is held, never under wsl's own lock: that is taken only to read and
// write what they said. A probe, a listing or an open of a distro's /
// that hangs held wsl's lock before, so the GUI listing agents, a
// request's stand-in and Copilot's sign-in all waited on a hung wsl.exe,
// and the gateway sent 504s after 30s (vsoullai on X, #6167).
var wslRefreshMu sync.Mutex

// wslRefresh lists the distros again when the list is older than
// wslListAge, asks which run, and probes the running ones not probed yet.
func wslRefresh() {
	wslRefreshMu.Lock()
	defer wslRefreshMu.Unlock()
	wsl.Lock()
	wslLoadLocked()
	list := time.Since(wsl.at) > wslListAge
	// another eipgam waited out a hung wsl.exe moments ago: a command
	// doesn't wait 10s more (this one lists again in a minute)
	skip := list && wsl.at.IsZero() && wslHungLately()
	wsl.Unlock()
	if list {
		var names []string
		var listed bool
		if !skip {
			start := time.Now()
			names, listed = wslList("-l", "-q")
			wslHung(!listed && time.Since(start) >= wslHungAfter)
		}
		wsl.Lock()
		wsl.names, wsl.listed, wsl.at = names, listed, time.Now()
		if !listed || len(names) == 0 {
			// none to run: a wsl.exe that couldn't list them isn't asked
			// again which run
			wsl.running, wsl.runOK, wsl.runAt = map[string]bool{}, listed, time.Now()
		}
		wsl.Unlock()
	}
	wslRunningWithin(-1)
	wsl.Lock()
	var probe []string
	for _, n := range wsl.names {
		// a distro that stopped is probed again once it runs: WSL may have
		// been restarted meanwhile in another networking mode
		if !wsl.running[n] {
			delete(wsl.probed, n)
		}
		if wsl.running[n] && !wsl.probed[n] {
			if t, ok := wsl.failed[n]; !ok || time.Since(t) > wslRetryAge {
				probe = append(probe, n)
			}
		}
	}
	wsl.Unlock()
	for _, n := range probe {
		d := wslProbe(n)
		wsl.Lock()
		if d == nil {
			wsl.failed[n] = time.Now()
		} else {
			wsl.probed[n] = true
			if wslKept(*d) {
				if old := wsl.seen[n]; old != nil {
					d.Values = old.Values
					d.Was = wslWas(old.Was, old.Gateway, d.Gateway)
				}
				wsl.seen[n] = d
			} else {
				delete(wsl.seen, n)
			}
			wsl.dirty = true
		}
		wsl.Unlock()
	}
	wsl.Lock()
	defer wsl.Unlock()
	// an unregistered distro is forgotten
	if wsl.listed {
		installed := map[string]bool{}
		for _, n := range wsl.names {
			installed[n] = true
		}
		for n := range wsl.seen {
			if !installed[n] {
				delete(wsl.seen, n)
				wsl.dirty = true
			}
		}
	}
	wslSaveLocked()
}

// wslLoadLocked reads wsl.json the first time it is wanted.
func wslLoadLocked() {
	if wsl.seen != nil {
		return
	}
	wsl.seen, wsl.probed, wsl.failed = map[string]*distro{}, map[string]bool{}, map[string]time.Time{}
	if b, err := os.ReadFile(wslStatePath()); err == nil {
		json.Unmarshal(b, &wsl.seen)
	}
	wslForgetOldBins()
}

// wslDistros are the distros agents were found in, after a refresh
// (wslRefresh), which this waits for: for the GUI and what runs in the
// background. A request's path takes wslDistrosWithin.
func wslDistros() []distro {
	wslRefresh()
	return wslKnown()
}

// wslBg is the refresh wslDistrosWithin started, while it runs.
var wslBg struct {
	sync.Mutex
	done chan struct{}
}

// wslDistrosWithin is wslDistros waiting at most wait for the refresh,
// one at a time in the background: what is known then is answered, and
// a distro whose running wsl.exe hasn't said in time is taken as stopped.
func wslDistrosWithin(wait time.Duration) []distro {
	wslBg.Lock()
	if wslBg.done == nil {
		done := make(chan struct{})
		wslBg.done = done
		go func() {
			wslRefresh()
			wslBg.Lock()
			wslBg.done = nil
			wslBg.Unlock()
			close(done)
		}()
	}
	done := wslBg.done
	wslBg.Unlock()
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-done:
	case <-t.C:
	}
	return wslKnown()
}

// wslKnown are the distros as last refreshed, each with whether it runs:
// none does while wsl.exe is being asked and its last answer is older
// than wslRunningAge.
func wslKnown() []distro {
	asking := wslRunAsking()
	cfg := wslConfig()
	wsl.Lock()
	wslLoadLocked()
	known := !asking || wslRunFreshLocked()
	var out []distro
	for _, n := range wsl.names {
		d := wsl.seen[n]
		if d == nil {
			continue
		}
		c := *d
		c.Running = known && wsl.running[n]
		c.up = func() bool { return wslUp(n) }
		out = append(out, c)
	}
	wsl.Unlock()
	for i := range out {
		out[i].mirror(cfg)
	}
	return out
}

// wslWas is the addresses a distro reached Windows at before, with the
// last one (was) added when it reaches Windows elsewhere now: the newest
// few, now's left out.
func wslWas(before []string, was, now string) []string {
	out := slices.DeleteFunc(slices.Clone(before), func(h string) bool { return h == was || h == now })
	if was != "" && was != now {
		out = append(out, was)
	}
	if len(out) > 4 {
		out = out[len(out)-4:]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// wslForgetOldBins drops the commands a probe before wslProbeVersion
// found: it took a Windows command on the PATH WSL inherits (Windows npm's
// pi under /mnt/c, #406) for one in the distro. A distro then found by
// nothing else is forgotten, until a probe of it while it runs says again.
func wslForgetOldBins() {
	for n, d := range wsl.seen {
		if d == nil || d.Probe >= wslProbeVersion {
			continue
		}
		for k := range d.Has {
			if strings.HasPrefix(k, "bin:") {
				delete(d.Has, k)
				wsl.dirty = true
			}
		}
		if !wslKept(*d) {
			delete(wsl.seen, n)
			wsl.dirty = true
		}
	}
}

// wslRunAsk is the wsl.exe -l --running in flight, and when it started.
// It is asked outside wsl's lock, once at a time however many wait on it.
var wslRunAsk struct {
	sync.Mutex
	done  chan struct{}
	start time.Time
}

// wslRunningWait is how long what reads a distro's files waits for
// wsl.exe to say which run, from when it was asked: one that hangs is
// taken as running none, and a stand-in, a request's or Copilot's
// sign-in, goes without the distro's files rather than waiting out its
// 10s (vsoullai on X, #6167). A var for tests.
var wslRunningWait = 2 * time.Second

// wslRunFreshLocked is whether which distros run is as wsl.exe said at
// most wslRunningAge ago (wslRunningFailedAge when it couldn't say).
func wslRunFreshLocked() bool {
	age := wslRunningAge
	if !wsl.runOK {
		age = wslRunningFailedAge
	}
	return !wsl.runAt.IsZero() && time.Since(wsl.runAt) <= age
}

// wslRunAsking is whether wsl.exe is being asked which distros run.
func wslRunAsking() bool {
	wslRunAsk.Lock()
	defer wslRunAsk.Unlock()
	return wslRunAsk.done != nil
}

// wslRunningWithin lists the running distros again (wsl.exe -l
// --running, which starts none) when the last list is older than
// wslRunningAge, and waits for the answer at most wait from when it was
// asked (a negative wait: until wsl.exe answers or times out). false:
// wsl.exe hasn't said in time, and none is to be taken to run. When
// wsl.exe can't say, none is taken to run either: a distro is never
// opened on a guess. Never called under wsl's lock.
func wslRunningWithin(wait time.Duration) bool {
	wsl.Lock()
	fresh := wslRunFreshLocked()
	wsl.Unlock()
	if fresh {
		return true
	}
	wslRunAsk.Lock()
	if wslRunAsk.done == nil {
		done := make(chan struct{})
		wslRunAsk.done, wslRunAsk.start = done, time.Now()
		go func() {
			run, ok := wslList("-l", "--running", "-q")
			wsl.Lock()
			wsl.runOK = ok
			wsl.running = map[string]bool{}
			for _, n := range run {
				wsl.running[n] = true
			}
			wsl.runAt = time.Now()
			wsl.Unlock()
			wslRunAsk.Lock()
			wslRunAsk.done = nil
			wslRunAsk.Unlock()
			close(done)
		}()
	}
	done, start := wslRunAsk.done, wslRunAsk.start
	wslRunAsk.Unlock()
	if wait < 0 {
		<-done
		return true
	}
	t := time.NewTimer(time.Until(start.Add(wait)))
	defer t.Stop()
	select {
	case <-done:
		return true
	case <-t.C:
		return false
	}
}

// wslUp is whether distro runs, as wsl.exe said at most wslRunningAge ago:
// what reads a distro's files in the background (a field shown, the
// catalog synced, requests counted, sessions listed) asks it first, so a
// distro stopped since its agent was made isn't started again.
func wslUp(distro string) bool {
	if !wslRunningWithin(wslRunningWait) {
		return false
	}
	wsl.Lock()
	defer wsl.Unlock()
	return wsl.running[distro]
}

// WSLRunning is whether the WSL distro runs now (wslUp); false off
// Windows. What another package does in a distro on its own, rather than
// at the user's asking, asks it first.
func WSLRunning(distro string) bool { return wslLooks() && wslUp(distro) }

// wslSave writes wsl.json if anything in it changed.
func wslSave() {
	wsl.Lock()
	defer wsl.Unlock()
	wslSaveLocked()
}

// wslSaveLocked writes wsl.seen when it changed. wsl.json holds every
// distro's last probe, and wslDistros reads a file that doesn't parse as
// no distro: such a file is copied aside first, and one that can't be
// copied isn't written over (#2263).
func wslSaveLocked() {
	if !wsl.dirty {
		return
	}
	if err := lastgood.KeepUnparsed(wslStatePath(), lastgood.Into[map[string]*distro]); err != nil {
		log.Printf("wsl.json wasn't written over: %v", err)
		return
	}
	if b, err := json.MarshalIndent(wsl.seen, "", "  "); err == nil && edit.WriteAtomic(wslStatePath(), b) == nil {
		wsl.dirty = false
	}
}

// wslRemember keeps what a distro's field reads, for while it is stopped.
func wslRemember(name, key, v string) {
	wsl.Lock()
	defer wsl.Unlock()
	d := wsl.seen[name]
	if d == nil || d.Values[key] == v {
		return
	}
	if d.Values == nil {
		d.Values = map[string]string{}
	}
	d.Values[key] = v
	wsl.dirty = true
}

// WSLKeep has eipgam remember v under key for the WSL distro name, in
// wsl.json beside what its probe found, for while the distro is stopped:
// what another package learnt of the distro's files while it ran (the
// library's places for each agent there), which it can't read again then
// without starting it (#3896). Nothing is kept for a distro never found.
func WSLKeep(name, key, v string) {
	wslRemember(name, key, v)
	wslSave()
}

// WSLKept is what WSLKeep last kept under key for the distro; "" for
// nothing.
func WSLKept(name, key string) string { return wslLastSeen(name, key) }

func wslLastSeen(name, key string) string {
	wsl.Lock()
	defer wsl.Unlock()
	if d := wsl.seen[name]; d != nil {
		return d.Values[key]
	}
	return ""
}

// wslReady are the distros running now that the probe found kind in, each
// with its mirrored networking settled, ready for a read of its files.
// Only eipgam's memory of them and which run (wslRunningWithin) are
// asked — nothing is probed, and a stopped distro is never opened.
// Detection off (wslLooks) reads none: a request's stand-in asks wsl.exe
// nothing, which on a machine without WSL would answer with a console
// window asking to install it (#4872).
func wslReady(kind string) []distro {
	if !wslLooks() {
		return nil
	}
	if !wslRunningWithin(wslRunningWait) {
		return nil
	}
	wsl.Lock()
	var ds []distro
	for _, n := range wsl.names {
		if d := wsl.seen[n]; d != nil && wsl.running[n] && wslKindOf(kind).found(*d) {
			ds = append(ds, *d)
		}
	}
	wsl.Unlock()
	cfg := wslConfig()
	for i := range ds {
		ds[i].mirror(cfg)
	}
	return ds
}

// wslClaudeStandIn is StandIn for a Claude Code in a WSL distro routed
// through eipgam: the first, of the distros running now, whose
// settings.json has a stand-in.
func wslClaudeStandIn(model string) string {
	for _, d := range wslReady("claude") {
		path := filepath.Join(d.local(d.Home), ".claude", "settings.json")
		if m := claudeStandInAt(d.place("claude@wsl:"+d.Name), path, model); m != "" {
			return m
		}
	}
	return ""
}

// wslClaudeClassifier is ClassifierStandIn for a Claude Code in a running
// distro, known by the same User-Agent. Detection off reads none (#4872).
func wslClaudeClassifier(model string) string {
	if !wslLooks() {
		return ""
	}
	if !wslRunningWithin(wslRunningWait) {
		return ""
	}
	wsl.Lock()
	var ds []distro
	for _, n := range wsl.names {
		if d := wsl.seen[n]; d != nil && wsl.running[n] && wslKindOf("claude").found(*d) {
			ds = append(ds, *d)
		}
	}
	wsl.Unlock()
	cfg := wslConfig()
	for _, d := range ds {
		d.mirror(cfg)
		if m := claudeClassifierIn(d.place("claude@wsl:"+d.Name), model); m != "" {
			return m
		}
	}
	return ""
}

// wslCodexStandIn is StandIn for a Codex in a WSL distro routed through
// eipgam: the first, of the distros running now, whose config.toml has
// eipgam as its provider, as this machine's codexStandIn reads it.
func wslCodexStandIn() string {
	for _, d := range wslReady("codex") {
		path := filepath.Join(d.place("codex@wsl:"+d.Name).codexHome(), "config.toml")
		if m := codexStandIn(path); m != "" {
			return m
		}
	}
	return ""
}

// wslList is the names wsl.exe lists with args; ok is false when it
// couldn't say (no WSL, or none installed). With no distro registered at
// all wsl.exe isn't run (proc.ErrNoWSLDistro), and that is an answer:
// none. One refused while WSL asks to be updated is not, so nothing
// remembered of a distro is forgotten for it.
func wslList(args ...string) (names []string, ok bool) {
	b, err := wslRun(10*time.Second, args...)
	if errors.Is(err, proc.ErrNoWSLDistro) {
		return nil, true
	}
	if err != nil {
		return nil, false
	}
	return parseDistros(b), true
}

// wslProbeVersion is that of wslProbeScript(): 2 says where each command is,
// so one from Windows' drives isn't taken for the distro's own.
// (wslinfo's networking mode came without a new version: a probe that
// didn't ask leaves Net "", read as .wslconfig says, as before.)
const wslProbeVersion = 2

// wslProbeScript() prints the distro's home, what of each of wslKinds it
// has (each command with where it is), where Windows' drives are mounted,
// its default route (the Windows host under NAT), and WSL's networking
// mode as wslinfo (WSL 2.0 on) says it; and the version of each that asks
// for it (omp, whose models.yml takes max only from 16.4.0).
//
// A command is also looked for in the folders its installers put it in
// (wslbin): sh -l reads ~/.profile but not ~/.bashrc, where bun's and
// nvm's installers add theirs to PATH (whqtian on Discord: a WSL omp from
// bun was never asked its version, so its models still offered xhigh).
// It is asked with its own folder on PATH, where bun's script finds bun.
//
// Of nvm's Nodes, the one a terminal there gets is looked in first: the
// version nvm's default alias resolves to, asked of nvm itself
// (nvm_version default, nvm.sh read by bash with --no-use), then the rest
// highest first. The glob this replaced took them in name order, so a
// command under v18 was found before the default v22's, and v9's after
// both (as proc.UserPath did on a Mac or Linux, 9f407cafa).
//
// The folders are those of proc.UserBinDirs: fnm's default and then its
// versions, mise's installs, ~/.npmrc's prefix and Homebrew's among them
// (千里不再 on Discord: a WSL Pi from fnm, not run yet so with no ~/.pi,
// wasn't found). The shell isn't asked interactively for the PATH ~/.bashrc
// builds: under timeout(1) an interactive bash stops itself on the
// terminal (state T) and outlived its timeout on WSL (Ubuntu 24.04).
//
// A command on Windows' drives isn't the distro's (parseProbe), so one
// there is looked past (wslwin): WSL puts Windows' PATH on the distro's,
// and a Pi npm installed on Windows hid the distro's own from wslbin.
var wslProbeScript = sync.OnceValue(func() string {
	s := `nv="${NVM_DIR:-$HOME/.nvm}"; nd=; [ -s "$nv/nvm.sh" ] && command -v bash >/dev/null 2>&1 && nd=$(NVM_DIR="$nv" timeout 10 bash -c '. "$NVM_DIR/nvm.sh" --no-use >/dev/null 2>&1; nvm_version default 2>/dev/null' </dev/null 2>/dev/null); ` +
		// Windows' drives, as /proc/mounts has them (C:\ written C:\134)
		`wm=$(awk '$1 ~ /^[A-Za-z]:/ {print $2}' /proc/mounts 2>/dev/null); ` +
		`wslwin() { case "$1" in /mnt/[A-Za-z]/*) return 0;; esac; for m in $wm; do case "$1/" in "$m"/*) return 0;; esac; done; return 1; }; ` +
		`inpath() { o=$IFS; IFS=:; r=; for d in $2; do case "$d" in /*) wslwin "$d" || { [ -f "$d/$1" ] && [ -x "$d/$1" ] && { r="$d/$1"; break; }; };; esac; done; IFS=$o; [ -n "$r" ] && echo "$r"; }; ` +
		`np=$(sed -n 's/^[[:space:]]*prefix[[:space:]]*=[[:space:]]*//p' "$HOME/.npmrc" 2>/dev/null | head -n1 | tr -d "\"'\r"); case "$np" in "~/"*) np="$HOME/${np#??}";; /*) ;; *) np=;; esac; ` +
		`fr=; for r in "$FNM_DIR" "${XDG_DATA_HOME:-$HOME/.local/share}/fnm" "$HOME/.fnm"; do case "$r" in /*) [ -d "$r/node-versions" ] && { fr=$r; break; };; esac; done; ` +
		`vs() { ls "$1" 2>/dev/null | sed 's/^v*\(.*\)$/\1 &/' | sort -t. -k1,1nr -k2,2nr -k3,3nr | cut -d' ' -f2; }; ` +
		`wslbin() { for d in "$HOME/.bun/bin" "$HOME/.local/bin" "$HOME/.npm-global/bin" "$HOME/.volta/bin" "$HOME/.local/share/pnpm"; do [ -x "$d/$1" ] && { echo "$d/$1"; return 0; }; done; ` +
		`for v in $nd $(ls "$nv/versions/node" 2>/dev/null | sort -t. -k1.2,1nr -k2,2nr -k3,3nr); do d="$nv/versions/node/$v/bin"; [ -x "$d/$1" ] && { echo "$d/$1"; return 0; }; done; ` +
		`if [ -n "$fr" ]; then for d in "$fr/aliases/default/bin" $(vs "$fr/node-versions" | sed "s|.*|$fr/node-versions/&/installation/bin|"); do [ -x "$d/$1" ] && { echo "$d/$1"; return 0; }; done; fi; ` +
		`for d in $(vs "$HOME/.local/share/mise/installs/node" | sed "s|.*|$HOME/.local/share/mise/installs/node/&/bin|"); do [ -x "$d/$1" ] && { echo "$d/$1"; return 0; }; done; ` +
		`for d in "$HOME/.local/share/mise/shims" "$HOME/.asdf/shims" "$HOME/.npm/bin" ${np:+"$np/bin"} "$HOME/.cargo/bin" "$HOME/.deno/bin" /home/linuxbrew/.linuxbrew/bin "$HOME/.linuxbrew/bin"; do [ -x "$d/$1" ] && { echo "$d/$1"; return 0; }; done; return 1; }; ` +
		`wslcmd() { p=$(command -v "$1" 2>/dev/null) && case "$p" in /*) wslwin "$p" || { echo "$p"; return 0; };; esac; inpath "$1" "$PATH" || wslbin "$1"; }; `
	s += `echo "home:$HOME"; `
	for _, k := range wslKinds {
		if k.dir != "" {
			s += `[ -d "$HOME/` + k.dir + `" ] && echo dir:` + k.dir + `; `
		}
		if k.bin != "" {
			s += `p=$(wslcmd ` + k.bin + `) && echo "bin:` + k.bin + ` $p"`
			if k.version {
				// not one on Windows' drives (WSL's /mnt/c), which isn't the distro's
				s += ` && case "$p" in /mnt/*) ;; *) echo "ver:` + k.id + ` $(PATH="${p%/*}:$PATH" timeout 10 "$p" --version </dev/null 2>&1 | head -n1)";; esac`
			}
			s += `; `
		}
	}
	s += `[ -d "$HOME/` + wslCopilotDir + `" ] && echo dir:` + wslCopilotDir + `; `
	for _, dir := range wslAntigravityDirs {
		s += `[ -d "$HOME/` + dir + `" ] && echo dir:` + dir + `; `
	}
	// where each agent folder that leads elsewhere leads (distro.open):
	// "link:.codex /mnt/c/Users/me/.codex", or "link:.codex" alone where
	// readlink -f couldn't say
	// (a $HOME that is itself through a link is left as it is)
	s += `h=$(readlink -f -- "$HOME" 2>/dev/null); for r in ` + strings.Join(wslLinkPaths(), " ") + `; do p="$HOME/$r"; { [ -e "$p" ] || [ -L "$p" ]; } || continue; ` +
		`if t=$(readlink -f -- "$p" 2>/dev/null) && [ -n "$t" ]; then [ "$t" = "$p" ] || [ "$t" = "$h/$r" ] || echo "link:$r $t"; else echo "link:$r"; fi; done; `
	// a drive's source in /proc/mounts is C:\ (written C:\134), under any
	// automount root: its mount point, and the drive
	s += `awk '$1 ~ /^[A-Za-z]:/ {print "win:" $2 " " toupper(substr($1,1,1)) ":"}' /proc/mounts 2>/dev/null; `
	return s + `ip route show default 2>/dev/null | head -n1 | sed 's/^/route:/'; ` +
		`grep -m1 '^nameserver' /etc/resolv.conf 2>/dev/null | sed 's/^/ns:/'; ` +
		`command -v wslinfo >/dev/null 2>&1 && echo "net:$(wslinfo --networking-mode 2>/dev/null)"; true`
})

// wslLinkPaths are the paths under a distro's $HOME the probe asks where
// they lead (distro.Links): each agent's folder and its skills, and the
// folders and files the agents keep beside theirs.
func wslLinkPaths() []string {
	out := []string{".config", ".local", ".local/share", ".agents", ".agents/skills", ".gemini", ".gemini/config", ".gemini/skills", ".kimi", ".reasonix", ".claude.json", ".slock", ".kimi-code/sessions", wslCopilotDir}
	for _, k := range wslKinds {
		if k.dir != "" {
			out = append(out, k.dir, k.dir+"/skills")
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func wslProbe(name string) *distro {
	b, err := wslRun(30*time.Second, "-d", name, "-e", "sh", "-lc", wslProbeScript())
	if err != nil {
		return nil
	}
	d := parseProbe(name, string(b))
	if d == nil {
		return nil
	}
	if wslKept(*d) {
		if d.Root = wslRoot(name); d.Root == "" {
			return nil
		}
	}
	return d
}

// parseProbe reads wslProbeScript()'s output. A command on one of Windows'
// drives — WSL puts Windows' PATH on its own, so Windows npm's pi is
// /mnt/c/Users/me/AppData/Roaming/npm/pi there — is Windows', not the
// distro's.
func parseProbe(name, out string) *distro {
	d := &distro{Name: name, Has: map[string]bool{}, Probe: wslProbeVersion}
	var ns string
	bins := map[string]string{}
	var win []string
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		switch k, v, _ := strings.Cut(l, ":"); k {
		case "home":
			d.Home = strings.TrimRight(v, "/")
		case "dir":
			d.Has[l] = true
		case "ver":
			id, said, _ := strings.Cut(v, " ")
			if ver := parseVersion(said); ver != "" {
				if d.Versions == nil {
					d.Versions = map[string]string{}
				}
				d.Versions[id] = ver
			}
		case "bin":
			bin, path, _ := strings.Cut(v, " ")
			bins[bin] = strings.TrimSpace(path)
		case "win":
			// win:/mnt/c C: (a probe before it said only /mnt/c)
			f := strings.Fields(v)
			if len(f) == 0 {
				continue
			}
			if m := strings.TrimRight(f[0], "/"); strings.HasPrefix(m, "/") {
				win = append(win, m+"/")
				if len(f) > 1 && len(f[1]) == 2 && f[1][1] == ':' && isLetter(f[1][0]) {
					if d.Drives == nil {
						d.Drives = map[string]string{}
					}
					d.Drives[m] = strings.ToUpper(f[1])
				}
			}
		case "link":
			// link:.codex /mnt/c/Users/me/.codex; link:.codex alone where
			// it couldn't be resolved
			rel, to, _ := strings.Cut(v, " ")
			if !slices.Contains(wslLinkPaths(), rel) {
				continue
			}
			if to = strings.TrimSpace(to); !strings.HasPrefix(to, "/") {
				to = ""
			}
			if d.Links == nil {
				d.Links = map[string]string{}
			}
			d.Links[rel] = strings.TrimRight(to, "/")
		case "route":
			// default via 172.20.0.1 dev eth0 …
			if f := strings.Fields(v); len(f) >= 3 && f[1] == "via" && net.ParseIP(f[2]) != nil {
				d.Gateway = f[2]
			}
		case "ns":
			if f := strings.Fields(v); len(f) >= 2 && net.ParseIP(f[1]) != nil {
				ns = f[1]
			}
		case "net":
			// one word; anything else (an old wslinfo's usage) says nothing
			if m := strings.ToLower(strings.TrimSpace(v)); m != "" && !strings.ContainsAny(m, " \t") {
				d.Net = m
			}
		}
	}
	if !strings.HasPrefix(d.Home, "/") {
		return nil
	}
	for bin, path := range bins {
		if !onWindowsDrive(path, win) {
			d.Has["bin:"+bin] = true
			if strings.HasPrefix(path, "/") {
				if d.Paths == nil {
					d.Paths = map[string]string{}
				}
				d.Paths[bin] = path
			}
		}
	}
	if d.Gateway == "" {
		d.Gateway = ns
	}
	// the default route of a distro whose 127.0.0.1 is Windows' is
	// Windows' own next hop (consomme: 198.18.0.2 under a TUN proxy,
	// #1230), not Windows: an agent left on it is moved to 127.0.0.1
	// (wslMovedFrom). A bridged distro's is the LAN's router: Windows is
	// at its address on the bridge's adapter (wslrun.HostAt), and an agent
	// an older eipgam pointed at the router is written over on the next
	// sync, the router being among the addresses it was seen at (Was)
	d.Gateway = wslrun.HostAt(d.Net, d.Gateway, gateway.WSLNets())
	return d
}

// onWindowsDrive is whether path, inside a distro, is on one of Windows'
// drives: under a mount win lists, or under /mnt/<letter>/ as WSL mounts
// them unless told otherwise.
func onWindowsDrive(path string, win []string) bool {
	for _, w := range win {
		if strings.HasPrefix(path, w) {
			return true
		}
	}
	rest, ok := strings.CutPrefix(path, "/mnt/")
	return ok && len(rest) >= 2 && rest[1] == '/' &&
		(rest[0] >= 'a' && rest[0] <= 'z' || rest[0] >= 'A' && rest[0] <= 'Z')
}

// parseDistros reads wsl.exe -l -q: UTF-16LE (with or without a BOM), or
// UTF-8 under WSL_UTF8. Docker Desktop's own distros are left out.
func parseDistros(b []byte) []string {
	s := decodeWSL(b)
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(strings.Trim(l, "\x00\ufeff\r"))
		if l == "" || strings.HasPrefix(strings.ToLower(l), "docker-desktop") {
			continue
		}
		out = append(out, l)
	}
	return out
}

func decodeWSL(b []byte) string {
	utf16le := len(b) >= 2 && (b[0] == 0xff && b[1] == 0xfe || b[1] == 0 && b[0] != 0)
	if !utf16le {
		return string(b)
	}
	if b[0] == 0xff && b[1] == 0xfe {
		b = b[2:]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return string(utf16.Decode(u))
}

// wslConfig is %UserProfile%\.wslconfig, "" if there is none: UTF-8, or
// UTF-16 as Notepad's "Unicode" saves it, which WSL reads as well.
func wslConfig() string {
	home, _ := os.UserHomeDir() // %UserProfile% on Windows
	b, _ := os.ReadFile(filepath.Join(home, ".wslconfig"))
	return decodeText(b)
}

// decodeText is a text file's contents: UTF-16 with a BOM (either byte
// order), UTF-16LE without one, or else UTF-8.
func decodeText(b []byte) string {
	if len(b) >= 2 && b[0] == 0xfe && b[1] == 0xff {
		u := make([]uint16, (len(b)-2)/2)
		for i := range u {
			u[i] = uint16(b[2+2*i])<<8 | uint16(b[3+2*i])
		}
		return string(utf16.Decode(u))
	}
	return decodeWSL(b)
}

// wslMirrored reports whether a .wslconfig puts WSL 2 in mirrored
// networking: networkingMode=mirrored under [wsl2].
func wslMirrored(cfg string) bool {
	v, _ := wslSetting(cfg, "networkingMode")
	return strings.EqualFold(v, "mirrored")
}

// wslOff reports whether a .wslconfig turns key under [wsl2] off: false,
// as WSL reads a boolean (false, or 0).
func wslOff(cfg, key string) bool {
	v, ok := wslSetting(cfg, key)
	return ok && (strings.EqualFold(v, "false") || v == "0")
}

// wslSetting is key's value under [wsl2] in a .wslconfig, the last one
// set, unquoted and without a trailing comment; ok is false when it isn't
// set there.
func wslSetting(cfg, key string) (val string, ok bool) {
	section := ""
	sc := bufio.NewScanner(strings.NewReader(strings.TrimPrefix(cfg, "\ufeff")))
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" || l[0] == '#' || l[0] == ';' {
			continue
		}
		if strings.HasPrefix(l, "[") {
			// [wsl2], or [wsl2] # a comment
			if end := strings.Index(l, "]"); end > 0 {
				section = strings.ToLower(strings.TrimSpace(l[1:end]))
				continue
			}
		}
		k, v, found := strings.Cut(l, "=")
		if !found || section != "wsl2" || !strings.EqualFold(strings.TrimSpace(k), key) {
			continue
		}
		if i := strings.IndexAny(v, "#;"); i >= 0 {
			v = v[:i]
		}
		val, ok = strings.Trim(strings.TrimSpace(v), `"`), true
	}
	return val, ok
}

// what an agent in WSL says after a change, besides its kind's restart
// (notice.go)
var (
	noticeWSLNotMirrored = newNotice("WSL {distro} is in {net} networking, not mirrored, so its {agent} was pointed at Windows ({url}) rather than 127.0.0.1; eipgam listens there for WSL, and it answers once Windows' firewall lets WSL in. For 127.0.0.1, set networkingMode=mirrored under [wsl2] in %UserProfile%\\.wslconfig and run wsl --shutdown, then pick the model again.")
	noticeWSLNoMode      = newNotice("eipgam couldn't ask WSL {distro} its networking mode (it has no wslinfo) and %UserProfile%\\.wslconfig doesn't set it to mirrored, so its {agent} was pointed at Windows ({url}) rather than 127.0.0.1; eipgam listens there for WSL, and it answers once Windows' firewall lets WSL in. For 127.0.0.1, set networkingMode=mirrored under [wsl2] in %UserProfile%\\.wslconfig and run wsl --shutdown, then pick the model again.")
	noticeWSLNoRelay     = newNotice("WSL {distro} is in {net} networking with localhostForwarding=false in %UserProfile%\\.wslconfig, so 127.0.0.1 there, where its {agent} was pointed, isn't relayed to Windows, where eipgam listens. Remove localhostForwarding=false under [wsl2] (it is on by default) and run wsl --shutdown.")
	noticeWSLAsleep      = newNotice("WSL {distro} isn't running: eipgam shows what it last saw there, and starts it only to change something.")
)

// H1_B1_K1 on X: a company's firewall (火绒企业版), which its user can't
// open, kept a distro under NAT off Windows' address on vEthernet (WSL),
// while Windows' own address on the LAN answered from WSL. Where the
// gateway's address on the WSL network (or the WSL via in its place) is
// found shut from inside a distro under NAT, eipgam asks the gateway to
// listen at each of Windows' other addresses as well (gateway.SetWSLVia,
// let in only from Windows itself or WSL), asks from the distro which of
// them answers, keeps that one as the via and lets the others go. Every
// NAT distro's agents on eipgam are then set again to reach it there
// (distro.host, Rewire), the address they were on being eipgam's to move
// (wslHost, wslMovedFrom). Where the via is found shut later and the WSL
// network's address answers again, the via is dropped. A setting isn't
// needed: what answers is found.

// wslViaCandidates are Windows' own IPv4 addresses off the WSL networks:
// its LAN's first, other vEthernet adapters' (Hyper-V's Default Switch)
// last. None off Windows. A var for the tests.
var wslViaCandidates = func() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var first, last []string
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 || strings.Contains(i.Name, "(WSL") {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok || n.IP.To4() == nil || n.IP.IsLinkLocalUnicast() || n.IP.IsLoopback() || gateway.OnWSLNet(n.IP.String()) {
				continue
			}
			if strings.HasPrefix(i.Name, "vEthernet") {
				last = append(last, n.IP.String())
			} else {
				first = append(first, n.IP.String())
			}
		}
	}
	return append(first, last...)
}

// wslViaListening waits until the gateway listens at ip, as told from here
// (a connection from Windows itself is let in there); the gateway in
// another process looks for new vias every 10s. A var for the tests.
var wslViaListening = func(ip string) bool {
	addr := net.JoinHostPort(ip, gateway.Port())
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(250 * time.Millisecond) {
		if c, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
			c.Close()
			return true
		}
	}
	return false
}

// wslViaMu lets one search for a via run at a time.
var wslViaMu sync.Mutex

// wslFindVia looks for a way from distro d to the gateway, once shut, the
// address it found shut: the WSL network's again, else another of
// Windows' addresses (see above), tried one by one while the gateway
// listens at all of them. The one that answers is kept; the agents of
// every distro under NAT on eipgam then are set again to reach it there.
func wslFindVia(d distro, shut string) {
	wslViaMu.Lock()
	defer wslViaMu.Unlock()
	port := gateway.Port()
	url := func(ip string) string { return "http://" + net.JoinHostPort(ip, port) }
	var old []string
	for _, ip := range gateway.WSLVia() {
		old = append(old, ip.String())
	}
	keep := func(via []string) {
		if slices.Equal(via, old) {
			if err := gateway.SetWSLVia(old, nil); err != nil {
				log.Printf("wsl: can't keep the way to eipgam from WSL: %v", err)
			}
			return
		}
		on := wslOnGateway()
		from := make([]string, len(on))
		for i, a := range on {
			from[i] = a.pointedAt()
		}
		if err := gateway.SetWSLVia(via, nil); err != nil {
			log.Printf("wsl: can't keep the way to eipgam from WSL: %v", err)
			return
		}
		if len(old) > 0 {
			wslViaLeft(old[0])
		}
		// one that moves its own addresses (Codex's two URLs) is moved;
		// the others are set again
		var rest []*Agent
		var moved []string
		var errs []error
		for i, a := range on {
			to := ""
			if a.Gateway != nil {
				to = a.Gateway()
			}
			if a.move == nil || from[i] == "" || to == "" || from[i] == to {
				rest = append(rest, a)
				continue
			}
			if err := a.move(from[i], to); err != nil {
				errs = append(errs, err)
				continue
			}
			moved = append(moved, a.Name)
		}
		m, err := Rewire(rest)
		moved = append(moved, m...)
		if err != nil {
			errs = append(errs, err)
		}
		if len(moved) > 0 || len(errs) > 0 {
			log.Printf("wsl: set again to reach eipgam from WSL: %v %v", moved, joinErrs(errs))
		}
	}
	if len(old) > 0 && shut != url(old[0]) {
		// found already, by another agent's ask
		if ok, told := wslReach(d.Name, url(old[0])); ok && told {
			return
		}
	}
	if len(old) > 0 && shut != url(d.Gateway) {
		// the via shut: the WSL network's address may answer again
		if ok, told := wslReach(d.Name, url(d.Gateway)); ok && told {
			log.Printf("wsl: WSL %s reaches eipgam at %s again", d.Name, url(d.Gateway))
			keep(nil)
			return
		}
	}
	var cands []string
	for _, c := range wslViaCandidates() {
		if url(c) != shut {
			cands = append(cands, c)
		}
	}
	if len(cands) == 0 {
		return
	}
	if err := gateway.SetWSLVia(old, cands); err != nil {
		log.Printf("wsl: can't listen for WSL beyond its network: %v", err)
		return
	}
	for _, c := range cands {
		if !wslViaListening(c) {
			continue
		}
		if ok, told := wslReach(d.Name, url(c)); ok && told {
			log.Printf("wsl: WSL %s can't reach eipgam at %s; it reaches it at %s", d.Name, shut, c)
			keep([]string{c})
			return
		}
	}
	keep(old)
}

// wslOnGateway are the agents on eipgam in distros under NAT, taken
// before their way to Windows moves (as OnGateway before a port change).
// A var for the tests, set in init: its agents' probes find vias.
var wslOnGateway func() []*Agent

func init() { wslOnGateway = wslOnGatewayNow }

func wslOnGatewayNow() []*Agent {
	var out []*Agent
	for _, a := range wslAgents() {
		if len(a.Fields) == 0 || !a.Wired() {
			continue
		}
		if dr := a.Drift(); dr != nil && dr.Kind == "unwired" {
			continue
		}
		out = append(out, a)
	}
	return out
}

// wslViaLeft notes a via let go as an address every NAT distro reached
// Windows at before (Was), so an agent still on it is eipgam's to move.
func wslViaLeft(via string) {
	wsl.Lock()
	defer wsl.Unlock()
	for _, d := range wsl.seen {
		if d == nil || d.Gateway == "" || d.Net == "bridged" || wslrun.HostLoopback(d.Net) || slices.Contains(d.Was, via) {
			continue
		}
		d.Was = append(d.Was, via)
		wsl.dirty = true
	}
	wslSaveLocked()
}
