// Package command runs a shell command on an interval and shows its stdout,
// with timeout, error display, and optional ANSI stripping.
package command

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/theme"
)

func init() {
	panel.RegisterPanelWithDescription("command", "Run a shell command on an interval", New)
}

// Options configure the command panel.
type Options struct {
	Command   string `yaml:"command"`
	Interval  string `yaml:"interval"`   // default 30s
	Timeout   string `yaml:"timeout"`    // default 10s
	StripANSI bool   `yaml:"strip_ansi"` // strip escape codes from output
	Shell     string `yaml:"shell"`      // default sh
}

// Command is the command panel.
type Command struct {
	id     string
	th     theme.Theme
	opts   Options
	out    string
	err    error
	stale  bool
	loaded bool
	poller *panel.Poller
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

// New builds a command panel.
func New(ctx panel.PanelContext) (panel.Panel, error) {
	opts := Options{Interval: "30s", Timeout: "10s", Shell: "sh"}
	if err := panel.DecodeOptions(ctx.Options, &opts); err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.Command) == "" {
		return nil, fmt.Errorf("command panel: option %q is required", "command")
	}
	interval, err := time.ParseDuration(opts.Interval)
	if err != nil {
		return nil, fmt.Errorf("command panel: invalid interval %q", opts.Interval)
	}
	timeout, err := time.ParseDuration(opts.Timeout)
	if err != nil {
		return nil, fmt.Errorf("command panel: invalid timeout %q", opts.Timeout)
	}
	c := &Command{id: ctx.ID, th: ctx.Theme, opts: opts}
	c.poller = &panel.Poller{
		PanelID:  ctx.ID,
		Interval: interval,
		Timeout:  timeout + time.Second,
		Fetch: func(fctx context.Context) (any, error) {
			return runCommand(fctx, opts.Shell, opts.Command, timeout)
		},
	}
	return c, nil
}

func runCommand(ctx context.Context, shell, cmdline string, timeout time.Duration) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, shell, "-c", cmdline)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if cctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out after %s", timeout)
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return stdout.String(), nil
}

func (c *Command) Init() tea.Cmd { return c.poller.Start() }

func (c *Command) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
	cmd, _ := c.poller.Update(msg)
	if m, ok := msg.(panel.DataMsg); ok && m.ForPanel(c.id) {
		c.loaded = true
		c.err = m.Err
		c.stale = m.Stale
		if s, ok := m.Data.(string); ok {
			c.out = s
		}
	}
	return c, cmd
}

// Refresh re-runs the command immediately.
func (c *Command) Refresh() tea.Cmd { return c.poller.Refresh() }

func (c *Command) KeyBindings() []key.Binding { return nil }

func (c *Command) View(size panel.Size, focused bool) string {
	if !c.loaded {
		return panel.LoadingView(c.th, size, "command output")
	}
	if c.err != nil && c.out == "" {
		return panel.ErrorView(c.th, size, c.err.Error())
	}
	out := c.out
	if c.opts.StripANSI {
		out = ansiRE.ReplaceAllString(out, "")
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	maxLines := size.Height
	if c.stale {
		maxLines--
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	body := c.th.Text().Render(strings.Join(lines, "\n"))
	if c.stale {
		body += "\n" + panel.StaleBadge(c.th)
	}
	return lipgloss.NewStyle().MaxWidth(size.Width).MaxHeight(size.Height).Render(body)
}
