package library

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/jsonc"

	"github.com/sichengchen/eipgam/internal/agent"
	"github.com/sichengchen/eipgam/internal/agentplug"
	"github.com/sichengchen/eipgam/internal/appdir"
	"github.com/sichengchen/eipgam/internal/desktopdir"
	"github.com/sichengchen/eipgam/internal/plugin"
	"github.com/sichengchen/eipgam/internal/provider"
)

// Target is where one agent keeps each of the three: an empty path is
// something it has no user-wide place for.
type Target struct {
	Agent *agent.Agent
	// Instructions is the file the agent reads before every conversation;
	// Override, when it exists, is read instead of it (Codex's
	// AGENTS.override.md), which the page warns of.
	Instructions, Override string
	// Field, when it is set, says the instructions are not the file itself
	// but the string at this key path of the JSON file Instructions names
	// (WorkBuddy's personalization.customPrompt): instr_field.go.
	Field  string
	MCP    *mcpFile
	Skills string // the folder the agent finds skills in
	// SkillsAlso are agents whose skills this one reads as well, as
	// OpenCode reads Claude Code's.
	SkillsAlso []string
	// MCPVia is the extension the agent reads its MCP servers through, for
	// one that has none of its own.
	MCPVia string
	// Note is what the page says of the agent's instructions file.
	Note string
	// Copy is an agent that gets a copy of each skill, made again when
	// the library's changes, rather than a link: one in a WSL distro whose
	// skills folder is on a Windows drive (wslStored), LiveAgent, and on
	// Windows one that skips a junction (skipsJunctions).
	Copy bool
	// Store, for an agent in a WSL distro, is eipgam's folder there
	// (wslStoreRel) holding one copy of each skill given to the distro's
	// agents, as eipgam opens it: each agent gets a link the distro makes
	// to that copy, as it can't follow one to a Windows folder
	// (wsl_skills.go).
	Store string
	// Desktop are Claude Desktop's skills-plugin folders, one for each
	// account (desktop_skills.go): each gets a copy of the skills and their
	// entries in its manifest.json. Skills is the first one's skills.
	Desktop []string
	// Settings is Claude Code's settings.json, whose permissions.deny
	// leaves out the tools of a server turned off (mcp_tools_off.go)
	Settings string
	// Gone is an agent no longer installed here that still has what eipgam
	// gave it (held): a sync only takes things out of it, as they are
	// unticked, and gives it nothing.
	Gone bool
	// SharedToo is an agent that reads ~/.agents/skills besides its own
	// folder only from some version on (Pi, omp: sharedSince), with that
	// version here; the others that always do are in readsShared
	SharedToo bool
	// Profile says the instructions file is the user's profile to the
	// agent, not instructions alone (Alma's USER.md): eipgam adds its part
	// after the user's, but never moves the user's own lines out of it
	// (ImportInstructions), as the agent reads who its owner is there.
	Profile bool
	// Unreached, for an agent in a WSL distro, is why its folder there
	// can't be opened (agent.WSLOpen): a link the distro couldn't
	// resolve. Nothing is written to it; each sync says so.
	Unreached error
}

// unreached says whether t's folder can't be opened, and, once in res,
// why.
func (r *Result) unreached(t *Target) bool {
	if t.Unreached == nil {
		return false
	}
	for _, p := range r.Problems {
		if p.Agent == t.Agent.ID && p.What == "wsl" {
			return true
		}
	}
	r.fail(t.Agent.ID, "wsl", t.Unreached)
	return true
}

func home() string { h, _ := os.UserHomeDir(); return h }

// grokDir is where Grok Build keeps its config.toml: $GROK_HOME, else ~/.grok
// (internal/agent/grok.go grokIn).
func grokDir() string {
	if d := appdir.Getenv("GROK_HOME"); d != "" {
		return d
	}
	return filepath.Join(home(), ".grok")
}

func claudeDir() string { return appdir.ClaudeDirIn(home()) }

// claudeJSON is where Claude Code keeps its user-wide MCP servers: beside
// its folder, or in it when CLAUDE_CONFIG_DIR moves it.
func claudeJSON() string {
	if d := appdir.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, ".claude.json")
	}
	return filepath.Join(home(), ".claude.json")
}

func codexDir() string { return appdir.CodexHomeIn(home()) }

// targetOf is where a known agent keeps them, or nil for one eipgam can't
// give any of them to.
func targetOf(a *agent.Agent) *Target {
	if a.WSL != "" {
		return wslTargetOf(a)
	}
	h := home()
	t := &Target{Agent: a}
	if strings.HasPrefix(a.ID, "hermes#") {
		// another Hermes profile (agent.hermesProfileAgents) is a Hermes
		// home of its own: its config.yaml holds its mcp_servers, and its
		// skills folder is beside it
		t.MCP = &mcpFile{Path: a.Path, Format: fmtHermes}
		t.Skills = filepath.Join(a.Dir, "skills")
		return t
	}
	if strings.HasPrefix(a.ID, "claude#") {
		// another Claude Code config folder (agent.claudeDirAgents), the
		// isolated Desktop's Code tab's among them (xMopf7RF on X): Claude
		// Code run with CLAUDE_CONFIG_DIR there reads CLAUDE.md and skills
		// in it, and its MCP servers from the .claude.json in it; the
		// default ~/.claude's are in ~/.claude.json beside it
		t.Instructions = filepath.Join(a.Dir, "CLAUDE.md")
		t.Skills = filepath.Join(a.Dir, "skills")
		mcp := filepath.Join(a.Dir, ".claude.json")
		if filepath.Clean(a.Dir) == filepath.Join(h, ".claude") {
			mcp = filepath.Join(h, ".claude.json")
		}
		t.MCP = &mcpFile{Path: mcp, Format: fmtClaude}
		return t
	}
	if n := a.SharedName(); n != a.Name {
		// the files are another app's too (the Cursor editor's ~/.cursor):
		// named for both, not for the CLI alone
		named := *a
		named.Name = n
		t.Agent = &named
	}
	switch a.ID {
	case "claude":
		d := claudeDir()
		t.Instructions = filepath.Join(d, "CLAUDE.md")
		t.MCP = &mcpFile{Path: claudeJSON(), Format: fmtClaude}
		t.Skills = filepath.Join(d, "skills")
		t.Settings = filepath.Join(d, "settings.json")
	case "codex":
		d := codexDir()
		t.Instructions = filepath.Join(d, "AGENTS.md")
		t.Override = filepath.Join(d, "AGENTS.override.md")
		t.MCP = &mcpFile{Path: filepath.Join(d, "config.toml"), Format: fmtCodex}
		t.Skills = filepath.Join(d, "skills")
	case "gemini":
		d := filepath.Join(h, ".gemini")
		t.Instructions = filepath.Join(d, "GEMINI.md")
		t.MCP = &mcpFile{Path: filepath.Join(d, "settings.json"), Format: fmtGemini}
		t.Skills = filepath.Join(d, "skills")
	case "agy":
		// Antigravity — the app and agy alike — reads its user-wide
		// customizations from ~/.gemini/config: GEMINI.md there (as well as
		// Gemini CLI's ~/.gemini/GEMINI.md), mcp_config.json and skills
		// (antigravity.google/docs/rules, /mcp, /skills; agy 1.2's `mcp
		// add` writes that file); agy 1.2 reads no mcp_config.json from the
		// IDE's older ~/.gemini/antigravity.
		d := filepath.Join(h, ".gemini", "config")
		t.Instructions = filepath.Join(d, "GEMINI.md")
		t.MCP = &mcpFile{Path: filepath.Join(d, "mcp_config.json"), Format: fmtAntigravity}
		t.Skills = filepath.Join(d, "skills")
		t.Copy = skipsJunctions(a.ID)
	case "opencode":
		// DevEco Code is OpenCode's fork and reads as it does: AGENTS.md
		// in its config folder, else Claude Code's CLAUDE.md, skills there
		// and under ~/.claude and ~/.agents (packages/opencode/src/session
		// /instruction.ts, skill/index.ts)
		d := filepath.Dir(a.Path)
		t.Instructions = filepath.Join(d, "AGENTS.md")
		t.Note = "opencode-claude"
		t.MCP = &mcpFile{Path: a.Path, Format: fmtOpenCode}
		t.Skills = filepath.Join(d, "skills")
		t.SkillsAlso = []string{"claude"}
	case "pi":
		// PI_CODING_AGENT_DIR's, else ~/.pi/agent (agent.piDir). Pi reads
		// AGENTS.override.md there when it is there, else the first there
		// is of AGENTS.md, AGENTS.MD, CLAUDE.md and CLAUDE.MD
		// (loadContextFileFromDir in dist/core/resource-loader.js,
		// @earendil-works/pi-coding-agent 1.2.0; the older
		// @mariozechner/pi-coding-agent has the same order without the
		// override), so a CLAUDE.md of the user's own is the one to edit
		d := a.Dir
		t.Instructions = firstThere(d, "AGENTS.md", "AGENTS.MD", "CLAUDE.md", "CLAUDE.MD")
		t.Override = filepath.Join(d, "AGENTS.override.md")
		// Pi 0.99 reads MCP servers itself, from its mcp.json; before it,
		// and while an MCP extension replaces its own (pi-mcp-adapter,
		// pi-mcp-extension), each extension reads its own file (pimcp.go)
		v := piVersion(a)
		t.MCP, t.MCPVia = piMCP(h, d, v, piGlobalRoots())
		t.Skills = filepath.Join(d, "skills")
		t.SharedToo = since(v, piSharedSince)
	case "cursor":
		d := filepath.Join(h, ".cursor")
		t.MCP = &mcpFile{Path: filepath.Join(d, "mcp.json"), Format: fmtCursor}
		t.Skills = filepath.Join(d, "skills")
	case "copilot":
		d := appdir.Getenv("COPILOT_HOME")
		if d == "" {
			d = filepath.Join(h, ".copilot")
		}
		t.Instructions = filepath.Join(d, "copilot-instructions.md")
		t.MCP = &mcpFile{Path: filepath.Join(d, "mcp-config.json"), Format: fmtCopilot}
		t.Skills = filepath.Join(d, "skills")
	case "kimi":
		// Kimi Code reads one user-wide folder of each kind, the first there
		// is: ~/.kimi/skills, ~/.claude/skills, ~/.codex/skills — so one of
		// its own would hide Claude Code's from it — and ~/.config/agents/
		// skills, ~/.agents/skills (kimi_cli/skill), wherever KIMI_SHARE_DIR
		// is; before its 1.x brand/generic split, only the first of all five.
		// The new Kimi Code (2.x) reads its own folder of skills
		// ($KIMI_CODE_HOME/skills, else ~/.kimi-code/skills, docs/
		// customization/skills.md "Skill Locations") and ~/.agents/skills:
		// its skills go into its own, for one in the shared folder reaches
		// every reader of it whatever the library ticks (#4379).
		//
		// Its MCP servers are the mcp.json in its folder, which the old
		// kimi-cli (fastmcp's MCPConfig) and the new Kimi Code read alike.
		t.MCP = &mcpFile{Path: filepath.Join(a.Dir, "mcp.json"), Format: fmtKimi}
		if _, legacy := agent.KimiDir(h); !legacy {
			// the new Kimi Code reads AGENTS.md in its folder ($KIMI_CODE_HOME,
			// else ~/.kimi-code) before every conversation; kimi-cli reads no
			// user-wide one (loadAgentsMdForRoots in packages/agent-core-v2/
			// src/agent/profile/context.ts, MoonshotAI/kimi-code afd95f64)
			t.Instructions = filepath.Join(a.Dir, "AGENTS.md")
			t.Skills = filepath.Join(a.Dir, "skills")
		} else if d := filepath.Join(h, ".config", "agents", "skills"); isDir(d) {
			t.Skills = d
		} else {
			t.Skills = sharedSkillsDir()
		}
	case "devin":
		// Devin reads its user-wide MCP servers from mcp_config.json
		// beside its config.json (devin mcp add --scope user)
		t.MCP = &mcpFile{Path: filepath.Join(a.Dir, "mcp_config.json"), Format: fmtDevin}
		t.Skills = filepath.Join(a.Dir, "skills")
		// and its global rules from AGENTS.md there, else the singular
		// AGENT.md it also reads, at the start of every session (Rules &
		// AGENTS.md → Global Rules in the docs Devin CLI 3000.11.92 ships,
		// share/devin/docs/extensibility/rules.mdx, #3151)
		t.Instructions = firstThere(a.Dir, "AGENTS.md", "AGENT.md")
	case "grok":
		// Grok Build reads [mcp_servers.<name>] from its config.toml, as
		// `grok mcp add` writes them; its user-wide instructions are
		// $GROK_HOME/AGENTS.md, loaded before a project's own
		t.MCP = &mcpFile{Path: a.Path, Format: fmtGrok}
		t.Instructions = filepath.Join(a.Dir, "AGENTS.md")
		t.Skills = filepath.Join(a.Dir, "skills")
	case "grok-app":
		// Independent mode (Use on a custom provider) starts the CLI with
		// GROK_HOME at agent-home, beside config.toml (grok-app's
		// providers.md). Shared mode keeps ~/.grok, which is Grok Build's
		// row. Skills there are read as well as ~/.agents/skills.
		home := filepath.Dir(a.Path)
		t.MCP = &mcpFile{Path: a.Path, Format: fmtGrok}
		t.Instructions = filepath.Join(home, "AGENTS.md")
		t.Skills = filepath.Join(home, "skills")
	case "zed":
		// Zed reads its MCP servers from context_servers in the settings.json
		// eipgam sets its model in (~/.config/zed, $XDG_CONFIG_HOME/zed,
		// %APPDATA%\Zed or $EIPGAM_ZED_CONFIG_DIR: agent.zed), and reloads
		// the file as it changes (crates/settings_content/src/project.rs
		// ProjectSettingsContent.context_servers, zed-industries/zed
		// 2c99f547; lc on Discord)
		t.MCP = &mcpFile{Path: a.Path, Format: fmtZed}
	case "kiro-cli":
		// Kiro CLI — and the Kiro IDE, which reads the same files — finds
		// its global skills in ~/.kiro/skills, its user-wide MCP servers
		// in ~/.kiro/settings/mcp.json, mcpServers of command/args/env or
		// url/headers that hot-reloads, and ~/.kiro/steering/AGENTS.md of
		// its global steering, always included (kiro.dev/docs/skills,
		// /docs/mcp/configuration, /docs/steering; #4769). The app is
		// found by the kiro-cli program alone (apps): ~/.kiro is Kiro
		// Crew's and the Kiro IDE's too
		d := filepath.Join(h, ".kiro")
		t.Instructions = filepath.Join(d, "steering", "AGENTS.md")
		t.MCP = &mcpFile{Path: filepath.Join(d, "settings", "mcp.json"), Format: fmtKiro}
		t.Skills = filepath.Join(d, "skills")
	case "claude-desktop":
		// Claude Desktop reads only commands from its file: a remote server
		// is added in its own Connectors settings. In its 3p mode (eipgam's
		// gateway, or any other) it reads the file in Claude-3p instead, so
		// where that folder is the servers go into both: each mode finds
		// them, a switch between the two leaves nothing behind in either
		t.MCP = &mcpFile{Path: filepath.Join(filepath.Dir(a.Path), "claude_desktop_config.json"), Format: fmtDesktop}
		for _, p := range agent.DesktopConfigs3p(h) {
			if p != t.MCP.Path && isDir(filepath.Dir(p)) {
				t.MCP.Also = append(t.MCP.Also, p)
			}
		}
		// Cowork's skills, in every account's skills-plugin (#638)
		if t.Desktop = desktopSkillRoots(a.Dir); len(t.Desktop) > 0 {
			t.Skills = filepath.Join(t.Desktop[0], "skills")
		}
	case agent.DesktopTwinID:
		// Claude eipgam, the isolated Desktop: its servers in the one file
		// it reads, its Cowork skills in its own skills-plugin, neither
		// the user's own Desktop's (xMopf7RF on X)
		t.MCP = &mcpFile{Path: a.Path, Format: fmtDesktop}
		if t.Desktop = desktopSkillRootsIn([]string{a.Dir}); len(t.Desktop) > 0 {
			t.Skills = filepath.Join(t.Desktop[0], "skills")
		}
	default:
		if a.Plugin != "" {
			return pluginTarget(a)
		}
		return nil
	}
	return t
}

// firstThere is the first of names in dir that is there, as an agent that
// reads only the first of them finds it; the first name when none is.
func firstThere(dir string, names ...string) string {
	for _, n := range names {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			return filepath.Join(dir, n)
		}
	}
	return filepath.Join(dir, names[0])
}

// contextFile is the name of the file Qwen Code, or Qoder, reads its
// user-wide instructions from: the first of its settings' context.fileName,
// a name or a list of them, else def (Qwen's packages/cli/src/config/
// config.ts sets it with setMemoryFilename, which trims each name and skips
// an empty one).
func contextFile(settings, def string) string {
	raw, _ := os.ReadFile(settings)
	v := gjson.GetBytes(jsonc.ToJSON(raw), "context.fileName")
	names := []gjson.Result{v}
	if v.IsArray() {
		names = v.Array()
	}
	for _, n := range names {
		if name := strings.TrimSpace(n.String()); n.Type == gjson.String && name != "" {
			return name
		}
	}
	return def
}

// pluginMCP is each MCP format an agent plugin may name
// (agentplug.MCPFormats), by name.
var pluginMCP = map[string]mcpFormat{
	"alma": fmtAlma, "antigravity": fmtAntigravity, "claude": fmtClaude,
	"cline": fmtCline, "codex": fmtCodex, "commandcode": fmtCommandCode,
	"copilot": fmtCopilot, "crush": fmtCrush, "cursor": fmtCursor,
	"devin": fmtDevin, "gemini": fmtGemini, "goose": fmtGoose, "grok": fmtGrok,
	"hermes": fmtHermes, "kimi": fmtKimi, "omp": fmtOmp, "opencode": fmtOpenCode,
	"zed": fmtZed,
}

// pluginTarget is where an agent a plugin adds reads the Library's three,
// as its agent.library says; nil when it names none. A path that isn't
// absolute once ~ and $VAR are expanded is left out: relative to what, no
// one knows.
func pluginTarget(a *agent.Agent) *Target {
	list, _ := agentplug.List()
	var l agentplug.Places
	for _, p := range list {
		if p.Spec == a.Plugin {
			l = p.Desc.Library
		}
	}
	at := func(p string) string {
		if p == "" {
			return ""
		}
		if p = agent.PluginPath(p); !filepath.IsAbs(p) {
			return ""
		}
		return p
	}
	t := &Target{Agent: a, Instructions: at(l.Instructions), Skills: at(l.Skills)}
	if f, ok := pluginMCP[l.MCPFormat]; ok && at(l.MCP) != "" {
		t.MCP = &mcpFile{Path: at(l.MCP), Format: f}
	}
	if t.Instructions == "" && t.MCP == nil && t.Skills == "" {
		return nil
	}
	return t
}

// wslTargetOf is where an agent in a WSL distro keeps them: its files at
// their defaults under the distro's $HOME (the distro's variables that
// move them aren't read), opened through \\wsl.localhost; nil while the
// distro is stopped, which opening them would start (asked again here: the
// agent may have been made before the user stopped it). What is written is
// the same as for this machine's agent: nothing in it names a place on
// Windows, and each skill is a link the distro makes to eipgam's one copy
// of it there (wsl_skills.go).
func wslTargetOf(a *agent.Agent) *Target {
	h := a.Home
	if h == "" || !agent.WSLRunning(a.WSL) {
		return nil
	}
	t := &Target{Agent: a}
	// a folder there that is a link is opened where it leads (#4502)
	j := func(elem ...string) string { return wslOpen(t, filepath.Join(elem...)) }
	id, _, _ := strings.Cut(a.ID, "@")
	switch id {
	case "claude":
		d := j(h, ".claude")
		t.Instructions = filepath.Join(d, "CLAUDE.md")
		t.MCP = &mcpFile{Path: j(h, ".claude.json"), Format: fmtClaude}
		t.Skills = filepath.Join(d, "skills")
		t.Settings = filepath.Join(d, "settings.json")
	case "codex":
		d := j(h, ".codex")
		t.Instructions = filepath.Join(d, "AGENTS.md")
		t.Override = filepath.Join(d, "AGENTS.override.md")
		t.MCP = &mcpFile{Path: filepath.Join(d, "config.toml"), Format: fmtCodex}
		t.Skills = filepath.Join(d, "skills")
	case "pi":
		// its MCP servers go where the Pi installed there reads them, as on
		// this machine (piMCP): its version is what the distro's pi said
		// at the probe, and its extensions are those its settings.json and
		// the distro's npm folders have (leokon on Discord: only the
		// Windows Pi was offered MCP servers)
		d := j(h, ".pi", "agent")
		t.Instructions = firstThere(d, "AGENTS.md", "AGENTS.MD", "CLAUDE.md", "CLAUDE.MD")
		t.Override = filepath.Join(d, "AGENTS.override.md")
		t.MCP, t.MCPVia = piMCP(h, d, piVersion(a), wslPiRoots(h))
		t.Skills = filepath.Join(d, "skills")
	case "gemini":
		// as targetOf's, under the distro's home
		d := j(h, ".gemini")
		t.Instructions = filepath.Join(d, "GEMINI.md")
		t.MCP = &mcpFile{Path: filepath.Join(d, "settings.json"), Format: fmtGemini}
		t.Skills = filepath.Join(d, "skills")
	case "agy":
		d := j(h, ".gemini", "config")
		t.Instructions = filepath.Join(d, "GEMINI.md")
		t.MCP = &mcpFile{Path: filepath.Join(d, "mcp_config.json"), Format: fmtAntigravity}
		t.Skills = filepath.Join(d, "skills")
	case "kimi":
		// its folder is the distro's (agent.kimiDir); the new Kimi Code's
		// AGENTS.md is in it. The skills it shares with other agents (the
		// distro's ~/.agents/skills) aren't given here
		t.MCP = &mcpFile{Path: filepath.Join(a.Dir, "mcp.json"), Format: fmtKimi}
		if filepath.Base(a.Dir) == ".kimi-code" {
			t.Instructions = filepath.Join(a.Dir, "AGENTS.md")
		}
	default:
		return wslOwnFolder(a)
	}
	wslOpened(t)
	wslStored(t, h)
	if t.MCP != nil {
		// a Windows program isn't given to it (mcpFile.side)
		t.MCP.WSL, t.MCP.Distro, t.MCP.Home = true, a.WSL, linuxHome(h)
	}
	return t
}

// wslOpen is path, one of t's in a WSL distro, where eipgam can open it
// (agent.WSLOpen): below a link the distro has, where the link leads,
// since nothing below one can be opened through \\wsl.localhost. A link
// that couldn't be resolved leaves path as it is and t unreached.
func wslOpen(t *Target, path string) string {
	if path == "" {
		return ""
	}
	if err := t.Agent.Unreached(); err != nil && t.Unreached == nil {
		t.Unreached = err
	}
	o, err := agent.WSLOpen(t.Agent, path)
	if err != nil {
		if t.Unreached == nil {
			t.Unreached = err
		}
		return path
	}
	return o
}

// wslOpened opens each of t's paths where eipgam can (wslOpen): a link
// below the agent's folder, its skills folder one, as well as the folder.
func wslOpened(t *Target) {
	for _, p := range []*string{&t.Instructions, &t.Override, &t.Skills, &t.Settings} {
		*p = wslOpen(t, *p)
	}
	if t.MCP != nil {
		t.MCP.Path = wslOpen(t, t.MCP.Path)
		for _, ps := range [][]string{t.MCP.Also, t.MCP.Extra} {
			for i := range ps {
				ps[i] = wslOpen(t, ps[i])
			}
		}
	}
}

// ownFolder are the agents that keep all the library gives them in their
// own folder (a.Dir, or beside a.Path), read from no variable of this
// machine's and from nothing under its home: one eipgam found in a distro
// has that folder at the distro's home already, so its target there is
// the one targetOf makes on this machine, given copies (#1323).
var ownFolder = map[string]bool{
	"opencode": true, "grok": true,
}

// wslOwnFolder is the target of a, an agent in a running WSL distro that
// keeps everything in its own folder; nil for any other.
func wslOwnFolder(a *agent.Agent) *Target {
	id, at, _ := strings.Cut(a.ID, "@")
	if !ownFolder[id] {
		return nil
	}
	local := *a
	local.ID, local.WSL = id, ""
	t := targetOf(&local)
	if t == nil {
		return nil
	}
	t.Agent, t.Copy = a, false
	wslOpened(t)
	wslStored(t, a.Home)
	if t.MCP != nil {
		t.MCP.WSL, t.MCP.Distro, t.MCP.Home = true, a.WSL, linuxHome(a.Home)
	}
	// the skills it reads besides its own are those agents' in the same
	// distro, not this machine's
	for i, o := range t.SkillsAlso {
		t.SkillsAlso[i] = o + "@" + at
	}
	return t
}

// apps are what the library can give MCP servers to that aren't agents
// eipgam sets up: known by the folder they keep their settings in
// (Claude Desktop; on Windows by its program, agent.DesktopApp), or by
// their program alone (Kiro CLI).
func apps() []*agent.Agent {
	// Desktop's own folder (%APPDATA%\Claude on Windows, the MSIX
	// package's for a packaged Desktop, which doesn't see a file written
	// into %APPDATA%). Desktop only ever run in its 3p mode has no Claude
	// folder, only Claude-3p: its file is the one there
	d := desktopdir.Here()
	home, _ := os.UserHomeDir()
	dir := d.Data
	if !isDir(dir) && isDir(d.ThreeP) {
		dir = d.ThreeP
	}
	out := []*agent.Agent{
		agent.DesktopApp(home, dir, filepath.Join(dir, "claude_desktop_config.json")),
	}
	// Claude eipgam, the isolated Desktop, an app of its own (xMopf7RF on X)
	if tw := agent.DesktopTwinApp(home); tw != nil {
		out = append(out, tw)
	}
	// Kiro CLI (#4769) is no agent eipgam wires a model into, but the
	// library can give it skills, MCP servers and instructions. It is
	// found by the kiro-cli program alone — PATH, ~/.local/bin, Kiro
	// CLI.app, where provider.KiroExecutable looks — never by ~/.kiro,
	// which Kiro Crew and the Kiro IDE keep too. Bin is the path itself,
	// so a copy the installer put where eipgam's PATH doesn't reach is
	// found as it was (Detected looks a Bin up by PATH alone). It is
	// listed when kiro-cli isn't found too, and then not Detected, so not
	// a target: held still lists one uninstalled with what eipgam gave it,
	// for that to be unticked and taken out of ~/.kiro
	bin := provider.KiroExecutable()
	if bin == "" {
		bin = "kiro-cli"
	}
	out = append(out, &agent.Agent{ID: "kiro-cli", Name: "Kiro CLI", Icon: "kiro-color", Bin: bin})
	return out
}

// Targets are the agents on this machine that eipgam can give any of the
// three to, in the order the rest of eipgam lists them.
func Targets() []*Target { return targets(false) }

// everyTarget is Targets with the agents gone for their CLI too: a folder
// an uninstall left behind is still that agent's own, which a project or
// an instruction set is never kept in.
func everyTarget() []*Target { return targets(true) }

func targets(gone bool) []*Target {
	out, _ := targetsAsleep(gone)
	return out
}

// targetsAsleep is targets, and the agents in WSL distros that aren't
// running now: none is a target, as its files can't be opened without
// starting the distro, but the page lists them (asleepView). Each running
// one's places are kept for when it is stopped (keepShape).
func targetsAsleep(gone bool) ([]*Target, []*agent.Agent) {
	var out []*Target
	var asleep []*agent.Agent
	own := map[string]bool{}
	for _, a := range apps() {
		own[a.ID] = true
	}
	for _, a := range agent.All() {
		// the agents here, and a folder an agent shares with its vendor's
		// other app when only that app is here: the Cursor editor reads
		// ~/.cursor's mcp.json and skills with no cursor-agent installed
		if !a.Detected() && !a.SharedHere() {
			continue
		}
		// an app eipgam also sets up as an agent (Claude Desktop) keeps its
		// MCP servers where apps says, once
		if own[a.ID] {
			continue
		}
		// an uninstall that left the folder behind (#2177, #2173): the
		// Agents page calls the agent gone and offers none of its settings,
		// since nothing reads them; the library gives nothing to it either,
		// unless an app of its vendor's reads the folder too: VS Code's
		// Copilot reads ~/.copilot's copilot-instructions.md with no
		// Copilot CLI here
		if !gone && a.CLIMissing() && !a.SharedRead() {
			continue
		}
		if isAsleep(a) {
			asleep = append(asleep, a)
			noteUp(a.WSL, false)
			continue
		}
		if t := targetOf(a); t != nil && t.open() {
			out = append(out, t)
		}
	}
	for _, a := range apps() {
		if !a.Detected() && !a.SharedHere() {
			continue
		}
		if t := targetOf(a); t != nil && t.open() {
			out = append(out, t)
		}
	}
	readsWith(out)
	for _, t := range out {
		if t.Agent.WSL != "" {
			keepShape(t)
		}
	}
	return out, asleep
}

// held are the agents not among have (Targets) — uninstalled, or gone for
// their CLI — that the library still gives something, or that hold a skill,
// server or hook eipgam put there: each is listed on the rows of what it
// has, for that to be unticked and taken out of it, and on no other row
// (@jiahao_jayden on X: an agent that isn't installed was on every row). A
// sync gives them nothing new (Gone).
func (l *Library) held(have []*Target) []*Target {
	in := map[string]bool{}
	for _, t := range have {
		in[t.Agent.ID] = true
	}
	// what eipgam wrote here, not a tick: one a synced library brought from
	// another computer for an agent this one never had is nothing to take
	// out of it, and listed on every row of the servers and skills ticked
	// for it (izhidigua on X: MCP listed the Claude family and Pi, which
	// aren't installed, and Skills didn't)
	has := func(id string) bool {
		a := l.Applied[id]
		return a != nil && (len(a.Skills) > 0 || len(a.MCP) > 0 || len(a.Hooks) > 0 || len(a.Env) > 0 || a.Router || len(a.RouterRules) > 0)
	}
	var out []*Target
	for _, a := range append(apps(), agent.All()...) {
		if in[a.ID] || a.WSL != "" || !has(a.ID) {
			continue
		}
		in[a.ID] = true
		if t := targetOf(a); t != nil && t.open() {
			t.Gone = true
			out = append(out, t)
		}
	}
	return out
}

// junctionBlind are the agents that, on Windows, skip a junction in a
// skills folder while they take a folder or a symlink there: Go programs
// whose walk keeps an entry only when it IsDir or is a ModeSymlink, and Go
// from 1.23 lists a junction as neither but ModeIrregular. Antigravity,
// the app and agy alike (customizations' discoverInSubDirsFiltered, #5107,
// #6610), and Crush (internal/skills on fastwalk with Follow, which follows
// only a ModeSymlink). eipgam's link on Windows is a junction when the
// user may not make a symlink (dirLink, #973), so these get copies there:
// GetAllSkills of Antigravity 2.30's language server and Crush 0.101's
// prompt list a skill in a folder or behind a symlink, and not one behind
// a junction.
var junctionBlind = []string{"agy"}

// linkOS is the system eipgam's links are made on, runtime.GOOS but in a
// test.
var linkOS = runtime.GOOS

// skipsJunctions is whether the agent skips eipgam's links in its skills
// folders on this system (junctionBlind).
func skipsJunctions(id string) bool {
	return linkOS == "windows" && slices.Contains(junctionBlind, id)
}

// readsShared are the agents that read ~/.agents/skills as well as their
// own folder (each one's docs or source): Kimi Code's own is
// ~/.kimi-code/skills besides it (#4379). OmO's engine, senpi, a fork of
// Pi from after Pi 0.54 (its oldest on npm is 0.74.0), reads it as Pi
// does (core/package-manager.js, userAgentsSkillsDir, @code-yeongyu/senpi
// 2026.5.13). ZCode reads it beside ~/.zcode/skills (#4987). mu is Pi
// 1.4.0 (core/package-manager.ts, qybaihe/mu b71cd18).
var readsShared = []string{"codex", "gemini", "opencode", "devin", "grok", "grok-app", "kimi"}

// Pi reads ~/.agents/skills from 0.54.0 (2026-02-19, its CHANGELOG: "Added
// default skill auto-discovery for .agents/skills locations … global
// skills from ~/.agents/skills"; userAgentsSkillsDir in
// dist/core/package-manager.js, @earendil-works/pi-coding-agent 1.5.0),
// and omp from 11.0.0 (src/discovery/agents.ts, "Load skills from
// ~/.agents/skills"; 10.6.2 has no such provider). A Pi or omp whose
// version isn't known is taken for one before, as eipgam took every Pi:
// it is given its own link, which a Pi from 0.68 reads once with the
// shared folder's (it dedupes skills by their real path).
const (
	piSharedSince  = "0.54.0"
	ompSharedSince = "11.0.0"
)

// ompVersion is the version of the omp on PATH, "" when not known.
var ompVersion = func(a *agent.Agent) string { return a.InstalledVersion() }

// since is whether version v, when known, is at or after first.
func since(v, first string) bool { return v != "" && !agent.Newer(first, v) }

// readsSharedDir is whether the agent reads ~/.agents/skills besides its
// own folder, so a skill kept there reaches it whatever is ticked.
func (t *Target) readsSharedDir() bool {
	return t.SharedToo || slices.Contains(readsShared, t.Agent.ID)
}

// readsWith adds to each agent's SkillsAlso the agents whose folder it
// reads skills from too: one that is the very same folder (Kimi Code's,
// Goose's and Cindy's, all ~/.agents/skills), or ~/.agents/skills for one
// that reads it besides its own — a skill eipgam gives there once, for
// any of them, every one of them has.
func readsWith(ts []*Target) {
	real := make([]string, len(ts))
	for i, t := range ts {
		if t.Skills != "" {
			real[i] = realDir(t.Skills)
		}
	}
	shared := realDir(sharedSkillsDir())
	for i, t := range ts {
		for j, o := range ts {
			if i == j || real[i] == "" || real[j] == "" || slices.Contains(t.SkillsAlso, o.Agent.ID) {
				continue
			}
			if real[j] == real[i] || real[j] == shared && t.readsSharedDir() {
				t.SkillsAlso = append(t.SkillsAlso, o.Agent.ID)
			}
		}
	}
}

// open leaves out of the target each place a file stands where a folder
// of it would be — another tool's ~/.dsh or ~/.gemini: nothing can be
// written there, and the agent isn't reading anything from it — and says
// whether any place is left.
func (t *Target) open() bool {
	if t.Instructions != "" && agent.Taken(filepath.Dir(t.Instructions)) {
		t.Instructions, t.Override, t.Note, t.Profile = "", "", "", false
	}
	if t.MCP != nil && slices.ContainsFunc(t.MCP.files(), func(p string) bool { return agent.Taken(filepath.Dir(p)) }) {
		t.MCP, t.MCPVia = nil, ""
	}
	if t.Skills != "" && agent.Taken(t.Skills) {
		t.Skills, t.SkillsAlso = "", nil
	}
	return t.Instructions != "" || t.MCP != nil || t.Skills != ""
}

func targetByID(id string) *Target {
	for _, t := range Targets() {
		if t.Agent.ID == id {
			return t
		}
	}
	return nil
}

// Takes is the id of the agent q names (its id, an alias, its name) when
// the library can give it kind — "instructions", "mcp" or "skills" — or
// why not: an agent it has no place for isn't recorded as getting it and
// then given nothing.
func Takes(q, kind string) (string, error) {
	var a *agent.Agent
	for _, x := range apps() {
		if strings.EqualFold(x.ID, q) || strings.EqualFold(x.Name, q) {
			a = x
		}
	}
	app := a != nil
	if a == nil {
		var err error
		if a, err = agent.Find(q); err != nil {
			return "", err
		}
	}
	what := map[string]string{"instructions": "instructions", "mcp": "MCP servers", "skills": "skills"}[kind]
	// an app found by its program alone (Kiro CLI) that isn't here: told
	// so, not recorded and then given nothing, as Targets leaves it out
	if app && a.Bin != "" && !a.Detected() {
		return "", fmt.Errorf("%s isn't installed here; nothing reads its %s until it is", a.Name, what)
	}
	// gone for its CLI, as Targets leaves it out (#2254): told so, not
	// recorded and then given nothing ("every agent already has it")
	if a.CLIMissing() {
		return "", fmt.Errorf("%s's CLI isn't installed here; nothing reads its %s until it is installed again%s", a.Name, what, takenBy(kind, what))
	}
	t := targetOf(a)
	var has bool
	if t == nil && isAsleep(a) {
		// one eipgam saw it give instructions, MCP servers or skills a
		// place to is recorded, and given them when the distro next runs
		// (Woken)
		if sh := shapeOf(a); sh != nil && (kind == "instructions" && sh.Instructions != "" || kind == "mcp" && sh.MCP != "" || kind == "skills" && sh.Skills != "") {
			return a.ID, nil
		}
		return "", fmt.Errorf("WSL %s isn't running: start it, and %s can be given %s", a.WSL, a.Name, what)
	}
	if t != nil {
		switch kind {
		case "instructions":
			has = t.Instructions != ""
		case "mcp":
			has = t.MCP != nil
		case "skills":
			has = t.Skills != ""
		}
	}
	if !has {
		return "", fmt.Errorf("%s%s", noPlace(a, kind, what), takenBy(kind, what))
	}
	return a.ID, nil
}

// noPlace says why an agent can't be given kind, and where it gets it
// instead when another agent's files are what it reads (MOMO on Discord).
func noPlace(a *agent.Agent, kind, what string) string {
	switch {
	case a.ID == "openchamber":
		// OpenChamber's server reads the global AGENTS.md, skills folder
		// and opencode.json from OpenCode's config folder
		// (packages/web/server/lib/opencode/shared.js), and the OpenCode it
		// runs reads them there too
		return fmt.Sprintf("OpenChamber has no place of its own for %s: it runs OpenCode on OpenCode's config, so it has what OpenCode is given · give them to opencode", what)
	case a.ID == "claude-desktop" && kind == "instructions":
		// Desktop's chat takes its instructions in its own settings, kept
		// with the Claude account, not in a file; its Code tab is Claude
		// Code, which reads Claude Code's CLAUDE.md
		return "Claude Desktop keeps its instructions in its own settings, with your Claude account, not in a file eipgam can write; its Code tab runs Claude Code, which reads Claude Code's · give them to claude"
	case a.ID == agent.DesktopTwinID && kind == "instructions":
		// as Claude Desktop, and its Code tab is Claude Code on the
		// folder Claude eipgam.app names: its own, or ~/.claude
		to := "claude"
		if iso, ok := agent.DesktopIsolatedHere(); ok && iso.Claude {
			to = "claude#" + strings.TrimPrefix(filepath.Base(iso.ClaudeDir), ".claude-")
		}
		return fmt.Sprintf("%s keeps its instructions in its own settings, not in a file eipgam can write; its Code tab runs Claude Code, which reads that Claude Code's · give them to %s", a.Name, to)
	case a.Plugin != "":
		return fmt.Sprintf("%s's plugin (%s) says no place for %s: its agent.library.%s names none", a.Name, plugin.Name(a.Plugin), what, kind)
	}
	return fmt.Sprintf("%s has no user-wide place for %s that eipgam knows of", a.Name, what)
}

// takenBy lists the agents here that kind can be given to.
func takenBy(kind, what string) string {
	var ids []string
	for _, t := range Targets() {
		if kind == "instructions" && t.Instructions != "" || kind == "mcp" && t.MCP != nil || kind == "skills" && t.Skills != "" {
			ids = append(ids, t.Agent.ID)
		}
	}
	if len(ids) == 0 {
		return ""
	}
	return "\n  agents here that take " + what + ": " + strings.Join(ids, ", ")
}
