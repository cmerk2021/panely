package config

import (
	"os"
	"regexp"
)

var envRef = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}`)

// ExpandSecrets expands ${VAR} and ${VAR:-default} references in every string
// value of the config, including nested option maps. All providers receive
// already-expanded options. Secret values must never be logged.
func ExpandSecrets(cfg *Config) {
	cfg.Theme = ExpandString(cfg.Theme)
	for ri := range cfg.Rows {
		for pi := range cfg.Rows[ri].Panels {
			p := &cfg.Rows[ri].Panels[pi]
			p.Title = ExpandString(p.Title)
			p.Options = expandAny(p.Options).(map[string]any)
		}
	}
}

// ExpandString expands ${VAR} and ${VAR:-default} in a single string.
func ExpandString(s string) string {
	return envRef.ReplaceAllStringFunc(s, func(m string) string {
		groups := envRef.FindStringSubmatch(m)
		name, hasDefault, def := groups[1], groups[2] != "", groups[3]
		if v, ok := os.LookupEnv(name); ok {
			return v
		}
		if hasDefault {
			return def
		}
		return ""
	})
}

func expandAny(v any) any {
	switch t := v.(type) {
	case string:
		return ExpandString(t)
	case map[string]any:
		if t == nil {
			return map[string]any{}
		}
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = expandAny(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = expandAny(val)
		}
		return out
	default:
		return v
	}
}
