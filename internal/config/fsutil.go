package config

import (
	"os"
	"path/filepath"
	"time"
)

func dirOf(p string) string { return filepath.Dir(p) }

func statFile(p string) (time.Time, error) {
	st, err := os.Stat(p)
	if err != nil {
		return time.Time{}, err
	}
	return st.ModTime(), nil
}
