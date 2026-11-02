package panel

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// DataMsg delivers the result of an async fetch to a panel, tagged by the
// panel's ID so the root model can route it.
type DataMsg struct {
	PanelID string
	Data    any
	Err     error
	// Stale reports whether Data is from a previous successful fetch that is
	// being re-delivered because the latest fetch failed.
	Stale     bool
	FetchedAt time.Time
}

// ForPanel reports whether this message targets the given panel ID.
func (m DataMsg) ForPanel(id string) bool { return m.PanelID == id }

// pollTickMsg schedules the next poll for a given panel/generation.
type pollTickMsg struct {
	panelID string
	gen     int
}

// Poller repeatedly runs a fetch function on an interval without ever
// blocking Update. On error it backs off exponentially (up to 8x interval)
// and re-delivers the last good data with Stale set, so panels keep showing
// something useful.
type Poller struct {
	PanelID  string
	Interval time.Duration
	Timeout  time.Duration
	Fetch    func(ctx context.Context) (any, error)

	gen        int
	lastGood   any
	lastGoodAt time.Time
	hasGood    bool
	failures   int
}

// Start returns the initial fetch command.
func (p *Poller) Start() tea.Cmd {
	p.gen++
	return p.fetchCmd()
}

// Refresh triggers an immediate re-fetch, canceling pending scheduled polls.
func (p *Poller) Refresh() tea.Cmd {
	p.gen++
	return p.fetchCmd()
}

// Update processes poller-related messages; returns a command and whether the
// message was consumed. Call this from the panel's Update for every message.
func (p *Poller) Update(msg tea.Msg) (tea.Cmd, bool) {
	switch m := msg.(type) {
	case pollTickMsg:
		if m.panelID != p.PanelID || m.gen != p.gen {
			return nil, m.panelID == p.PanelID
		}
		return p.fetchCmd(), true
	case DataMsg:
		if m.PanelID != p.PanelID {
			return nil, false
		}
		// Track success for stale re-delivery; schedule the next poll.
		if m.Err == nil && !m.Stale {
			p.lastGood, p.lastGoodAt, p.hasGood = m.Data, m.FetchedAt, true
			p.failures = 0
		} else {
			p.failures++
		}
		if p.Interval <= 0 {
			return nil, false // one-shot
		}
		delay := p.Interval
		if p.failures > 0 {
			backoff := p.Interval * time.Duration(1<<uint(min(p.failures, 3)))
			if backoff > 8*p.Interval {
				backoff = 8 * p.Interval
			}
			delay = backoff
		}
		gen := p.gen
		id := p.PanelID
		return tea.Tick(delay, func(time.Time) tea.Msg {
			return pollTickMsg{panelID: id, gen: gen}
		}), false // not consumed: the panel also wants the DataMsg
	}
	return nil, false
}

func (p *Poller) fetchCmd() tea.Cmd {
	id := p.PanelID
	fetch := p.Fetch
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	lastGood, lastGoodAt, hasGood := p.lastGood, p.lastGoodAt, p.hasGood
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		data, err := fetch(ctx)
		if err != nil && hasGood {
			// Keep showing stale data alongside the error.
			return DataMsg{PanelID: id, Data: lastGood, Err: err, Stale: true, FetchedAt: lastGoodAt}
		}
		return DataMsg{PanelID: id, Data: data, Err: err, FetchedAt: time.Now()}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
