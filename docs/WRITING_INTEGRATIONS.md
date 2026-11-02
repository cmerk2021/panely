# Writing integrations

panely integrations are **compiled-in Go packages that self-register** from
`init()`. There is no runtime plugin loading: to add an integration you add
one package and one blank-import line in `internal/builtin/builtin.go`.

Two extension points exist:

- **Panels** — things drawn on the dashboard (`internal/panel.RegisterPanel`).
- **Providers** — swappable data backends for existing panels, selected with
  the `provider:` option (`internal/provider.Register*Provider`).

---

## 1. Writing a panel

### The interface

```go
// internal/panel/panel.go
type Panel interface {
    Init() tea.Cmd
    Update(msg tea.Msg) (Panel, tea.Cmd)
    View(size Size, focused bool) string
    KeyBindings() []key.Binding // shown in the help footer when focused
}
```

A panel is a miniature Bubble Tea model. Rules:

- **Never block in `Update`.** All I/O goes through `tea.Cmd`s — ideally the
  `panel.Poller` helper (below).
- **Style only through `ctx.Theme`.** No hard-coded colors; theme switching
  must restyle everything.
- `View` must fit its output into `size.Width × size.Height`; the border and
  title are drawn by the app around your content.

Optional interfaces your panel may also implement:

- `panel.Refresher` — `Refresh() tea.Cmd`, bound to `r` when focused.
- `panel.InputCapturer` — `InputCaptured() bool`; return true while editing
  text so global keys (`q`, `t`, `e`, …) are suspended.

### A complete minimal panel

`internal/panels/hello/hello.go`:

```go
// Package hello is a tutorial panel that greets someone.
package hello

import (
    "github.com/charmbracelet/bubbles/key"
    tea "github.com/charmbracelet/bubbletea"

    "github.com/cmerk2021/panely/internal/panel"
    "github.com/cmerk2021/panely/internal/theme"
)

func init() {
    panel.RegisterPanelWithDescription("hello", "Tutorial greeting panel", New)
}

type options struct {
    Name string `yaml:"name"`
}

type Model struct {
    opts  options
    theme theme.Theme
}

func New(ctx panel.PanelContext) (panel.Panel, error) {
    opts := options{Name: "world"} // defaults first
    if err := panel.DecodeOptions(ctx.Options, &opts); err != nil {
        return nil, err
    }
    return &Model{opts: opts, theme: ctx.Theme}, nil
}

func (m *Model) Init() tea.Cmd                          { return nil }
func (m *Model) Update(msg tea.Msg) (panel.Panel, tea.Cmd) { return m, nil }
func (m *Model) KeyBindings() []key.Binding             { return nil }

func (m *Model) View(size panel.Size, focused bool) string {
    return m.theme.AccentText().Render("Hello, " + m.opts.Name + "!")
}
```

Register it in `internal/builtin/builtin.go`:

```go
_ "github.com/cmerk2021/panely/internal/panels/hello"
```

Use it in config:

```yaml
rows:
  - panels:
      - type: hello
        options: { name: Ada }
```

That's the whole process — one package, one import line.

### Option decoding

`panel.DecodeOptions(ctx.Options, &opts)` re-marshals the YAML options map
into your typed struct with **strict field checking** (unknown keys are
errors, giving users typo feedback). Use `panel.DecodeOptionsLoose` when the
options map is shared with a provider (the todo and calendar panels do this,
since provider-specific keys live alongside panel keys).

Set defaults on the struct *before* decoding. Validate after, returning a
descriptive error — it is shown inline in the panel as an error box.

All string values in `ctx.Options` have already had `${ENV}` /
`${ENV:-default}` references expanded. **Never log them** — they may contain
secrets.

### Async data: the Poller

Panels that fetch data use `panel.Poller`, which polls a fetch function on
an interval with exponential backoff on error and delivers results as
`panel.DataMsg`s tagged with your panel ID:

```go
type data struct{ /* whatever your fetch returns */ }

m.poller = &panel.Poller{
    PanelID:  ctx.ID,
    Interval: 15 * time.Minute,
    Fetch: func(ctx context.Context) (any, error) {
        return fetchMyData(ctx)
    },
}

func (m *Model) Init() tea.Cmd { return m.poller.Start() }

func (m *Model) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
    cmd, _ := m.poller.Update(msg) // keep the poller ticking
    if dm, ok := msg.(panel.DataMsg); ok && dm.PanelID == m.id {
        if dm.Err != nil {
            m.err = dm.Err // previous data (if any) is re-delivered with dm.Stale=true
        } else {
            m.data, m.err, m.stale = dm.Data.(data), nil, dm.Stale
        }
    }
    return m, cmd
}
```

When a refresh fails but older data exists, the Poller re-delivers the old
data with `Stale: true` so you can keep rendering it with a stale indicator
(`panel.StaleBadge(theme)` renders the standard one).

Shared render helpers for consistent states: `panel.LoadingView`,
`panel.ErrorView`, `panel.EmptyView`.

### HTTP

Use `internal/httpx` instead of `http.DefaultClient`: it sets timeouts and a
`panely/<version>` user agent, and `httpx.GetCached` adds on-disk response
caching (used by the weather panel for its 15-minute cache).

---

## 2. Writing a provider

Providers let one panel support multiple backends. The todo panel, for
example, works against anything implementing:

```go
type TodoProvider interface {
    Name() string
    List(ctx context.Context) ([]Todo, error)
    Add(ctx context.Context, t Todo) (Todo, error)
    Update(ctx context.Context, t Todo) error
    Delete(ctx context.Context, id string) error
}
```

Register a factory:

```go
func init() {
    provider.RegisterTodoProvider("mybackend", New)
}

func New(ctx provider.ProviderContext) (provider.TodoProvider, error) {
    // ctx.Options — the panel's options map (secrets already expanded)
    // ctx.Secrets — resolver for any late ${ENV} expansion
    // ctx.DataDir — app data directory for local persistence
}
```

…add the blank import in `internal/builtin`, and users select it with:

```yaml
- type: todo
  options: { provider: mybackend }
```

`CalendarProvider` works the same way via `RegisterCalendarProvider`.

## 3. Worked example: the CalDAV todo provider

`internal/providers/caldavtodo` is a real, shipping provider you can crib
from. It implements `TodoProvider` against a CalDAV server using
`github.com/emersion/go-webdav`:

- **Options:** `url`, `username`, `password` (all secret-expandable — use
  `${CALDAV_PASSWORD}` in config), and `calendar` (display name; the first
  todo-capable calendar is used if omitted).
- **Discovery:** it resolves the current-user principal, finds the calendar
  home set, and lists calendars, picking the one matching `calendar`.
- **Mapping:** todos round-trip as iCalendar `VTODO` components —
  `SUMMARY` ⇄ text, `STATUS:COMPLETED` ⇄ done, `PRIORITY` ⇄ priority,
  `DUE` ⇄ due date.

```yaml
- type: todo
  options:
    provider: caldav
    url: https://cloud.example.com/remote.php/dav
    username: ${CALDAV_USER}
    password: ${CALDAV_PASSWORD}
    calendar: Tasks
```

## 4. Testing providers

You rarely need a real server. Two patterns, both used in
`internal/providers/caldavtodo/caldavtodo_test.go`:

1. **Fake provider** — implement the interface in-memory to test panel logic
   against the contract:

```go
type fakeTodoProvider struct {
    mu    sync.Mutex
    items map[string]provider.Todo
}

func (f *fakeTodoProvider) Name() string { return "fake" }
func (f *fakeTodoProvider) List(ctx context.Context) ([]provider.Todo, error) { ... }
// ... Add/Update/Delete mutate the map ...

func TestFakeProviderContract(t *testing.T) {
    var p provider.TodoProvider = newFake()
    added, _ := p.Add(context.Background(), provider.Todo{Text: "buy milk"})
    // assert List/Update/Delete behavior
}
```

2. **`httptest.Server`** — for providers that speak HTTP, point the provider
   at a local test server that replays canned responses (the CalDAV test
   serves recorded PROPFIND/REPORT bodies).

Run everything with `go test -race ./...`.

## 5. Checklist for a new integration

- [ ] Package under `internal/panels/<name>` or `internal/providers/<name>`
- [ ] `init()` calls the matching `Register*` function
- [ ] Blank import added to `internal/builtin/builtin.go`
- [ ] Options decoded into a typed struct with defaults and validation
- [ ] No blocking calls in `Update` (use `panel.Poller` / `tea.Cmd`)
- [ ] All styling via `theme.Theme`; no literal colors
- [ ] Secrets never logged
- [ ] Loading / error / empty states handled
- [ ] Tests (fake provider or `httptest`), passing under `-race`
- [ ] Panel options documented in README's panel catalog
