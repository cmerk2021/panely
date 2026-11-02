package config

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/cmerk2021/panely/internal/storage"
)

// Save writes the config to path atomically.
//
// Comment preservation: scalar-only updates (such as the theme) go through
// UpdateScalar, which edits the YAML node tree in place and keeps all
// comments. Structural edits (adding/moving/deleting panels or rows) use
// node-tree surgery on `rows` only, so comments outside `rows` are preserved;
// comments attached to reordered panels may move or be dropped. This
// limitation is documented in docs/DECISIONS.md.
func Save(path string, cfg *Config) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		// No existing file: plain marshal.
		return writeMarshaled(path, cfg)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil || len(root.Content) == 0 {
		return writeMarshaled(path, cfg)
	}
	doc := root.Content[0]
	// Replace the rows value with a freshly marshaled node, keep the rest.
	var rowsNode yaml.Node
	if err := rowsNode.Encode(cfg.Rows); err != nil {
		return err
	}
	setMapValue(doc, "rows", &rowsNode)
	var themeNode yaml.Node
	if err := themeNode.Encode(cfg.Theme); err != nil {
		return err
	}
	setMapValue(doc, "theme", &themeNode)
	return writeNode(path, &root)
}

func writeMarshaled(path string, cfg *Config) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return storage.WriteFileAtomic(path, buf.Bytes(), 0o644)
}

func writeNode(path string, root *yaml.Node) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root.Content[0]); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return storage.WriteFileAtomic(path, buf.Bytes(), 0o644)
}

// setMapValue sets key to value in a mapping node, appending if missing.
func setMapValue(m *yaml.Node, key string, value *yaml.Node) {
	if m.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			// Keep comments attached to the old value's key.
			m.Content[i+1] = value
			return
		}
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		value)
}

// DefaultConfigYAML is the commented starter configuration written on first run.
const DefaultConfigYAML = `# panely configuration
# This file is the source of truth for your dashboard layout.
# Docs: docs/CONFIGURATION.md — edit live, panely hot-reloads on save.

# Theme: catppuccin-mocha, catppuccin-latte, tokyo-night, gruvbox-dark,
# nord, dracula, default (adapts to terminal background), or a custom
# theme defined under "themes:". Press "t" in the app to cycle themes.
theme: catppuccin-mocha

# Global UI tick (clock updates etc.)
refresh: 1s

rows:
  - height: 30%
    panels:
      - type: clock
        weight: 1
        options:
          timezones: [Local]
          format24h: true
          seconds: true
      - type: weather
        weight: 2
        options:
          location: "Berlin"
          units: metric
  - panels:
      - type: todo
        weight: 1
        options:
          provider: local
      - type: calendar
        weight: 1
      - type: sysinfo
        weight: 2
        options:
          processes: 5
  - height: 25%
    panels:
      - type: notes
        weight: 1
      - type: quote
        weight: 1

# Custom themes (optional). Inherit from a bundled theme and override colors:
# themes:
#   my-theme:
#     base: catppuccin-mocha
#     colors:
#       accent: "#ff79c6"
`

// WriteDefault writes the default config file to path (creating directories).
// It fails if the file already exists.
func WriteDefault(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists: %s", path)
	}
	return storage.WriteFileAtomic(path, []byte(DefaultConfigYAML), 0o644)
}
