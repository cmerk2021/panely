package layout

import (
	"testing"

	"github.com/cmerk2021/panely/internal/config"
)

func cfg(rows ...config.Row) *config.Config { return &config.Config{Rows: rows} }

func row(height string, weights ...int) config.Row {
	r := config.Row{Height: height}
	for _, w := range weights {
		r.Panels = append(r.Panels, config.PanelConfig{Type: "clock", Weight: w})
	}
	return r
}

func TestLayoutExactFill(t *testing.T) {
	cases := []struct {
		name          string
		cfg           *config.Config
		width, height int
	}{
		{"single", cfg(row("", 1)), 80, 24},
		{"two rows weights", cfg(row("30%", 1, 2), row("", 1, 1, 2)), 80, 24},
		{"odd width thirds", cfg(row("", 1, 1, 1)), 79, 23},
		{"fixed and auto", cfg(row("5", 1), row("", 2, 1), row("25%", 1)), 101, 37},
		{"tiny", cfg(row("", 1, 1)), 20, 6},
		{"huge", cfg(row("10%", 1, 2, 3), row("", 5, 1), row("auto", 1)), 500, 200},
		{"prime sizes", cfg(row("", 3, 7, 2)), 97, 31},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rects := Layout(tc.cfg, tc.width, tc.height)
			if rects == nil {
				t.Fatalf("expected rects, got nil")
			}
			// Sum of areas must equal the full area, with no overlap and
			// rows exactly filling each band.
			area := 0
			for _, r := range rects {
				if r.X < 0 || r.Y < 0 || r.X+r.Width > tc.width || r.Y+r.Height > tc.height {
					t.Errorf("rect out of bounds: %+v", r)
				}
				area += r.Width * r.Height
			}
			if area != tc.width*tc.height {
				t.Errorf("area %d != %d (rects do not exactly fill the space)", area, tc.width*tc.height)
			}
		})
	}
}

func TestLayoutWeightsProportional(t *testing.T) {
	rects := Layout(cfg(row("", 2, 1)), 90, 24)
	if rects[0].Width != 60 || rects[1].Width != 30 {
		t.Errorf("weights 2:1 over 90 => got %d, %d; want 60, 30", rects[0].Width, rects[1].Width)
	}
}

func TestLayoutTooSmallTerminal(t *testing.T) {
	if r := Layout(cfg(row("", 1)), 5, 3); r != nil {
		t.Errorf("expected nil for tiny terminal, got %v", r)
	}
	if r := Layout(nil, 80, 24); r != nil {
		t.Errorf("expected nil for nil config")
	}
}

func TestLayoutMinSizeFlag(t *testing.T) {
	c := cfg(config.Row{Panels: []config.PanelConfig{
		{Type: "clock", MinWidth: 100},
		{Type: "clock"},
	}})
	rects := Layout(c, 80, 24)
	if !rects[0].TooSmall {
		t.Error("expected first panel flagged TooSmall")
	}
	if rects[1].TooSmall {
		t.Error("second panel should fit")
	}
}

func TestHitTest(t *testing.T) {
	rects := Layout(cfg(row("50%", 1, 1), row("", 1)), 80, 24)
	if i := HitTest(rects, 0, 0); i != 0 {
		t.Errorf("hit (0,0) => %d, want 0", i)
	}
	if i := HitTest(rects, 79, 0); i != 1 {
		t.Errorf("hit (79,0) => %d, want 1", i)
	}
	if i := HitTest(rects, 40, 23); i != 2 {
		t.Errorf("hit (40,23) => %d, want 2", i)
	}
	if i := HitTest(rects, 200, 200); i != -1 {
		t.Errorf("hit outside => %d, want -1", i)
	}
}

func TestLayoutFuzzExactFill(t *testing.T) {
	// Sweep many sizes to guarantee exact fill with rounding.
	c := cfg(row("33%", 1, 2), row("", 1, 1, 1), row("7", 3, 2))
	for w := MinTerminalWidth; w <= 200; w += 7 {
		for h := MinTerminalHeight; h <= 100; h += 5 {
			rects := Layout(c, w, h)
			area := 0
			for _, r := range rects {
				area += r.Width * r.Height
			}
			if area != w*h {
				t.Fatalf("size %dx%d: area %d != %d", w, h, area, w*h)
			}
		}
	}
}
