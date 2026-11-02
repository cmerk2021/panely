package panel

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/theme"
)

// Safe wraps a panel so a panic in Update or View is caught and rendered as
// an error box instead of crashing the app.
type Safe struct {
	Inner Panel
	Theme theme.Theme
	err   error
}

// NewSafe wraps p in a panic-recovering shell.
func NewSafe(p Panel, th theme.Theme) *Safe { return &Safe{Inner: p, Theme: th} }

func (s *Safe) Init() (cmd tea.Cmd) {
	defer s.recover("Init")
	return s.Inner.Init()
}

func (s *Safe) Update(msg tea.Msg) (out Panel, cmd tea.Cmd) {
	out = s
	defer s.recover("Update")
	inner, cmd := s.Inner.Update(msg)
	s.Inner = inner
	return s, cmd
}

func (s *Safe) View(size Size, focused bool) (view string) {
	if s.err != nil {
		return ErrorView(s.Theme, size, fmt.Sprintf("panel crashed: %v", s.err))
	}
	defer func() {
		if r := recover(); r != nil {
			s.err = fmt.Errorf("%v", r)
			view = ErrorView(s.Theme, size, fmt.Sprintf("panel crashed: %v", s.err))
		}
	}()
	return s.Inner.View(size, focused)
}

func (s *Safe) KeyBindings() []key.Binding {
	if s.err != nil {
		return nil
	}
	return s.Inner.KeyBindings()
}

// Refresh forwards to the inner panel if it supports manual refresh, and
// clears a previous crash so the panel gets another chance.
func (s *Safe) Refresh() tea.Cmd {
	s.err = nil
	if r, ok := s.Inner.(Refresher); ok {
		return r.Refresh()
	}
	return nil
}

// InputCaptured forwards to the inner panel.
func (s *Safe) InputCaptured() bool {
	if s.err != nil {
		return false
	}
	if c, ok := s.Inner.(InputCapturer); ok {
		return c.InputCaptured()
	}
	return false
}

func (s *Safe) recover(where string) {
	if r := recover(); r != nil {
		s.err = fmt.Errorf("%s panicked: %v", where, r)
	}
}

// Shared state views so every panel's loading/error/empty rendering matches.

// LoadingView renders a consistent loading state.
func LoadingView(th theme.Theme, size Size, what string) string {
	msg := th.MutedText().Render("⋯ loading " + what)
	return center(msg, size)
}

// ErrorView renders a consistent error state (message is wrapped).
func ErrorView(th theme.Theme, size Size, msg string) string {
	txt := th.ErrorText().Render("✗ ") + th.MutedText().Render(truncateLines(msg, size.Width-3, max(1, size.Height)))
	return lipgloss.NewStyle().Width(size.Width).MaxHeight(size.Height).Render(txt)
}

// EmptyView renders a consistent empty state.
func EmptyView(th theme.Theme, size Size, msg string) string {
	return center(th.MutedText().Render(msg), size)
}

// StaleBadge returns a small indicator appended when data is stale.
func StaleBadge(th theme.Theme) string {
	return th.WarningText().Render(" ⟳ stale")
}

func center(s string, size Size) string {
	return lipgloss.Place(size.Width, size.Height, lipgloss.Center, lipgloss.Center, s)
}

func truncateLines(s string, width, maxLines int) string {
	if width < 4 {
		width = 4
	}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		for len(line) > width {
			out = append(out, line[:width])
			line = line[width:]
		}
		out = append(out, line)
	}
	if len(out) > maxLines {
		out = out[:maxLines]
	}
	return strings.Join(out, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
