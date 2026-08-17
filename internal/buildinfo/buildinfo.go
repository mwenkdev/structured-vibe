// Package buildinfo exposes build-time identity for the svibe release.
//
// The CLI version is also the resolver/rules version: Structured Vibe does not
// maintain a separate rules-version number (architecture 14).
package buildinfo

// Version is the release version, injected at build time via -ldflags.
// It is the plain SemVer value with no leading "v".
var Version = "0.0.0-dev"
