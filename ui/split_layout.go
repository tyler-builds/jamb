package ui

import (
	"math"
	"slices"

	"github.com/tyler-builds/jamb/internal/layout"
)

// paneIn is a pane as the layout of a split sees it. See resolvePanes.
type paneIn struct {
	sized  bool    // the pane has a size, a pixel or percentage length
	size   Length  // the size, when sized
	lo, hi float64 // the limits in logical pixels at the split's length; hi is +Inf with no most
	weight float64 // the share of the fill space a fill pane takes, relative to the other fill panes
	hidden bool    // the pane is collapsed, and takes no space
}

// resolvePanes returns the size of each pane of a split whose panes have avail logical pixels
// between them: the length of the split, less the dividers. A sized pane takes its size, a
// percentage of avail for a percentage size, clamped to its limits. The fill panes share what
// the sized panes leave, by weight, each within its limits. When the sized panes need more than
// avail, the fill panes go to their least sizes and the sized panes shrink toward theirs, in
// proportion to how far each is above its least size; content that still does not fit is
// clipped. When no pane can take the space that is left over, the last visible pane takes it
// for display: base keeps the size the pane stores, and shown is the size it is drawn at.
// Collapsed panes have no size.
func resolvePanes(ps []paneIn, avail float64) (base, shown []float64) {
	base = make([]float64, len(ps))
	var sized, fills []int
	sizedSum, fillLo := 0.0, 0.0
	last := -1
	for i, p := range ps {
		if p.hidden {
			continue
		}
		last = i
		if !p.sized {
			fills = append(fills, i)
			fillLo += p.lo
			continue
		}
		base[i] = min(max(pxOf(p.size, avail), p.lo), p.hi)
		sized = append(sized, i)
		sizedSum += base[i]
	}
	if sizedSum+fillLo > avail {
		for _, i := range fills {
			base[i] = ps[i].lo
		}
		excess := sizedSum - (avail - fillLo)
		slack := 0.0
		for _, i := range sized {
			slack += base[i] - ps[i].lo
		}
		for _, i := range sized {
			if slack <= excess {
				base[i] = ps[i].lo
			} else {
				base[i] -= excess * (base[i] - ps[i].lo) / slack
			}
		}
	} else {
		waterFill(ps, fills, base, avail-sizedSum)
	}
	shown = slices.Clone(base)
	if last >= 0 {
		used := 0.0
		for i, p := range ps {
			if !p.hidden {
				used += base[i]
			}
		}
		if left := avail - used; left > 0 {
			shown[last] += left
		}
	}
	return base, shown
}

// waterFill shares remaining among the fill panes, in proportion to their weights, and stores the
// shares in base. A pane whose share is past one of its limits takes that limit, and the others
// share what is left. The panes share equally when their weights are all zero.
func waterFill(ps []paneIn, fills []int, base []float64, remaining float64) {
	active := slices.Clone(fills)
	for len(active) > 0 {
		total := 0.0
		for _, i := range active {
			total += max(ps[i].weight, 0)
		}
		equal := total <= 0
		if equal {
			total = float64(len(active))
		}
		share := func(i int) float64 {
			if equal {
				return remaining / total
			}
			return remaining * max(ps[i].weight, 0) / total
		}
		fixed := false
		for j, i := range active {
			s := share(i)
			if s >= ps[i].lo && s <= ps[i].hi {
				continue
			}
			base[i] = min(max(s, ps[i].lo), ps[i].hi)
			remaining -= base[i]
			active = slices.Delete(active, j, j+1)
			fixed = true
			break
		}
		if !fixed {
			for _, i := range active {
				base[i] = share(i)
			}
			return
		}
	}
}

// pxOf returns the logical pixels of l at a length of avail pixels: a pixel length as it is, and a
// percentage of avail.
func pxOf(l Length, avail float64) float64 {
	if l.Unit == layout.UnitPercent {
		return float64(l.Value) / 100 * avail
	}
	return float64(l.Value)
}

// paneLimits returns the least and the greatest size of a pane, in logical pixels, at a split
// length of avail. The greatest is never less than the least, so the least wins when the two
// contradict each other.
func paneLimits(p *PaneElement, avail float64) (lo, hi float64) {
	lo = max(pxOf(p.min, avail), 0)
	hi = math.Inf(1)
	if p.max.Unit != layout.UnitAuto {
		hi = max(pxOf(p.max, avail), 0)
	}
	return lo, max(hi, lo)
}
