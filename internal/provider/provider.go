// Package provider defines data-source interfaces and registries so panels
// can swap backends (e.g. local todos vs CalDAV) via config.
package provider

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// SecretResolver expands ${ENV} references in strings handed to providers.
type SecretResolver func(s string) string

// Todo is a single todo item.
type Todo struct {
	ID       string     `json:"id"`
	Text     string     `json:"text"`
	Done     bool       `json:"done"`
	Priority int        `json:"priority,omitempty"` // 0 none, 1 low, 2 med, 3 high
	Due      *time.Time `json:"due,omitempty"`
	Order    int        `json:"order"`
}

// TodoProvider is a swappable todo backend.
type TodoProvider interface {
	Name() string
	List(ctx context.Context) ([]Todo, error)
	Add(ctx context.Context, t Todo) (Todo, error)
	Update(ctx context.Context, t Todo) error
	Delete(ctx context.Context, id string) error
}

// Event is a calendar event.
type Event struct {
	ID       string
	Title    string
	Start    time.Time
	End      time.Time
	AllDay   bool
	Location string
}

// CalendarProvider yields events in a time range.
type CalendarProvider interface {
	Events(ctx context.Context, from, to time.Time) ([]Event, error)
}

// Registries. Factories receive the already-secret-expanded options map plus
// a SecretResolver for any late expansion, and the app data dir.

// ProviderContext carries common construction inputs for providers.
type ProviderContext struct {
	Options map[string]any
	Secrets SecretResolver
	DataDir string
}

type (
	TodoFactory     func(ctx ProviderContext) (TodoProvider, error)
	CalendarFactory func(ctx ProviderContext) (CalendarProvider, error)
)

var (
	mu          sync.RWMutex
	todoReg     = map[string]TodoFactory{}
	calendarReg = map[string]CalendarFactory{}
)

// RegisterTodoProvider registers a todo backend under name.
func RegisterTodoProvider(name string, f TodoFactory) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := todoReg[name]; dup {
		panic(fmt.Sprintf("todo provider %q registered twice", name))
	}
	todoReg[name] = f
}

// NewTodoProvider constructs the named todo backend.
func NewTodoProvider(name string, ctx ProviderContext) (TodoProvider, error) {
	mu.RLock()
	f, ok := todoReg[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown todo provider %q (available: %v)", name, TodoProviders())
	}
	return f(ctx)
}

// TodoProviders lists registered todo provider names.
func TodoProviders() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(todoReg))
	for n := range todoReg {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// RegisterCalendarProvider registers a calendar backend under name.
func RegisterCalendarProvider(name string, f CalendarFactory) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := calendarReg[name]; dup {
		panic(fmt.Sprintf("calendar provider %q registered twice", name))
	}
	calendarReg[name] = f
}

// NewCalendarProvider constructs the named calendar backend.
func NewCalendarProvider(name string, ctx ProviderContext) (CalendarProvider, error) {
	mu.RLock()
	f, ok := calendarReg[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown calendar provider %q", name)
	}
	return f(ctx)
}
