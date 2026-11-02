// Package panel defines the Panel interface, the panel registry, option
// decoding, and shared async polling helpers. Integrations are compiled-in
// Go packages that self-register from init(); see docs/WRITING_INTEGRATIONS.md.
package panel

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cmerk2021/panely/internal/theme"
)

// Size is the content area available to a panel (inside its border).
type Size struct {
	Width  int
	Height int
}

// Panel is the interface every panel implements. Implementations must never
// block in Update; use the Poller helper or tea.Cmd for I/O.
type Panel interface {
	// Init returns the panel's startup command (timers, first fetch, ...).
	Init() tea.Cmd
	// Update handles a message and returns the (possibly replaced) panel.
	Update(msg tea.Msg) (Panel, tea.Cmd)
	// View renders the panel's content for the given size. focused reports
	// whether this panel currently has keyboard focus.
	View(size Size, focused bool) string
	// KeyBindings lists panel-specific keys, shown in help when focused.
	KeyBindings() []key.Binding
}

// Refresher is optionally implemented by panels that support manual refresh
// (bound to "r" on the focused panel).
type Refresher interface {
	Refresh() tea.Cmd
}

// InputCapturer is optionally implemented by panels that capture raw text
// input (e.g. while editing); global keybindings are suspended while
// InputCaptured reports true.
type InputCapturer interface {
	InputCaptured() bool
}

// PanelContext is handed to a Factory when a panel instance is created.
type PanelContext struct {
	// ID uniquely identifies this panel instance for message routing.
	ID string
	// Options is the decoded options map from config (secrets expanded).
	Options map[string]any
	// Theme is the active theme. Always style through it.
	Theme theme.Theme
	// DataDir is the app data directory for persistence.
	DataDir string
	// Logger is a structured logger. Never log secret values.
	Logger *slog.Logger
}

// Factory creates a panel instance from its context.
type Factory func(ctx PanelContext) (Panel, error)

type registration struct {
	factory     Factory
	description string
}

var (
	regMu    sync.RWMutex
	registry = map[string]registration{}
)

// RegisterPanel registers a panel type. Call from init() in the panel package
// and add a blank import in internal/builtin.
func RegisterPanel(typeName string, f Factory) {
	RegisterPanelWithDescription(typeName, "", f)
}

// RegisterPanelWithDescription registers a panel type with a short
// human-readable description (shown in the edit-mode add-panel picker).
func RegisterPanelWithDescription(typeName, description string, f Factory) {
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := registry[typeName]; dup {
		panic(fmt.Sprintf("panel type %q registered twice", typeName))
	}
	registry[typeName] = registration{factory: f, description: description}
}

// New creates a panel of the given registered type.
func New(typeName string, ctx PanelContext) (Panel, error) {
	regMu.RLock()
	reg, ok := registry[typeName]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown panel type %q", typeName)
	}
	return reg.factory(ctx)
}

// Types returns all registered panel type names, sorted.
func Types() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Description returns the registered description for a panel type.
func Description(typeName string) string {
	regMu.RLock()
	defer regMu.RUnlock()
	return registry[typeName].description
}
