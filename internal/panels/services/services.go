// Package services performs uptime checks for HTTP(S) and TCP targets
// (ping-style checks fall back to TCP connect, which needs no privileges).
package services

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/httpx"
	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/theme"
)

func init() {
	panel.RegisterPanelWithDescription("services", "HTTP/TCP uptime checks", New)
}

// Target is one monitored service.
type Target struct {
	Name string `yaml:"name"`
	// URL is an http(s)://, tcp://host:port, or ping://host target.
	URL string `yaml:"url"`
}

// Options configure the services panel.
type Options struct {
	Targets  []Target `yaml:"targets"`
	Interval string   `yaml:"interval"` // default 30s
	Timeout  string   `yaml:"timeout"`  // default 5s
	History  int      `yaml:"history"`  // kept checks per service, default 20
}

type status struct {
	up      bool
	latency time.Duration
	err     string
}

type result struct {
	statuses []status
}

// Services is the services panel.
type Services struct {
	id      string
	th      theme.Theme
	opts    Options
	timeout time.Duration
	cur     []status
	history [][]bool
	loaded  bool
	poller  *panel.Poller
}

// New builds a services panel.
func New(ctx panel.PanelContext) (panel.Panel, error) {
	opts := Options{Interval: "30s", Timeout: "5s", History: 20}
	if err := panel.DecodeOptions(ctx.Options, &opts); err != nil {
		return nil, err
	}
	interval, err := time.ParseDuration(opts.Interval)
	if err != nil {
		return nil, fmt.Errorf("services panel: invalid interval %q", opts.Interval)
	}
	timeout, err := time.ParseDuration(opts.Timeout)
	if err != nil {
		return nil, fmt.Errorf("services panel: invalid timeout %q", opts.Timeout)
	}
	s := &Services{
		id: ctx.ID, th: ctx.Theme, opts: opts, timeout: timeout,
		history: make([][]bool, len(opts.Targets)),
	}
	s.poller = &panel.Poller{
		PanelID:  ctx.ID,
		Interval: interval,
		Timeout:  timeout*2 + 5*time.Second,
		Fetch:    s.checkAll,
	}
	return s, nil
}

func (s *Services) checkAll(ctx context.Context) (any, error) {
	statuses := make([]status, len(s.opts.Targets))
	done := make(chan int, len(s.opts.Targets))
	for i, t := range s.opts.Targets {
		go func(i int, t Target) {
			statuses[i] = check(ctx, t.URL, s.timeout)
			done <- i
		}(i, t)
	}
	for range s.opts.Targets {
		<-done
	}
	return &result{statuses: statuses}, nil
}

func check(ctx context.Context, target string, timeout time.Duration) status {
	start := time.Now()
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	switch {
	case strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://"):
		req, err := http.NewRequestWithContext(cctx, http.MethodGet, target, nil)
		if err != nil {
			return status{err: err.Error()}
		}
		resp, err := httpx.Shared.Do(req)
		if err != nil {
			return status{err: shortErr(err)}
		}
		_ = resp.Body.Close()
		lat := time.Since(start)
		if resp.StatusCode >= 500 {
			return status{latency: lat, err: resp.Status}
		}
		return status{up: true, latency: lat}
	case strings.HasPrefix(target, "tcp://"), strings.HasPrefix(target, "ping://"):
		addr := strings.TrimPrefix(strings.TrimPrefix(target, "tcp://"), "ping://")
		if !strings.Contains(addr, ":") {
			addr += ":80" // ping fallback uses a TCP connect
		}
		var d net.Dialer
		conn, err := d.DialContext(cctx, "tcp", addr)
		if err != nil {
			return status{err: shortErr(err)}
		}
		_ = conn.Close()
		return status{up: true, latency: time.Since(start)}
	default:
		return status{err: "unsupported scheme (use http://, https://, tcp://, ping://)"}
	}
}

func shortErr(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 {
		msg = msg[i+2:]
	}
	return msg
}

func (s *Services) Init() tea.Cmd { return s.poller.Start() }

func (s *Services) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
	cmd, _ := s.poller.Update(msg)
	if m, ok := msg.(panel.DataMsg); ok && m.ForPanel(s.id) {
		s.loaded = true
		if res, ok := m.Data.(*result); ok {
			s.cur = res.statuses
			for i, st := range res.statuses {
				s.history[i] = append(s.history[i], st.up)
				if len(s.history[i]) > s.opts.History {
					s.history[i] = s.history[i][len(s.history[i])-s.opts.History:]
				}
			}
		}
	}
	return s, cmd
}

// Refresh re-checks all services now.
func (s *Services) Refresh() tea.Cmd { return s.poller.Refresh() }

func (s *Services) KeyBindings() []key.Binding { return nil }

func (s *Services) View(size panel.Size, focused bool) string {
	if len(s.opts.Targets) == 0 {
		return panel.EmptyView(s.th, size, "no services configured\nadd options.targets in config")
	}
	if !s.loaded {
		return panel.LoadingView(s.th, size, "service checks")
	}
	nameW := 0
	for _, t := range s.opts.Targets {
		if len(t.Name) > nameW {
			nameW = len(t.Name)
		}
	}
	if nameW > 20 {
		nameW = 20
	}
	var lines []string
	for i, t := range s.opts.Targets {
		if i >= len(s.cur) || len(lines) >= size.Height {
			break
		}
		st := s.cur[i]
		mark := s.th.ErrorText().Render("●")
		detail := s.th.ErrorText().Render(clip(st.err, 24))
		if st.up {
			mark = s.th.SuccessText().Render("●")
			detail = s.th.MutedText().Render(fmt.Sprintf("%4dms", st.latency.Milliseconds()))
		}
		name := t.Name
		if len(name) > nameW {
			name = name[:nameW]
		}
		hist := historyBar(s.th, s.history[i])
		lines = append(lines, fmt.Sprintf("%s %s %s %s",
			mark,
			s.th.Text().Render(fmt.Sprintf("%-*s", nameW, name)),
			detail,
			hist))
	}
	return lipgloss.NewStyle().MaxWidth(size.Width).MaxHeight(size.Height).Render(strings.Join(lines, "\n"))
}

func historyBar(th theme.Theme, hist []bool) string {
	var sb strings.Builder
	for _, up := range hist {
		if up {
			sb.WriteString(th.SuccessText().Render("▪"))
		} else {
			sb.WriteString(th.ErrorText().Render("▪"))
		}
	}
	return sb.String()
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
