# Design decisions and assumptions

This document records judgment calls made while building panely v1.

## Module path

The spec asked for `github.com/OWNER/panely` with a placeholder owner. The
module is declared as `github.com/cmerk2021/panely` (the actual repository
owner) so that `go install github.com/cmerk2021/panely/cmd/panely@latest`
works out of the box. Forks should update `go.mod` and the import paths
(one `sed` invocation; see README).

## CLI

Stdlib `flag` with hand-rolled subcommand dispatch instead of cobra, to keep
the dependency tree small. Subcommands: `version`, `themes`,
`config path|validate|init`.

## YAML comment preservation

`config.Save` re-reads the existing file into a `yaml.Node` tree and
surgically replaces only the `rows` and `theme` values, leaving every other
node — and its comments — untouched. Consequences:

- Top-level comments, comments on `refresh`, `themes`, etc. survive edit-mode
  saves and theme switches.
- Comments *inside* the `rows:` block are lost when edit mode rewrites the
  layout, because the whole sequence is re-serialized. Preserving per-panel
  comments across structural edits (moves, deletes) has no well-defined
  answer, so this is accepted and documented.

## Panel rebuild on layout change

Editing the layout (or hot-reloading the config, or switching themes)
rebuilds all panel instances. Runtime-only state (scroll positions, in-flight
edits) is reset; persisted state (todos, notes) is unaffected. This keeps the
edit-mode implementation simple and predictable. Stale async results from old
instances are discarded via monotonically increasing panel instance IDs.

## Spatial focus navigation

Moving focus with `h/j/k/l`/arrows picks the panel whose center is nearest in
the movement direction, weighting perpendicular distance 4×. Bare movement
keys are forwarded to the focused panel if it binds them; `ctrl+h/j/k/l` and
`alt+arrows` always move focus.

## Ping checks

The `services` panel's `ping` target type uses a TCP connect (port 80/
specified port) rather than ICMP, because raw sockets require privileges.
This matches the spec's "TCP connect fallback" allowance; real ICMP was not
implemented at all to avoid capability/setuid complications.

## Docker stats

The docker panel lists containers via the socket with the stdlib HTTP client.
Per-container CPU/memory stats were skipped: `/containers/{id}/stats` requires
one streaming request per container, which is not "cheap" for many
containers. State, image, and ports are shown.

## Process CPU percentages

gopsutil's `CPUPercent` without a sampling interval reports lifetime-average
CPU per process (instantaneous sampling would block). Accepted for v1.

## Calendar recurrence

ICS recurring events are expanded with `teambition/rrule-go` from the
`RRULE` property. `EXDATE`/`RDATE` and overridden instances
(`RECURRENCE-ID`) are not handled in v1 — rare in practice, documented here.

## Status bar height

The layout engine reserves exactly one terminal row for the status/help bar;
`Layout()` guarantees the panel rects exactly fill `width × (height-1)`.
Below 20×6 the dashboard shows a "terminal too small" hint instead of panels.

## Weather geocoding

Open-Meteo's geocoding API resolves `location` strings; results are cached in
memory for the process lifetime, and forecasts are cached for 15 minutes on
disk via the shared HTTP cache.

## nfpm maintainer

The `.goreleaser.yaml` nfpm maintainer/homepage use placeholder values
(`OWNER <owner@example.com>`); update before publishing packages.
