// Package app contains the root Bubble Tea model: layout, focus, key and
// mouse routing, edit mode, theme switching, and config hot reload.
package app

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cmerk2021/panely/internal/config"
	"github.com/cmerk2021/panely/internal/layout"
	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/theme"
)

// Model is the root Bubble Tea model.
type Model struct {
	cfgPath string
	cfg     *config.Config
	th      theme.Theme
	logger  *slog.Logger
	dataDir string
	watcher *config.Watcher

	width, height int
	rects         []layout.Rect
	panels        []*panel.Safe // parallel to rects
	panelIDs      []string
	focus         int

	editMode   bool
	confirmDel bool
	picking    bool // add-panel picker overlay
	pickCursor int

	help       help.Model
	showHelp   bool
	status     string
	statusErr  bool
	firstRun   bool
	instanceCt int
}

type reloadMsg struct{}

// New builds the root model. firstRun shows a welcome message.
func New(cfgPath string, cfg *config.Config, dataDir string, logger *slog.Logger, firstRun bool) *Model {
	m := &Model{
		cfgPath:  cfgPath,
		cfg:      cfg,
		logger:   logger,
		dataDir:  dataDir,
		firstRun: firstRun,
		help:     help.New(),
	}
	m.applyTheme()
	if firstRun {
		m.setStatus("welcome to panely! config written to "+cfgPath+" — press ? for help", false)
	}
	return m
}

func (m *Model) applyTheme() {
	th, err := theme.Resolve(m.cfg.Theme, m.cfg)
	if err != nil {
		m.setStatus(err.Error(), true)
	}
	m.th = th
	m.help.Styles.ShortKey = th.AccentText()
	m.help.Styles.ShortDesc = th.MutedText()
	m.help.Styles.ShortSeparator = th.MutedText()
	m.help.Styles.FullKey = th.AccentText()
	m.help.Styles.FullDesc = th.MutedText()
	m.help.Styles.FullSeparator = th.MutedText()
}

func (m *Model) setStatus(msg string, isErr bool) {
	m.status = msg
	m.statusErr = isErr
}

// Init starts the watcher subscription and panel initializers.
func (m *Model) Init() tea.Cmd {
	m.watcher = config.NewWatcher(m.cfgPath)
	cmds := []tea.Cmd{m.waitReload()}
	cmds = append(cmds, m.buildPanels()...)
	return tea.Batch(cmds...)
}

func (m *Model) waitReload() tea.Cmd {
	ch := m.watcher.Changed()
	return func() tea.Msg {
		<-ch
		return reloadMsg{}
	}
}

// buildPanels (re)creates all panel instances from the config and returns
// their Init commands.
func (m *Model) buildPanels() []tea.Cmd {
	m.panels = nil
	m.panelIDs = nil
	var cmds []tea.Cmd
	for ri, row := range m.cfg.Rows {
		for pi, pc := range row.Panels {
			m.instanceCt++
			id := fmt.Sprintf("panel-%d-%d-%d", ri, pi, m.instanceCt)
			ctx := panel.PanelContext{
				ID:      id,
				Options: pc.Options,
				Theme:   m.th,
				DataDir: m.dataDir,
				Logger:  m.logger,
			}
			inner, err := panel.New(pc.Type, ctx)
			if err != nil {
				inner = newErrPanel(m.th, pc.Type, err)
			}
			sp := panel.NewSafe(inner, m.th)
			m.panels = append(m.panels, sp)
			m.panelIDs = append(m.panelIDs, id)
			cmds = append(cmds, sp.Init())
		}
	}
	m.recomputeLayout()
	if m.focus >= len(m.panels) {
		m.focus = 0
	}
	return cmds
}

func (m *Model) recomputeLayout() {
	// Reserve one line for the status/help bar.
	m.rects = layout.Layout(m.cfg, m.width, maxInt(0, m.height-1))
}

// reloadConfig re-reads the config file; on error it keeps the old config
// and surfaces the problem in the status bar.
func (m *Model) reloadConfig() tea.Cmd {
	cfg, err := config.Load(m.cfgPath, panel.Types())
	if err != nil {
		m.logger.Error("config reload failed", "error", err.Error())
		m.setStatus("config error (keeping previous): "+firstLine(err.Error()), true)
		return nil
	}
	m.cfg = cfg
	m.applyTheme()
	m.setStatus("config reloaded", false)
	cmds := m.buildPanels()
	return tea.Batch(cmds...)
}

// saveConfig persists the in-memory config, suppressing the watcher event
// caused by our own write.
func (m *Model) saveConfig() {
	m.watcher.IgnoreNext()
	if err := config.Save(m.cfgPath, m.cfg); err != nil {
		m.logger.Error("config save failed", "error", err.Error())
		m.setStatus("save failed: "+firstLine(err.Error()), true)
		return
	}
	m.setStatus("saved", false)
}

// Update is the root message handler.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.Width = msg.Width
		m.recomputeLayout()
		return m, nil

	case reloadMsg:
		cmd := m.reloadConfig()
		return m, tea.Batch(cmd, m.waitReload())

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	// Broadcast everything else (ticks, DataMsgs, ...) to all panels.
	return m, m.broadcast(msg)
}

func (m *Model) broadcast(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for i, p := range m.panels {
		np, cmd := p.Update(msg)
		if sp, ok := np.(*panel.Safe); ok {
			m.panels[i] = sp
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) focusedPanel() *panel.Safe {
	if m.focus >= 0 && m.focus < len(m.panels) {
		return m.panels[m.focus]
	}
	return nil
}

// inputCaptured reports whether the focused panel is capturing text input.
func (m *Model) inputCaptured() bool {
	if p := m.focusedPanel(); p != nil {
		return p.InputCaptured()
	}
	return false
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// errPanel is shown when a panel fails to construct.
type errPanel struct {
	th  theme.Theme
	typ string
	err error
}

func newErrPanel(th theme.Theme, typ string, err error) panel.Panel {
	return &errPanel{th: th, typ: typ, err: err}
}

func (e *errPanel) Init() tea.Cmd                         { return nil }
func (e *errPanel) Update(tea.Msg) (panel.Panel, tea.Cmd) { return e, nil }
func (e *errPanel) KeyBindings() []key.Binding            { return nil }
func (e *errPanel) View(size panel.Size, focused bool) string {
	return panel.ErrorView(e.th, size, fmt.Sprintf("%s: %v", e.typ, e.err))
}
