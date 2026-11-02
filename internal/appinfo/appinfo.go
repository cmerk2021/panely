// Package appinfo holds the application identity. The Name constant is the
// single source of truth for the binary name, config/data directory names,
// and help text.
package appinfo

// Name is the application name, used for the binary, config dir, and data dir.
const Name = "panely"

// Version information, injected at build time via ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
