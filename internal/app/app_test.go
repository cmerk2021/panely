package app

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	_ "github.com/cmerk2021/panely/internal/builtin"
	"github.com/cmerk2021/panely/internal/config"
	"github.com/cmerk2021/panely/internal/layout"
	"github.com/cmerk2021/panely/internal/panel"
)

func testModel(t *testing.T) *Model {
	yaml := testYAML
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath, panel.Types())
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := New(cfgPath, cfg, dir, logger, false)
	m.Init()
	t.Cleanup(m.watcher.Close)
	return m
}

const testYAML = `
theme: nord
rows:
  - height: 30%
    panels:
      - type: clock
        title: Clock
  - panels:
      - type: notes
      - type: quote
        weight: 2
`

func TestViewRendersBordersAndTitles(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	v := m.View()
	if !strings.Contains(v, "╭") || !strings.Contains(v, "╰") {
		t.Error("missing rounded borders")
	}
	if !strings.Contains(v, "Clock") {
		t.Error("missing custom title")
	}
	if !strings.Contains(v, "quote") {
		t.Error("missing type-derived title")
	}
	lines := strings.Split(v, "\n")
	if len(lines) != 30 {
		t.Errorf("view has %d lines, want 30", len(lines))
	}
}

func TestResizeNeverPanics(t *testing.T) {
	m := testModel(t)
	for w := 1; w <= 220; w += 13 {
		for h := 1; h <= 90; h += 7 {
			m.Update(tea.WindowSizeMsg{Width: w, Height: h})
			_ = m.View()
		}
	}
	// Extreme cases.
	for _, s := range [][2]int{{0, 0}, {1, 1000}, {1000, 1}, {500, 200}} {
		m.Update(tea.WindowSizeMsg{Width: s[0], Height: s[1]})
		_ = m.View()
	}
}

func TestFocusCycling(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.focus != 0 {
		t.Fatalf("initial focus %d", m.focus)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != 1 {
		t.Errorf("after tab, focus %d want 1", m.focus)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.focus != 2 {
		t.Errorf("after 2x shift+tab, focus %d want 2", m.focus)
	}
}

func TestSpatialFocus(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	// From clock (row 0, full width, center-right leans to quote) move down.
	m.moveFocusSpatial(0, +1)
	if m.focus != 2 {
		t.Errorf("down from clock => %d, want 2 (quote, nearest center)", m.focus)
	}
	m.moveFocusSpatial(-1, 0)
	if m.focus != 1 {
		t.Errorf("left from quote => %d, want 1 (notes)", m.focus)
	}
	m.moveFocusSpatial(0, -1)
	if m.focus != 0 {
		t.Errorf("up from notes => %d, want 0 (clock)", m.focus)
	}
}

func TestMouseClickFocuses(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	// Click inside the quote panel (right side, lower half).
	r := m.rects[2]
	m.handleMouse(tea.MouseMsg{
		X: r.X + 3, Y: r.Y + 2,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	if m.focus != 2 {
		t.Errorf("click should focus panel 2, got %d", m.focus)
	}
}

func TestEditModePersistsWeightChange(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if !m.editMode {
		t.Fatal("edit mode not entered")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.editMode {
		t.Fatal("edit mode not exited")
	}
	// Verify the change persisted to the config file.
	cfg, err := config.Load(m.cfgPath, panel.Types())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Rows[0].Panels[0].Weight != 2 {
		t.Errorf("weight not persisted, got %d", cfg.Rows[0].Panels[0].Weight)
	}
}

func TestEditModeDeletePanel(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !m.confirmDel {
		t.Fatal("delete confirmation not shown")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	cfg, err := config.Load(m.cfgPath, panel.Types())
	if err != nil {
		t.Fatal(err)
	}
	// Deleting the only panel in row 0 removes the row.
	if len(cfg.Rows) != 1 {
		t.Errorf("expected 1 row after delete, got %d", len(cfg.Rows))
	}
}

func TestEditModeMovePanelAcrossRows(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	// Move clock (row 0, only panel) down into row 1.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	cfg, _ := config.Load(m.cfgPath, panel.Types())
	if len(cfg.Rows) != 1 || len(cfg.Rows[0].Panels) != 3 {
		t.Fatalf("expected single row of 3 panels, got %+v", cfg.Rows)
	}
	// Move it back up, creating a new row.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	cfg, _ = config.Load(m.cfgPath, panel.Types())
	if len(cfg.Rows) != 2 {
		t.Fatalf("expected 2 rows after move up, got %+v", cfg.Rows)
	}
}

func TestInvalidReloadKeepsOldConfig(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if err := os.WriteFile(m.cfgPath, []byte("rows: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.Update(reloadMsg{})
	if len(m.cfg.Rows) != 2 {
		t.Error("old config not kept after invalid reload")
	}
	if !m.statusErr {
		t.Error("expected error shown in status bar")
	}
	_ = m.View() // still renders
}

func TestValidReloadApplies(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	newCfg := "theme: dracula\nrows:\n  - panels:\n      - type: clock\n"
	if err := os.WriteFile(m.cfgPath, []byte(newCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	m.Update(reloadMsg{})
	if m.cfg.Theme != "dracula" || len(m.cfg.Rows) != 1 {
		t.Errorf("reload not applied: %+v", m.cfg)
	}
	if len(m.panels) != 1 {
		t.Errorf("panels not rebuilt: %d", len(m.panels))
	}
}

func TestThemeCyclePersists(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	before := m.cfg.Theme
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if m.cfg.Theme == before {
		t.Error("theme did not change")
	}
	cfg, err := config.Load(m.cfgPath, panel.Types())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != m.cfg.Theme {
		t.Errorf("theme not persisted: file=%s mem=%s", cfg.Theme, m.cfg.Theme)
	}
}

func TestLayoutRectsFillDashboardArea(t *testing.T) {
	m := testModel(t)
	for _, s := range [][2]int{{80, 24}, {121, 41}, {37, 13}} {
		m.Update(tea.WindowSizeMsg{Width: s[0], Height: s[1]})
		area := 0
		for _, r := range m.rects {
			area += r.Width * r.Height
		}
		want := s[0] * (s[1] - 1) // one line reserved for status bar
		if m.rects != nil && area != want {
			t.Errorf("size %v: rect area %d != %d", s, area, want)
		}
	}
}

func TestAllRegisteredPanelsConstruct(t *testing.T) {
	// Every registered panel type must construct with minimal valid options.
	opts := map[string]map[string]any{
		"weather":   {"location": "Berlin"},
		"command":   {"command": "echo hi"},
		"http_json": {"url": "http://localhost:1/x"},
	}
	for _, typ := range panel.Types() {
		ctx := panel.PanelContext{
			ID:      "test-" + typ,
			Options: opts[typ],
			DataDir: t.TempDir(),
			Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		}
		p, err := panel.New(typ, ctx)
		if err != nil {
			t.Errorf("panel %q failed to construct: %v", typ, err)
			continue
		}
		// View must not panic at small and normal sizes.
		sp := panel.NewSafe(p, m0().th)
		_ = sp.View(panel.Size{Width: 5, Height: 2}, false)
		_ = sp.View(panel.Size{Width: 60, Height: 15}, true)
	}
}

// m0 returns a throwaway model for theme access in helper tests.
func m0() *Model {
	cfg := &config.Config{Theme: "default", Rows: []config.Row{{Panels: []config.PanelConfig{{Type: "clock"}}}}}
	m := &Model{cfg: cfg}
	m.applyTheme()
	return m
}

var _ = layout.Rect{}
