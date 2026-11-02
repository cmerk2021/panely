// Package caldavtodo is a TodoProvider backed by a CalDAV server (VTODO
// items), built on github.com/emersion/go-webdav. It doubles as the worked
// example for docs/WRITING_INTEGRATIONS.md.
//
// Config options on the todo panel:
//
//	options:
//	  provider: caldav
//	  url: https://dav.example.com/          # CalDAV endpoint
//	  username: me
//	  password: ${CALDAV_PASSWORD}           # secret-expanded
//	  calendar: Tasks                        # calendar display name (optional)
package caldavtodo

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"

	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/provider"
)

func init() {
	provider.RegisterTodoProvider("caldav", New)
}

// Options are the caldav provider's config options.
type Options struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Calendar string `yaml:"calendar"`
}

// New constructs the provider from a ProviderContext (registry factory).
func New(ctx provider.ProviderContext) (provider.TodoProvider, error) {
	var opts Options
	if err := panel.DecodeOptionsLoose(ctx.Options, &opts); err != nil {
		return nil, err
	}
	// Late expansion in case values were templated at provider level.
	if ctx.Secrets != nil {
		opts.URL = ctx.Secrets(opts.URL)
		opts.Username = ctx.Secrets(opts.Username)
		opts.Password = ctx.Secrets(opts.Password)
	}
	if opts.URL == "" {
		return nil, fmt.Errorf("caldav todo provider: option %q is required", "url")
	}
	var hc webdav.HTTPClient = &http.Client{Timeout: 20 * time.Second}
	if opts.Username != "" {
		hc = webdav.HTTPClientWithBasicAuth(hc, opts.Username, opts.Password)
	}
	client, err := caldav.NewClient(hc, opts.URL)
	if err != nil {
		return nil, fmt.Errorf("caldav client: %w", err)
	}
	return &Provider{client: client, calendarName: opts.Calendar}, nil
}

// Provider implements provider.TodoProvider against a CalDAV server.
type Provider struct {
	client       *caldav.Client
	calendarName string

	mu      sync.Mutex
	calPath string            // resolved calendar collection path
	paths   map[string]string // todo ID (UID) -> object path
}

func (p *Provider) Name() string { return "caldav" }

// resolveCalendar discovers the calendar collection, preferring one whose
// display name matches the configured calendar and that supports VTODO.
func (p *Provider) resolveCalendar(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calPath != "" {
		return p.calPath, nil
	}
	principal, err := p.client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		return "", fmt.Errorf("find principal: %w", err)
	}
	home, err := p.client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		return "", fmt.Errorf("find calendar home: %w", err)
	}
	cals, err := p.client.FindCalendars(ctx, home)
	if err != nil {
		return "", fmt.Errorf("find calendars: %w", err)
	}
	var fallback string
	for _, c := range cals {
		supportsTodo := len(c.SupportedComponentSet) == 0
		for _, comp := range c.SupportedComponentSet {
			if comp == ical.CompToDo {
				supportsTodo = true
			}
		}
		if !supportsTodo {
			continue
		}
		if fallback == "" {
			fallback = c.Path
		}
		if p.calendarName != "" && strings.EqualFold(c.Name, p.calendarName) {
			p.calPath = c.Path
			return p.calPath, nil
		}
	}
	if p.calendarName != "" {
		return "", fmt.Errorf("calendar %q not found (or does not support todos)", p.calendarName)
	}
	if fallback == "" {
		return "", fmt.Errorf("no todo-capable calendar found")
	}
	p.calPath = fallback
	return p.calPath, nil
}

func (p *Provider) List(ctx context.Context) ([]provider.Todo, error) {
	cal, err := p.resolveCalendar(ctx)
	if err != nil {
		return nil, err
	}
	query := &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{
			Name:     ical.CompCalendar,
			AllProps: true,
			Comps:    []caldav.CalendarCompRequest{{Name: ical.CompToDo, AllProps: true}},
		},
		CompFilter: caldav.CompFilter{
			Name:  ical.CompCalendar,
			Comps: []caldav.CompFilter{{Name: ical.CompToDo}},
		},
	}
	objs, err := p.client.QueryCalendar(ctx, cal, query)
	if err != nil {
		return nil, fmt.Errorf("query todos: %w", err)
	}
	paths := map[string]string{}
	var todos []provider.Todo
	for _, obj := range objs {
		if obj.Data == nil {
			continue
		}
		for _, comp := range obj.Data.Children {
			if comp.Name != ical.CompToDo {
				continue
			}
			t := decodeTodo(comp)
			t.Order = len(todos)
			if t.ID != "" {
				paths[t.ID] = obj.Path
			}
			todos = append(todos, t)
		}
	}
	p.mu.Lock()
	p.paths = paths
	p.mu.Unlock()
	return todos, nil
}

func (p *Provider) Add(ctx context.Context, t provider.Todo) (provider.Todo, error) {
	calPath, err := p.resolveCalendar(ctx)
	if err != nil {
		return t, err
	}
	if t.ID == "" {
		t.ID = fmt.Sprintf("panely-%d", time.Now().UnixNano())
	}
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropProductID, "-//panely//panely//EN")
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Children = append(cal.Children, encodeTodo(t))
	path := strings.TrimSuffix(calPath, "/") + "/" + t.ID + ".ics"
	if _, err := p.client.PutCalendarObject(ctx, path, cal); err != nil {
		return t, fmt.Errorf("add todo: %w", err)
	}
	p.mu.Lock()
	if p.paths == nil {
		p.paths = map[string]string{}
	}
	p.paths[t.ID] = path
	p.mu.Unlock()
	return t, nil
}

func (p *Provider) Update(ctx context.Context, t provider.Todo) error {
	path, err := p.pathFor(ctx, t.ID)
	if err != nil {
		return err
	}
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropProductID, "-//panely//panely//EN")
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Children = append(cal.Children, encodeTodo(t))
	if _, err := p.client.PutCalendarObject(ctx, path, cal); err != nil {
		return fmt.Errorf("update todo: %w", err)
	}
	return nil
}

func (p *Provider) Delete(ctx context.Context, id string) error {
	path, err := p.pathFor(ctx, id)
	if err != nil {
		return err
	}
	if err := p.client.RemoveAll(ctx, path); err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}
	p.mu.Lock()
	delete(p.paths, id)
	p.mu.Unlock()
	return nil
}

func (p *Provider) pathFor(ctx context.Context, id string) (string, error) {
	p.mu.Lock()
	path, ok := p.paths[id]
	p.mu.Unlock()
	if ok {
		return path, nil
	}
	// Refresh the listing once in case the cache is cold.
	if _, err := p.List(ctx); err != nil {
		return "", err
	}
	p.mu.Lock()
	path, ok = p.paths[id]
	p.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("todo %s not found on server", id)
	}
	return path, nil
}

// decodeTodo converts a VTODO component into a provider.Todo.
func decodeTodo(comp *ical.Component) provider.Todo {
	var t provider.Todo
	if prop := comp.Props.Get(ical.PropUID); prop != nil {
		t.ID = prop.Value
	}
	if prop := comp.Props.Get(ical.PropSummary); prop != nil {
		t.Text = prop.Value
	}
	if prop := comp.Props.Get(ical.PropStatus); prop != nil {
		t.Done = strings.EqualFold(prop.Value, "COMPLETED")
	}
	if prop := comp.Props.Get(ical.PropPriority); prop != nil {
		// iCalendar: 1 (highest) .. 9 (lowest); map to 3/2/1.
		if n, err := strconv.Atoi(prop.Value); err == nil && n > 0 {
			switch {
			case n <= 3:
				t.Priority = 3
			case n <= 6:
				t.Priority = 2
			default:
				t.Priority = 1
			}
		}
	}
	if prop := comp.Props.Get(ical.PropDue); prop != nil {
		if due, err := prop.DateTime(time.Local); err == nil {
			t.Due = &due
		}
	}
	return t
}

// encodeTodo converts a provider.Todo into a VTODO component.
func encodeTodo(t provider.Todo) *ical.Component {
	comp := ical.NewComponent(ical.CompToDo)
	comp.Props.SetText(ical.PropUID, t.ID)
	comp.Props.SetText(ical.PropSummary, t.Text)
	comp.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	if t.Done {
		comp.Props.SetText(ical.PropStatus, "COMPLETED")
	} else {
		comp.Props.SetText(ical.PropStatus, "NEEDS-ACTION")
	}
	if t.Priority > 0 {
		ic := map[int]string{1: "9", 2: "5", 3: "1"}[t.Priority]
		comp.Props.SetText(ical.PropPriority, ic)
	}
	if t.Due != nil {
		comp.Props.SetDateTime(ical.PropDue, *t.Due)
	}
	return comp
}
