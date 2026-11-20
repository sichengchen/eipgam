// Package iconproxy routes remote images through an explicitly configured proxy.
package iconproxy

import (
	"net/url"
	"os"
)

// URL returns the proxy address for an image, or an empty string if unconfigured.
func URL(u string) string {
	base := os.Getenv("EIPGAM_ICON_PROXY")
	if base == "" {
		return ""
	}
	return base + "?url=" + url.QueryEscape(u)
}
