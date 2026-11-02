// Package notes is a persistent scratchpad with an edit mode backed by
// bubbles/textarea. Content is saved atomically to the data directory.
package notes

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/storage"
	"github.com/cmerk2021/panely/internal/theme"
)

func init() {
	panel.RegisterPanelWithDescription("notes", "Persistent scratchpad", New)
}

// Options configure the notes panel.
type Options struct {
	File string `yaml:"file"` // file name inside the data dir
}

// Notes is the notes panel.
type Notes struct {
	id      string
	th      theme.Theme
	path    string
	content string
	editing bool
	ta      textarea.Model
	saveErr error
}

// InputCaptured reports whether the panel is capturing text input, so
// global keybindings are suspended.
func (n *Notes) InputCaptured() bool { return n.editing }

// New builds a notes panel.
func New(ctx panel.PanelContext) (panel.Panel, error) {
	opts := Options{File: "notes.txt"}
	if err := panel.DecodeOptions(ctx.Options, &opts); err != nil {
		return nil, err
	}
	path := filepath.Join(ctx.DataDir, opts.File)
	content := ""
	if data, err := os.ReadFile(path); err == nil {
		content = string(data)
	}
	ta := textarea.New()
	ta.Placeholder = "type your notes..."
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.SetValue(content)
	return &Notes{id: ctx.ID, th: ctx.Theme, path: path, content: content, ta: ta}, nil
}

func (n *Notes) Init() tea.Cmd { return nil }

var nkeys = struct {
	edit, save, cancel key.Binding
}{
	edit:   key.NewBinding(key.WithKeys("enter", "i"), key.WithHelp("enter/i", "edit notes")),
	save:   key.NewBinding(key.WithKeys("ctrl+s", "esc"), key.WithHelp("esc", "save & stop editing")),
	cancel: key.NewBinding(key.WithKeys("ctrl+q"), key.WithHelp("ctrl+q", "discard edits")),
}

func (n *Notes) KeyBindings() []key.Binding {
	if n.editing {
		return []key.Binding{nkeys.save, nkeys.cancel}
	}
	return []key.Binding{nkeys.edit}
}

func (n *Notes) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
	if m, ok := msg.(tea.KeyMsg); ok {
		if n.editing {
			switch {
			case key.Matches(m, nkeys.save):
				n.content = n.ta.Value()
				n.editing = false
				n.ta.Blur()
				n.saveErr = storage.WriteFileAtomic(n.path, []byte(n.content), 0o644)
				return n, nil
			case key.Matches(m, nkeys.cancel):
				n.ta.SetValue(n.content)
				n.editing = false
				n.ta.Blur()
				return n, nil
			}
			var cmd tea.Cmd
			n.ta, cmd = n.ta.Update(msg)
			return n, cmd
		}
		if key.Matches(m, nkeys.edit) {
			n.editing = true
			return n, n.ta.Focus()
		}
	}
	if n.editing {
		var cmd tea.Cmd
		n.ta, cmd = n.ta.Update(msg)
		return n, cmd
	}
	return n, nil
}

func (n *Notes) View(size panel.Size, focused bool) string {
	if n.editing {
		n.ta.SetWidth(size.Width)
		n.ta.SetHeight(maxInt(1, size.Height-1))
		hint := n.th.MutedText().Render("esc save · ctrl+q discard")
		return lipgloss.JoinVertical(lipgloss.Left, n.ta.View(), hint)
	}
	if strings.TrimSpace(n.content) == "" {
		msg := "empty scratchpad"
		if focused {
			msg += "\npress enter to edit"
		}
		return panel.EmptyView(n.th, size, msg)
	}
	lines := strings.Split(strings.TrimRight(n.content, "\n"), "\n")
	if len(lines) > size.Height {
		lines = lines[:size.Height]
	}
	body := n.th.Text().Render(strings.Join(lines, "\n"))
	if n.saveErr != nil {
		body += "\n" + n.th.ErrorText().Render("✗ save failed")
	}
	return lipgloss.NewStyle().MaxWidth(size.Width).MaxHeight(size.Height).Render(body)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
