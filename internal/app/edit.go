package app

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cmerk2021/panely/internal/config"
	"github.com/cmerk2021/panely/internal/panel"
)

// rcOf maps the flat focus index to (row, col) in the config.
func (m *Model) rcOf(idx int) (int, int, bool) {
	i := 0
	for ri, row := range m.cfg.Rows {
		for pi := range row.Panels {
			if i == idx {
				return ri, pi, true
			}
			i++
		}
	}
	return 0, 0, false
}

func (m *Model) flatIndexOf(ri, pi int) int {
	i := 0
	for r, row := range m.cfg.Rows {
		for p := range row.Panels {
			if r == ri && p == pi {
				return i
			}
			i++
		}
	}
	return 0
}

func (m *Model) handleEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.picking {
		return m.handlePickerKey(msg)
	}
	if m.confirmDel {
		switch msg.String() {
		case "y", "Y":
			m.confirmDel = false
			return m, m.deleteFocused()
		default:
			m.confirmDel = false
			m.setStatus("delete canceled", false)
			return m, nil
		}
	}
	ri, pi, ok := m.rcOf(m.focus)
	if !ok && msg.String() != "esc" && msg.String() != "q" && msg.String() != "A" {
		return m, nil
	}
	switch msg.String() {
	case "esc", "q":
		m.editMode = false
		m.setStatus("left edit mode", false)
		return m, nil
	case "h", "left":
		return m, m.movePanelInRow(ri, pi, -1)
	case "l", "right":
		return m, m.movePanelInRow(ri, pi, +1)
	case "k", "up":
		return m, m.movePanelAcrossRows(ri, pi, -1)
	case "j", "down":
		return m, m.movePanelAcrossRows(ri, pi, +1)
	case "+", "=":
		return m, m.adjustWeight(ri, pi, +1)
	case "-", "_":
		return m, m.adjustWeight(ri, pi, -1)
	case "]":
		return m, m.adjustRowHeight(ri, +5)
	case "[":
		return m, m.adjustRowHeight(ri, -5)
	case "a":
		m.picking = true
		m.pickCursor = 0
		return m, nil
	case "A":
		return m, m.addRow()
	case "d", "x":
		m.confirmDel = true
		m.setStatus("delete this panel? (y/n)", false)
		return m, nil
	case "tab":
		m.moveFocus(+1)
		return m, nil
	case "shift+tab":
		m.moveFocus(-1)
		return m, nil
	}
	return m, nil
}

func (m *Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	types := panel.Types()
	switch msg.String() {
	case "esc", "q":
		m.picking = false
		return m, nil
	case "up", "k":
		if m.pickCursor > 0 {
			m.pickCursor--
		}
	case "down", "j":
		if m.pickCursor < len(types)-1 {
			m.pickCursor++
		}
	case "enter":
		m.picking = false
		return m, m.addPanel(types[m.pickCursor])
	}
	return m, nil
}

// applyLayoutChange saves the config and rebuilds panels after an edit.
func (m *Model) applyLayoutChange(newFocusRow, newFocusCol int) tea.Cmd {
	m.saveConfig()
	cmds := m.buildPanels()
	m.focus = m.flatIndexOf(newFocusRow, newFocusCol)
	return tea.Batch(cmds...)
}

func (m *Model) movePanelInRow(ri, pi, dir int) tea.Cmd {
	row := &m.cfg.Rows[ri]
	j := pi + dir
	if j < 0 || j >= len(row.Panels) {
		return nil
	}
	row.Panels[pi], row.Panels[j] = row.Panels[j], row.Panels[pi]
	return m.applyLayoutChange(ri, j)
}

// movePanelAcrossRows moves a panel up or down a row, creating a new row at
// the edges when needed and dropping rows that become empty.
func (m *Model) movePanelAcrossRows(ri, pi, dir int) tea.Cmd {
	rows := m.cfg.Rows
	pc := rows[ri].Panels[pi]
	target := ri + dir
	// Remove from the current row.
	rows[ri].Panels = append(rows[ri].Panels[:pi], rows[ri].Panels[pi+1:]...)
	emptied := len(rows[ri].Panels) == 0
	switch {
	case target < 0:
		newRow := config.Row{Panels: []config.PanelConfig{pc}}
		rows = append([]config.Row{newRow}, rows...)
		target = 0
	case target >= len(rows):
		rows = append(rows, config.Row{Panels: []config.PanelConfig{pc}})
		target = len(rows) - 1
	default:
		rows[target].Panels = append(rows[target].Panels, pc)
	}
	if emptied {
		// Drop the now-empty source row.
		src := ri
		if target < src {
			src++ // a row was prepended above
		}
		rows = append(rows[:src], rows[src+1:]...)
		if target > src {
			target--
		}
	}
	m.cfg.Rows = rows
	return m.applyLayoutChange(target, len(m.cfg.Rows[target].Panels)-1)
}

func (m *Model) adjustWeight(ri, pi, delta int) tea.Cmd {
	p := &m.cfg.Rows[ri].Panels[pi]
	w := p.EffectiveWeight() + delta
	if w < 1 {
		w = 1
	}
	if w > 12 {
		w = 12
	}
	p.Weight = w
	m.setStatus("weight: "+strconv.Itoa(w), false)
	return m.applyLayoutChange(ri, pi)
}

func (m *Model) adjustRowHeight(ri, deltaPct int) tea.Cmd {
	row := &m.cfg.Rows[ri]
	cur := 0
	if strings.HasSuffix(row.Height, "%") {
		cur, _ = strconv.Atoi(strings.TrimSuffix(row.Height, "%"))
	}
	if cur == 0 {
		cur = 100 / maxInt(1, len(m.cfg.Rows)) // approximate auto share
	}
	cur += deltaPct
	if cur < 10 {
		cur = 10
	}
	if cur > 90 {
		cur = 90
	}
	row.Height = strconv.Itoa(cur) + "%"
	m.setStatus("row height: "+row.Height, false)
	_, pi, _ := m.rcOf(m.focus)
	return m.applyLayoutChange(ri, pi)
}

func (m *Model) addPanel(typeName string) tea.Cmd {
	pc := config.PanelConfig{Type: typeName}
	if len(m.cfg.Rows) == 0 {
		m.cfg.Rows = []config.Row{{Panels: []config.PanelConfig{pc}}}
		return m.applyLayoutChange(0, 0)
	}
	ri, pi, ok := m.rcOf(m.focus)
	if !ok {
		ri, pi = 0, len(m.cfg.Rows[0].Panels)-1
	}
	row := &m.cfg.Rows[ri]
	row.Panels = append(row.Panels, config.PanelConfig{})
	copy(row.Panels[pi+2:], row.Panels[pi+1:])
	row.Panels[pi+1] = pc
	m.setStatus("added "+typeName+" panel", false)
	return m.applyLayoutChange(ri, pi+1)
}

func (m *Model) addRow() tea.Cmd {
	ri, _, _ := m.rcOf(m.focus)
	newRow := config.Row{Panels: []config.PanelConfig{{Type: "clock"}}}
	rows := m.cfg.Rows
	rows = append(rows, config.Row{})
	copy(rows[ri+2:], rows[ri+1:])
	rows[ri+1] = newRow
	m.cfg.Rows = rows
	m.setStatus("added row", false)
	return m.applyLayoutChange(ri+1, 0)
}

func (m *Model) deleteFocused() tea.Cmd {
	ri, pi, ok := m.rcOf(m.focus)
	if !ok {
		return nil
	}
	row := &m.cfg.Rows[ri]
	row.Panels = append(row.Panels[:pi], row.Panels[pi+1:]...)
	if len(row.Panels) == 0 {
		m.cfg.Rows = append(m.cfg.Rows[:ri], m.cfg.Rows[ri+1:]...)
		if ri >= len(m.cfg.Rows) {
			ri = len(m.cfg.Rows) - 1
		}
		pi = 0
	} else if pi >= len(row.Panels) {
		pi = len(row.Panels) - 1
	}
	m.setStatus("panel deleted", false)
	if len(m.cfg.Rows) == 0 {
		m.saveConfig()
		m.panels = nil
		m.rects = nil
		m.focus = 0
		return nil
	}
	return m.applyLayoutChange(ri, pi)
}
