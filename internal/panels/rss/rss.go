// Package rss shows headlines from RSS and Atom feeds with unread markers,
// scrolling, and open-in-browser.
package rss

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/httpx"
	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/panels/links"
	"github.com/cmerk2021/panely/internal/theme"
)

func init() {
	panel.RegisterPanelWithDescription("rss", "RSS/Atom headlines", New)
}

// Options configure the rss panel.
type Options struct {
	Feeds    []string `yaml:"feeds"`
	Interval string   `yaml:"interval"` // default 10m
	Limit    int      `yaml:"limit"`    // max items, default 50
}

// Item is one feed entry.
type Item struct {
	Title string
	Link  string
	Date  time.Time
	Feed  string
}

// RSS is the rss panel.
type RSS struct {
	id      string
	th      theme.Theme
	opts    Options
	items   []Item
	read    map[string]bool
	cursor  int
	scroll  int
	loaded  bool
	loadErr error
	stale   bool
	poller  *panel.Poller
	rowAt   []int
}

// New builds an rss panel.
func New(ctx panel.PanelContext) (panel.Panel, error) {
	opts := Options{Interval: "10m", Limit: 50}
	if err := panel.DecodeOptions(ctx.Options, &opts); err != nil {
		return nil, err
	}
	interval, err := time.ParseDuration(opts.Interval)
	if err != nil {
		return nil, fmt.Errorf("rss panel: invalid interval %q", opts.Interval)
	}
	r := &RSS{id: ctx.ID, th: ctx.Theme, opts: opts, read: map[string]bool{}}
	r.poller = &panel.Poller{
		PanelID:  ctx.ID,
		Interval: interval,
		Fetch:    r.fetchAll,
	}
	return r, nil
}

func (r *RSS) fetchAll(ctx context.Context) (any, error) {
	var all []Item
	var firstErr error
	for _, feed := range r.opts.Feeds {
		body, err := httpx.Shared.GetCached(ctx, feed, time.Minute)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		items, err := ParseFeed(body)
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", feed, err)
		}
		all = append(all, items...)
	}
	if len(all) == 0 && firstErr != nil {
		return nil, firstErr
	}
	// Newest first.
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].Date.After(all[i].Date) {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
	if len(all) > r.opts.Limit {
		all = all[:r.opts.Limit]
	}
	return all, nil
}

// ParseFeed parses RSS 2.0 or Atom bytes into items.
func ParseFeed(body []byte) ([]Item, error) {
	// Try RSS 2.0.
	var rss struct {
		Channel struct {
			Title string `xml:"title"`
			Items []struct {
				Title   string `xml:"title"`
				Link    string `xml:"link"`
				PubDate string `xml:"pubDate"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(body, &rss); err == nil && len(rss.Channel.Items) > 0 {
		var items []Item
		for _, it := range rss.Channel.Items {
			items = append(items, Item{
				Title: strings.TrimSpace(it.Title),
				Link:  strings.TrimSpace(it.Link),
				Date:  parseDate(it.PubDate),
				Feed:  rss.Channel.Title,
			})
		}
		return items, nil
	}
	// Try Atom.
	var atom struct {
		XMLName xml.Name `xml:"feed"`
		Title   string   `xml:"title"`
		Entries []struct {
			Title string `xml:"title"`
			Links []struct {
				Href string `xml:"href,attr"`
				Rel  string `xml:"rel,attr"`
			} `xml:"link"`
			Updated   string `xml:"updated"`
			Published string `xml:"published"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(body, &atom); err != nil {
		return nil, fmt.Errorf("not a valid RSS/Atom feed: %w", err)
	}
	var items []Item
	for _, e := range atom.Entries {
		link := ""
		for _, l := range e.Links {
			if l.Rel == "" || l.Rel == "alternate" {
				link = l.Href
				break
			}
		}
		date := e.Published
		if date == "" {
			date = e.Updated
		}
		items = append(items, Item{
			Title: strings.TrimSpace(e.Title),
			Link:  link,
			Date:  parseDate(date),
			Feed:  atom.Title,
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no items found in feed")
	}
	return items, nil
}

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range []string{
		time.RFC1123Z, time.RFC1123, time.RFC3339, time.RFC822Z, time.RFC822,
		"Mon, 2 Jan 2006 15:04:05 -0700", "2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (r *RSS) Init() tea.Cmd { return r.poller.Start() }

var rkeys = struct {
	up, down, open, markAll key.Binding
}{
	up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "prev item")),
	down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "next item")),
	open:    key.NewBinding(key.WithKeys("enter", "o"), key.WithHelp("enter/o", "open in browser")),
	markAll: key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mark all read")),
}

func (r *RSS) KeyBindings() []key.Binding {
	return []key.Binding{rkeys.up, rkeys.down, rkeys.open, rkeys.markAll}
}

func (r *RSS) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
	pcmd, _ := r.poller.Update(msg)
	switch m := msg.(type) {
	case panel.DataMsg:
		if m.ForPanel(r.id) {
			r.loaded = true
			r.loadErr = m.Err
			r.stale = m.Stale
			if items, ok := m.Data.([]Item); ok {
				r.items = items
				if r.cursor >= len(items) {
					r.cursor = maxInt(0, len(items)-1)
				}
			}
		}
	case tea.KeyMsg:
		switch {
		case key.Matches(m, rkeys.up):
			if r.cursor > 0 {
				r.cursor--
			}
		case key.Matches(m, rkeys.down):
			if r.cursor < len(r.items)-1 {
				r.cursor++
			}
		case key.Matches(m, rkeys.open):
			return r, tea.Batch(pcmd, r.open(r.cursor))
		case key.Matches(m, rkeys.markAll):
			for _, it := range r.items {
				r.read[it.Link] = true
			}
		}
	case tea.MouseMsg:
		switch {
		case m.Action == tea.MouseActionPress && m.Button == tea.MouseButtonLeft:
			if m.Y >= 0 && m.Y < len(r.rowAt) && r.rowAt[m.Y] >= 0 {
				r.cursor = r.rowAt[m.Y]
				return r, tea.Batch(pcmd, r.open(r.cursor))
			}
		case m.Button == tea.MouseButtonWheelUp:
			if r.cursor > 0 {
				r.cursor--
			}
		case m.Button == tea.MouseButtonWheelDown:
			if r.cursor < len(r.items)-1 {
				r.cursor++
			}
		}
	}
	return r, pcmd
}

func (r *RSS) open(i int) tea.Cmd {
	if i < 0 || i >= len(r.items) {
		return nil
	}
	item := r.items[i]
	r.read[item.Link] = true
	return func() tea.Msg {
		_ = links.OpenURL(item.Link)
		return nil
	}
}

// Refresh re-fetches all feeds.
func (r *RSS) Refresh() tea.Cmd { return r.poller.Refresh() }

func (r *RSS) View(size panel.Size, focused bool) string {
	if len(r.opts.Feeds) == 0 {
		return panel.EmptyView(r.th, size, "no feeds configured\nadd options.feeds in config")
	}
	if !r.loaded {
		return panel.LoadingView(r.th, size, "feeds")
	}
	if r.loadErr != nil && len(r.items) == 0 && !r.stale {
		return panel.ErrorView(r.th, size, "feeds unavailable (offline?)\n"+r.loadErr.Error())
	}
	if len(r.items) == 0 {
		return panel.EmptyView(r.th, size, "no headlines")
	}
	r.rowAt = make([]int, size.Height)
	for i := range r.rowAt {
		r.rowAt[i] = -1
	}
	height := size.Height
	if r.stale {
		height--
	}
	if r.cursor < r.scroll {
		r.scroll = r.cursor
	}
	if r.cursor >= r.scroll+height {
		r.scroll = r.cursor - height + 1
	}
	var lines []string
	for i := r.scroll; i < len(r.items) && len(lines) < height; i++ {
		it := r.items[i]
		marker := r.th.AccentText().Render("•")
		style := r.th.Text()
		if r.read[it.Link] {
			marker = " "
			style = r.th.MutedText()
		}
		cursor := "  "
		if i == r.cursor && focused {
			cursor = r.th.AccentText().Render("▸ ")
		}
		when := ""
		if !it.Date.IsZero() {
			when = " " + r.th.MutedText().Render(relTime(it.Date))
		}
		r.rowAt[len(lines)] = i
		row := cursor + marker + " " + style.Render(it.Title) + when
		lines = append(lines, lipgloss.NewStyle().MaxWidth(size.Width).Render(row))
	}
	if r.stale {
		lines = append(lines, panel.StaleBadge(r.th))
	}
	return lipgloss.NewStyle().MaxWidth(size.Width).MaxHeight(size.Height).Render(strings.Join(lines, "\n"))
}

func relTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
