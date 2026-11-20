// Package agentenv names the environment variables through which an agent
// really installed on this machine reaches eipgam: the one list a test
// sandbox clears, so that a test meets only the agents it put there itself.
//
// A sandbox fakes eipgam's own folders through HOME, USERPROFILE and the XDG
// variables (appdir decides those), and an agent's folder through the
// variable that agent's own CLI reads before it falls back to its folder in
// the home: CLAUDE_CONFIG_DIR, CODEX_HOME, KIMI_CODE_HOME and the rest.
// Any one of them left over from the developer's shell points eipgam at a
// real agent, whose sessions and settings a test then reads — and, where
// eipgam writes what it wires, writes into (#522).
//
// Each package keeps its own sandbox, because a test in internal/sessions
// cannot import internal/agent, which imports it, to ask which variables
// there are; this is the list they all clear. A name here is one eipgam
// reads, and TestVarsAreRead says so, so a variable eipgam stopped reading
// does not linger — that is how GEMINI_CLI_HOME and OPENCODE_CONFIG came to
// be cleared by a sandbox while OPENCODE_CONFIG_DIR, the one eipgam reads,
// was not. What is deliberately not here: APPDATA and LOCALAPPDATA, Windows'
// folders rather than an agent's, which a sandbox sets to a folder of its
// own instead of clearing; and CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC, a
// behaviour flag whose one test sets it on purpose to read what eipgam then
// does.
package agentenv

// Vars are the variables, by the agent they belong to. A sandbox clears all
// of them, and sets back the ones its own fixtures need.
var Vars = []string{
	// Claude Code, Codex and Copilot CLI
	"CLAUDE_CONFIG_DIR", "CODEX_HOME", "COPILOT_HOME", "COPILOT_PROVIDERS_CONFIG",
	// Claude Code's temp folder, the images its sessions were given
	"CLAUDE_CODE_TMPDIR",
	// Zed, or a fork of it that keeps its settings (ZedG)
	"EIPGAM_ZED_BIN", "EIPGAM_ZED_CONFIG_DIR", "EIPGAM_ZED_PROCESS_NAMES",
	// Codex's state database, when it is kept apart from CODEX_HOME
	"CODEX_SQLITE_HOME",
	// Gemini CLI's session/config home
	"GEMINI_CLI_HOME",
	// Pi and its forks (OmO, Senpi), whose profile and config are apart
	"PI_CODING_AGENT_DIR", "PI_CODING_AGENT_SESSION_DIR",
	// Raft (slock), whose agents keep their Pi sessions in its folder
	"SLOCK_HOME",
	"PRIME_AGENT_CODING_AGENT_DIR",
	// mu's agent folder (agentDirFor in kyrn/bin/mu.mjs), KYRN_ its old name
	"MU_AGENT_DIR", "KYRN_AGENT_DIR", "MU_CODING_AGENT_DIR", "KYRN_CODING_AGENT_DIR",
	// Kiro Crew's CLI search path adds $BUN_INSTALL/bin
	"BUN_INSTALL",
	// Devin CLI's sessions folder ($DEVIN_SESSIONS_DIR/cli)
	"DEVIN_SESSIONS_DIR",
	// Grok: its home, and where its CLI is installed
	"GROK_HOME", "GROK_BIN_DIR",
	// Grok App's data folder (its agent-home/config.toml), else the OS's app-data one
	"GROK_APP_HOME",
	// Kimi Code and its shared folder
	"KIMI_CODE_HOME", "KIMI_SHARE_DIR",
	// Avibe's home (its config/config.json and its control socket), ~/.avibe without it
	"AVIBE_HOME",
	// Mistral Vibe's folder (config.toml, skills, AGENTS.md), ~/.vibe without it
	"VIBE_HOME",
	// OpenClaw: its config file, its state folder (openclaw.json in it), a
	// named profile's folder (~/.openclaw-<profile>) and its own ~
	"OPENCLAW_CONFIG_PATH", "OPENCLAW_STATE_DIR", "OPENCLAW_HOME",
	// MiMo Code, MiniMax Code, OpenHanako, Hermes, dsh, WorkBuddy, CodeBuddy Code
	"MINIMAX_DATA_DIR", "MAVIS_DATA_DIR", "HANA_HOME", "HERMES_HOME",
	// the skills CLI's lock ($XDG_STATE_HOME/skills/.skill-lock.json), which
	// the Library groups skills by; the desktop app has it from the login shell
	"XDG_STATE_HOME",
	// PI-Desktop's data folder, where Sessions reads its chat titles
	"PI_DESKTOP_DATA_DIR",
	// T3 Code's base folder (its settings in userdata/)
	"T3CODE_HOME",
	// QwenPaw's working folder (~/.qwenpaw, or a legacy ~/.copaw, without
	// it) and its secret folder, <working folder>.secret without it
	"QWENPAW_WORKING_DIR", "QWENPAW_SECRET_DIR",
	// billion-context's config file, the Library's billion-context tab
	// changes; $XDG_CONFIG_HOME/billion-context/billion-context.json without it
	"BILI_CONFIG_FILE",
	// Cursor's CLI: its config folder (its chats) and its data folder
	"CURSOR_CONFIG_DIR", "CURSOR_DATA_DIR",
	// OpenCode and OpenChamber
	"OPENCODE_CONFIG_DIR", "OPENCODE_DB",
	// Droid's home, Windsurf's API server, ZCode's credential seed: what
	// makes an account or an installation visible that the test didn't make
	"WINDSURF_API_SERVER_URL", "ZCODE_CREDENTIAL_SECRET",
	// nvm's and fnm's folders, which hold the Node versions an agent's CLI
	// is installed under, and which of them a terminal uses
	"NVM_DIR", "FNM_DIR",
	// mise's and asdf's folders, whose shims stand for an agent's CLI and
	// whose installs hold the Node versions it is under
	"MISE_DATA_DIR", "ASDF_DIR",
	// cargo's folder, whose bin holds an rtk cargo installed: the Library
	// finds rtk there and upgrades it with cargo (cargoBins)
	"CARGO_HOME",
	// Google Cloud's credentials file and gcloud's folder, which Vertex AI's
	// tokens are minted from (and Gemini CLI's own, on Vertex AI)
	"GOOGLE_APPLICATION_CREDENTIALS", "CLOUDSDK_CONFIG",
}

// NotPaths are the Vars that name no folder or file under the working
// folder: a profile's name, a server's address, a secret, or a name its
// agent puts under a folder of its own (PI_CONFIG_DIR under the home,
// OPENCODE_DB in OpenCode's data folder), which a relative value is meant
// for.
var NotPaths = map[string]bool{
	"PI_PROFILE": true, "OMP_PROFILE": true,
	"WINDSURF_API_SERVER_URL": true, "ZCODE_CREDENTIAL_SECRET": true,
	"PI_CONFIG_DIR": true, "OPENCODE_DB": true,
	"STEPCODE_CONFIG_DIR": true,
	"EIPGAM_ZED_BIN":      true, "EIPGAM_ZED_PROCESS_NAMES": true,
	"PENGUIN_PROJECT_ID": true,
	"OPENCLAW_PROFILE":   true,
}

// ClaudeProviders are Claude Code's switches that send its requests to a
// cloud's endpoint (Amazon Bedrock, Vertex AI, …) in place of
// ANTHROPIC_BASE_URL, so a Claude Code wired to eipgam with one on never
// reaches it (jw1dev on X). The login shell is asked for them
// (proc.LoginEnv), for eipgam to say when a terminal's Claude Code has one
// on, but they are never set in eipgam's own environment, and a test's
// sandbox clears them. They are Claude Code 2.1.300's (Ie() in its bundle).
var ClaudeProviders = []string{
	"CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY",
	"CLAUDE_CODE_USE_ANTHROPIC_AWS", "CLAUDE_CODE_USE_ANTHROPIC_GOOGLE_CLOUD", "CLAUDE_CODE_USE_MANTLE",
}
