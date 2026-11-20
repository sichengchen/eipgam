// Package notices holds the third-party notices every eipgam binary
// carries: THIRD_PARTY_NOTICES at the repository's root, which package main
// embeds and hands over before anything runs. `eipgam licenses` prints it,
// and Settings › About shows it.
package notices

// Text is THIRD_PARTY_NOTICES, set by package main.
var Text string
