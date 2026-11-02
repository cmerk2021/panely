# panely

A beautiful, configurable terminal dashboard — in the spirit of
[Homarr](https://homarr.dev), but for your terminal. Built in Go with
[Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Bubbles](https://github.com/charmbracelet/bubbles), and
[Lip Gloss](https://github.com/charmbracelet/lipgloss).

> 📸 *Screenshot placeholder — `panely` running the default dashboard.*

## Features

- **14 built-in panels**: clock, weather, calendar, todo, notes, sysinfo,
  services, links, rss, docker, git, quote, command, http_json.
- **YAML config as the source of truth**, with hot reload — edit the file and
  the dashboard updates live; invalid edits show an error, never a crash.
- **Live layout editing** inside the TUI: move, resize, add, and delete
  panels and rows; changes are written back to the config atomically.
- **7 bundled themes** (catppuccin mocha/latte, tokyo-night, gruvbox-dark,
  nord, dracula, adaptive default) plus custom themes with inheritance.
  Cycle live with `t`.
- **Keyboard-first** with full mouse support: click to focus, click to toggle
  todos and open links, wheel to scroll.
- **Extensible by design**: panels and data providers are compiled-in Go
  packages that self-register — adding one means one package + one import
  line. A real CalDAV todo provider ships as the worked example.
- **No API keys required**: weather uses Open-Meteo; every network panel
  degrades to a friendly offline state.
- **Static binaries** (`CGO_ENABLED=0`) for Linux amd64/arm64/armv7 —
  Raspberry Pi friendly.

## Install

**Release binaries** (Linux amd64 / arm64 / armv7): grab a `tar.gz` from the
[releases page](https://github.com/cmerk2021/panely/releases), or a
`.deb`/`.rpm`/`.apk` package.

```sh
tar -xzf panely_*.tar.gz
sudo install -m755 panely /usr/local/bin/
```

**Go install:**

```sh
go install github.com/cmerk2021/panely/cmd/panely@latest
```

**From source:**

```sh
git clone https://github.com/cmerk2021/panely
cd panely && make build   # binary in ./bin/panely
```

> The module path is `github.com/cmerk2021/panely` (the repository owner). If
> you fork, update `go.mod` and the imports:
> `find . -name '*.go' -exec sed -i 's#github.com/cmerk2021/panely#github.com/YOU/panely#g' {} +`

## Quick start

```sh
panely
```

On first run panely writes a commented default config to
`~/.config/panely/config.yaml` and shows a starter dashboard. Then:

- edit the YAML (it hot-reloads on save), or
- press `e` for in-app layout editing.

Useful commands:

```sh
panely --config ./my.yaml   # alternate config
panely version              # version info
panely themes               # list available themes
panely config path          # print config path
panely config validate      # validate the config
panely config init          # write the default config
panely --debug              # log to ~/.local/share/panely/panely.log
```

See [examples/config.yaml](examples/config.yaml) for a kitchen-sink config.

## Key bindings

| Key | Action |
|---|---|
| `tab` / `shift+tab` | Focus next / previous panel |
| `h/j/k/l`, arrows | Move focus spatially (when the panel doesn't use them) |
| `ctrl+h/j/k/l`, `alt+arrows` | Always move focus |
| mouse click | Focus panel (and interact: toggle todo, open link) |
| `r` | Refresh the focused panel |
| `t` | Cycle themes (persisted to config) |
| `e` | Toggle edit mode |
| `?` | Toggle full help |
| `q` / `ctrl+c` | Quit |

**Edit mode** (`e`): `h/l` move the panel within its row, `j/k` move it
between rows (creating rows as needed), `+`/`-` adjust weight, `[`/`]` adjust
row height, `a` add a panel (picker), `A` add a row, `d` delete (with
confirmation), `esc` exit. Every change saves to the YAML file immediately.

While a panel is capturing text input (todo entry, notes editor), global keys
are suspended.

## Panel catalog

Full option tables live in [docs/CONFIGURATION.md](docs/CONFIGURATION.md).

| Type | What it shows | Key options |
|---|---|---|
| `clock` | Time/date, multiple timezones, optional big digits | `timezones`, `format24h`, `seconds`, `big_digits` |
| `weather` | Current conditions + forecast (Open-Meteo, no key) | `location` or `lat`/`lon`, `units` |
| `calendar` | Month view + agenda from `.ics` files/URLs | `sources`, `provider` |
| `todo` | Full CRUD todo list, local JSON or CalDAV | `provider` (+ CalDAV `url`/`username`/`password`/`calendar`) |
| `notes` | Persistent scratchpad with textarea editing | `file` |
| `sysinfo` | CPU sparklines, memory/swap/disk gauges, network, top processes | `interval`, `processes`, `sort_by` |
| `services` | HTTP/TCP/ping uptime checks with latency + history | `targets`, `interval`, `timeout` |
| `links` | Quick-launch links with groups and icons | `links` |
| `rss` | RSS/Atom headlines with unread markers | `feeds`, `interval`, `limit` |
| `docker` | Containers with state, image, ports (via socket) | `socket`, `all`, `interval` |
| `git` | Branch, ahead/behind, dirty count per repo | `repos`, `interval` |
| `quote` | Quote of the day (offline embedded set) | `url` (optional) |
| `command` | Any shell command's stdout on an interval | `command`, `interval`, `timeout`, `strip_ansi` |
| `http_json` | Fetch JSON, extract values, render a template | `url`, `extract`, `template`, `headers` |

`command` and `http_json` are the no-code escape hatches: most integrations
need neither Go nor a plugin.

## Theming

Bundled themes: `catppuccin-mocha`, `catppuccin-latte`, `tokyo-night`,
`gruvbox-dark`, `nord`, `dracula`, `default` (adapts to light/dark
terminals). Define custom themes with inheritance:

```yaml
theme: my-theme
themes:
  my-theme:
    base: catppuccin-mocha
    colors: { accent: "#ff79c6" }
```

All panels style exclusively through the theme — no hard-coded colors.

## Configuration

The YAML file at `~/.config/panely/config.yaml` defines everything: rows of
panels with weights and heights, panel options, and themes. All string values
support `${ENV_VAR}` and `${ENV_VAR:-default}` expansion, so secrets stay out
of the file. Validation errors include line numbers and typo suggestions.

Full reference: [docs/CONFIGURATION.md](docs/CONFIGURATION.md).

## Writing integrations

Integrations are compiled in and self-register from `init()` — one new
package plus one blank-import line in `internal/builtin`:

```go
_ "github.com/cmerk2021/panely/internal/panels/hello"
```

The `Panel` interface is four methods; swappable data backends (like the
shipped CalDAV todo provider — `provider: caldav`) implement small provider
interfaces. Full walkthrough with a tutorial panel, the CalDAV worked
example, and testing patterns:
[docs/WRITING_INTEGRATIONS.md](docs/WRITING_INTEGRATIONS.md).

## Development

```sh
make build      # build with version ldflags
make test       # go test -race ./...
make lint       # golangci-lint
make snapshot   # goreleaser snapshot build
```

Design notes and judgment calls: [docs/DECISIONS.md](docs/DECISIONS.md).

## License

[MIT](LICENSE)
