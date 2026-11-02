package app

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/theme"
)

// Global keybindings.
var gkeys = struct {
	quit, help, edit, themeCycle, refresh                 key.Binding
	next, prev, focusLeft, focusRight, focusUp, focusDown key.Binding
}{
	quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
	edit:       key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit layout")),
	themeCycle: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "cycle theme")),
	refresh:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh panel")),
	next:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next panel")),
	prev:       key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev panel")),
	focusLeft:  key.NewBinding(key.WithKeys("ctrl+h", "alt+left"), key.WithHelp("alt+←", "focus left")),
	focusRight: key.NewBinding(key.WithKeys("ctrl+l", "alt+right"), key.WithHelp("alt+→", "focus right")),
	focusUp:    key.NewBinding(key.WithKeys("ctrl+k", "alt+up"), key.WithHelp("alt+↑", "focus up")),
	focusDown:  key.NewBinding(key.WithKeys("ctrl+j", "alt+down"), key.WithHelp("alt+↓", "focus down")),
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// While a panel captures text input, only forward (panel handles esc).
	if m.inputCaptured() && !m.editMode {
		return m, m.forwardToFocused(msg)
	}
	if m.editMode {
		return m.handleEditKey(msg)
	}
	switch {
	case key.Matches(msg, gkeys.quit):
		m.watcher.Close()
		return m, tea.Quit
	case key.Matches(msg, gkeys.help):
		m.showHelp = !m.showHelp
		m.help.ShowAll = m.showHelp
		return m, nil
	case key.Matches(msg, gkeys.edit):
		m.editMode = true
		m.setStatus("EDIT MODE — move h/j/k/l · weight +/- · row height [/] · a add · d delete · esc done", false)
		return m, nil
	case key.Matches(msg, gkeys.themeCycle):
		return m, m.cycleTheme()
	case key.Matches(msg, gkeys.refresh):
		if p := m.focusedPanel(); p != nil {
			return m, p.Refresh()
		}
		return m, nil
	case key.Matches(msg, gkeys.next):
		m.moveFocus(+1)
		return m, nil
	case key.Matches(msg, gkeys.prev):
		m.moveFocus(-1)
		return m, nil
	case key.Matches(msg, gkeys.focusLeft):
		m.moveFocusSpatial(-1, 0)
		return m, nil
	case key.Matches(msg, gkeys.focusRight):
		m.moveFocusSpatial(+1, 0)
		return m, nil
	case key.Matches(msg, gkeys.focusUp):
		m.moveFocusSpatial(0, -1)
		return m, nil
	case key.Matches(msg, gkeys.focusDown):
		m.moveFocusSpatial(0, +1)
		return m, nil
	}
	// Spatial focus with bare arrows/hjkl only if the focused panel does not
	// claim them for itself.
	if !m.panelWantsKey(msg) {
		switch msg.String() {
		case "left", "h":
			m.moveFocusSpatial(-1, 0)
			return m, nil
		case "right", "l":
			m.moveFocusSpatial(+1, 0)
			return m, nil
		case "up", "k":
			m.moveFocusSpatial(0, -1)
			return m, nil
		case "down", "j":
			m.moveFocusSpatial(0, +1)
			return m, nil
		}
	}
	return m, m.forwardToFocused(msg)
}

// panelWantsKey reports whether the focused panel binds this key itself.
func (m *Model) panelWantsKey(msg tea.KeyMsg) bool {
	p := m.focusedPanel()
	if p == nil {
		return false
	}
	for _, b := range p.KeyBindings() {
		if key.Matches(msg, b) {
			return true
		}
	}
	return false
}

func (m *Model) forwardToFocused(msg tea.Msg) tea.Cmd {
	p := m.focusedPanel()
	if p == nil {
		return nil
	}
	np, cmd := p.Update(msg)
	if sp, ok := np.(*panel.Safe); ok {
		m.panels[m.focus] = sp
	}
	return cmd
}

func (m *Model) moveFocus(delta int) {
	if len(m.panels) == 0 {
		return
	}
	m.focus = (m.focus + delta + len(m.panels)) % len(m.panels)
}

// moveFocusSpatial moves focus to the nearest panel in direction (dx, dy)
// using panel rect centers.
func (m *Model) moveFocusSpatial(dx, dy int) {
	if m.focus < 0 || m.focus >= len(m.rects) || len(m.rects) == 0 {
		return
	}
	cur := m.rects[m.focus]
	cx, cy := cur.X+cur.Width/2, cur.Y+cur.Height/2
	best, bestDist := -1, 1<<30
	for i, r := range m.rects {
		if i == m.focus {
			continue
		}
		rx, ry := r.X+r.Width/2, r.Y+r.Height/2
		ddx, ddy := rx-cx, ry-cy
		// Must be in the requested direction.
		if dx > 0 && ddx <= 0 || dx < 0 && ddx >= 0 || dy > 0 && ddy <= 0 || dy < 0 && ddy >= 0 {
			continue
		}
		// Weight the orthogonal axis heavily to prefer aligned panels.
		var dist int
		if dx != 0 {
			dist = abs(ddx) + 4*abs(ddy)
		} else {
			dist = abs(ddy) + 4*abs(ddx)
		}
		if dist < bestDist {
			best, bestDist = i, dist
		}
	}
	if best >= 0 {
		m.focus = best
	}
}

func (m *Model) cycleTheme() tea.Cmd {
	names := theme.Names(m.cfg)
	cur := 0
	for i, n := range names {
		if n == m.cfg.Theme {
			cur = i
			break
		}
	}
	m.cfg.Theme = names[(cur+1)%len(names)]
	m.applyTheme()
	m.saveConfig()
	m.setStatus("theme: "+m.cfg.Theme, false)
	// Rebuild panels so they pick up the new theme.
	return tea.Batch(m.buildPanels()...)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
