# Configuration reference

panely is configured by a single YAML file.

- **Path:** `$XDG_CONFIG_HOME/panely/config.yaml` (default
  `~/.config/panely/config.yaml`); on macOS/Windows, `os.UserConfigDir` +
  `/panely/config.yaml`. Override with `--config <path>`.
- **Data dir:** `$XDG_DATA_HOME/panely/` (default `~/.local/share/panely/`)
  stores todos, notes, HTTP caches, and `panely.log`.
- **First run:** a commented default config is written automatically.
- **Hot reload:** panely watches the file and reloads on save. If the new
  content fails to parse or validate, the current dashboard keeps running and
  the error is shown in the status bar.
- Print the active path with `panely config path`; check a file with
  `panely config validate`.

## Top level

| Key       | Type     | Default            | Description |
|-----------|----------|--------------------|-------------|
| `theme`   | string   | `catppuccin-mocha` | Active theme name (bundled or custom). |
| `refresh` | duration | `1s`               | Global UI tick (clock updates, etc.). |
| `rows`    | list     | —                  | The dashboard layout (required, non-empty). |
| `themes`  | map      | —                  | Custom theme definitions (see Theming). |

## Secrets / environment expansion

Every **string** value anywhere in the config supports:

- `${VAR}` — replaced with the environment variable's value (empty if unset).
- `${VAR:-default}` — replaced with the value, or `default` if unset/empty.

Expansion happens once at load time through `config.ExpandSecrets`; expanded
values are passed to panels and providers but **never logged**.

## Rows and panels

```yaml
rows:
  - height: 30%        # optional: "12" (lines), "30%", or omit for auto
    panels:
      - type: clock    # required: a registered panel type
        title: Clock   # optional: override the border title
        weight: 2      # optional: width share within the row (default 1)
        min_width: 20  # optional: below this the panel collapses
        min_height: 6  # optional
        options: {}    # panel-specific options (see below)
```

- **Rows** stack vertically. `height` is fixed lines (`"12"`), a percentage
  of the dashboard (`"30%"`), or auto — auto rows share the remaining space
  equally.
- **Panels** within a row split the width proportionally to `weight`
  (weights 2 and 1 → ⅔ and ⅓). Rounding is distributed so the row always
  exactly fills the terminal width.
- If the terminal is smaller than 20×6 a "terminal too small" hint is shown
  instead of panels; panely never panics on resize.

## Panel options

### clock

| Option | Type | Default | Description |
|---|---|---|---|
| `timezones` | list of strings | `[Local]` | IANA names (`America/New_York`) or `Local`. |
| `format24h` | bool | `false` | 24-hour format. |
| `seconds` | bool | `false` | Show seconds. |
| `big_digits` | bool | `false` | Large block-digit clock for the first timezone. |

### weather

Open-Meteo, no API key. Responses are cached and refreshed every 15 minutes.

| Option | Type | Default | Description |
|---|---|---|---|
| `location` | string | — | Place name, geocoded via Open-Meteo. |
| `lat`, `lon` | float | — | Explicit coordinates (skip geocoding). |
| `units` | string | `metric` | `metric` or `imperial`. |

### calendar

Month view with today highlighted (`←/h` `→/l` to change month, `g` for
today) plus an agenda of upcoming events.

| Option | Type | Default | Description |
|---|---|---|---|
| `provider` | string | `ics` | Calendar provider name. |
| `sources` | list of strings | — | `.ics` file paths (`~` expanded) or `http(s)` URLs. |
| `source` | string | — | Shorthand for a single source. |

### todo

| Option | Type | Default | Description |
|---|---|---|---|
| `provider` | string | `local` | `local` (JSON in the data dir) or `caldav`. |

CalDAV provider options (alongside `provider: caldav`):

| Option | Type | Description |
|---|---|---|
| `url` | string | CalDAV endpoint, e.g. `https://cloud.example.com/remote.php/dav`. |
| `username` | string | Login (supports `${ENV}`). |
| `password` | string | Password (use `${ENV}`!). |
| `calendar` | string | Calendar display name; first todo calendar if omitted. |

Keys: `space` toggle, `a` add, `i` edit, `d` delete, `p` priority,
`J/K` reorder. Mouse click toggles an item.

### notes

| Option | Type | Default | Description |
|---|---|---|---|
| `file` | string | `notes.txt` | File name inside the data dir. |

`enter`/`i` edits, `esc`/`ctrl+s` saves, `ctrl+q` discards.

### sysinfo

| Option | Type | Default | Description |
|---|---|---|---|
| `interval` | duration | `2s` | Sampling interval. |
| `processes` | int | `5` | Top-N processes shown. |
| `sort_by` | string | `cpu` | `cpu` or `mem`. |

### services

| Option | Type | Default | Description |
|---|---|---|---|
| `targets` | list | — | Each: `name` + `url`. |
| `interval` | duration | `30s` | Check interval. |
| `timeout` | duration | `5s` | Per-check timeout. |
| `history` | int | `20` | Checks kept per service (history strip). |

Target URL schemes: `http://`/`https://` (GET, up = status < 500),
`tcp://host:port` (connect), `ping://host` (TCP connect fallback; no ICMP
privileges needed).

### links

| Option | Type | Description |
|---|---|---|
| `links` | list | Each: `name`, `url`, optional `icon` (emoji), optional `group`. |

`enter`/`o` or mouse click opens via `xdg-open`/`open`/`start`.

### rss

| Option | Type | Default | Description |
|---|---|---|---|
| `feeds` | list of URLs | — | RSS or Atom feeds. |
| `interval` | duration | `10m` | Refresh interval. |
| `limit` | int | `50` | Max items kept. |

Unread items are marked; `enter`/`o` opens in browser, `m` marks all read.

### docker

| Option | Type | Default | Description |
|---|---|---|---|
| `socket` | string | `/var/run/docker.sock` | Docker socket path. |
| `interval` | duration | `10s` | Refresh interval. |
| `all` | bool | `false` | Include stopped containers. |

Shows a friendly "Docker not available" state when the socket is missing.

### git

| Option | Type | Default | Description |
|---|---|---|---|
| `repos` | list of paths | — | Repository paths (`~` expanded). |
| `interval` | duration | `60s` | Refresh interval. |

Shows branch, ahead/behind, dirty count, and last commit (shells out to
`git`).

### quote

| Option | Type | Default | Description |
|---|---|---|---|
| `url` | string | — | Optional remote source returning plain text or JSON; otherwise an embedded offline set is used. |

### command

| Option | Type | Default | Description |
|---|---|---|---|
| `command` | string | — | Shell command (required). |
| `interval` | duration | `30s` | Run interval. |
| `timeout` | duration | `10s` | Kill after this long. |
| `strip_ansi` | bool | `false` | Strip ANSI escapes instead of passing through. |
| `shell` | string | `sh` | Shell binary. |

### http_json

| Option | Type | Default | Description |
|---|---|---|---|
| `url` | string | — | URL to fetch (required). |
| `interval` | duration | `60s` | Fetch interval. |
| `headers` | map | — | Extra request headers (secrets expand here). |
| `extract` | map | — | `name: path` pairs, e.g. `stars: data.items[0].count`. |
| `template` | string | raw body | Go `text/template` over the extracted values (`{{.stars}}`) or `{{.json}}` for the whole document. |

## Theming

Bundled: `catppuccin-mocha`, `catppuccin-latte`, `tokyo-night`,
`gruvbox-dark`, `nord`, `dracula`, and `default` (adapts to the terminal
background).

Custom themes inherit from a base and override individual colors (validated
hex):

```yaml
themes:
  my-theme:
    base: catppuccin-mocha
    colors:
      accent: "#ff79c6"
      border_focused: "#ff79c6"
```

Color keys: `bg`, `fg`, `muted`, `border`, `border_focused`, `accent`,
`accent_alt`, `success`, `warning`, `error`, `info`.

Press `t` to cycle themes live; the choice is persisted back to `theme:` in
the config file.

## Validation errors

`panely config validate` (and hot reload) report errors with YAML line
numbers and typo suggestions:

```
config.yaml:12: rows[1].panels[0].type: unknown panel type "wether", did you mean "weather"?
```

## Comment preservation

panely writes the config back when you switch themes or use edit mode. All
comments outside the `rows:` block are preserved; comments inside `rows:` are
lost when the layout is rewritten (see docs/DECISIONS.md).
