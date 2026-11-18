// Package source sends magpie's own downloads (npm's registry, GitHub,
// Bun's releases, Hugging Face's models) to their official addresses.
// magpie names no mirror of its own: one the user chose is the caller's
// (Settings' UpdateMirror for magpie update, an npm registry set on the
// Plugins page).
package source

import "net/http"

// Do sends req through c to its own address, and to no other.
func Do(c *http.Client, req *http.Request) (*http.Response, error) {
	return c.Do(req)
}
