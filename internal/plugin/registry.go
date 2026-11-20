package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/sichengchen/eipgam/internal/lazyre"
	"github.com/sichengchen/eipgam/internal/netproxy"
	"github.com/sichengchen/eipgam/internal/proc"
	"github.com/sichengchen/eipgam/internal/settings"
	"github.com/sichengchen/eipgam/internal/source"
)

// Where the npm registry in use came from (RegistryInUse.From).
const (
	FromMagpie  = "eipgam"  // set on the Plugins page or with `eipgam plugin registry`
	FromEnv     = "env"     // eipgam's environment: BUN_CONFIG_REGISTRY, NPM_CONFIG_REGISTRY
	FromShell   = "shell"   // the login shell's, which eipgam opened from the Dock doesn't have
	FromBunfig  = "bunfig"  // install.registry in a .bunfig.toml
	FromNPMRC   = "npmrc"   // registry= in an .npmrc
	FromDefault = "default" // npm's own
)

// RegistryInUse is the npm registry plugins are installed from and npm's
// answers are asked of, and where it came from: Var names the variable
// (env, shell) and Path the file (bunfig, npmrc).
type RegistryInUse struct {
	URL  string `json:"url"`
	From string `json:"from"`
	Var  string `json:"var,omitempty"`
	Path string `json:"path,omitempty"`
}

// registryVars are the variables bun takes a registry from, in the order
// it takes them.
var registryVars = []string{"BUN_CONFIG_REGISTRY", "NPM_CONFIG_REGISTRY", "npm_config_registry"}

// loginEnv is what the login shell said of a variable (a var for tests).
var loginEnv = proc.LoginEnvKnown

// Registry is the npm registry in use: the one set in eipgam, then one the
// user's own setup names (eipgam's environment, the login shell's, a
// .bunfig.toml, an .npmrc: a registry of the user's own, which bun reads
// too), then npm's own (timwangdev on X: plugins from a custom npm
// registry).
func Registry() RegistryInUse {
	if u, err := CleanRegistry(settings.Shared().NPMRegistry); err == nil && u != "" {
		return RegistryInUse{URL: u, From: FromMagpie}
	}
	if r, ok := userRegistry(); ok {
		return r
	}
	return RegistryInUse{URL: npmDefault, From: FromDefault}
}

// npmDefault is npm's own registry, as Registry names it; lookups of it go
// to npmRegistry, which tests move.
const npmDefault = "https://registry.npmjs.org"

// userRegistry is the registry the user's own setup names, as bun finds it:
// its variables, then the plugins folder's bunfig.toml and .npmrc, then the
// global .bunfig.toml and ~/.npmrc.
func userRegistry() (RegistryInUse, bool) {
	for _, k := range registryVars {
		if u, err := CleanRegistry(os.Getenv(k)); err == nil && u != "" {
			return RegistryInUse{URL: u, From: FromEnv, Var: k}, true
		}
	}
	for _, k := range registryVars {
		if u, err := CleanRegistry(loginEnv(k)); err == nil && u != "" {
			return RegistryInUse{URL: u, From: FromShell, Var: k}, true
		}
	}
	for _, f := range registryFiles() {
		var u string
		if f.bunfig {
			u = bunfigRegistry(f.path)
		} else {
			u = npmrcOf(f.path).registry
		}
		if u, err := CleanRegistry(u); err == nil && u != "" {
			from := FromNPMRC
			if f.bunfig {
				from = FromBunfig
			}
			return RegistryInUse{URL: u, From: from, Path: f.path}, true
		}
	}
	return RegistryInUse{}, false
}

type registryFile struct {
	path   string
	bunfig bool
}

// registryFiles are the files bun reads a registry from, the plugins
// folder's (where it installs) before the user's.
func registryFiles() []registryFile {
	out := []registryFile{{filepath.Join(Dir(), "bunfig.toml"), true}, {filepath.Join(Dir(), ".npmrc"), false}}
	if x := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(x) {
		out = append(out, registryFile{filepath.Join(x, ".bunfig.toml"), true})
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		out = append(out, registryFile{filepath.Join(home, ".bunfig.toml"), true}, registryFile{filepath.Join(home, ".npmrc"), false})
	}
	return out
}

// CleanRegistry is u as a registry's address, without its trailing slash:
// "" for "", an error for anything but an http(s) URL with a host.
func CleanRegistry(u string) (string, error) {
	u = strings.TrimSpace(u)
	if u == "" {
		return "", nil
	}
	p, err := url.Parse(u)
	if err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" || p.User != nil || p.RawQuery != "" || p.Fragment != "" {
		return "", fmt.Errorf("%q isn't a registry's address: give an http:// or https:// one, like https://registry.example.com", u)
	}
	return strings.TrimRight(u, "/"), nil
}

// SetRegistry keeps u as the registry plugins use, "" or "default" for
// none of eipgam's own; an address that isn't one is refused, not kept.
func SetRegistry(u string) (RegistryInUse, error) {
	if strings.EqualFold(strings.TrimSpace(u), "default") {
		u = ""
	}
	c, err := CleanRegistry(u)
	if err != nil {
		return Registry(), err
	}
	s := settings.Load()
	if s.NPMRegistry != c {
		s.NPMRegistry = c
		if err := settings.Save(s); err != nil {
			return Registry(), err
		}
		ReloadInfo()
	}
	return Registry(), nil
}

// registryEnv is the registry bun installs from, given the environment it
// runs in: the one set in eipgam and the login shell's (which env lacks)
// are told to it; one env, a bunfig or an .npmrc names bun reads itself.
func registryEnv(env []string) []string {
	r := Registry()
	switch r.From {
	case FromMagpie, FromShell:
	default:
		return nil
	}
	if r.From != FromMagpie {
		// a variable the environment it runs in has (a test's) is its own
		for _, kv := range env {
			k, v, _ := strings.Cut(kv, "=")
			if k = strings.ToUpper(k); (k == "BUN_CONFIG_REGISTRY" || k == "NPM_CONFIG_REGISTRY") && strings.TrimSpace(v) != "" {
				return nil
			}
		}
	}
	return []string{"BUN_CONFIG_REGISTRY=" + r.URL + "/", "NPM_CONFIG_REGISTRY=" + r.URL + "/"}
}

// npmrc is what an .npmrc says of registries: the default one, each
// scope's, and the tokens it keeps for each, by npm's "nerf dart"
// (//host/path/).
type npmrc struct {
	registry string
	scopes   map[string]string
	tokens   map[string]string
}

// npmrcVar is a ${VAR} in an .npmrc's value, which npm expands.
var npmrcVar = lazyre.New(`\$\{[A-Za-z_][A-Za-z0-9_]*\}`)

// npmrcOf reads the .npmrc at path; an empty one when it can't be read. A
// ${VAR} is the environment's, as npm expands it.
func npmrcOf(path string) npmrc {
	rc := npmrc{scopes: map[string]string{}, tokens: map[string]string{}}
	b, err := os.ReadFile(path)
	if err != nil {
		return rc
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), npmrcVar.ReplaceAllStringFunc(strings.Trim(strings.TrimSpace(v), `"'`), func(m string) string { return os.Getenv(m[2 : len(m)-1]) })
		switch {
		case k == "registry":
			rc.registry = v
		case strings.HasPrefix(k, "@") && strings.HasSuffix(k, ":registry"):
			rc.scopes[strings.TrimSuffix(k, ":registry")] = v
		case strings.HasPrefix(k, "//") && strings.HasSuffix(k, ":_authToken"):
			rc.tokens[strings.TrimSuffix(k, ":_authToken")] = v
		}
	}
	return rc
}

// bunfigRegistry is install.registry in the bunfig at path: an address,
// or a table with its url; "" when it names none.
func bunfigRegistry(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var c struct {
		Install struct {
			Registry any `toml:"registry"`
		} `toml:"install"`
	}
	if toml.Unmarshal(b, &c) != nil {
		return ""
	}
	switch r := c.Install.Registry.(type) {
	case string:
		return r
	case map[string]any:
		u, _ := r["url"].(string)
		return u
	}
	return ""
}

// npmrcs are the .npmrc files bun reads, the plugins folder's first.
func npmrcs() []npmrc {
	var out []npmrc
	for _, f := range registryFiles() {
		if !f.bunfig {
			out = append(out, npmrcOf(f.path))
		}
	}
	return out
}

// npmBase is the registry eipgam asks of package name, and the token an
// .npmrc keeps for it: a scope's own registry (@scope:registry=) for a
// package of that scope (bun installs it from there whatever the default
// registry is); Registry's otherwise. official is whether it is npm's own.
func npmBase(name string) (base, token string, official bool) {
	r := Registry()
	rcs := npmrcs()
	if scope, _, ok := strings.Cut(name, "/"); ok && strings.HasPrefix(scope, "@") {
		for _, rc := range rcs {
			if u, err := CleanRegistry(rc.scopes[scope]); err == nil && u != "" {
				return u, tokenFor(rcs, u), false
			}
		}
	}
	if r.From == FromDefault {
		return npmRegistry, "", true
	}
	return r.URL, tokenFor(rcs, r.URL), false
}

// tokenFor is the token an .npmrc keeps for the registry at base: the one
// whose nerf dart (//host/path/) is the longest that base's starts with,
// as npm matches them; "" when none.
func tokenFor(rcs []npmrc, base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	dart := "//" + u.Host + strings.TrimRight(u.EscapedPath(), "/") + "/"
	best, tok := "", ""
	for _, rc := range rcs {
		for k, v := range rc.tokens {
			k = strings.TrimRight(k, "/") + "/"
			if strings.HasPrefix(dart, k) && len(k) > len(best) && v != "" {
				best, tok = k, v
			}
		}
	}
	return tok
}

// fetchNPM is what the registry in use says at suffix (/<package>,
// /<package>/latest), for package name. npm's own is asked without a
// token; a registry the user named is asked with the token their .npmrc
// keeps for it and no other.
func fetchNPM(ctx context.Context, name, suffix string, limit int64) ([]byte, error) {
	base, token, official := npmBase(name)
	if official {
		return fetchJSON(ctx, base+suffix, limit)
	}
	return fetchRegistry(ctx, base+suffix, token, limit)
}

// fetchRegistry gets u from a registry of the user's, with its token; the
// token never goes to any other address (a redirect elsewhere).
func fetchRegistry(ctx context.Context, u, token string, limit int64) ([]byte, error) {
	ctx = netproxy.Downloads(ctx) // plugin downloads' proxy
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "eipgam")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	c := *http.DefaultClient
	c.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if r.URL.Host != via[0].URL.Host {
			r.Header.Del("Authorization")
		}
		return nil
	}
	res, err := source.Do(&c, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, &statusError{URL: u, Code: res.StatusCode, Status: res.Status}
	}
	return io.ReadAll(io.LimitReader(res.Body, limit))
}
