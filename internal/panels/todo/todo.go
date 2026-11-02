// Package todo is an interactive todo list panel backed by a swappable
// TodoProvider (local JSON by default, CalDAV optional).
package todo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/config"
	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/provider"
	"github.com/cmerk2021/panely/internal/theme"
)

func init() {
	panel.RegisterPanelWithDescription("todo", "Todo list (local or CalDAV)", New)
}

// Options configure the todo panel.
type Options struct {
	Provider string `yaml:"provider"` // default "local"
}

type opDoneMsg struct {
	id  string
	err error
}

// Todo is the todo panel.
type Todo struct {
	id      string
	th      theme.Theme
	prov    provider.TodoProvider
	todos   []provider.Todo
	cursor  int
	scroll  int
	loaded  bool
	loadErr error
	opErr   error
	stale   bool
	mode    string // "" | "add" | "edit"
	input   textinput.Model
	poller  *panel.Poller
	rowAt   []int // view line -> todo index for mouse
}

// InputCaptured reports whether a text input is active.
func (t *Todo) InputCaptured() bool { return t.mode != "" }

// New builds a todo panel.
func New(ctx panel.PanelContext) (panel.Panel, error) {
	opts := Options{Provider: "local"}
	if err := panel.DecodeOptionsLoose(ctx.Options, &opts); err != nil {
		return nil, err
	}
	prov, err := provider.NewTodoProvider(opts.Provider, provider.ProviderContext{
		Options: ctx.Options,
		Secrets: config.ExpandString,
		DataDir: ctx.DataDir,
	})
	if err != nil {
		return nil, fmt.Errorf("todo panel: %w", err)
	}
	ti := textinput.New()
	ti.Placeholder = "todo text  (!1-!3 priority, @YYYY-MM-DD due)"
	ti.Prompt = "> "
	t := &Todo{id: ctx.ID, th: ctx.Theme, prov: prov, input: ti}
	t.poller = &panel.Poller{
		PanelID:  ctx.ID,
		Interval: 30 * time.Second,
		Fetch: func(c context.Context) (any, error) {
			return prov.List(c)
		},
	}
	return t, nil
}

func (t *Todo) Init() tea.Cmd { return t.poller.Start() }

var tkeys = struct {
	up, down, toggle, add, edit, del, moveUp, moveDown, prio key.Binding
}{
	up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	toggle:   key.NewBinding(key.WithKeys(" ", "enter"), key.WithHelp("space", "toggle done")),
	add:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add todo")),
	edit:     key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "edit todo")),
	del:      key.NewBinding(key.WithKeys("d", "x"), key.WithHelp("d", "delete todo")),
	moveUp:   key.NewBinding(key.WithKeys("K", "shift+up"), key.WithHelp("K", "move up")),
	moveDown: key.NewBinding(key.WithKeys("J", "shift+down"), key.WithHelp("J", "move down")),
	prio:     key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "cycle priority")),
}

func (t *Todo) KeyBindings() []key.Binding {
	if t.mode != "" {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	}
	return []key.Binding{tkeys.up, tkeys.down, tkeys.toggle, tkeys.add, tkeys.edit, tkeys.del, tkeys.moveUp, tkeys.moveDown, tkeys.prio}
}

func (t *Todo) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
	pcmd, _ := t.poller.Update(msg)
	switch m := msg.(type) {
	case panel.DataMsg:
		if m.ForPanel(t.id) {
			t.loaded = true
			t.loadErr = m.Err
			t.stale = m.Stale
			if list, ok := m.Data.([]provider.Todo); ok {
				t.todos = list
				t.clampCursor()
			}
		}
		return t, pcmd
	case opDoneMsg:
		if m.id == t.id {
			t.opErr = m.err
			return t, t.poller.Refresh()
		}
		return t, pcmd
	case tea.KeyMsg:
		return t.handleKey(m, pcmd)
	case tea.MouseMsg:
		return t.handleMouse(m, pcmd)
	}
	if t.mode != "" {
		var cmd tea.Cmd
		t.input, cmd = t.input.Update(msg)
		return t, tea.Batch(pcmd, cmd)
	}
	return t, pcmd
}

func (t *Todo) handleKey(m tea.KeyMsg, pcmd tea.Cmd) (panel.Panel, tea.Cmd) {
	if t.mode != "" {
		switch m.String() {
		case "enter":
			text := strings.TrimSpace(t.input.Value())
			mode := t.mode
			t.mode = ""
			t.input.Blur()
			if text == "" {
				return t, pcmd
			}
			return t, tea.Batch(pcmd, t.commitInput(mode, text))
		case "esc":
			t.mode = ""
			t.input.Blur()
			return t, pcmd
		}
		var cmd tea.Cmd
		t.input, cmd = t.input.Update(m)
		return t, tea.Batch(pcmd, cmd)
	}
	switch {
	case key.Matches(m, tkeys.up):
		if t.cursor > 0 {
			t.cursor--
		}
	case key.Matches(m, tkeys.down):
		if t.cursor < len(t.todos)-1 {
			t.cursor++
		}
	case key.Matches(m, tkeys.toggle):
		return t, tea.Batch(pcmd, t.toggle(t.cursor))
	case key.Matches(m, tkeys.add):
		t.mode = "add"
		t.input.SetValue("")
		return t, tea.Batch(pcmd, t.input.Focus())
	case key.Matches(m, tkeys.edit):
		if cur := t.current(); cur != nil {
			t.mode = "edit"
			t.input.SetValue(cur.Text)
			return t, tea.Batch(pcmd, t.input.Focus())
		}
	case key.Matches(m, tkeys.del):
		if cur := t.current(); cur != nil {
			id, pid := cur.ID, t.id
			prov := t.prov
			return t, tea.Batch(pcmd, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				return opDoneMsg{id: pid, err: prov.Delete(ctx, id)}
			})
		}
	case key.Matches(m, tkeys.moveUp):
		return t, tea.Batch(pcmd, t.move(-1))
	case key.Matches(m, tkeys.moveDown):
		return t, tea.Batch(pcmd, t.move(1))
	case key.Matches(m, tkeys.prio):
		if cur := t.current(); cur != nil {
			up := *cur
			up.Priority = (up.Priority + 1) % 4
			return t, tea.Batch(pcmd, t.update(up))
		}
	}
	return t, pcmd
}

func (t *Todo) handleMouse(m tea.MouseMsg, pcmd tea.Cmd) (panel.Panel, tea.Cmd) {
	switch {
	case m.Action == tea.MouseActionPress && m.Button == tea.MouseButtonLeft:
		if m.Y >= 0 && m.Y < len(t.rowAt) && t.rowAt[m.Y] >= 0 {
			t.cursor = t.rowAt[m.Y]
			return t, tea.Batch(pcmd, t.toggle(t.cursor))
		}
	case m.Button == tea.MouseButtonWheelUp:
		if t.cursor > 0 {
			t.cursor--
		}
	case m.Button == tea.MouseButtonWheelDown:
		if t.cursor < len(t.todos)-1 {
			t.cursor++
		}
	}
	return t, pcmd
}

func (t *Todo) commitInput(mode, text string) tea.Cmd {
	item := provider.Todo{Text: text}
	if mode == "edit" {
		if cur := t.current(); cur != nil {
			item = *cur
		}
	}
	item.Text, item.Priority, item.Due = parseMeta(text, item.Priority, item.Due)
	prov, pid := t.prov, t.id
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var err error
		if mode == "add" {
			_, err = prov.Add(ctx, item)
		} else {
			err = prov.Update(ctx, item)
		}
		return opDoneMsg{id: pid, err: err}
	}
}

// parseMeta extracts "!N" priority and "@YYYY-MM-DD" due-date tokens.
func parseMeta(text string, prio int, due *time.Time) (string, int, *time.Time) {
	words := strings.Fields(text)
	var kept []string
	for _, w := range words {
		switch {
		case len(w) == 2 && w[0] == '!' && w[1] >= '1' && w[1] <= '3':
			prio = int(w[1] - '0')
		case strings.HasPrefix(w, "@"):
			if d, err := time.ParseInLocation("2006-01-02", w[1:], time.Local); err == nil {
				due = &d
			} else {
				kept = append(kept, w)
			}
		default:
			kept = append(kept, w)
		}
	}
	return strings.Join(kept, " "), prio, due
}

func (t *Todo) toggle(i int) tea.Cmd {
	if i < 0 || i >= len(t.todos) {
		return nil
	}
	item := t.todos[i]
	item.Done = !item.Done
	t.todos[i] = item // optimistic
	return t.update(item)
}

func (t *Todo) update(item provider.Todo) tea.Cmd {
	prov, pid := t.prov, t.id
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return opDoneMsg{id: pid, err: prov.Update(ctx, item)}
	}
}

func (t *Todo) move(dir int) tea.Cmd {
	j := t.cursor + dir
	if t.cursor < 0 || t.cursor >= len(t.todos) || j < 0 || j >= len(t.todos) {
		return nil
	}
	a, b := t.todos[t.cursor], t.todos[j]
	a.Order, b.Order = b.Order, a.Order
	t.todos[t.cursor], t.todos[j] = b, a
	t.cursor = j
	prov, pid := t.prov, t.id
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := prov.Update(ctx, a); err != nil {
			return opDoneMsg{id: pid, err: err}
		}
		return opDoneMsg{id: pid, err: prov.Update(ctx, b)}
	}
}

func (t *Todo) current() *provider.Todo {
	if t.cursor < 0 || t.cursor >= len(t.todos) {
		return nil
	}
	cp := t.todos[t.cursor]
	return &cp
}

func (t *Todo) clampCursor() {
	if t.cursor >= len(t.todos) {
		t.cursor = len(t.todos) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
}

func prioMark(th theme.Theme, p int) string {
	switch p {
	case 3:
		return th.ErrorText().Render("!!!")
	case 2:
		return th.WarningText().Render("!! ")
	case 1:
		return th.InfoText().Render("!  ")
	default:
		return "   "
	}
}

func (t *Todo) View(size panel.Size, focused bool) string {
	if !t.loaded {
		return panel.LoadingView(t.th, size, "todos")
	}
	if t.loadErr != nil && len(t.todos) == 0 && !t.stale {
		return panel.ErrorView(t.th, size, t.loadErr.Error())
	}
	t.rowAt = make([]int, size.Height)
	for i := range t.rowAt {
		t.rowAt[i] = -1
	}
	var lines []string
	line := 0
	if t.mode != "" {
		t.input.Width = size.Width - 4
		lines = append(lines, t.input.View())
		line++
	}
	listHeight := size.Height - line
	if t.stale || t.opErr != nil {
		listHeight--
	}
	if len(t.todos) == 0 {
		if t.mode == "" {
			return panel.EmptyView(t.th, size, "no todos\npress a to add one")
		}
	}
	// Scroll window.
	if t.cursor < t.scroll {
		t.scroll = t.cursor
	}
	if t.cursor >= t.scroll+listHeight {
		t.scroll = t.cursor - listHeight + 1
	}
	end := t.scroll + listHeight
	if end > len(t.todos) {
		end = len(t.todos)
	}
	for i := t.scroll; i < end; i++ {
		item := t.todos[i]
		box := "☐"
		style := t.th.Text()
		if item.Done {
			box = "☑"
			style = t.th.MutedText().Strikethrough(true)
		}
		cursor := "  "
		if i == t.cursor && focused && t.mode == "" {
			cursor = t.th.AccentText().Render("▸ ")
		}
		txt := item.Text
		if item.Due != nil {
			dueStyle := t.th.MutedText()
			if !item.Done && item.Due.Before(time.Now()) {
				dueStyle = t.th.ErrorText()
			}
			txt += " " + dueStyle.Render("("+item.Due.Format("Jan 2")+")")
		}
		row := cursor + prioMark(t.th, item.Priority) + " " + style.Render(box+" ") + style.Render(txt)
		if line < size.Height {
			t.rowAt[line] = i
		}
		lines = append(lines, row)
		line++
	}
	if t.opErr != nil {
		lines = append(lines, t.th.ErrorText().Render("✗ "+truncate(t.opErr.Error(), size.Width-2)))
	} else if t.stale {
		lines = append(lines, panel.StaleBadge(t.th))
	}
	return lipgloss.NewStyle().MaxWidth(size.Width).MaxHeight(size.Height).Render(strings.Join(lines, "\n"))
}

func truncate(s string, n int) string {
	if n < 1 {
		return ""
	}
	if len(s) > n {
		return s[:n]
	}
	return s
}
