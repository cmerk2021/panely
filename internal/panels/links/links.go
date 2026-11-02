// Package links is a quick-launch panel: a list of URLs opened via the OS
// opener (xdg-open, open, start), with optional groups and icons.
package links

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/theme"
)

func init() {
	panel.RegisterPanelWithDescription("links", "Quick-launch links", New)
}

// Link is one entry.
type Link struct {
	Name  string `yaml:"name"`
	URL   string `yaml:"url"`
	Icon  string `yaml:"icon"`
	Group string `yaml:"group"`
}

// Options configure the links panel.
type Options struct {
	Links []Link `yaml:"links"`
}

type openedMsg struct {
	id  string
	err error
}

// Links is the links panel.
type Links struct {
	id      string
	th      theme.Theme
	links   []Link
	cursor  int
	lastErr error
	rowAt   []int // view line -> link index for mouse clicks
	scroll  int
}

// New builds a links panel.
func New(ctx panel.PanelContext) (panel.Panel, error) {
	var opts Options
	if err := panel.DecodeOptions(ctx.Options, &opts); err != nil {
		return nil, err
	}
	return &Links{id: ctx.ID, th: ctx.Theme, links: opts.Links}, nil
}

func (l *Links) Init() tea.Cmd { return nil }

var keys = struct {
	up, down, open key.Binding
}{
	up:   key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "prev link")),
	down: key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "next link")),
	open: key.NewBinding(key.WithKeys("enter", "o"), key.WithHelp("enter/o", "open link")),
}

func (l *Links) KeyBindings() []key.Binding {
	return []key.Binding{keys.up, keys.down, keys.open}
}

func (l *Links) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(m, keys.up):
			if l.cursor > 0 {
				l.cursor--
			}
		case key.Matches(m, keys.down):
			if l.cursor < len(l.links)-1 {
				l.cursor++
			}
		case key.Matches(m, keys.open):
			return l, l.open(l.cursor)
		}
	case tea.MouseMsg:
		if m.Action == tea.MouseActionPress && m.Button == tea.MouseButtonLeft {
			if m.Y >= 0 && m.Y < len(l.rowAt) && l.rowAt[m.Y] >= 0 {
				l.cursor = l.rowAt[m.Y]
				return l, l.open(l.cursor)
			}
		}
		if m.Button == tea.MouseButtonWheelUp && l.cursor > 0 {
			l.cursor--
		}
		if m.Button == tea.MouseButtonWheelDown && l.cursor < len(l.links)-1 {
			l.cursor++
		}
	case openedMsg:
		if m.id == l.id {
			l.lastErr = m.err
		}
	}
	return l, nil
}

func (l *Links) open(i int) tea.Cmd {
	if i < 0 || i >= len(l.links) {
		return nil
	}
	url := l.links[i].URL
	id := l.id
	return func() tea.Msg {
		return openedMsg{id: id, err: OpenURL(url)}
	}
}

// OpenURL opens a URL with the platform opener.
func OpenURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open %s: %w", url, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func (l *Links) View(size panel.Size, focused bool) string {
	if len(l.links) == 0 {
		return panel.EmptyView(l.th, size, "no links configured\nadd options.links in config")
	}
	l.rowAt = make([]int, size.Height)
	for i := range l.rowAt {
		l.rowAt[i] = -1
	}
	// Keep cursor visible.
	if l.cursor < l.scroll {
		l.scroll = l.cursor
	}

	var lines []string
	lineNo := 0
	lastGroup := ""
	type rowRef struct {
		text string
		link int
	}
	var rows []rowRef
	for i, lk := range l.links {
		if lk.Group != "" && lk.Group != lastGroup {
			rows = append(rows, rowRef{text: l.th.MutedText().Bold(true).Render(lk.Group), link: -1})
			lastGroup = lk.Group
		}
		icon := lk.Icon
		if icon == "" {
			icon = "↗"
		}
		label := fmt.Sprintf("%s %s", icon, lk.Name)
		style := l.th.Text()
		prefix := "  "
		if i == l.cursor && focused {
			style = l.th.AccentText().Bold(true)
			prefix = l.th.AccentText().Render("▸ ")
		}
		rows = append(rows, rowRef{text: prefix + style.Render(label), link: i})
	}
	// Scroll so the cursor's row is visible.
	cursorRow := 0
	for ri, r := range rows {
		if r.link == l.cursor {
			cursorRow = ri
		}
	}
	start := 0
	if cursorRow >= size.Height {
		start = cursorRow - size.Height + 1
	}
	for ri := start; ri < len(rows) && lineNo < size.Height; ri++ {
		if rows[ri].link >= 0 {
			l.rowAt[lineNo] = rows[ri].link
		}
		lines = append(lines, rows[ri].text)
		lineNo++
	}
	out := lipgloss.JoinVertical(lipgloss.Left, lines...)
	if l.lastErr != nil {
		out += "\n" + l.th.ErrorText().Render("✗ open failed")
	}
	return lipgloss.NewStyle().MaxWidth(size.Width).MaxHeight(size.Height).Render(out)
}
