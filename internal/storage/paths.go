// Package storage provides XDG-compliant paths and atomic file writes.
package storage

import (
	"os"
	"path/filepath"

	"github.com/cmerk2021/panely/internal/appinfo"
)

// ConfigDir returns the per-user configuration directory for the app,
// honoring $XDG_CONFIG_HOME on Linux and using OS conventions elsewhere.
func ConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, appinfo.Name), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appinfo.Name), nil
}

// DataDir returns the per-user data directory for the app, honoring
// $XDG_DATA_HOME on Linux with sensible fallbacks elsewhere.
func DataDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, appinfo.Name), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// os.UserConfigDir-style fallbacks for data.
	switch {
	case os.Getenv("LOCALAPPDATA") != "": // Windows
		return filepath.Join(os.Getenv("LOCALAPPDATA"), appinfo.Name), nil
	case fileExists(filepath.Join(home, "Library", "Application Support")): // macOS
		return filepath.Join(home, "Library", "Application Support", appinfo.Name), nil
	default: // Linux / other Unix
		return filepath.Join(home, ".local", "share", appinfo.Name), nil
	}
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
