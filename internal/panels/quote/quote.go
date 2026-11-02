// Package quote shows a quote of the day from an embedded offline set, with
// an optional URL source returning JSON {"text": ..., "author": ...}.
package quote

import (
	"context"
	_ "embed"
	"encoding/json"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/httpx"
	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/theme"
)

//go:embed quotes.json
var embeddedQuotes []byte

func init() {
	panel.RegisterPanelWithDescription("quote", "Quote of the day (offline)", New)
}

// Options configure the quote panel.
type Options struct {
	URL string `yaml:"url"` // optional remote source
}

type entry struct {
	Text   string `json:"text"`
	Author string `json:"author"`
}

// Quote is the quote panel.
type Quote struct {
	id     string
	th     theme.Theme
	opts   Options
	q      entry
	err    error
	poller *panel.Poller
}

// New builds a quote panel.
func New(ctx panel.PanelContext) (panel.Panel, error) {
	var opts Options
	if err := panel.DecodeOptions(ctx.Options, &opts); err != nil {
		return nil, err
	}
	q := &Quote{id: ctx.ID, th: ctx.Theme, opts: opts}
	q.q = offlineQuote(time.Now())
	if opts.URL != "" {
		q.poller = &panel.Poller{
			PanelID:  ctx.ID,
			Interval: 6 * time.Hour,
			Fetch: func(c context.Context) (any, error) {
				body, err := httpx.Shared.Get(c, opts.URL)
				if err != nil {
					return nil, err
				}
				var e entry
				if err := json.Unmarshal(body, &e); err != nil {
					return nil, err
				}
				return e, nil
			},
		}
	}
	return q, nil
}

func offlineQuote(now time.Time) entry {
	var quotes []entry
	if err := json.Unmarshal(embeddedQuotes, &quotes); err != nil || len(quotes) == 0 {
		return entry{Text: "No quotes available.", Author: "panely"}
	}
	return quotes[now.YearDay()%len(quotes)]
}

func (q *Quote) Init() tea.Cmd {
	if q.poller != nil {
		return q.poller.Start()
	}
	return nil
}

func (q *Quote) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
	if q.poller == nil {
		return q, nil
	}
	cmd, _ := q.poller.Update(msg)
	if m, ok := msg.(panel.DataMsg); ok && m.ForPanel(q.id) {
		if m.Err != nil {
			// Fall back silently to the offline quote.
			q.err = m.Err
		} else if e, ok := m.Data.(entry); ok {
			q.q, q.err = e, nil
		}
	}
	return q, cmd
}

// Refresh re-fetches when a URL is configured.
func (q *Quote) Refresh() tea.Cmd {
	if q.poller != nil {
		return q.poller.Refresh()
	}
	return nil
}

func (q *Quote) KeyBindings() []key.Binding { return nil }

func (q *Quote) View(size panel.Size, focused bool) string {
	w := size.Width - 4
	if w < 10 {
		w = size.Width
	}
	text := q.th.Text().Italic(true).Width(w).Render("“" + strings.TrimSpace(q.q.Text) + "”")
	author := q.th.AccentText().Render("— " + q.q.Author)
	content := lipgloss.JoinVertical(lipgloss.Right, text, author)
	return lipgloss.Place(size.Width, size.Height, lipgloss.Center, lipgloss.Center, content)
}
