// Package gitpanel shows status for configured git repositories (branch,
// ahead/behind, dirty count, last commit) by shelling out to git.
package gitpanel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/theme"
)

func init() {
	panel.RegisterPanelWithDescription("git", "Status for local git repos", New)
}

// Options configure the git panel.
type Options struct {
	Repos    []string `yaml:"repos"`
	Interval string   `yaml:"interval"` // default 60s
}

// RepoStatus is one repo's summary.
type RepoStatus struct {
	Name       string
	Branch     string
	Ahead      int
	Behind     int
	Dirty      int
	LastCommit string
	Err        string
}

// Git is the git panel.
type Git struct {
	id     string
	th     theme.Theme
	opts   Options
	repos  []RepoStatus
	loaded bool
	poller *panel.Poller
}

// New builds a git panel.
func New(ctx panel.PanelContext) (panel.Panel, error) {
	opts := Options{Interval: "60s"}
	if err := panel.DecodeOptions(ctx.Options, &opts); err != nil {
		return nil, err
	}
	interval, err := time.ParseDuration(opts.Interval)
	if err != nil {
		return nil, fmt.Errorf("git panel: invalid interval %q", opts.Interval)
	}
	g := &Git{id: ctx.ID, th: ctx.Theme, opts: opts}
	g.poller = &panel.Poller{
		PanelID:  ctx.ID,
		Interval: interval,
		Fetch:    g.fetchAll,
	}
	return g, nil
}

func (g *Git) fetchAll(ctx context.Context) (any, error) {
	out := make([]RepoStatus, len(g.opts.Repos))
	for i, repo := range g.opts.Repos {
		out[i] = statusFor(ctx, expandHome(repo))
	}
	return out, nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	b, err := cmd.Output()
	return strings.TrimSpace(string(b)), err
}

func statusFor(ctx context.Context, dir string) RepoStatus {
	st := RepoStatus{Name: filepath.Base(dir)}
	if _, err := os.Stat(dir); err != nil {
		st.Err = "not found"
		return st
	}
	branch, err := git(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		st.Err = "not a git repo"
		return st
	}
	st.Branch = branch
	if out, err := git(ctx, dir, "rev-list", "--left-right", "--count", "@{upstream}...HEAD"); err == nil {
		parts := strings.Fields(out)
		if len(parts) == 2 {
			st.Behind, _ = strconv.Atoi(parts[0])
			st.Ahead, _ = strconv.Atoi(parts[1])
		}
	}
	if out, err := git(ctx, dir, "status", "--porcelain"); err == nil && out != "" {
		st.Dirty = len(strings.Split(out, "\n"))
	}
	if out, err := git(ctx, dir, "log", "-1", "--format=%s (%cr)"); err == nil {
		st.LastCommit = out
	}
	return st
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func (g *Git) Init() tea.Cmd { return g.poller.Start() }

func (g *Git) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
	cmd, _ := g.poller.Update(msg)
	if m, ok := msg.(panel.DataMsg); ok && m.ForPanel(g.id) {
		g.loaded = true
		if rs, ok := m.Data.([]RepoStatus); ok {
			g.repos = rs
		}
	}
	return g, cmd
}

// Refresh re-reads all repos.
func (g *Git) Refresh() tea.Cmd { return g.poller.Refresh() }

func (g *Git) KeyBindings() []key.Binding { return nil }

func (g *Git) View(size panel.Size, focused bool) string {
	if len(g.opts.Repos) == 0 {
		return panel.EmptyView(g.th, size, "no repos configured\nadd options.repos in config")
	}
	if !g.loaded {
		return panel.LoadingView(g.th, size, "git status")
	}
	var lines []string
	for _, r := range g.repos {
		if len(lines) >= size.Height {
			break
		}
		if r.Err != "" {
			lines = append(lines, g.th.Text().Render(r.Name+" ")+g.th.ErrorText().Render("✗ "+r.Err))
			continue
		}
		var parts []string
		parts = append(parts, g.th.AccentText().Bold(true).Render(r.Name))
		parts = append(parts, g.th.InfoText().Render(" "+r.Branch))
		if r.Ahead > 0 {
			parts = append(parts, g.th.SuccessText().Render(fmt.Sprintf("↑%d", r.Ahead)))
		}
		if r.Behind > 0 {
			parts = append(parts, g.th.WarningText().Render(fmt.Sprintf("↓%d", r.Behind)))
		}
		if r.Dirty > 0 {
			parts = append(parts, g.th.ErrorText().Render(fmt.Sprintf("±%d", r.Dirty)))
		} else {
			parts = append(parts, g.th.SuccessText().Render("✓"))
		}
		lines = append(lines, lipgloss.NewStyle().MaxWidth(size.Width).Render(strings.Join(parts, " ")))
		if r.LastCommit != "" && len(lines) < size.Height {
			lines = append(lines, lipgloss.NewStyle().MaxWidth(size.Width).Render(
				g.th.MutedText().Render("  "+r.LastCommit)))
		}
	}
	return lipgloss.NewStyle().MaxWidth(size.Width).MaxHeight(size.Height).Render(strings.Join(lines, "\n"))
}
