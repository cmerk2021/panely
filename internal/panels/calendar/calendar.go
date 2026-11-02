// Package calendar renders a month view with today highlighted plus an
// agenda of upcoming events from a CalendarProvider (ICS files/URLs today,
// CalDAV later via the provider registry).
package calendar

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/config"
	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/provider"
	"github.com/cmerk2021/panely/internal/theme"
)

func init() {
	panel.RegisterPanelWithDescription("calendar", "Month view + agenda from ICS", New)
}

// Options configure the calendar panel.
type Options struct {
	Provider string `yaml:"provider"` // default "ics"
}

// Calendar is the calendar panel.
type Calendar struct {
	id      string
	th      theme.Theme
	prov    provider.CalendarProvider
	month   time.Time // first day of displayed month
	events  []provider.Event
	loaded  bool
	loadErr error
	stale   bool
	poller  *panel.Poller
	hasProv bool
}

// New builds a calendar panel.
func New(ctx panel.PanelContext) (panel.Panel, error) {
	opts := Options{Provider: "ics"}
	if err := panel.DecodeOptionsLoose(ctx.Options, &opts); err != nil {
		return nil, err
	}
	now := time.Now()
	c := &Calendar{
		id:    ctx.ID,
		th:    ctx.Theme,
		month: time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local),
	}
	prov, err := provider.NewCalendarProvider(opts.Provider, provider.ProviderContext{
		Options: ctx.Options,
		Secrets: config.ExpandString,
		DataDir: ctx.DataDir,
	})
	if err != nil {
		return nil, fmt.Errorf("calendar panel: %w", err)
	}
	c.prov = prov
	c.hasProv = true
	c.poller = &panel.Poller{
		PanelID:  ctx.ID,
		Interval: 5 * time.Minute,
		Fetch: func(fc context.Context) (any, error) {
			from := time.Now().AddDate(0, -1, 0)
			to := time.Now().AddDate(0, 2, 0)
			return prov.Events(fc, from, to)
		},
	}
	return c, nil
}

func (c *Calendar) Init() tea.Cmd { return c.poller.Start() }

var ckeys = struct {
	prev, next, today key.Binding
}{
	prev:  key.NewBinding(key.WithKeys("left", "h", "p"), key.WithHelp("←/h", "prev month")),
	next:  key.NewBinding(key.WithKeys("right", "l", "n"), key.WithHelp("→/l", "next month")),
	today: key.NewBinding(key.WithKeys("g", "."), key.WithHelp("g", "today")),
}

func (c *Calendar) KeyBindings() []key.Binding {
	return []key.Binding{ckeys.prev, ckeys.next, ckeys.today}
}

func (c *Calendar) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
	pcmd, _ := c.poller.Update(msg)
	switch m := msg.(type) {
	case panel.DataMsg:
		if m.ForPanel(c.id) {
			c.loaded = true
			c.loadErr = m.Err
			c.stale = m.Stale
			if evs, ok := m.Data.([]provider.Event); ok {
				c.events = evs
			}
		}
	case tea.KeyMsg:
		switch {
		case key.Matches(m, ckeys.prev):
			c.month = c.month.AddDate(0, -1, 0)
		case key.Matches(m, ckeys.next):
			c.month = c.month.AddDate(0, 1, 0)
		case key.Matches(m, ckeys.today):
			now := time.Now()
			c.month = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		}
	}
	return c, pcmd
}

// Refresh re-fetches events.
func (c *Calendar) Refresh() tea.Cmd { return c.poller.Refresh() }

func (c *Calendar) View(size panel.Size, focused bool) string {
	monthView := c.renderMonth()
	monthWidth := lipgloss.Width(monthView)
	if size.Width >= monthWidth+24 {
		agenda := c.renderAgenda(size.Width-monthWidth-3, size.Height)
		out := lipgloss.JoinHorizontal(lipgloss.Top, monthView, "  ", agenda)
		return lipgloss.NewStyle().MaxWidth(size.Width).MaxHeight(size.Height).Render(out)
	}
	// Narrow: month on top, agenda below if it fits.
	parts := []string{monthView}
	if rest := size.Height - lipgloss.Height(monthView) - 1; rest > 1 {
		parts = append(parts, c.renderAgenda(size.Width, rest))
	}
	out := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return lipgloss.NewStyle().MaxWidth(size.Width).MaxHeight(size.Height).Render(out)
}

// eventDays returns the set of year-days in the shown month with events.
func (c *Calendar) eventDays() map[int]bool {
	days := map[int]bool{}
	for _, e := range c.events {
		d := e.Start
		for !d.After(e.End) && !d.After(c.month.AddDate(0, 1, 0)) {
			if d.Year() == c.month.Year() && d.Month() == c.month.Month() {
				days[d.Day()] = true
			}
			d = d.AddDate(0, 0, 1)
			if e.End.Sub(e.Start) < 24*time.Hour {
				break
			}
		}
	}
	return days
}

func (c *Calendar) renderMonth() string {
	var b []string
	title := c.th.AccentText().Bold(true).Render(c.month.Format("January 2006"))
	b = append(b, lipgloss.PlaceHorizontal(20, lipgloss.Center, title))
	b = append(b, c.th.MutedText().Render("Mo Tu We Th Fr Sa Su"))
	today := time.Now()
	evDays := c.eventDays()
	// Monday-first weekday offset.
	first := int(c.month.Weekday()+6) % 7
	daysIn := c.month.AddDate(0, 1, -1).Day()
	cells := make([]string, 0, 42)
	for i := 0; i < first; i++ {
		cells = append(cells, "  ")
	}
	for d := 1; d <= daysIn; d++ {
		s := fmt.Sprintf("%2d", d)
		style := c.th.Text()
		if evDays[d] {
			style = c.th.InfoText().Underline(true)
		}
		if today.Year() == c.month.Year() && today.Month() == c.month.Month() && today.Day() == d {
			style = lipgloss.NewStyle().Foreground(c.th.Bg).Background(c.th.Accent).Bold(true)
		}
		cells = append(cells, style.Render(s))
	}
	for i := 0; i < len(cells); i += 7 {
		end := i + 7
		if end > len(cells) {
			end = len(cells)
		}
		b = append(b, strings.Join(cells[i:end], " "))
	}
	return strings.Join(b, "\n")
}

func (c *Calendar) renderAgenda(width, height int) string {
	if width < 8 || height < 1 {
		return ""
	}
	if !c.loaded {
		return c.th.MutedText().Render("⋯ loading events")
	}
	if c.loadErr != nil && len(c.events) == 0 && !c.stale {
		return c.th.ErrorText().Render("✗ ") + c.th.MutedText().Render(clip(c.loadErr.Error(), width-2))
	}
	now := time.Now()
	var lines []string
	lines = append(lines, c.th.MutedText().Bold(true).Render("Upcoming"))
	count := 0
	for _, e := range c.events {
		if e.End.Before(now) {
			continue
		}
		if count >= height-1 {
			break
		}
		when := e.Start.Format("Jan 2 15:04")
		if e.AllDay {
			when = e.Start.Format("Jan 2") + " (all day)"
		}
		line := c.th.AccentText().Render(when) + " " + c.th.Text().Render(clip(e.Title, width-len(when)-1))
		lines = append(lines, clipStyled(line, width))
		count++
	}
	if count == 0 {
		lines = append(lines, c.th.MutedText().Render("no upcoming events"))
	}
	if c.stale {
		lines = append(lines, panel.StaleBadge(c.th))
	}
	return strings.Join(lines, "\n")
}

func clip(s string, n int) string {
	if n < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func clipStyled(s string, width int) string {
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}
