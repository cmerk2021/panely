// Package layout computes panel rectangles from the config, terminal width,
// and height. It is a pure function of its inputs so it can be unit tested.
package layout

import (
	"strconv"
	"strings"

	"github.com/cmerk2021/panely/internal/config"
)

// Rect is a panel's position and size in terminal cells.
type Rect struct {
	Row, Col      int // indices into config rows/panels
	X, Y          int
	Width, Height int
	// TooSmall marks a panel whose min_width/min_height cannot be satisfied.
	TooSmall bool
}

// MinTerminalWidth and MinTerminalHeight are the smallest usable sizes;
// below these, Layout returns nil and callers should show a hint.
const (
	MinTerminalWidth  = 20
	MinTerminalHeight = 6
)

// Layout computes rects for every panel. The union of rects exactly fills
// width x height. Returns nil if the terminal is unusably small.
func Layout(cfg *config.Config, width, height int) []Rect {
	if cfg == nil || len(cfg.Rows) == 0 || width < MinTerminalWidth || height < MinTerminalHeight {
		return nil
	}
	rowHeights := distributeRows(cfg.Rows, height)
	var rects []Rect
	y := 0
	for ri, row := range cfg.Rows {
		h := rowHeights[ri]
		if h <= 0 {
			continue
		}
		widths := distributeWeights(panelWeights(row.Panels), width)
		x := 0
		for pi := range row.Panels {
			w := widths[pi]
			r := Rect{Row: ri, Col: pi, X: x, Y: y, Width: w, Height: h}
			p := row.Panels[pi]
			if (p.MinWidth > 0 && w < p.MinWidth) || (p.MinHeight > 0 && h < p.MinHeight) {
				r.TooSmall = true
			}
			rects = append(rects, r)
			x += w
		}
		y += h
	}
	return rects
}

// HitTest returns the index in rects containing (x, y), or -1.
func HitTest(rects []Rect, x, y int) int {
	for i, r := range rects {
		if x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height {
			return i
		}
	}
	return -1
}

func panelWeights(panels []config.PanelConfig) []int {
	ws := make([]int, len(panels))
	for i, p := range panels {
		ws[i] = p.EffectiveWeight()
	}
	return ws
}

// distributeRows assigns heights to rows: fixed line counts and percentages
// first, then remaining space split evenly among auto rows. The result always
// sums exactly to total (clamping if fixed rows oversubscribe).
func distributeRows(rows []config.Row, total int) []int {
	n := len(rows)
	heights := make([]int, n)
	autoIdx := []int{}
	used := 0
	for i, r := range rows {
		switch {
		case r.Height == "" || r.Height == "auto":
			autoIdx = append(autoIdx, i)
		case strings.HasSuffix(r.Height, "%"):
			pct, _ := strconv.Atoi(strings.TrimSuffix(r.Height, "%"))
			heights[i] = total * pct / 100
			used += heights[i]
		default:
			lines, _ := strconv.Atoi(r.Height)
			heights[i] = lines
			used += lines
		}
	}
	remaining := total - used
	if len(autoIdx) > 0 {
		share := distributeWeights(ones(len(autoIdx)), max0(remaining))
		for j, i := range autoIdx {
			heights[i] = share[j]
		}
	}
	// Fix any over/undershoot so rows exactly fill the height: adjust the
	// largest row (never below 1 while shrinking).
	diff := total - sum(heights)
	for diff != 0 {
		i := largestIdx(heights)
		if diff > 0 {
			heights[i]++
			diff--
		} else {
			if heights[i] <= 1 {
				// Everything is at minimum; shrink anyway to honor the
				// exact-fill contract (a row may hit 0 on tiny terminals).
				heights[i]--
				diff++
				continue
			}
			heights[i]--
			diff++
		}
	}
	return heights
}

// distributeWeights splits total into len(weights) integer parts proportional
// to weights, summing exactly to total (largest-remainder method).
func distributeWeights(weights []int, total int) []int {
	n := len(weights)
	out := make([]int, n)
	if n == 0 || total <= 0 {
		return out
	}
	wsum := sum(weights)
	if wsum <= 0 {
		wsum = n
		weights = ones(n)
	}
	remainders := make([]int, n) // scaled remainders
	assigned := 0
	for i, w := range weights {
		out[i] = total * w / wsum
		remainders[i] = total*w - out[i]*wsum
		assigned += out[i]
	}
	// Hand out the leftover cells to the largest remainders.
	for assigned < total {
		best := 0
		for i := 1; i < n; i++ {
			if remainders[i] > remainders[best] {
				best = i
			}
		}
		out[best]++
		remainders[best] = -1
		assigned++
	}
	return out
}

func ones(n int) []int {
	o := make([]int, n)
	for i := range o {
		o[i] = 1
	}
	return o
}

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

func largestIdx(xs []int) int {
	best := 0
	for i, x := range xs {
		if x > xs[best] {
			best = i
		}
	}
	return best
}

func max0(x int) int {
	if x < 0 {
		return 0
	}
	return x
}
