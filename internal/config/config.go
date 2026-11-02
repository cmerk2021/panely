// Package config loads, validates, saves, and watches the panely YAML
// configuration file, which is the source of truth for layout and settings.
package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration.
type Config struct {
	Theme   string                 `yaml:"theme"`
	Refresh string                 `yaml:"refresh,omitempty"`
	Rows    []Row                  `yaml:"rows"`
	Themes  map[string]CustomTheme `yaml:"themes,omitempty"`
}

// Row is a horizontal band of panels.
type Row struct {
	// Height is "auto", a fixed line count ("12"), or a percentage ("30%").
	Height string        `yaml:"height,omitempty"`
	Panels []PanelConfig `yaml:"panels"`
}

// PanelConfig configures a single panel instance.
type PanelConfig struct {
	Type      string         `yaml:"type"`
	Title     string         `yaml:"title,omitempty"`
	Weight    int            `yaml:"weight,omitempty"`
	MinWidth  int            `yaml:"min_width,omitempty"`
	MinHeight int            `yaml:"min_height,omitempty"`
	Options   map[string]any `yaml:"options,omitempty"`
}

// CustomTheme is a user-defined theme with optional inheritance.
type CustomTheme struct {
	Base   string            `yaml:"base,omitempty"`
	Colors map[string]string `yaml:"colors,omitempty"`
}

// EffectiveWeight returns the panel weight, defaulting to 1.
func (p PanelConfig) EffectiveWeight() int {
	if p.Weight <= 0 {
		return 1
	}
	return p.Weight
}

// Load reads and parses the config file at path, expanding ${ENV} secrets in
// all string values and validating the result against the given set of known
// panel types.
func Load(path string, knownPanelTypes []string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data, knownPanelTypes)
}

// Parse parses raw YAML bytes into a validated Config.
func Parse(data []byte, knownPanelTypes []string) (*Config, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	var cfg Config
	if err := root.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	ExpandSecrets(&cfg)
	if err := Validate(&cfg, knownPanelTypes, &root); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks the config for structural errors, producing clear messages
// with line numbers where available. node may be nil.
func Validate(cfg *Config, knownPanelTypes []string, node *yaml.Node) error {
	var errs []string
	if len(cfg.Rows) == 0 {
		errs = append(errs, "rows: at least one row is required")
	}
	for ri, row := range cfg.Rows {
		if len(row.Panels) == 0 {
			errs = append(errs, fmt.Sprintf("rows[%d].panels: row has no panels%s", ri, lineHint(node, ri, -1)))
		}
		if row.Height != "" && row.Height != "auto" {
			h := strings.TrimSuffix(row.Height, "%")
			if !isUint(h) {
				errs = append(errs, fmt.Sprintf("rows[%d].height: invalid height %q (use a line count, a percentage like 30%%, or auto)", ri, row.Height))
			}
		}
		for pi, p := range row.Panels {
			if p.Type == "" {
				errs = append(errs, fmt.Sprintf("rows[%d].panels[%d].type: panel type is required%s", ri, pi, lineHint(node, ri, pi)))
				continue
			}
			if len(knownPanelTypes) > 0 && !contains(knownPanelTypes, p.Type) {
				msg := fmt.Sprintf("rows[%d].panels[%d].type: unknown panel type %q", ri, pi, p.Type)
				if s := closest(p.Type, knownPanelTypes); s != "" {
					msg += fmt.Sprintf(", did you mean %q?", s)
				}
				errs = append(errs, msg+lineHint(node, ri, pi))
			}
			if p.Weight < 0 {
				errs = append(errs, fmt.Sprintf("rows[%d].panels[%d].weight: must be >= 1", ri, pi))
			}
		}
	}
	for name, t := range cfg.Themes {
		for key, val := range t.Colors {
			if !validHexColor(val) {
				errs = append(errs, fmt.Sprintf("themes.%s.colors.%s: %q is not a valid hex color (expected #rrggbb)", name, key, val))
			}
		}
	}
	if len(errs) > 0 {
		sort.Strings(errs)
		return fmt.Errorf("invalid config:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

func validHexColor(s string) bool {
	if len(s) != 7 && len(s) != 4 {
		return false
	}
	if s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

func isUint(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// closest returns the candidate with the smallest Levenshtein distance to s,
// if the distance is small enough to be a plausible typo.
func closest(s string, candidates []string) string {
	best, bestDist := "", 3
	for _, c := range candidates {
		if d := levenshtein(s, c); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

// lineHint returns a " (line N)" suffix for rows[ri].panels[pi] (pi<0 means
// the row itself) if the YAML node tree permits locating it.
func lineHint(node *yaml.Node, ri, pi int) string {
	n := findPanelNode(node, ri, pi)
	if n == nil {
		return ""
	}
	return fmt.Sprintf(" (line %d)", n.Line)
}

func findPanelNode(node *yaml.Node, ri, pi int) *yaml.Node {
	if node == nil || len(node.Content) == 0 {
		return nil
	}
	doc := node.Content[0]
	rows := mapValue(doc, "rows")
	if rows == nil || ri >= len(rows.Content) {
		return nil
	}
	row := rows.Content[ri]
	if pi < 0 {
		return row
	}
	panels := mapValue(row, "panels")
	if panels == nil || pi >= len(panels.Content) {
		return row
	}
	return panels.Content[pi]
}

func mapValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}
