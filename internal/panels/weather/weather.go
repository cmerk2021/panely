// Package weather shows current conditions and a multi-day forecast from
// Open-Meteo (no API key), with geocoding and 15-minute caching.
package weather

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/httpx"
	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/providers/openmeteo"
	"github.com/cmerk2021/panely/internal/theme"
)

func init() {
	panel.RegisterPanelWithDescription("weather", "Current conditions + forecast (Open-Meteo)", New)
}

// Options configure the weather panel.
type Options struct {
	Location string  `yaml:"location"`
	Lat      float64 `yaml:"lat"`
	Lon      float64 `yaml:"lon"`
	Units    string  `yaml:"units"` // metric | imperial
}

// Weather is the weather panel.
type Weather struct {
	id      string
	th      theme.Theme
	opts    Options
	rep     *openmeteo.Report
	loaded  bool
	loadErr error
	stale   bool
	poller  *panel.Poller

	geoOnce  sync.Once
	lat, lon float64
	place    string
	geoErr   error
}

// New builds a weather panel.
func New(ctx panel.PanelContext) (panel.Panel, error) {
	opts := Options{Units: "metric"}
	if err := panel.DecodeOptions(ctx.Options, &opts); err != nil {
		return nil, err
	}
	if opts.Units != "metric" && opts.Units != "imperial" {
		return nil, fmt.Errorf("weather panel: units must be metric or imperial, got %q", opts.Units)
	}
	if opts.Location == "" && opts.Lat == 0 && opts.Lon == 0 {
		return nil, fmt.Errorf("weather panel: set options.location or options.lat/lon")
	}
	w := &Weather{id: ctx.ID, th: ctx.Theme, opts: opts, lat: opts.Lat, lon: opts.Lon, place: opts.Location}
	w.poller = &panel.Poller{
		PanelID:  ctx.ID,
		Interval: openmeteo.CacheTTL,
		Fetch:    w.fetch,
	}
	return w, nil
}

func (w *Weather) fetch(ctx context.Context) (any, error) {
	w.geoOnce.Do(func() {
		if w.opts.Location != "" {
			lat, lon, name, err := openmeteo.Geocode(ctx, httpx.Shared, w.opts.Location)
			if err != nil {
				w.geoErr = err
				return
			}
			w.lat, w.lon, w.place = lat, lon, name
		}
	})
	if w.geoErr != nil {
		err := w.geoErr
		// Allow retrying geocoding on the next poll.
		w.geoOnce = sync.Once{}
		w.geoErr = nil
		return nil, err
	}
	rep, err := openmeteo.Fetch(ctx, httpx.Shared, w.lat, w.lon, w.opts.Units)
	if err != nil {
		return nil, err
	}
	rep.Place = w.place
	return rep, nil
}

func (w *Weather) Init() tea.Cmd { return w.poller.Start() }

func (w *Weather) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
	cmd, _ := w.poller.Update(msg)
	if m, ok := msg.(panel.DataMsg); ok && m.ForPanel(w.id) {
		w.loaded = true
		w.loadErr = m.Err
		w.stale = m.Stale
		if rep, ok := m.Data.(*openmeteo.Report); ok {
			w.rep = rep
		}
	}
	return w, cmd
}

// Refresh re-fetches the weather.
func (w *Weather) Refresh() tea.Cmd { return w.poller.Refresh() }

func (w *Weather) KeyBindings() []key.Binding { return nil }

func (w *Weather) View(size panel.Size, focused bool) string {
	if !w.loaded {
		return panel.LoadingView(w.th, size, "weather")
	}
	if w.rep == nil {
		msg := "weather unavailable (offline?)"
		if w.loadErr != nil {
			msg = w.loadErr.Error()
		}
		return panel.ErrorView(w.th, size, msg)
	}
	unit := "°C"
	wind := "km/h"
	if w.rep.Units == "imperial" {
		unit = "°F"
		wind = "mph"
	}
	cur := w.rep.Current
	icon := openmeteo.Icon(cur.Code, cur.IsDay)
	head := lipgloss.JoinVertical(lipgloss.Left,
		w.th.AccentText().Bold(true).Render(w.placeName()),
		w.th.Text().Bold(true).Render(fmt.Sprintf("%s %.1f%s", icon, cur.Temperature, unit))+
			w.th.MutedText().Render(fmt.Sprintf("  feels %.0f%s", cur.FeelsLike, unit)),
		w.th.MutedText().Render(fmt.Sprintf("%s · %d%% rh · %.0f %s wind",
			openmeteo.Describe(cur.Code), cur.Humidity, cur.WindSpeed, wind)),
	)
	var fc []string
	for _, d := range w.rep.Days {
		fc = append(fc, lipgloss.JoinVertical(lipgloss.Center,
			w.th.MutedText().Render(d.Date.Format("Mon")),
			openmeteo.Icon(d.Code, true),
			w.th.Text().Render(fmt.Sprintf("%.0f°", d.Max)),
			w.th.MutedText().Render(fmt.Sprintf("%.0f°", d.Min)),
		))
	}
	forecast := ""
	if len(fc) > 0 {
		cols := make([]string, 0, len(fc)*2)
		for i, c := range fc {
			if i > 0 {
				cols = append(cols, "   ")
			}
			cols = append(cols, c)
		}
		forecast = lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	}
	body := head
	if size.Height >= 8 && forecast != "" {
		body = lipgloss.JoinVertical(lipgloss.Left, head, "", forecast)
	}
	if w.stale {
		body = lipgloss.JoinVertical(lipgloss.Left, body, panel.StaleBadge(w.th))
	}
	return lipgloss.Place(size.Width, size.Height, lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().MaxWidth(size.Width).Render(body))
}

func (w *Weather) placeName() string {
	if w.rep.Place != "" {
		return w.rep.Place
	}
	return strings.TrimSpace(fmt.Sprintf("%.2f, %.2f", w.rep.Lat, w.rep.Lon))
}
