package provider

import (
	"cmp"
	"net"
	"strings"

	"github.com/yetone/magpie/internal/catalog"
)

// PricesInYuan is whether p sells in China's yuan, for Settings' currency
// "provider", which shows each provider's costs in what it prices in. A
// preset's provider at the preset's own site does when its id ends in -cn
// or its first catalog does (a China region); one moved to another site
// is not that vendor. Any provider on a .cn host does, and so does a plugin's id
// ending in -cn (Qoder CN). A provider of its own (no preset) does with a
// yuan catalog (catalog.PricedInYuan) served at that catalog's own site.
// Everything else is in dollars.
func (p Provider) PricesInYuan() bool {
	h := strings.ToLower(p.Host())
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}
	if strings.HasSuffix(h, ".cn") {
		return true
	}
	cs := p.Catalogs()
	if pr := Preset(p.Preset); pr != nil {
		if h != "" && !sameSite(h, HostOf(cmp.Or(pr.Chat, pr.Anthropic, pr.Responses, pr.Decide))) {
			return false
		}
		id := strings.ToLower(pr.ID)
		return strings.HasSuffix(id, "-cn") || len(cs) > 0 && strings.HasSuffix(cs[0], "-cn")
	}
	if strings.HasSuffix(strings.ToLower(p.ID), "-cn") {
		return true
	}
	if len(cs) == 0 || !catalog.PricedInYuan(cs[0]) {
		return false
	}
	return h == "" || sameSite(h, catalog.APIHost(cs[0]))
}

// sameSite is whether two hosts share their last two labels
// (api.deepseek.com and platform.deepseek.com).
func sameSite(a, b string) bool {
	site := func(h string) string {
		parts := strings.Split(h, ".")
		if len(parts) > 2 {
			parts = parts[len(parts)-2:]
		}
		return strings.Join(parts, ".")
	}
	return a != "" && b != "" && site(a) == site(b)
}

// YuanPriced are the ids of the providers that PricesInYuan, for the
// window to show their costs in yuan; nil when none do.
func YuanPriced() []string {
	var out []string
	for _, p := range sharedAll() {
		if p.PricesInYuan() {
			out = append(out, p.ID)
		}
	}
	return out
}

// YuanPricedSet is YuanPriced as a set, for a table of rows looked up one by
// one (the TUI's and the CLI's).
func YuanPricedSet() map[string]bool {
	out := map[string]bool{}
	for _, id := range YuanPriced() {
		out[id] = true
	}
	return out
}
