// Package clock renders time and date for one or more timezones.
package clock

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/theme"
)

func init() {
	panel.RegisterPanelWithDescription("clock", "Time and date, multiple timezones", New)
}

// Options configure the clock panel.
type Options struct {
	Timezones []string `yaml:"timezones"`
	Format24h bool     `yaml:"format24h"`
	Seconds   bool     `yaml:"seconds"`
	BigDigits bool     `yaml:"big_digits"`
}

type tickMsg struct{ id string }

// Clock is the clock panel.
type Clock struct {
	id    string
	th    theme.Theme
	opts  Options
	zones []*time.Location
	names []string
	now   time.Time
}

// New builds a clock panel from config options.
func New(ctx panel.PanelContext) (panel.Panel, error) {
	opts := Options{Format24h: true, Seconds: true}
	if err := panel.DecodeOptions(ctx.Options, &opts); err != nil {
		return nil, err
	}
	if len(opts.Timezones) == 0 {
		opts.Timezones = []string{"Local"}
	}
	c := &Clock{id: ctx.ID, th: ctx.Theme, opts: opts, now: time.Now()}
	for _, name := range opts.Timezones {
		loc := time.Local
		display := name
		if !strings.EqualFold(name, "local") {
			l, err := time.LoadLocation(name)
			if err != nil {
				return nil, fmt.Errorf("clock: unknown timezone %q", name)
			}
			loc = l
			if i := strings.LastIndex(name, "/"); i >= 0 {
				display = strings.ReplaceAll(name[i+1:], "_", " ")
			}
		} else {
			display = "Local"
		}
		c.zones = append(c.zones, loc)
		c.names = append(c.names, display)
	}
	return c, nil
}

func (c *Clock) Init() tea.Cmd { return c.tick() }

func (c *Clock) tick() tea.Cmd {
	id := c.id
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{id: id} })
}

func (c *Clock) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
	if m, ok := msg.(tickMsg); ok && m.id == c.id {
		c.now = time.Now()
		return c, c.tick()
	}
	return c, nil
}

func (c *Clock) KeyBindings() []key.Binding { return nil }

func (c *Clock) View(size panel.Size, focused bool) string {
	timeFmt := "15:04"
	if !c.opts.Format24h {
		timeFmt = "3:04"
	}
	if c.opts.Seconds {
		timeFmt += ":05"
	}
	if !c.opts.Format24h {
		timeFmt += " PM"
	}
	var blocks []string
	for i, loc := range c.zones {
		t := c.now.In(loc)
		timeStr := t.Format(timeFmt)
		if c.opts.BigDigits {
			timeStr = bigDigits(t.Format(strings.TrimSuffix(timeFmt, " PM")))
		}
		block := lipgloss.JoinVertical(lipgloss.Center,
			c.th.AccentText().Bold(true).Render(timeStr),
			c.th.Text().Render(t.Format("Mon, Jan 2 2006")),
			c.th.MutedText().Render(c.names[i]),
		)
		blocks = append(blocks, block)
	}
	content := lipgloss.JoinVertical(lipgloss.Center, joinSpaced(blocks)...)
	return lipgloss.Place(size.Width, size.Height, lipgloss.Center, lipgloss.Center, content)
}

func joinSpaced(blocks []string) []string {
	var out []string
	for i, b := range blocks {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, b)
	}
	return out
}

// bigDigits renders a time string with block characters.
var digitFont = map[rune][3]string{
	'0': {"█▀█", "█ █", "█▄█"},
	'1': {"▀█ ", " █ ", "▄█▄"},
	'2': {"▀▀█", "█▀▀", "█▄▄"},
	'3': {"▀▀█", " ▀█", "▄▄█"},
	'4': {"█ █", "▀▀█", "  █"},
	'5': {"█▀▀", "▀▀█", "▄▄█"},
	'6': {"█▀▀", "█▀█", "█▄█"},
	'7': {"▀▀█", "  █", "  █"},
	'8': {"█▀█", "█▀█", "█▄█"},
	'9': {"█▀█", "▀▀█", "▄▄█"},
	':': {"   ", " ▀ ", " ▄ "},
}

func bigDigits(s string) string {
	rows := [3][]string{}
	for _, r := range s {
		glyph, ok := digitFont[r]
		if !ok {
			glyph = [3]string{"   ", "   ", "   "}
		}
		for i := 0; i < 3; i++ {
			rows[i] = append(rows[i], glyph[i])
		}
	}
	lines := make([]string, 3)
	for i := 0; i < 3; i++ {
		lines[i] = strings.Join(rows[i], " ")
	}
	return strings.Join(lines, "\n")
}
