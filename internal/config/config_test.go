package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var knownTypes = []string{"clock", "weather", "todo", "calendar", "sysinfo"}

func TestParseValid(t *testing.T) {
	cfg, err := Parse([]byte(`
theme: nord
rows:
  - height: 30%
    panels:
      - type: clock
        weight: 1
  - panels:
      - type: todo
      - type: sysinfo
        weight: 2
`), knownTypes)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "nord" || len(cfg.Rows) != 2 {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if cfg.Rows[1].Panels[1].EffectiveWeight() != 2 {
		t.Error("weight not parsed")
	}
	if cfg.Rows[1].Panels[0].EffectiveWeight() != 1 {
		t.Error("default weight should be 1")
	}
}

func TestParseUnknownPanelTypeSuggestion(t *testing.T) {
	_, err := Parse([]byte(`
rows:
  - panels:
      - type: wether
`), knownTypes)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `did you mean "weather"`) {
		t.Errorf("expected typo suggestion, got: %v", err)
	}
	if !strings.Contains(err.Error(), "line 4") {
		t.Errorf("expected line number, got: %v", err)
	}
}

func TestParseInvalidHeight(t *testing.T) {
	_, err := Parse([]byte(`
rows:
  - height: tall
    panels: [{type: clock}]
`), knownTypes)
	if err == nil || !strings.Contains(err.Error(), "height") {
		t.Errorf("expected height error, got %v", err)
	}
}

func TestParseBadThemeColor(t *testing.T) {
	_, err := Parse([]byte(`
rows:
  - panels: [{type: clock}]
themes:
  mine:
    colors: {accent: "red"}
`), knownTypes)
	if err == nil || !strings.Contains(err.Error(), "hex color") {
		t.Errorf("expected hex color error, got %v", err)
	}
}

func TestExpandSecrets(t *testing.T) {
	t.Setenv("PANELY_TEST_SECRET", "s3cret")
	_ = os.Unsetenv("PANELY_TEST_MISSING")
	cfg := &Config{
		Rows: []Row{{Panels: []PanelConfig{{
			Type: "clock",
			Options: map[string]any{
				"password": "${PANELY_TEST_SECRET}",
				"fallback": "${PANELY_TEST_MISSING:-default-val}",
				"empty":    "${PANELY_TEST_MISSING}",
				"nested":   map[string]any{"inner": "pre-${PANELY_TEST_SECRET}-post"},
				"list":     []any{"${PANELY_TEST_SECRET}"},
				"number":   42,
			},
		}}}},
	}
	ExpandSecrets(cfg)
	opts := cfg.Rows[0].Panels[0].Options
	if opts["password"] != "s3cret" {
		t.Errorf("password: %v", opts["password"])
	}
	if opts["fallback"] != "default-val" {
		t.Errorf("fallback: %v", opts["fallback"])
	}
	if opts["empty"] != "" {
		t.Errorf("empty: %v", opts["empty"])
	}
	if opts["nested"].(map[string]any)["inner"] != "pre-s3cret-post" {
		t.Errorf("nested: %v", opts["nested"])
	}
	if opts["list"].([]any)[0] != "s3cret" {
		t.Errorf("list: %v", opts["list"])
	}
	if opts["number"] != 42 {
		t.Errorf("number changed: %v", opts["number"])
	}
}

func TestSaveRoundTripPreservesComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := `# my precious comment
theme: nord
refresh: 1s
rows:
  - panels:
      - type: clock
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path, knownTypes)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Theme = "dracula"
	cfg.Rows[0].Panels = append(cfg.Rows[0].Panels, PanelConfig{Type: "sysinfo", Weight: 2})
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	s := string(out)
	if !strings.Contains(s, "# my precious comment") {
		t.Error("top comment lost")
	}
	if !strings.Contains(s, "dracula") {
		t.Error("theme change lost")
	}
	if !strings.Contains(s, "refresh: 1s") {
		t.Error("unrelated key lost")
	}
	// Reload and verify structure survived.
	cfg2, err := Load(path, knownTypes)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg2.Rows[0].Panels) != 2 || cfg2.Rows[0].Panels[1].Type != "sysinfo" {
		t.Errorf("structural edit lost: %+v", cfg2.Rows)
	}
}

func TestWriteDefaultAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.yaml")
	if err := WriteDefault(path); err != nil {
		t.Fatal(err)
	}
	if err := WriteDefault(path); err == nil {
		t.Error("expected error overwriting existing config")
	}
	cfg, err := Load(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Rows) == 0 {
		t.Error("default config has no rows")
	}
	if cfg.Theme == "" {
		t.Error("default config has no theme")
	}
}

func TestLevenshtein(t *testing.T) {
	if d := levenshtein("wether", "weather"); d != 1 {
		t.Errorf("levenshtein = %d, want 1", d)
	}
	if closest("zzzzz", knownTypes) != "" {
		t.Error("closest should reject distant matches")
	}
}
