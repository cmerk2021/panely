// Package httpjson is a generic no-code integration panel: fetch a URL,
// extract values with simple path expressions, render with a Go template.
package httpjson

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/httpx"
	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/theme"
)

func init() {
	panel.RegisterPanelWithDescription("http_json", "Fetch a URL and render JSON via template", New)
}

// Options configure the http_json panel.
type Options struct {
	URL      string            `yaml:"url"`
	Interval string            `yaml:"interval"` // default 60s
	Template string            `yaml:"template"` // Go text/template over {{.<extract key>}} or {{.json}}
	Extract  map[string]string `yaml:"extract"`  // name -> path expression like data.items[0].name
	Headers  map[string]string `yaml:"headers"`
}

// Panel is the http_json panel.
type Panel struct {
	id     string
	th     theme.Theme
	opts   Options
	tmpl   *template.Template
	out    string
	err    error
	stale  bool
	loaded bool
	poller *panel.Poller
}

// New builds an http_json panel.
func New(ctx panel.PanelContext) (panel.Panel, error) {
	opts := Options{Interval: "60s"}
	if err := panel.DecodeOptions(ctx.Options, &opts); err != nil {
		return nil, err
	}
	if opts.URL == "" {
		return nil, fmt.Errorf("http_json panel: option %q is required", "url")
	}
	interval, err := time.ParseDuration(opts.Interval)
	if err != nil {
		return nil, fmt.Errorf("http_json panel: invalid interval %q", opts.Interval)
	}
	p := &Panel{id: ctx.ID, th: ctx.Theme, opts: opts}
	if opts.Template != "" {
		t, err := template.New("httpjson").Parse(opts.Template)
		if err != nil {
			return nil, fmt.Errorf("http_json panel: bad template: %w", err)
		}
		p.tmpl = t
	}
	p.poller = &panel.Poller{
		PanelID:  ctx.ID,
		Interval: interval,
		Fetch:    p.fetch,
	}
	return p, nil
}

func (p *Panel) fetch(ctx context.Context) (any, error) {
	body, err := httpx.Shared.Get(ctx, p.opts.URL)
	if err != nil {
		return nil, err
	}
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	data := map[string]any{"json": doc}
	for name, expr := range p.opts.Extract {
		v, err := ExtractPath(doc, expr)
		if err != nil {
			data[name] = fmt.Sprintf("<%v>", err)
			continue
		}
		data[name] = v
	}
	if p.tmpl == nil {
		pretty, _ := json.MarshalIndent(doc, "", "  ")
		return string(pretty), nil
	}
	var sb strings.Builder
	if err := p.tmpl.Execute(&sb, data); err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}
	return sb.String(), nil
}

// ExtractPath evaluates a simple path expression like "data.items[0].name"
// against decoded JSON.
func ExtractPath(doc any, expr string) (any, error) {
	cur := doc
	for _, part := range splitPath(expr) {
		if part.index >= 0 {
			arr, ok := cur.([]any)
			if !ok {
				return nil, fmt.Errorf("%s: not an array", part.raw)
			}
			if part.index >= len(arr) {
				return nil, fmt.Errorf("%s: index out of range", part.raw)
			}
			cur = arr[part.index]
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s: not an object", part.raw)
		}
		v, ok := m[part.key]
		if !ok {
			return nil, fmt.Errorf("%s: key not found", part.raw)
		}
		cur = v
	}
	return cur, nil
}

type pathPart struct {
	key   string
	index int
	raw   string
}

func splitPath(expr string) []pathPart {
	var parts []pathPart
	for _, seg := range strings.Split(expr, ".") {
		rest := seg
		// Split "items[0][1]" into key + indexes.
		if i := strings.IndexByte(rest, '['); i >= 0 {
			if key := rest[:i]; key != "" {
				parts = append(parts, pathPart{key: key, index: -1, raw: key})
			}
			rest = rest[i:]
			for strings.HasPrefix(rest, "[") {
				j := strings.IndexByte(rest, ']')
				if j < 0 {
					break
				}
				n, err := strconv.Atoi(rest[1:j])
				if err != nil {
					n = 0
				}
				parts = append(parts, pathPart{index: n, raw: rest[:j+1]})
				rest = rest[j+1:]
			}
		} else if rest != "" {
			parts = append(parts, pathPart{key: rest, index: -1, raw: rest})
		}
	}
	return parts
}

func (p *Panel) Init() tea.Cmd { return p.poller.Start() }

func (p *Panel) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
	cmd, _ := p.poller.Update(msg)
	if m, ok := msg.(panel.DataMsg); ok && m.ForPanel(p.id) {
		p.loaded = true
		p.err = m.Err
		p.stale = m.Stale
		if s, ok := m.Data.(string); ok {
			p.out = s
		}
	}
	return p, cmd
}

// Refresh re-fetches immediately.
func (p *Panel) Refresh() tea.Cmd { return p.poller.Refresh() }

func (p *Panel) KeyBindings() []key.Binding { return nil }

func (p *Panel) View(size panel.Size, focused bool) string {
	if !p.loaded {
		return panel.LoadingView(p.th, size, "data")
	}
	if p.err != nil && p.out == "" {
		return panel.ErrorView(p.th, size, p.err.Error())
	}
	lines := strings.Split(strings.TrimRight(p.out, "\n"), "\n")
	maxLines := size.Height
	if p.stale {
		maxLines--
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	body := p.th.Text().Render(strings.Join(lines, "\n"))
	if p.stale {
		body += "\n" + panel.StaleBadge(p.th)
	}
	return lipgloss.NewStyle().MaxWidth(size.Width).MaxHeight(size.Height).Render(body)
}
