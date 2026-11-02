// Package docker lists containers via the Docker socket using the stdlib
// HTTP client (no SDK). Shows a friendly state when Docker is unavailable.
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/theme"
)

func init() {
	panel.RegisterPanelWithDescription("docker", "Docker containers via socket", New)
}

// Options configure the docker panel.
type Options struct {
	Socket   string `yaml:"socket"`   // default /var/run/docker.sock
	Interval string `yaml:"interval"` // default 10s
	All      bool   `yaml:"all"`      // include stopped containers
}

// Container is a summarized container.
type Container struct {
	Name   string
	Image  string
	State  string
	Status string
	Ports  string
}

// Docker is the docker panel.
type Docker struct {
	id         string
	th         theme.Theme
	opts       Options
	client     *http.Client
	containers []Container
	loaded     bool
	loadErr    error
	stale      bool
	poller     *panel.Poller
}

// New builds a docker panel.
func New(ctx panel.PanelContext) (panel.Panel, error) {
	opts := Options{Socket: "/var/run/docker.sock", Interval: "10s"}
	if err := panel.DecodeOptions(ctx.Options, &opts); err != nil {
		return nil, err
	}
	interval, err := time.ParseDuration(opts.Interval)
	if err != nil {
		return nil, fmt.Errorf("docker panel: invalid interval %q", opts.Interval)
	}
	d := &Docker{id: ctx.ID, th: ctx.Theme, opts: opts}
	d.client = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(c context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(c, "unix", opts.Socket)
			},
		},
	}
	d.poller = &panel.Poller{
		PanelID:  ctx.ID,
		Interval: interval,
		Fetch:    d.fetch,
	}
	return d, nil
}

func (d *Docker) fetch(ctx context.Context) (any, error) {
	url := "http://docker/containers/json"
	if d.opts.All {
		url += "?all=1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker not available")
	}
	defer func() { _ = resp.Body.Close() }()
	var raw []struct {
		Names  []string `json:"Names"`
		Image  string   `json:"Image"`
		State  string   `json:"State"`
		Status string   `json:"Status"`
		Ports  []struct {
			PrivatePort int    `json:"PrivatePort"`
			PublicPort  int    `json:"PublicPort"`
			Type        string `json:"Type"`
		} `json:"Ports"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("docker API: %w", err)
	}
	var out []Container
	for _, c := range raw {
		name := "?"
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		var ports []string
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, fmt.Sprintf("%d→%d", p.PublicPort, p.PrivatePort))
			}
		}
		out = append(out, Container{
			Name: name, Image: c.Image, State: c.State, Status: c.Status,
			Ports: strings.Join(ports, ","),
		})
	}
	return out, nil
}

func (d *Docker) Init() tea.Cmd { return d.poller.Start() }

func (d *Docker) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
	cmd, _ := d.poller.Update(msg)
	if m, ok := msg.(panel.DataMsg); ok && m.ForPanel(d.id) {
		d.loaded = true
		d.loadErr = m.Err
		d.stale = m.Stale
		if cs, ok := m.Data.([]Container); ok {
			d.containers = cs
		}
	}
	return d, cmd
}

// Refresh re-queries Docker.
func (d *Docker) Refresh() tea.Cmd { return d.poller.Refresh() }

func (d *Docker) KeyBindings() []key.Binding { return nil }

func (d *Docker) View(size panel.Size, focused bool) string {
	if !d.loaded {
		return panel.LoadingView(d.th, size, "containers")
	}
	if d.loadErr != nil && len(d.containers) == 0 && !d.stale {
		return panel.EmptyView(d.th, size, "Docker not available\n(is the daemon running?)")
	}
	if len(d.containers) == 0 {
		return panel.EmptyView(d.th, size, "no running containers")
	}
	var lines []string
	for _, c := range d.containers {
		if len(lines) >= size.Height {
			break
		}
		mark := d.th.SuccessText().Render("●")
		if c.State != "running" {
			mark = d.th.MutedText().Render("○")
		}
		name := clip(c.Name, 18)
		image := clip(c.Image, 24)
		row := fmt.Sprintf("%s %s %s %s",
			mark,
			d.th.Text().Render(fmt.Sprintf("%-18s", name)),
			d.th.MutedText().Render(fmt.Sprintf("%-24s", image)),
			d.th.InfoText().Render(c.Ports))
		lines = append(lines, lipgloss.NewStyle().MaxWidth(size.Width).Render(row))
	}
	if d.stale {
		lines = append(lines, panel.StaleBadge(d.th))
	}
	return lipgloss.NewStyle().MaxWidth(size.Width).MaxHeight(size.Height).Render(strings.Join(lines, "\n"))
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}
