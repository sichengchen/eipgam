// Package cli is eipgam's command line: every command, and the app, tray
// and TUI it opens (the root's main.go only calls Main).
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sichengchen/eipgam/internal/agent"
	"github.com/sichengchen/eipgam/internal/appdir"
	"github.com/sichengchen/eipgam/internal/backup"
	"github.com/sichengchen/eipgam/internal/browsermcp"
	"github.com/sichengchen/eipgam/internal/catalog"
	"github.com/sichengchen/eipgam/internal/ccswitchlink"
	"github.com/sichengchen/eipgam/internal/claudebridge"
	"github.com/sichengchen/eipgam/internal/claudecode"
	"github.com/sichengchen/eipgam/internal/claudetui"
	"github.com/sichengchen/eipgam/internal/codexrelay"
	"github.com/sichengchen/eipgam/internal/davsync"
	"github.com/sichengchen/eipgam/internal/gateway"
	"github.com/sichengchen/eipgam/internal/imagemcp"
	"github.com/sichengchen/eipgam/internal/library"
	"github.com/sichengchen/eipgam/internal/netproxy"
	"github.com/sichengchen/eipgam/internal/notices"
	"github.com/sichengchen/eipgam/internal/plugin"
	"github.com/sichengchen/eipgam/internal/pluginrt"
	"github.com/sichengchen/eipgam/internal/proc"
	"github.com/sichengchen/eipgam/internal/profile"
	"github.com/sichengchen/eipgam/internal/provider"
	"github.com/sichengchen/eipgam/internal/sessions"
	"github.com/sichengchen/eipgam/internal/settings"
	"github.com/sichengchen/eipgam/internal/shortcut"
	"github.com/sichengchen/eipgam/internal/tui"
	"github.com/sichengchen/eipgam/internal/update"
)

// version is the root main package's, set at build time
// (-ldflags "-X main.version=…") and handed over by Main.
var version = "dev"

const usage = `eipgam — one place to pick every agent's model

  eipgam                          open the app: a window plus a menu bar icon
  eipgam tray                     start in the menu bar only
  eipgam panel                    open the menu bar icon's quick panel, or close it
  eipgam autostart [on|off]       open eipgam (in the menu bar) when you log in, or say whether it does
  eipgam tui                      the same thing, in the terminal (serves the gateway while open when no eipgam does)
  eipgam web [--addr host:port] [--lan] [--no-open] [--gateway] [--tls-cert <file> --tls-key <file>]
                                  the app's window in a browser, with the gateway (no desktop needed: WSL, a server over SSH)
                                  a new key each run; EIPGAM_WEB_KEY (16+ letters, digits, - . _ ~) keeps one, signed in for 400 days
                                  behind a reverse proxy, EIPGAM_WEB_URL=https://<the page there> prints the link through it
                                  --gateway: gateway mode, no Agents, Sessions or Library (on by itself with no agents here; Settings › General turns it off)
                                  --tls-cert/--tls-key: the page and the gateway over https:// to other computers
  eipgam ls                       list detected agents and their settings
  eipgam <agent>                  show one agent
  eipgam <agent> <model>          set an agent's model   e.g. eipgam claude moonshot/kimi-k3
  eipgam <agent> <field>          show one field   e.g. eipgam codex effort
  eipgam <agent> <field> <value>  set another field   e.g. eipgam codex effort high
  eipgam <agent> default          take eipgam out: the agent back on what it had before
  eipgam <agent> <field> default  that field back to the agent's own default
  eipgam <command> --help         what a command or an agent takes; -h or --help anywhere does nothing else

  eipgam save <name>              snapshot every agent's settings as a profile
  eipgam use <name>               apply a profile
  eipgam profiles                 list profiles
  eipgam rename <old> <new>       rename a profile
  eipgam rm <name>                delete a profile

  eipgam backup [--no-keys] [--no-library] [file]    providers, keys, settings, profiles, agent models, the library and the usage history in one file, sealed with a passphrase
  eipgam restore [--no-agents] [--no-library] <file> put a backup in on this machine; its usage history is merged with the one here
                                                     --no-usage on either: the usage history left out
                                                     --sign-ins on backup: the subscriptions' and plugins' sign-ins too, sealed (off unless given); --no-sign-ins on restore leaves them out
  eipgam uninstall [--yes]                           disconnect every agent, putting back what each had, then remove eipgam's service and files
                                                     --keep-data: all but deleting eipgam's settings, providers and usage
  eipgam data [move <folder>|move --home]            where eipgam keeps its files and cache, and moving them to another disk (a link is left where they were)
  eipgam webdav [on <address>|set k=v…|now|off]      the same, kept the same on every computer through a WebDAV folder (eipgam webdav help)
  eipgam s3 [on s3://<bucket>[/<prefix>]|set k=v…|now|off]   the same through an S3-compatible bucket: AWS, R2, B2, MinIO… (eipgam s3 help)
  eipgam github [on github://<owner>/<repo>[/<folder>]|set k=v…|now|off]   the same in an encrypted GitHub repository (eipgam github help)

  eipgam library [sync|instructions|mcp|skill]   the instructions, MCP servers and skills written into every agent (eipgam library help)

  eipgam providers                list your providers: host, key, models, who uses them
  eipgam presets                  the vendors eipgam knows: add one with just a key
  eipgam provider add <preset> <key>   e.g. eipgam provider add moonshot sk-…
  eipgam provider add <name> k=v…      a custom vendor (eipgam provider for the fields)
  eipgam provider key|models|test|rm <id>
  eipgam provider fallback <id> <provider/model>…   use these when it's out of quota or down
  eipgam import [-y] <link>       add the provider a eipgam://import?… link describes
  eipgam models [<agent>]         every model agents can pick, as provider/model; an agent's, and why others aren't
  eipgam model name <provider/model> <name>|--reset       the name a model goes by, everywhere
  eipgam model efforts <provider/model> <l>,<l>|--reset   the reasoning levels a model offers (eipgam model help)
  eipgam model api <provider/model> chat|responses|anthropic|gemini|auto   the API a model is asked on
  eipgam visible [<agent> <family|provider|group>,… | all | --only-picked | --show-new]
                                  which models an agent is shown: families (eipgam provider/group set <id> family=…),
                                  or only the models ticked for it, a new one off until it is ticked
  eipgam search [add <api> <key>|rm <api>]   Tavily, Brave, Exa, Firecrawl, You.com, TinyFish or SearXNG for web search when no provider can search
  eipgam groups                   routing groups: several models agents pick as one, group/<id>
  eipgam group add <name> models=<m1>,<m2> [routing=smart|order|rotate|usage|pace|weight|free|fast|cheap|drain|expiring] [stays=auto|session|turn|off]
  eipgam group <id> | set <id> k=v… | rm <id>   show, change or remove one (eipgam group help for more)
  eipgam accounts [agent] [--json]  every subscription eipgam knows, with each one's allowance used and when it resets
  eipgam accounts add <agent>     sign in to one more Claude, ChatGPT or Google (Gemini CLI, Antigravity) subscription
  eipgam accounts add copilot [--host <name>.ghe.com]   one more Copilot account, on github.com or an enterprise's GHE.com
  eipgam accounts discovery [on|off]   read agents' sign-ins (default on; EIPGAM_NO_ACCOUNTS=1 disables too)
  eipgam accounts import <codex|claude|antigravity|factory|<plugin>> <file>... [--yes]   bring in accounts from other tools' files (Codex's auth.json, Cockpit Tools, CLIProxyAPI, Sub2API, claude-swap, a plugin's own); the tool a ChatGPT or Claude file came from is signed out of it
  eipgam accounts export <codex|claude> [<email>...] [--format sub2api|cliproxyapi] [--to <folder>] [--yes]   write accounts as a file Sub2API, CLIProxyAPI or eipgam imports (0600; holds live sign-ins)
  eipgam accounts import codex <file>... --access-only   keep each ChatGPT account with its access token only: never renewed, so the tool it came from stays signed in
  eipgam accounts access-only codex <email> [on|off]   keep a ChatGPT account access token only, or let eipgam renew its sign-in again
  eipgam accounts switch <agent> <email|name>   sign the agent in to another of them, by its email or the name you gave it
  eipgam accounts switch <claude|codex> next   to the next account in order with room, or the one with the most left
  eipgam accounts refresh         renew the saved ChatGPT and Claude sign-ins now (the gateway does it every 8 days)
  eipgam accounts project <gemini|antigravity> <email> <project>   the Google Cloud project a Google account's requests go to
  eipgam accounts alias <agent> <email> [<name>|--clear]   a name of your own for an account, shown on its card, in the menu bar and in eipgam quota
  eipgam accounts switch-only codex [on|off]   only switch Codex's own sign-in to an account with room; its requests go straight to OpenAI, never through eipgam
  eipgam accounts switch-only claude [on|off]  only switch Claude Code's own sign-in to an account with room; eipgam serves no requests on the Claude accounts
  eipgam claude-code [install [--yes]|remove]   the Claude Code a Claude subscription runs; install downloads Anthropic's own build, checked against its manifest, for a server or container without one
  eipgam plugin [add <package>|rm|update|on|off|login <provider>|logout <provider>]
                                  OpenCode provider plugins and pi packages: subscriptions signed in to, and served, through a plugin
  eipgam plugin move|migrate <subscription>   run a built-in subscription's accounts on its community plugin

  eipgam serve [--web[=host:port]] [--tls-cert <file> --tls-key <file>]
                                  run the gateway alone (the app runs it too); --web serves eipgam web's page too, from the same process (127.0.0.1:3430 unless told);
                                  with a certificate and key (PEM; EIPGAM_TLS_CERT, EIPGAM_TLS_KEY) other computers reach both over https://
  eipgam daemon [install|uninstall|start|stop|restart|status]
                                  eipgam serve as a service of yours, from boot or login and started again when it stops (systemd --user on Linux, launchd on the Mac)
  eipgam daemon install [--web] [--lan] [--addr host:port] [--no-web] [--tls-cert <file> --tls-key <file>|--no-tls]
                                  --web: the service serves eipgam web's page too, with a key kept across restarts; installed again, it keeps what it served
  eipgam daemon web [on|off] [--lan] [--addr host:port]
                                  the service's page: its link, or turn it on or off
  eipgam healthcheck [--addr host:port]
                                  exit 0 when the gateway answers (a container's HEALTHCHECK). It looks where
                                  --addr says, else at EIPGAM_ADDR, else where the running eipgam wrote down
                                  that it listens, else at the port in Settings' Gateway address
  eipgam status [--json] [--addr host:port]
                                  which eipgam serves the gateway: its role (app, tray, web, serve), pid,
                                  version and listen address, also in <config folder>/gateway.json
  eipgam mtls [on|off|issue <device>|list|revoke <device>]
                                  require a client certificate eipgam issued from other computers (mTLS)
  eipgam https [on|off|import <files>|remove|ca]
                                  other computers call eipgam at https://, with your certificate or one eipgam's own CA signs
  eipgam heap [<file>]            a Go heap profile of the running eipgam, with its memory numbers, for a report of it using too much memory
  eipgam cpu [<file>] [--seconds N]   a Go CPU profile of the running eipgam over 30s (5..120) while you use it, for a report of it using too much CPU
  eipgam goroutines [<file>]      every goroutine of the running eipgam, its stack and how long it waited, for a report of it stuck at something
  eipgam log [--path] [-n N] [-f]  where eipgam's logs are, to send with a report: the app's (app.log), the service's; -n prints the app log's last N lines, -f follows it
  eipgam gateway-key list|add <name> [--read-only]|rotate <id> [--key -]|remove <id>   manage the keys clients use to call a shared gateway
  eipgam gateway-key limit <id> [list|off|reset [n]|remove <n>|[add] day|week|month|<N>d|<N>h --tokens N --cost USD --cache-reads --model openai/*]   a key's own limits, and what it used; --model counts only those models
  eipgam gateway-key models <id> [all|<provider>/<model>|<provider>/* ...]   the models a key may use, every one unless it names some
  eipgam gateway-key accounts <id> [all|<provider>/<account>|<provider>/!<account>|@<group>|!@<group> ...]   the accounts a key may use, or keeps out
  eipgam gateway-key groups [add <name> <account>...|set <group> <account>...|rename <group> <name>|remove <group>]   named account groups a key's accounts can name
  eipgam gateway-key quotas <id> [on|off]   whether a key held to a budget, models or accounts is told the quotas of what it may use
  eipgam gateway-key concurrency <id> [<n>|off]   at most n of a key's requests at once; the rest wait their turn
  eipgam gateway-key mcp <id> [all|none|<server>|<server>/<tool> ...]   the MCP servers and tools a key may use
  eipgam gateway-key access <id> [read-only|full]   read-only: the key reads quotas and usage (GET /, /v1/eipgam/quotas, /v1/eipgam/usage) and uses no model, MCP server or write
  eipgam gateway-key admin <id> [off|local|any]   let the key manage providers, accounts and gateway keys through /v1/eipgam/admin/ (local: from this computer only)
  eipgam gateway-key move <id> <place>   put a key at a place in the list (from 1), the order list and the app show them in
  eipgam gateway-key issue --ttl 1h [--name n] [--models a,b] [--accounts a,b] [--tokens N] [--cost USD] [--read-only] [--from <id>] [--json]
                                         a short-lived key (1m to 30 days) that stops working by itself; --from holds it to a key that issues keys; prints the key
  eipgam gateway-key issuer <id> [on|off]   let a key issue short-lived keys under itself (POST /v1/eipgam/keys with it), each held to its models, accounts, limits and life
  eipgam gateway-key oidc add --key <id> --repo owner/name [--ref refs/heads/main] [--environment name] [--claim name=value]… [--ttl 1h] [--issuer URL] [--audience aud]
                                         let a CI job's OIDC token (GitHub Actions by default) be traded for a key issued under <id> (POST /v1/eipgam/keys/oidc); oidc list | remove <rule id>
  eipgam mcp image                the image and video generation MCP server an agent is given from the library (stdio)
  eipgam tools [<name>]           the tools eipgam's MCP servers give agents (images, vision, web search, plugins'), with their parameters
  eipgam tool <name> [--<param> <value>…] [<words>] [-o <file|folder>] [--json]   call one through the running eipgam; an image or video it makes is saved
  eipgam usage [today|7d|30d|all] tokens and cost per agent, model and subscription account (30d)
  eipgam usage --csv [--account <name>] [today|7d|30d|all]   every request as CSV (or one account's): the model asked for, sent and served, tokens, cost, time, status, account
  eipgam usage [--csv [--account <name>]] --last <dur>|--days <n>   the same for a window ending now (24h, 90m, 1.5 days), not from a midnight
  eipgam sessions [--model <m>] [--folder <f>] [--json]   the latest Claude Code, Codex, OpenCode and Pi sessions, with what each cost
  eipgam sessions --days N|today|all [--model <m>] [--folder <f>] [--json]
                                  what every session spent, day by day, with the top models and folders (7 days)
  eipgam sessions read [on|off all|<agent>…]   which agents' session history eipgam may read (none until allowed)
  eipgam sessions search <words…> [--agent <a>] [--days N|today|all] [--limit N] [--json]
                                  every agent's sessions that said those words, newest first, with where
  eipgam sessions show <agent:id> [--find <words>] [--from N] [--max-chars N] [--json]
                                  one session's conversation as text, from where the words were said
  eipgam quota [<provider>] [--json]  what is left of every subscription, plan and key balance
  eipgam quota wait <provider|account> [--timeout <d>] [--quiet]
                                  block until that subscription (any of its accounts) or account has allowance again
  eipgam quota history [<provider|account>] [--days N] [--json]
                                  each window's readings over time, kept 45 days
  eipgam statusline [<provider|account>]   one line for Claude Code's status line: the session's model and each account's windows
  eipgam sync                     refresh the model catalog and vendor model lists
  eipgam agents                   list every supported agent
  eipgam settings [list|get <key>|set <key> <value>|reset <key>] [--json]
                                  every setting the window's Settings page saves, read and set as it saves them (eipgam settings help)
  eipgam update [check] [--proxy <url>] [--mirror <prefix>] [--addr host:port] [--force]
                                  install the newest release (check: only say if there is one); the running
                                  eipgam is restarted into it, found by --addr, EIPGAM_ADDR or the address it
                                  wrote down (a service started with EIPGAM_ADDR needs none of them); a restart waits at most 10
                                  minutes for requests in flight, --force restarts at once; --proxy: an
                                  http(s):// or socks5:// proxy for it; --mirror: a GitHub download mirror put
                                  before the github.com URL (none unless given; verified against GitHub's build attestation)
  eipgam update mirror [<prefix>|off]  the mirror every update, the app's own too, is downloaded through
  eipgam update channel [stable|daily|every]  which versions updates offer: stable (one a week), daily (one
                                  a day that passed its tests), an urgent one at once on both, or every
                                  version as it comes out (the default)
  eipgam update auto [on|off] [download|notify] [30m|1h|6h|24h]
                                  whether the app looks for updates by itself, how often (6h), and whether
                                  it downloads one at once or only says it is out (notify: for metered networks)
  eipgam licenses                 the third-party notices of what eipgam is built with

agents: claude (cc), codex, gemini, opencode (oc), mimocode, deveco, pi, goose, cursor, zed, copilot, crush, aside
`

var (
	bold  = lipgloss.NewStyle().Bold(true)
	muted = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8B8F98", Dark: "#7C8290"})
	faint = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#C4C7CE", Dark: "#4A4F5A"})
	green = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#0F9D58", Dark: "#7EE2A8"})
)

// Main runs eipgam with os.Args, at the version the build was stamped with.
func Main(v string) {
	version = v
	// before anything reads or writes a file: no home, or a relative one,
	// would put the agents' configs and eipgam's keys under the working
	// folder. What needs no file still answers (eipgam version in a
	// container or a script without HOME): run makes eipgam's folders first.
	ignored, err := appdir.CheckEnv()
	if err != nil {
		if len(os.Args) > 1 {
			switch os.Args[1] {
			case "-v", "--version", "version":
				fmt.Println("eipgam", version)
				return
			case "-h", "--help", "help":
				fmt.Print(usage)
				return
			case "licenses":
				fmt.Print(notices.Text)
				return
			}
		}
		fmt.Fprintln(os.Stderr, "eipgam:", err)
		os.Exit(1)
	}
	// moved to a disk that isn't connected: never an empty eipgam in its place
	unreachableStop(os.Args[1:])
	slices.Sort(ignored)
	for _, v := range ignored {
		fmt.Fprintf(os.Stderr, "eipgam: ignoring %s: not an absolute path (the programs eipgam starts still get it)\n", v)
	}
	if provider.TookOpenedURL(os.Args[1:]) {
		// Claude Code, signing in for eipgam, handed over the page to open
		return
	}
	endProbesOnSignal()
	gateway.Version = version
	netproxy.Install()
	update.GUI = hasGUI
	err = run(os.Args[1:])
	proc.EndProbes() // a CLI still being asked something isn't left to init
	sessions.Saved() // the session index kept, for the next run
	if err != nil {
		// a command with exit codes of its own (quota wait) says which
		code := 1
		var e exitError
		if errors.As(err, &e) {
			code = e.code
		}
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(os.Stderr, "eipgam:", msg)
		}
		endProcess(code)
	}
	endProcess(0)
}

// runTUI runs the TUI, which quits on Ctrl+C and SIGTERM itself once it
// has started; it asks CLIs first, and a signal then ends those.
func runTUI() error {
	// the terminal closing (SIGHUP) drains the gateway it serves first
	defer stopGatewayOnSignal(tuiStop)()
	return tuiRun(ownSignals)
}

// tuiRun is tui.Run; a var so tests can stand in for it.
var tuiRun = tui.Run

// tuiStop is tui.StopGateway; a var so tests can stand in for it.
var tuiStop = tui.StopGateway

// syncAgentsOnChange sets what a catalog change (a provider or group
// saved, a vendor's list fetched anew) does: the model lists agents keep
// in files of their own are brought up to date behind the save, which
// answers without waiting for every agent's files to be written (on
// Windows, half a second for each one another program holds open).
func syncAgentsOnChange() { catalog.Changed = agent.SyncCatalogSoon }

// startUp brings the files eipgam and its agents keep up to what this
// eipgam reads, before a command that may read them; a var so a test can
// tell it ran.
var startUp = func() {
	// started by an update, the eipgam it replaces goes first: the moves
	// below write the files it may still be writing
	update.AwaitPredecessor()
	makeDirs()
	settings.Migrate()
	// every built-in plugin starts on (plugins.yml), on a new install too
	if _, err := pluginrt.Settle(); err != nil {
		fmt.Fprintf(os.Stderr, "eipgam: plugins.yml: %v\n", err)
	}
	if _, err := pluginrt.Refresh(); err != nil {
		fmt.Fprintf(os.Stderr, "eipgam: plugins.yml: %v\n", err)
	}
	// a vendor's keys saved a provider each ("NVIDIA #001"…) are one
	// provider with those keys, before anything lists them
	if _, err := provider.FoldCopies(); err != nil {
		fmt.Fprintf(os.Stderr, "eipgam: providers.json: %v\n", err)
	}
	// the providers and settings read once for every agent's fields, which
	// the moves below look at (a write among them reads them again)
	release := provider.Hold()
	agent.RenameLegacy()
	agent.MoveCursorEfforts()
	agent.MoveAntigravityEfforts()
	agent.MoveOffAccountIDs()
	release()
}

// servesPages is whether eipgam run with args serves plugins' pages,
// and so runs the services they talk to (plugin.ServePages): the app,
// tray, web, serve and the TUI do; a command that does its work and
// ends doesn't.
func servesPages(args []string) bool {
	if len(args) == 0 {
		return true
	}
	if strings.HasPrefix(strings.ToLower(args[0]), "eipgam:") {
		return hasGUI
	}
	switch args[0] {
	case "tui", "web", "serve", "tray", "app", "gui", "panel", "-Embedding":
		return true
	}
	return false
}

func run(args []string) error {
	// a command that saved a provider or a group ends with the agents'
	// files written, which its save no longer waits for; and a test's run
	// leaves catalog.Changed as it found it
	was := catalog.Changed
	defer func() {
		agent.SyncCatalogSettled()
		catalog.Changed = was
	}()
	// claude-eipgam (claude_launch_cli.go): every argument after run is
	// claude's, --help too
	if ok, err := claudeLaunchCmd(args); ok {
		return err
	}
	// what a shell's agy, or another agent's function, runs
	// (start_cli.go): every argument after run is the agent's, --help
	// too, and nothing is moved or started first
	if ok, err := startCmd(args); ok {
		return err
	}
	// Claude eipgam.app (claude_desktop_launch_cli.go), --help too
	if ok, err := claudeDesktopLaunchCmd(args); ok {
		return err
	}
	// before anything is read, moved or started: --help on any command
	// prints its usage and does nothing else
	if helpAsked(args) {
		fmt.Println(strings.TrimRight(helpFor(args), "\n"))
		return nil
	}
	// a ccswitch:// link an older eipgam's claim on CC Switch's scheme
	// handed over (Windows, Linux): CC Switch opens it and has its scheme
	// back (ccswitchlink), and nothing here is read, moved or started
	if len(args) > 0 && ccswitchlink.Is(args[0]) {
		return ccswitchlink.Forward(args[0])
	}
	if len(args) > 0 && args[0] == "healthcheck" {
		return healthcheck(args[1:]) // every few seconds in a container: nothing else
	}
	if len(args) > 0 && args[0] == "heap" {
		// the running eipgam's heap profile: asks it, and moves nothing here
		return heapCmd(args[1:])
	}
	if len(args) > 0 && args[0] == "mtls" {
		// eipgam's CA and the client certificates it issued: nothing
		// else is read or moved
		return mtlsCmd(args[1:])
	}
	if len(args) > 0 && args[0] == "https" {
		// HTTPS for other computers: the setting, the certificate and
		// eipgam's CA, nothing else
		return httpsCmd(args[1:])
	}
	if len(args) > 0 && args[0] == "cpu" {
		// the running eipgam's CPU profile: asks it, and moves nothing here
		return cpuCmd(args[1:])
	}
	if len(args) > 0 && args[0] == "goroutines" {
		// the running eipgam's goroutines: asks it, and moves nothing here
		return goroutinesCmd(args[1:])
	}
	if len(args) > 0 && (args[0] == "log" || args[0] == "logs") {
		// where eipgam's logs are, and the app log's last lines: reads
		// files, and moves nothing
		return logCmd(args[1:])
	}
	if len(args) == 2 && args[0] == agent.DryRunArg {
		// an agent disconnected on a copy of its files under a temporary
		// home, for the Agents page to show what disconnecting changes
		// (agent.DisconnectPreview)
		return agent.DryRun(args[1])
	}
	if len(args) > 1 && (args[0] == "accounts" || args[0] == "account") && args[1] == "discovery" {
		return accountsDiscoveryCmd(args[2:])
	}
	// what an agent starts for each session or message talks to the
	// running gateway and nothing else: the moves below list every
	// provider and account, which starts the plugin host and asks claude,
	// cursor-agent and devin who is signed in — five processes and a
	// Claude /usage for each Claude Code session's image MCP (#3318)
	if len(args) > 0 {
		switch args[0] {
		case "mcp":
			if len(args) > 1 && args[1] == "browser" {
				return browsermcp.Run(args[2:])
			}
			if len(args) > 1 && args[1] == "search" {
				return browsermcp.RunSearch(args[2:])
			}
			if len(args) > 1 && args[1] == "vision" {
				return browsermcp.RunVision(args[2:])
			}
			if len(args) > 1 && args[1] == "memory" {
				return browsermcp.RunMemory(args[2:])
			}
			if len(args) > 1 && args[1] == "sessions" {
				return browsermcp.RunSessions(args[2:])
			}
			return imagemcp.Run(args[1:])
		case "claude-mcp-helper": // internal: stdio MCP subprocess spawned by Claude Code
			return claudebridge.RunMCP(args[1:])
		case claudetui.Command: // internal: Claude Code's interactive mode for the bridge
			// it ends its Claude Code and removes its files on SIGINT,
			// SIGTERM and SIGHUP itself: eipgam dying of the signal first
			// left them behind
			leaveSignals()
			return claudetui.Run(args[1:])
		case "statusline":
			return statuslineCmd(args)
		case "tools":
			return toolsCmd(args[1:])
		case "tool":
			return toolCmd(args[1:])
		case codexrelay.Command: // internal: what a quitting eipgam leaves for Codex's open threads
			return codexrelay.Main(args[1:], gateway.CodexRelay())
		case "data": // where eipgam's files are, and moving them: before anything is made in them
			return dataCmd(args[1:])
		case "uninstall":
			// what the app starts as it quits from Settings › Uninstall
			// eipgam: nothing is made again before its folders go
			if len(args) > 1 && args[1] == "--data-after" {
				return uninstallData(args[2:])
			}
		}
	}
	// eipgam serve beside another eipgam serving the gateway (the app, a
	// service, the copy an update left) waits for the port before it moves
	// or starts anything, rather than doing it all to fail to bind (#4938)
	if len(args) > 0 && args[0] == "serve" && plainServe(args[1:]) {
		awaitGatewayFree(gatewayFreeEvery)
	}
	// a command that serves no pages starts a plugin host beside the
	// app's, whose page services it leaves to the app (#5187)
	plugin.ServePages(servesPages(args))
	// the app started again to move eipgam's folders (Settings › General)
	if servesPages(args) {
		movePending()
	}
	startUp()
	// a provider added, edited or removed, or a list fetched anew, reaches
	// the model lists agents keep in files of their own, behind the save
	// that changed it (agent.SyncCatalogSoon)
	syncAgentsOnChange()
	// a model Claude Code names that eipgam doesn't serve goes to the one
	// it is set to use for that tier
	gateway.StandIn = agent.StandIn
	// a bare name an agent asks for goes to its own subscription, unless
	// eipgam set the agent to the group eipgam found by that name
	gateway.SetToGroup = agent.SetToGroup
	// and its auto mode classifier, on a sonnet tier that follows a main
	// model /model has moved, to the model it moved to
	gateway.ClassifierStandIn = agent.ClassifierStandIn
	// a Codex that reaches eipgam for account failover alone is handed
	// only its own models (#1385)
	gateway.CodexOwnOnly = agent.CodexOwnOnly
	// what eipgam writes into an agent's TOML, YAML or dotenv config says
	// so in a comment, for whoever reads it once eipgam is gone
	agent.MarkWritten()
	// the Codex app's titles, descriptions and summaries, which it asks
	// for on a model of its own, on the model of eipgam's picked beside
	// its ChatGPT sign-in rather than on that account (atom_Ray on X)
	gateway.CodexSideModel = agent.CodexSideModel
	gateway.CodexModelNow = agent.CodexModelNow
	gateway.CodexSideSet = agent.CodexSideSet
	// the list the Codex app reads from a file beside the ChatGPT sign-in,
	// its picker showing eipgam's models only so (laddieyao on Discord),
	// written again when the backend's list changes
	agent.CodexDesktopList = gateway.CodexDesktopCatalog
	gateway.CodexDesktopChanged = agent.SyncCatalogSoon
	// the setup kept the same on every computer, by whichever serves
	gateway.WhileServing = append(gateway.WhileServing, davsync.Run)
	// and the setup backed up into a folder here by itself (jwaitq)
	gateway.WhileServing = append(gateway.WhileServing, backup.RunAuto)
	// and Cursor Private Inference's variables, which the Mac's launchd
	// forgets at a restart, for the gateway's address now
	gateway.WhileServing = append(gateway.WhileServing, agent.KeepCursorLocalEnv)
	// and Claude Science's daemon, which a restart of the computer ends,
	// started on the gateway again while it is connected
	gateway.WhileServing = append(gateway.WhileServing, agent.KeepClaudeScience)
	// and every agent's own telemetry off while Settings › Privacy says so,
	// an agent found since too
	gateway.WhileServing = append(gateway.WhileServing, agent.KeepTelemetryAllOff)
	// and the Claude Code eipgam downloaded for a machine with none (a
	// server's, a container's), at Claude Code's newest release: its runs
	// turn its own updater off
	gateway.WhileServing = append(gateway.WhileServing, claudecode.KeepCurrent)
	// and the library's skills from GitHub and git repositories, fetched
	// again by themselves when that is switched on (0xYaCo on X)
	gateway.WhileServing = append(gateway.WhileServing, library.KeepSkillsCurrent)
	// and the request archive, when it is on, goes to the bucket sync is to
	gateway.ArchiveBucket = func() (gateway.Putter, bool) {
		if b, ok := davsync.S3Bucket(); ok {
			return b, true
		}
		return nil, false
	}
	// and kept there under its size, a bucket grown before that brought
	// under it once the gateway starts
	gateway.WhileServing = append(gateway.WhileServing, gateway.KeepBucketArchiveBounded)
	if len(args) == 0 {
		if hasGUI {
			return runGUI(true, "")
		}
		gateway.Serves = "tui"
		return runTUI()
	}
	// a eipgam:// link the system handed over (Windows, Linux): the app
	// opens it for the user to confirm
	if strings.HasPrefix(strings.ToLower(args[0]), "eipgam:") {
		if !hasGUI {
			return importCmd(args)
		}
		return runGUI(false, args[0])
	}
	// how this process was started, for the Routing page of another eipgam on
	// this computer to name the one that serves the gateway (#2606)
	switch args[0] {
	case "tui", "web", "serve", "tray":
		gateway.Serves = args[0]
	case "app", "gui", "panel", "-Embedding":
		gateway.Serves = "app"
	}
	switch args[0] {
	case "tui":
		return runTUI()
	case "web":
		return webCmd(args[1:])
	case "app", "gui":
		// `eipgam gui settings`: the window on that tab, as a restart to
		// update from it comes back (update.RelaunchArgs)
		if len(args) > 1 {
			return runWindow(args[1])
		}
		return runGUI(true, "")
	case "tray":
		return runGUI(false, "")
	case "-Embedding":
		// Windows starting eipgam for a click on one of its notifications
		// (a usage alert, #368) left in the Action Center after it quit:
		// the window, on the Usage page
		return runWindow("usage")
	case "panel":
		return runPanel()
	case "autostart":
		return autostartCmd(args[1:])
	case "daemon":
		return daemonCmd(args[1:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return nil
	case "-v", "--version", "version":
		fmt.Println("eipgam", version)
		return nil
	case "licenses":
		fmt.Print(notices.Text)
		return nil
	case "ls", "list":
		// in the order the app lists them; those hidden there come last, dimmed
		shown, hidden := settings.Arrange(settings.Load(), agent.Detected(), func(a *agent.Agent) string { return a.ID })
		return list(append(shown, hidden...), true, len(shown))
	case "agents":
		return list(agent.All(), false, -1)
	case "sync":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := catalog.Sync(ctx); err != nil {
			return err
		}
		fmt.Println(green.Render("✓"), "catalog saved to", catalog.CachePath())
		refreshLive(ctx)
		return nil
	case "save", "use", "rm", "rename", "profiles":
		return profiles(args)
	case "import":
		return importCmd(args[1:])
	case "providers":
		return providers()
	case "presets":
		return presets()
	case "provider":
		return providerCmd(args)
	case "models":
		return models(args[1:])
	case "model":
		return modelCmd(args[1:])
	case "visible":
		return visibleCmd(args[1:])
	case "search":
		return searchCmd(args[1:])
	case "groups":
		return groups()
	case "group":
		return groupCmd(args)
	case "serve":
		return serveCmd(args[1:])
	case "gateway-key":
		return gatewayKeys(args)
	case "accounts", "account":
		return accountsCmd(args)
	case "usage":
		return usageCmd(args)
	case "sessions":
		return sessionsCmd(args)
	case "quota", "quotas":
		return quotaCmd(args)
	case "status":
		return statusCmd(args[1:])
	case "update":
		return updateCmd(args)
	case "library", "lib":
		return libraryCmd(args)
	case "backup":
		return backupCmd(args[1:])
	case "restore":
		return restoreCmd(args[1:])
	case "uninstall":
		// Windows' Installed apps: the app's window asks (shortcut.UninstallArgs)
		if hasGUI && slices.Equal(args, shortcut.UninstallArgs) {
			gateway.Serves = "app"
			return runUninstallWindow()
		}
		return uninstallCmd(args[1:])
	case "webdav", "dav":
		return webdavCmd(args[1:])
	case "plugin", "plugins":
		return pluginCmd(args)
	case "s3":
		return s3Cmd(args[1:])
	case "github":
		return githubCmd(args[1:])
	case "claude-code":
		return claudeCodeCmd(args[1:])
	case "settings":
		return settingsCmd(args[1:])
	}

	// the agent of a plugin that is gone (deleted on disk) is no agent
	// here, but what eipgam stashed of its file puts it back (#3810)
	if len(args) == 2 && strings.EqualFold(args[1], "default") {
		if g, ok, err := agent.RestoreGone(args[0]); ok {
			return goneDefault(args[0], g, err)
		}
	}
	a, err := agent.Find(args[0])
	if err != nil {
		return err
	}
	// no agent form takes a flag: one is never set as a model or a value
	// (`eipgam <agent> --verbose` took eipgam out of the agent, #1457)
	if f := flagArg(args[1:]); f != "" {
		return fmt.Errorf("unknown flag %s: nothing was changed\n\n%s", f, strings.TrimRight(agentHelp(a), "\n"))
	}
	switch len(args) {
	case 1:
		return list([]*agent.Agent{a}, true, -1)
	case 2:
		// default is eipgam's word in any case, as an agent's name and a
		// field's are: eipgam codex DEFAULT wrote model = "DEFAULT"
		if strings.EqualFold(args[1], "default") {
			if a.Wired() {
				return disconnect(a)
			}
			return set(a, a.Fields[0].Key, "")
		}
		// `eipgam codex model`: a field's name alone asks what it is set to,
		// where it was set as the model (model = "model"). Claude Code's
		// opus is a model's name as well as a tier's, and stays the model
		if f := a.Field(args[1]); f != nil && !modelAlias(a, args[1]) {
			return showField(a, f)
		}
		// `eipgam codex xhigh`: a bare value that belongs to a non-model field
		// (effort levels, for instance) is routed there; anything else is a model.
		if f := fieldForValue(a, args[1]); f != nil {
			return set(a, f.Key, args[1])
		}
		return set(a, a.Fields[0].Key, args[1])
	case 3:
		value := args[2]
		if strings.EqualFold(value, "default") {
			value = ""
		}
		return set(a, args[1], value)
	}
	return fmt.Errorf("too many arguments\n\n%s", usage)
}

func set(a *agent.Agent, key, value string) error {
	f := a.Field(key)
	if f == nil {
		var keys []string
		for _, f := range a.Fields {
			keys = append(keys, f.Key)
		}
		return fmt.Errorf("%s has no field %q (fields: %s)", a.Name, key, strings.Join(keys, ", "))
	}
	// a field's name is no value: eipgam codex subagent effort ran Codex's
	// subagents on a model called effort. Claude Code's opus is a model's
	// name as well as a tier's
	if g := a.Field(value); value != "" && g != nil && !modelAlias(a, value) {
		return fieldAsValue(a, f, g, value)
	}
	value, err := a.Spell(f.Key, value)
	if err != nil {
		return err
	}
	before := f.Get()
	wired := a.Wired()
	// a eipgam model picked on an agent not connected yet connects it
	// first, as the Agents page's picker does (#1900): the agent is wired
	// in, and Disconnect puts back what it had before
	if err := a.Pick(f.Key, value); err != nil {
		return err
	}
	// what the config reads now, not what was asked: an agent may name the
	// model under a provider of its own (OpenCode's eipgam-relay/…), and a
	// value it had already is said to be so
	now := f.Get()
	shown := now
	if value == "" || now == "" {
		shown = muted.Render("default")
	}
	if now == before {
		shown += " " + muted.Render("(unchanged)")
	}
	fmt.Println(green.Render("✓"), bold.Render(a.Name), muted.Render(f.Label), shown)
	// a value that is the agent's own, not one of eipgam's, takes eipgam
	// out: said, as Disconnect is, with the way back (#2000: eipgam claude
	// sonnet put Claude Code back on its DeepSeek endpoint, which refused
	// Sonnet, and only "✓ Claude Code model sonnet" was printed)
	if wired && now != "" && !a.Wired() {
		fmt.Println(muted.Render(fmt.Sprintf("  disconnected from eipgam: %s is %s's own, sent to the endpoint it had before eipgam; eipgam %s <model> with one of eipgam's models connects it again", now, a.Name, a.ID)))
	}
	if a.Notice != nil && (!f.Live || !wired && a.Wired()) {
		if n := a.Notice(); n != "" {
			fmt.Println(muted.Render("  ↻ " + n))
		}
	}
	return nil
}

// modelAlias says whether a word names the agent's model, in any case,
// though a field goes by it too: Claude Code's opus, a tier's name as well
func modelAlias(a *agent.Agent, w string) bool {
	return slices.ContainsFunc(a.ModelAliases, func(m string) bool { return strings.EqualFold(m, w) })
}

// fieldAsValue refuses field g's name given as field f's value, for what
// was meant: two words that are one field's label (eipgam codex subagent
// effort), a switch, which is turned on (Claude Code's ultracode), or the
// field, which its name alone shows.
func fieldAsValue(a *agent.Agent, f, g *agent.Field, value string) error {
	for _, name := range []string{f.Key + " " + value, f.Key + "_" + value} {
		if h := a.Field(name); h != nil {
			return fmt.Errorf("%q is one field of %s's: eipgam %s %s shows it", h.Label, a.Name, a.ID, h.Key)
		}
	}
	if g.Options != nil {
		if o := g.Options(a.Values()); len(o) == 1 && o[0].Value == "on" {
			return fmt.Errorf("%q is a switch of %s's, not a value for its %s: eipgam %s %s on", value, a.Name, f.Label, a.ID, g.Key)
		}
	}
	return fmt.Errorf("%q is a field of %s's, not a value for its %s: eipgam %s %s shows it", value, a.Name, f.Label, a.ID, g.Key)
}

// showField is `eipgam <agent> <field>`: what the field is set to, as set
// prints it
func showField(a *agent.Agent, f *agent.Field) error {
	v := f.Get()
	if v == "" {
		v = muted.Render("default")
	}
	fmt.Println(bold.Render(a.Name), muted.Render(f.Label), v)
	return nil
}

// disconnect is `eipgam <agent> default` on an agent eipgam is wired into:
// the Agents page's Disconnect, which puts back what the user had before
// eipgam — Claude Code's own model, its endpoint — where a field's default
// leaves the agent as installed (__jingling on X: eipgam claude default
// took the model they had set away with eipgam's)
func disconnect(a *agent.Agent) error {
	before := a.Values()
	if err := a.Disconnect(); err != nil {
		return err
	}
	now := a.Values()
	for _, f := range a.Fields {
		if before[f.Key] == now[f.Key] {
			continue
		}
		shown := now[f.Key]
		if shown == "" {
			shown = muted.Render("default")
		}
		fmt.Println(green.Render("✓"), bold.Render(a.Name), muted.Render(f.Label), shown)
	}
	fmt.Println(muted.Render("  disconnected from eipgam, back to what it had before"))
	if a.Notice != nil {
		if n := a.Notice(); n != "" {
			fmt.Println(muted.Render("  ↻ " + n))
		}
	}
	return nil
}

// goneDefault says what `eipgam <id> default` did for the agent of a
// plugin that is gone.
func goneDefault(id string, g agent.Gone, err error) error {
	for _, p := range g.Put {
		fmt.Println(green.Render("✓"), bold.Render(id), muted.Render(p))
	}
	for _, p := range g.Off {
		fmt.Println(green.Render("✓"), bold.Render(id), muted.Render(p+": eipgam was out of it already"))
	}
	if len(g.Put) > 0 {
		fmt.Println(muted.Render("  its plugin is gone; disconnected from eipgam, back to what it had before"))
	}
	if g.PluginOff != "" {
		fmt.Println(green.Render("✓"), bold.Render(g.PluginOff), muted.Render("is not on eipgam: its plugin is off, and eipgam put back what it had when the plugin was turned off"))
	}
	if err != nil {
		return fmt.Errorf("%s's plugin is gone, and eipgam couldn't put its config back: %w", id, err)
	}
	return nil
}

func fieldForValue(a *agent.Agent, v string) *agent.Field {
	vals := a.Values()
	// the agent's suffix after a model (omp's ":max") aside: a role on the
	// same model at that level offers it as typed, and would take it. A list
	// of models no picker offers: it is for the model
	if a.SplitSuffix != nil {
		m, _, one := a.SplitSuffix(v)
		if !one {
			return nil
		}
		v = m
	}
	// a model stays with the model, even where other fields offer it too
	// (Claude Code's opus/sonnet/haiku/fable)
	for _, o := range a.Fields[0].Options(vals) {
		if o.Value == v {
			return nil
		}
	}
	for i := 1; i < len(a.Fields); i++ {
		for _, o := range a.Fields[i].Options(vals) {
			if o.Value == v {
				return &a.Fields[i]
			}
		}
	}
	return nil
}

// list prints the agents; those from dimFrom on (when not -1) are the ones
// hidden in the app, and are dimmed.
func list(agents []*agent.Agent, detectedOnly bool, dimFrom int) error {
	if len(agents) == 0 {
		return fmt.Errorf("no supported agents found on this machine")
	}
	type row struct{ name, vals, path string }
	var rows []row
	nameW, valW := 0, 0
	for i, a := range agents {
		r := row{name: a.Name, path: tilde(a.Path)}
		if !detectedOnly && !a.Detected() {
			r.name = faint.Render(a.Name)
			r.vals = faint.Render("not detected")
			r.path = ""
		} else if a.CLIMissing() {
			// its folder left by an uninstall (#2173): what it holds is
			// read by nothing
			r.name = faint.Render(a.Name)
			r.vals = faint.Render("CLI not found")
		} else {
			vals := a.Values()
			dim := dimFrom >= 0 && i >= dimFrom
			label, value := muted, lipgloss.NewStyle()
			if dim {
				label, value = faint, faint
			}
			var parts []string
			for _, f := range a.Fields {
				v := vals[f.Key]
				if v == "" && f.Quiet {
					continue
				}
				if v == "" {
					v = faint.Render("—")
				} else {
					v = value.Render(v)
				}
				if f.Label == "model" {
					parts = append(parts, v)
				} else {
					parts = append(parts, label.Render(f.Label)+" "+v)
				}
			}
			r.name = bold.Render(a.Name)
			if dim {
				r.name = faint.Render(a.Name) + " " + faint.Render("hidden")
			}
			r.vals = strings.Join(parts, label.Render("  ·  "))
			// through eipgam for account failover while not connected (#1385)
			if said := a.FailoverSaid(); said != "" {
				r.vals += label.Render("  ·  " + said)
			}
			if a.Import != nil {
				// unknown reads as not added here: the line only offers the
				// command, never says eipgam is missing
				added := false
				if a.Added != nil {
					added, _ = a.Added()
				}
				if added {
					r.vals = value.Render("eipgam added")
				} else {
					r.vals = label.Render("eipgam "+a.ID+" add") + faint.Render("  to add eipgam")
				}
			}
		}
		nameW = max(nameW, lipgloss.Width(r.name))
		valW = max(valW, lipgloss.Width(r.vals))
		rows = append(rows, r)
	}
	for _, r := range rows {
		fmt.Printf("  %s  %s  %s\n", pad(r.name, nameW), pad(r.vals, valW), faint.Render(r.path))
	}
	return nil
}

func tilde(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

func profiles(args []string) error {
	switch args[0] {
	case "profiles":
		ps, err := profile.Load()
		if err != nil {
			return err
		}
		if len(ps) == 0 {
			fmt.Println(muted.Render("no profiles yet · eipgam save <name>"))
			return nil
		}
		detected := agent.Detected()
		for _, n := range profile.Names(ps) {
			// ✓ the one the setup is now (superhtoooth on X)
			mark := " "
			if profile.InUse(ps[n], detected, nil) {
				mark = green.Render("✓")
			}
			fmt.Printf("%s %s  %s\n", mark, bold.Render(n), muted.Render(profile.LongSummary(ps[n])))
		}
		return nil
	case "save":
		if len(args) < 2 {
			return fmt.Errorf("usage: eipgam save <name>")
		}
		p, err := profile.Snapshot()
		if err != nil {
			return err
		}
		if err := profile.Save(args[1], p); err != nil {
			return err
		}
		fmt.Println(green.Render("✓"), "saved profile", bold.Render(args[1]))
		if p.Library != nil {
			fmt.Println(" ", muted.Render("with the "+p.Library.Summary()))
		}
		return nil
	case "use":
		if len(args) < 2 {
			return fmt.Errorf("usage: eipgam use <name>")
		}
		ps, err := profile.Load()
		if err != nil {
			return err
		}
		p, ok := ps[args[1]]
		if !ok {
			return fmt.Errorf("no profile named %q", args[1])
		}
		a, err := profile.Apply(p)
		if err != nil {
			return err
		}
		fmt.Println(green.Render("✓"), "applied", bold.Render(args[1]), muted.Render(fmt.Sprintf("(%d changed)", a.Changed)))
		for _, line := range profile.Report(a) {
			fmt.Println(" ", muted.Render(line))
		}
		return nil
	case "rm":
		if len(args) < 2 {
			return fmt.Errorf("usage: eipgam rm <name>")
		}
		if err := profile.Delete(args[1]); err != nil {
			return err
		}
		fmt.Println(green.Render("✓"), "deleted profile", bold.Render(args[1]))
		return nil
	case "rename":
		// a flag is never taken for a name to write (eipgam snow --help
		// wrote "--help" into a profile)
		if len(args) != 3 || strings.HasPrefix(args[2], "-") {
			return fmt.Errorf("usage: eipgam rename <old> <new>")
		}
		if err := profile.Rename(args[1], args[2]); err != nil {
			return err
		}
		fmt.Println(green.Render("✓"), "renamed profile", bold.Render(args[1]), "to", bold.Render(args[2]))
		return nil
	}
	return nil
}

func pad(s string, w int) string {
	if n := w - lipgloss.Width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}
