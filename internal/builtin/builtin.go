// Package builtin registers all built-in panels and providers via blank
// imports. Adding an integration means adding one package and one import
// line here.
package builtin

import (
	// Panels.
	_ "github.com/cmerk2021/panely/internal/panels/calendar"
	_ "github.com/cmerk2021/panely/internal/panels/clock"
	_ "github.com/cmerk2021/panely/internal/panels/command"
	_ "github.com/cmerk2021/panely/internal/panels/docker"
	_ "github.com/cmerk2021/panely/internal/panels/gitpanel"
	_ "github.com/cmerk2021/panely/internal/panels/httpjson"
	_ "github.com/cmerk2021/panely/internal/panels/links"
	_ "github.com/cmerk2021/panely/internal/panels/notes"
	_ "github.com/cmerk2021/panely/internal/panels/quote"
	_ "github.com/cmerk2021/panely/internal/panels/rss"
	_ "github.com/cmerk2021/panely/internal/panels/services"
	_ "github.com/cmerk2021/panely/internal/panels/sysinfo"
	_ "github.com/cmerk2021/panely/internal/panels/todo"
	_ "github.com/cmerk2021/panely/internal/panels/weather"

	// Providers.
	_ "github.com/cmerk2021/panely/internal/providers/caldavtodo"
	_ "github.com/cmerk2021/panely/internal/providers/icscal"
	_ "github.com/cmerk2021/panely/internal/providers/localtodo"
)
