package theme

import (
	"testing"

	"github.com/cmerk2021/panely/internal/config"
)

func TestBundledThemesComplete(t *testing.T) {
	for name, th := range bundled {
		if th.Name != name {
			t.Errorf("theme %q has Name %q", name, th.Name)
		}
		if th.Fg == nil || th.Border == nil || th.Accent == nil || th.Error == nil {
			t.Errorf("theme %q missing colors", name)
		}
	}
	for _, want := range []string{"catppuccin-mocha", "catppuccin-latte", "tokyo-night", "gruvbox-dark", "nord", "dracula", "default"} {
		if _, ok := bundled[want]; !ok {
			t.Errorf("missing bundled theme %q", want)
		}
	}
}

func TestResolveCustomThemeInheritance(t *testing.T) {
	cfg := &config.Config{Themes: map[string]config.CustomTheme{
		"mine": {Base: "dracula", Colors: map[string]string{"accent": "#123456"}},
	}}
	th, err := Resolve("mine", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if th.Name != "mine" {
		t.Errorf("name: %s", th.Name)
	}
	if th.Accent != hex("#123456") {
		t.Errorf("accent override not applied: %v", th.Accent)
	}
	if th.Fg != bundled["dracula"].Fg {
		t.Errorf("base fg not inherited")
	}
}

func TestResolveUnknownFallsBack(t *testing.T) {
	th, err := Resolve("nope", nil)
	if err == nil {
		t.Error("expected error for unknown theme")
	}
	if th.Name != "default" {
		t.Errorf("expected default fallback, got %s", th.Name)
	}
}

func TestResolveUnknownColorKey(t *testing.T) {
	cfg := &config.Config{Themes: map[string]config.CustomTheme{
		"bad": {Colors: map[string]string{"sparkle": "#ffffff"}},
	}}
	if _, err := Resolve("bad", cfg); err == nil {
		t.Error("expected error for unknown color key")
	}
}

func TestResolveCycleDetected(t *testing.T) {
	cfg := &config.Config{Themes: map[string]config.CustomTheme{
		"a": {Base: "b"},
		"b": {Base: "a"},
	}}
	if _, err := Resolve("a", cfg); err == nil {
		t.Error("expected cycle error")
	}
}

func TestNamesIncludesCustom(t *testing.T) {
	cfg := &config.Config{Themes: map[string]config.CustomTheme{"mine": {}}}
	names := Names(cfg)
	found := false
	for _, n := range names {
		if n == "mine" {
			found = true
		}
	}
	if !found {
		t.Error("custom theme missing from Names")
	}
}
