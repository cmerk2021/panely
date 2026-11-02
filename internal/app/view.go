package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/layout"
	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/theme"
)

// View renders the whole dashboard.
func (m *Model) View() string {
	if m.width < layout.MinTerminalWidth || m.height < layout.MinTerminalHeight {
		return m.th.MutedText().Render("terminal too small for " + "panely" + " — please enlarge")
	}
	if m.rects == nil || len(m.panels) == 0 {
		hint := "no panels configured\npress e to enter edit mode and add panels"
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.th.MutedText().Render(hint))
	}
	// Render each panel box, grouped into row strips.
	var rowViews []string
	var strip []string
	curRow := -1
	flushRow := func() {
		if len(strip) > 0 {
			rowViews = append(rowViews, lipgloss.JoinHorizontal(lipgloss.Top, strip...))
			strip = nil
		}
	}
	for i, r := range m.rects {
		if r.Row != curRow {
			flushRow()
			curRow = r.Row
		}
		strip = append(strip, m.renderPanelBox(i, r))
	}
	flushRow()
	dash := lipgloss.JoinVertical(lipgloss.Left, rowViews...)
	bar := m.statusBarView()
	out := lipgloss.JoinVertical(lipgloss.Left, dash, bar)
	if m.picking {
		out = m.overlayPicker()
	}
	return out
}

// renderPanelBox draws one panel with a rounded border and inline title.
func (m *Model) renderPanelBox(idx int, r layout.Rect) string {
	focused := idx == m.focus
	title := m.panelTitle(r)
	if r.Width < 4 || r.Height < 3 {
		// Too small for a border; fill with spaces.
		return lipgloss.NewStyle().Width(r.Width).Height(r.Height).Render("")
	}
	innerW, innerH := r.Width-2, r.Height-2
	var content string
	if r.TooSmall {
		content = m.th.MutedText().Render("too small")
	} else if idx < len(m.panels) {
		content = m.panels[idx].View(panel.Size{Width: innerW, Height: innerH}, focused)
	}
	borderColor := m.th.Border
	if focused {
		borderColor = m.th.BorderFocused
	}
	if m.editMode && focused {
		borderColor = m.th.Warning
	}
	bs := lipgloss.NewStyle().Foreground(borderColor)
	ts := m.th.Title(focused)

	// Top border with embedded title.
	maxTitle := innerW - 4
	if maxTitle < 0 {
		maxTitle = 0
	}
	if len(title) > maxTitle {
		title = clipRunes(title, maxTitle)
	}
	var top string
	if title != "" {
		rest := innerW - lipgloss.Width(title) - 3
		if rest < 0 {
			rest = 0
		}
		top = bs.Render("╭─ ") + ts.Render(title) + bs.Render(" "+strings.Repeat("─", rest)+"╮")
	} else {
		top = bs.Render("╭" + strings.Repeat("─", innerW) + "╮")
	}
	bottom := bs.Render("╰" + strings.Repeat("─", innerW) + "╯")

	// Body lines padded to exact inner size.
	body := lipgloss.NewStyle().
		Width(innerW).Height(innerH).
		MaxWidth(innerW).MaxHeight(innerH).
		Render(content)
	lines := strings.Split(body, "\n")
	for len(lines) < innerH {
		lines = append(lines, strings.Repeat(" ", innerW))
	}
	side := bs.Render("│")
	var sb strings.Builder
	sb.WriteString(top)
	for i := 0; i < innerH; i++ {
		sb.WriteString("\n")
		line := lines[i]
		if w := lipgloss.Width(line); w < innerW {
			line += strings.Repeat(" ", innerW-w)
		}
		sb.WriteString(side + line + side)
	}
	sb.WriteString("\n" + bottom)
	return sb.String()
}

func (m *Model) panelTitle(r layout.Rect) string {
	if r.Row >= len(m.cfg.Rows) || r.Col >= len(m.cfg.Rows[r.Row].Panels) {
		return ""
	}
	pc := m.cfg.Rows[r.Row].Panels[r.Col]
	if pc.Title != "" {
		return pc.Title
	}
	return pc.Type
}

func (m *Model) statusBarView() string {
	var left string
	switch {
	case m.confirmDel:
		left = m.th.WarningText().Bold(true).Render("delete this panel? (y/n)")
	case m.editMode:
		left = m.th.WarningText().Bold(true).Render(" EDIT MODE ") + " " +
			m.th.MutedText().Render("h/j/k/l move · +/- weight · [/] row height · a add panel · A add row · d delete · esc done")
	case m.showHelp:
		left = m.help.View(m.keyMap())
	default:
		left = m.help.ShortHelpView(m.shortHelp())
	}
	right := ""
	if m.status != "" {
		st := m.th.MutedText()
		if m.statusErr {
			st = m.th.ErrorText()
		}
		right = st.Render(clipRunes(m.status, m.width/2))
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return lipgloss.NewStyle().MaxWidth(m.width).Render(left)
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) shortHelp() []key.Binding {
	bindings := []key.Binding{gkeys.help, gkeys.next, gkeys.edit, gkeys.themeCycle, gkeys.quit}
	if p := m.focusedPanel(); p != nil {
		pb := p.KeyBindings()
		if len(pb) > 3 {
			pb = pb[:3]
		}
		bindings = append(pb, bindings...)
	}
	return bindings
}

// keyMap adapts global + panel bindings to the bubbles/help interface.
type keyMap struct {
	global []key.Binding
	panel  []key.Binding
}

func (k keyMap) ShortHelp() []key.Binding { return append(k.panel, k.global...) }
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.global, k.panel}
}

func (m *Model) keyMap() keyMap {
	km := keyMap{
		global: []key.Binding{
			gkeys.next, gkeys.prev, gkeys.focusLeft, gkeys.focusRight, gkeys.focusUp, gkeys.focusDown,
			gkeys.edit, gkeys.themeCycle, gkeys.refresh, gkeys.help, gkeys.quit,
		},
	}
	if p := m.focusedPanel(); p != nil {
		km.panel = p.KeyBindings()
	}
	return km
}

// overlayPicker draws the add-panel picker centered on the screen.
func (m *Model) overlayPicker() string {
	types := panel.Types()
	var rows []string
	rows = append(rows, m.th.AccentText().Bold(true).Render("Add panel"))
	for i, t := range types {
		desc := panel.Description(t)
		cursor := "  "
		style := m.th.Text()
		if i == m.pickCursor {
			cursor = m.th.AccentText().Render("▸ ")
			style = m.th.AccentText().Bold(true)
		}
		line := cursor + style.Render(fmt.Sprintf("%-10s", t)) + " " + m.th.MutedText().Render(desc)
		rows = append(rows, line)
	}
	rows = append(rows, m.th.MutedText().Render("enter add · esc cancel"))
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.th.BorderFocused).
		Padding(0, 1).
		Render(strings.Join(rows, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.picking || m.confirmDel {
		return m, nil
	}
	target := layout.HitTest(m.rects, msg.X, msg.Y)
	if target < 0 {
		return m, nil
	}
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		m.focus = target
	}
	// Translate to panel-local coordinates (inside the border).
	r := m.rects[target]
	local := msg
	local.X = msg.X - r.X - 1
	local.Y = msg.Y - r.Y - 1
	if local.X < 0 || local.Y < 0 || local.X >= r.Width-2 || local.Y >= r.Height-2 {
		return m, nil // border click
	}
	if target >= len(m.panels) {
		return m, nil
	}
	np, cmd := m.panels[target].Update(local)
	if sp, ok := np.(*panel.Safe); ok {
		m.panels[target] = sp
	}
	return m, cmd
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		if n < 1 {
			return ""
		}
		return string(r[:n-1]) + "…"
	}
	return s
}

var _ = theme.Theme{} // keep import if unused in some builds
