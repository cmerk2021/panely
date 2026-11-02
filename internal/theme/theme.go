// Package theme defines semantic colors and Lip Gloss style helpers. All
// panels style exclusively through a Theme; no colors are hard-coded.
package theme

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/lipgloss"

	"github.com/cmerk2021/panely/internal/config"
)

// Theme is a set of named semantic colors.
type Theme struct {
	Name string

	Bg            lipgloss.TerminalColor
	Fg            lipgloss.TerminalColor
	Muted         lipgloss.TerminalColor
	Border        lipgloss.TerminalColor
	BorderFocused lipgloss.TerminalColor
	Accent        lipgloss.TerminalColor
	AccentAlt     lipgloss.TerminalColor
	Success       lipgloss.TerminalColor
	Warning       lipgloss.TerminalColor
	Error         lipgloss.TerminalColor
	Info          lipgloss.TerminalColor
}

// Style helpers. Panels should only use these (or colors from the struct).

func (t Theme) Text() lipgloss.Style      { return lipgloss.NewStyle().Foreground(t.Fg) }
func (t Theme) MutedText() lipgloss.Style { return lipgloss.NewStyle().Foreground(t.Muted) }
func (t Theme) AccentText() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Accent)
}
func (t Theme) SuccessText() lipgloss.Style { return lipgloss.NewStyle().Foreground(t.Success) }
func (t Theme) WarningText() lipgloss.Style { return lipgloss.NewStyle().Foreground(t.Warning) }
func (t Theme) ErrorText() lipgloss.Style   { return lipgloss.NewStyle().Foreground(t.Error) }
func (t Theme) InfoText() lipgloss.Style    { return lipgloss.NewStyle().Foreground(t.Info) }

// PanelBorder returns the rounded border style for a panel.
func (t Theme) PanelBorder(focused bool) lipgloss.Style {
	c := t.Border
	if focused {
		c = t.BorderFocused
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c)
}

// Title returns the style for panel titles.
func (t Theme) Title(focused bool) lipgloss.Style {
	c := t.Muted
	if focused {
		c = t.Accent
	}
	return lipgloss.NewStyle().Foreground(c).Bold(focused)
}

// StatusBar returns the style for the bottom status bar.
func (t Theme) StatusBar() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Muted)
}

func hex(s string) lipgloss.Color { return lipgloss.Color(s) }

// Bundled themes.
var bundled = map[string]Theme{
	"catppuccin-mocha": {
		Name: "catppuccin-mocha",
		Bg:   hex("#1e1e2e"), Fg: hex("#cdd6f4"), Muted: hex("#6c7086"),
		Border: hex("#45475a"), BorderFocused: hex("#cba6f7"),
		Accent: hex("#cba6f7"), AccentAlt: hex("#89b4fa"),
		Success: hex("#a6e3a1"), Warning: hex("#f9e2af"),
		Error: hex("#f38ba8"), Info: hex("#89dceb"),
	},
	"catppuccin-latte": {
		Name: "catppuccin-latte",
		Bg:   hex("#eff1f5"), Fg: hex("#4c4f69"), Muted: hex("#9ca0b0"),
		Border: hex("#bcc0cc"), BorderFocused: hex("#8839ef"),
		Accent: hex("#8839ef"), AccentAlt: hex("#1e66f5"),
		Success: hex("#40a02b"), Warning: hex("#df8e1d"),
		Error: hex("#d20f39"), Info: hex("#04a5e5"),
	},
	"tokyo-night": {
		Name: "tokyo-night",
		Bg:   hex("#1a1b26"), Fg: hex("#c0caf5"), Muted: hex("#565f89"),
		Border: hex("#3b4261"), BorderFocused: hex("#7aa2f7"),
		Accent: hex("#7aa2f7"), AccentAlt: hex("#bb9af7"),
		Success: hex("#9ece6a"), Warning: hex("#e0af68"),
		Error: hex("#f7768e"), Info: hex("#7dcfff"),
	},
	"gruvbox-dark": {
		Name: "gruvbox-dark",
		Bg:   hex("#282828"), Fg: hex("#ebdbb2"), Muted: hex("#928374"),
		Border: hex("#504945"), BorderFocused: hex("#fabd2f"),
		Accent: hex("#fabd2f"), AccentAlt: hex("#83a598"),
		Success: hex("#b8bb26"), Warning: hex("#fe8019"),
		Error: hex("#fb4934"), Info: hex("#8ec07c"),
	},
	"nord": {
		Name: "nord",
		Bg:   hex("#2e3440"), Fg: hex("#eceff4"), Muted: hex("#616e88"),
		Border: hex("#4c566a"), BorderFocused: hex("#88c0d0"),
		Accent: hex("#88c0d0"), AccentAlt: hex("#81a1c1"),
		Success: hex("#a3be8c"), Warning: hex("#ebcb8b"),
		Error: hex("#bf616a"), Info: hex("#5e81ac"),
	},
	"dracula": {
		Name: "dracula",
		Bg:   hex("#282a36"), Fg: hex("#f8f8f2"), Muted: hex("#6272a4"),
		Border: hex("#44475a"), BorderFocused: hex("#bd93f9"),
		Accent: hex("#bd93f9"), AccentAlt: hex("#ff79c6"),
		Success: hex("#50fa7b"), Warning: hex("#f1fa8c"),
		Error: hex("#ff5555"), Info: hex("#8be9fd"),
	},
	"default": {
		Name:          "default",
		Bg:            lipgloss.NoColor{},
		Fg:            lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#e4e4e4"},
		Muted:         lipgloss.AdaptiveColor{Light: "#8a8a8a", Dark: "#6c6c6c"},
		Border:        lipgloss.AdaptiveColor{Light: "#b2b2b2", Dark: "#4e4e4e"},
		BorderFocused: lipgloss.AdaptiveColor{Light: "#005faf", Dark: "#5fafff"},
		Accent:        lipgloss.AdaptiveColor{Light: "#005faf", Dark: "#5fafff"},
		AccentAlt:     lipgloss.AdaptiveColor{Light: "#8700af", Dark: "#d787ff"},
		Success:       lipgloss.AdaptiveColor{Light: "#008700", Dark: "#5fd75f"},
		Warning:       lipgloss.AdaptiveColor{Light: "#af8700", Dark: "#ffd75f"},
		Error:         lipgloss.AdaptiveColor{Light: "#d70000", Dark: "#ff5f5f"},
		Info:          lipgloss.AdaptiveColor{Light: "#0087af", Dark: "#5fd7ff"},
	},
}

// Names returns bundled theme names plus custom theme names, sorted, with
// custom themes from cfg appended.
func Names(cfg *config.Config) []string {
	names := make([]string, 0, len(bundled))
	for n := range bundled {
		names = append(names, n)
	}
	if cfg != nil {
		for n := range cfg.Themes {
			if _, clash := bundled[n]; !clash {
				names = append(names, n)
			}
		}
	}
	sort.Strings(names)
	return names
}

// Resolve returns the theme with the given name, resolving custom themes
// (with base inheritance) from cfg. Unknown names fall back to "default".
func Resolve(name string, cfg *config.Config) (Theme, error) {
	if t, ok := bundled[name]; ok {
		return t, nil
	}
	if cfg != nil {
		if ct, ok := cfg.Themes[name]; ok {
			return resolveCustom(name, ct, cfg, 0)
		}
	}
	return bundled["default"], fmt.Errorf("unknown theme %q, using default", name)
}

func resolveCustom(name string, ct config.CustomTheme, cfg *config.Config, depth int) (Theme, error) {
	if depth > 8 {
		return bundled["default"], fmt.Errorf("theme %q: inheritance too deep (cycle?)", name)
	}
	base := bundled["default"]
	if ct.Base != "" {
		if b, ok := bundled[ct.Base]; ok {
			base = b
		} else if cbt, ok := cfg.Themes[ct.Base]; ok {
			var err error
			base, err = resolveCustom(ct.Base, cbt, cfg, depth+1)
			if err != nil {
				return base, err
			}
		} else {
			return base, fmt.Errorf("theme %q: unknown base %q", name, ct.Base)
		}
	}
	t := base
	t.Name = name
	for key, val := range ct.Colors {
		c := hex(val)
		switch key {
		case "bg":
			t.Bg = c
		case "fg":
			t.Fg = c
		case "muted":
			t.Muted = c
		case "border":
			t.Border = c
		case "border_focused":
			t.BorderFocused = c
		case "accent":
			t.Accent = c
		case "accent_alt":
			t.AccentAlt = c
		case "success":
			t.Success = c
		case "warning":
			t.Warning = c
		case "error":
			t.Error = c
		case "info":
			t.Info = c
		default:
			return t, fmt.Errorf("theme %q: unknown color key %q", name, key)
		}
	}
	return t, nil
}
