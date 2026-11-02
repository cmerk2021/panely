// Package icscal is a CalendarProvider reading events from local .ics files
// or http(s) URLs. Recurring events are expanded via RRULE where supported
// by the underlying library; otherwise the base occurrence is used.
package icscal

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/teambition/rrule-go"

	"github.com/cmerk2021/panely/internal/httpx"
	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/provider"
)

func init() {
	provider.RegisterCalendarProvider("ics", func(ctx provider.ProviderContext) (provider.CalendarProvider, error) {
		var opts struct {
			Sources []string `yaml:"sources"`
			Source  string   `yaml:"source"`
		}
		if err := panel.DecodeOptionsLoose(ctx.Options, &opts); err != nil {
			return nil, err
		}
		sources := opts.Sources
		if opts.Source != "" {
			sources = append(sources, opts.Source)
		}
		return &Provider{Sources: sources}, nil
	})
}

// Provider reads one or more ICS sources (paths or URLs).
type Provider struct {
	Sources []string
}

// Events returns all events overlapping [from, to), sorted by start.
func (p *Provider) Events(ctx context.Context, from, to time.Time) ([]provider.Event, error) {
	var all []provider.Event
	var firstErr error
	for _, src := range p.Sources {
		evs, err := p.eventsFromSource(ctx, src, from, to)
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", src, err)
			continue
		}
		all = append(all, evs...)
	}
	if len(all) == 0 && firstErr != nil {
		return nil, firstErr
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Start.Before(all[j].Start) })
	return all, nil
}

func (p *Provider) eventsFromSource(ctx context.Context, src string, from, to time.Time) ([]provider.Event, error) {
	var data []byte
	var err error
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		data, err = httpx.Shared.GetCached(ctx, src, 5*time.Minute)
	} else {
		data, err = os.ReadFile(expandHome(src))
	}
	if err != nil {
		return nil, err
	}
	return ParseICS(data, from, to)
}

// ParseICS parses raw ICS bytes and returns events overlapping [from, to).
func ParseICS(data []byte, from, to time.Time) ([]provider.Event, error) {
	cal, err := ical.NewDecoder(bytes.NewReader(data)).Decode()
	if err != nil {
		return nil, fmt.Errorf("parse ics: %w", err)
	}
	var out []provider.Event
	for _, ev := range cal.Events() {
		e, ok := convertEvent(ev)
		if !ok {
			continue
		}
		// Expand recurrences if present.
		if ropt, err := ev.Props.RecurrenceRule(); err == nil && ropt != nil {
			dur := e.End.Sub(e.Start)
			ropt.Dtstart = e.Start
			rule, rerr := rrule.NewRRule(*ropt)
			if rerr != nil {
				if overlaps(e, from, to) {
					out = append(out, e)
				}
				continue
			}
			for _, occ := range rule.Between(from.Add(-dur), to, true) {
				oe := e
				oe.Start = occ
				oe.End = occ.Add(dur)
				if overlaps(oe, from, to) {
					out = append(out, oe)
				}
			}
			continue
		}
		if overlaps(e, from, to) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out, nil
}

func convertEvent(ev ical.Event) (provider.Event, bool) {
	var e provider.Event
	start, err := ev.DateTimeStart(time.Local)
	if err != nil || start.IsZero() {
		return e, false
	}
	end, err := ev.DateTimeEnd(time.Local)
	if err != nil || end.IsZero() {
		end = start.Add(time.Hour)
	}
	summary := ""
	if prop := ev.Props.Get(ical.PropSummary); prop != nil {
		summary = prop.Value
	}
	uid := ""
	if prop := ev.Props.Get(ical.PropUID); prop != nil {
		uid = prop.Value
	}
	loc := ""
	if prop := ev.Props.Get(ical.PropLocation); prop != nil {
		loc = prop.Value
	}
	allDay := false
	if prop := ev.Props.Get(ical.PropDateTimeStart); prop != nil {
		if typ := prop.Params.Get(ical.ParamValue); typ == "DATE" {
			allDay = true
		}
	}
	return provider.Event{
		ID: uid, Title: summary, Start: start, End: end,
		AllDay: allDay, Location: loc,
	}, true
}

func overlaps(e provider.Event, from, to time.Time) bool {
	return e.End.After(from) && e.Start.Before(to)
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return home + p[1:]
		}
	}
	return p
}
