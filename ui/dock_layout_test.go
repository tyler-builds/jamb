package ui

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// grp returns a tab group of the panels ps, showing the panel active.
func grp(active string, ps ...string) DockNode { return DockNode{Panels: ps, Active: active} }

// sized returns a split of kids, with the shares sizes.
func sized(vertical bool, sizes []float64, kids ...DockNode) DockNode {
	return DockNode{Vertical: vertical, Sizes: sizes, Children: kids}
}

// dockText returns the layout as a short text, for comparing the trees of the tests. A
// group is its panels in tab order, with the active one marked by an asterisk; a split
// is its direction and its children, each with its share.
func dockText(n DockNode) string {
	if !n.isSplit() {
		if n.empty() {
			return "none"
		}
		var ps []string
		for _, p := range n.Panels {
			if p == n.Active {
				p += "*"
			}
			ps = append(ps, p)
		}
		return "[" + strings.Join(ps, " ") + "]"
	}
	dir := "H"
	if n.Vertical {
		dir = "V"
	}
	var parts []string
	for i, c := range n.Children {
		parts = append(parts, fmt.Sprintf("%.2f %s", n.Sizes[i], dockText(c)))
	}
	return dir + "{" + strings.Join(parts, ", ") + "}"
}

// layoutText returns the layout's tree, and its floating groups after it.
func layoutText(l DockLayout) string {
	s := dockText(l.Root)
	for _, f := range l.Floating {
		s += fmt.Sprintf(" float(%s %d,%d %dx%d %s)", f.ID, f.X, f.Y, f.Width, f.Height, dockText(f.Group))
	}
	return s
}

// dockBase is the layout the operation tests change: a row of a group [a b] with the
// active a, and a column of the groups [c] and [d].
func dockBase() DockLayout {
	return DockLayout{Root: sized(false, []float64{0.5, 0.5},
		grp("a", "a", "b"),
		sized(true, []float64{0.5, 0.5}, grp("c", "c"), grp("d", "d")),
	)}
}

func TestDockLayoutOperations(t *testing.T) {
	base := dockBase()
	cases := []struct {
		name string
		l    DockLayout
		op   func(DockLayout) DockLayout
		want string
	}{
		{"move into a group as a tab", base, func(l DockLayout) DockLayout { return l.Move("a", "d", DockCenter) },
			"H{0.50 [b*], 0.50 V{0.50 [c*], 0.50 [d a*]}}"},
		{"move beside a group in a split of the same direction", base, func(l DockLayout) DockLayout {
			return l.Move("a", "c", DockBottom)
		}, "H{0.50 [b*], 0.50 V{0.25 [c*], 0.25 [a*], 0.50 [d*]}}"},
		{"move beside a group across the direction wraps it", base, func(l DockLayout) DockLayout {
			return l.Move("a", "c", DockRight)
		}, "H{0.50 [b*], 0.50 V{0.50 H{0.50 [c*], 0.50 [a*]}, 0.50 [d*]}}"},
		{"move to the top edge of the dock", base, func(l DockLayout) DockLayout { return l.Move("a", "", DockTop) },
			"V{0.50 [a*], 0.50 H{0.50 [b*], 0.50 V{0.50 [c*], 0.50 [d*]}}}"},
		{"split the own group, with another panel of it", base, func(l DockLayout) DockLayout {
			return l.Move("a", "a", DockRight)
		}, "H{0.25 [b*], 0.25 [a*], 0.50 V{0.50 [c*], 0.50 [d*]}}"},
		{"split a panel alone in its group does nothing", base, func(l DockLayout) DockLayout {
			return l.Move("c", "c", DockRight)
		}, "H{0.50 [a* b], 0.50 V{0.50 [c*], 0.50 [d*]}}"},
		{"a center drop on the own group moves the panel last and shows it", base, func(l DockLayout) DockLayout {
			return l.Move("b", "b", DockCenter)
		}, "H{0.50 [a b*], 0.50 V{0.50 [c*], 0.50 [d*]}}"},
		{"an edge zone with no target and a center zone change nothing", base, func(l DockLayout) DockLayout {
			return l.Move("b", "", DockCenter)
		}, "H{0.50 [a* b], 0.50 V{0.50 [c*], 0.50 [d*]}}"},
		{"an unknown target leaves the layout", base, func(l DockLayout) DockLayout { return l.Move("a", "zz", DockCenter) },
			"H{0.50 [a* b], 0.50 V{0.50 [c*], 0.50 [d*]}}"},
		{"close a panel of a group", base, func(l DockLayout) DockLayout { return l.Close("a") },
			"H{0.50 [b*], 0.50 V{0.50 [c*], 0.50 [d*]}}"},
		{"close the last panel of a split's group removes the split", base, func(l DockLayout) DockLayout {
			return l.Close("c")
		}, "H{0.50 [a* b], 0.50 [d*]}"},
		{"focus a panel of a group", base, func(l DockLayout) DockLayout { return l.Focus("b") },
			"H{0.50 [a b*], 0.50 V{0.50 [c*], 0.50 [d*]}}"},
		{"show an unknown panel in the first group", base, func(l DockLayout) DockLayout { return l.Show("z") },
			"H{0.50 [a b z*], 0.50 V{0.50 [c*], 0.50 [d*]}}"},
		{"show a panel in an empty dock", DockLayout{}, func(l DockLayout) DockLayout { return l.Show("z") },
			"[z*]"},
		{"float a panel", base, func(l DockLayout) DockLayout { return l.Float("a", 10, 20, 300, 200) },
			"H{0.50 [b*], 0.50 V{0.50 [c*], 0.50 [d*]}} float(float-1 10,20 300x200 [a*])"},
		{"float a panel that floats alone only moves its window", base.Float("a", 10, 20, 300, 200),
			func(l DockLayout) DockLayout { return l.Float("a", 40, 50, 320, 240) },
			"H{0.50 [b*], 0.50 V{0.50 [c*], 0.50 [d*]}} float(float-1 40,50 320x240 [a*])"},
		{"dock back a floating panel on the right edge", base.Float("a", 10, 20, 300, 200),
			func(l DockLayout) DockLayout { return l.Dock("a") },
			"H{0.33 [b*], 0.33 V{0.50 [c*], 0.50 [d*]}, 0.33 [a*]}"},
		{"dock back a panel that is docked does nothing", base, func(l DockLayout) DockLayout { return l.Dock("a") },
			"H{0.50 [a* b], 0.50 V{0.50 [c*], 0.50 [d*]}}"},
		{"move a panel to the side group, the first one in the direction", base, func(l DockLayout) DockLayout {
			return l.MoveSide("a", DockRight)
		}, "H{0.50 [b*], 0.50 V{0.50 [c a*], 0.50 [d*]}}"},
		{"move a panel left into the group that is beside it", base, func(l DockLayout) DockLayout {
			return l.MoveSide("c", DockLeft)
		}, "H{0.50 [a b c*], 0.50 [d*]}"},
		{"move a panel where no group lies does nothing", base, func(l DockLayout) DockLayout {
			return l.MoveSide("a", DockLeft)
		}, "H{0.50 [a* b], 0.50 V{0.50 [c*], 0.50 [d*]}}"},
		{"normalize merges a split into one of the same direction", DockLayout{Root: sized(false, []float64{0.5, 0.5},
			sized(false, []float64{0.5, 0.5}, grp("a", "a"), grp("b", "b")), grp("c", "c"))},
			func(l DockLayout) DockLayout { return l.Focus("a") },
			"H{0.25 [a*], 0.25 [b*], 0.50 [c*]}"},
		{"normalize keeps one copy of a repeated panel", DockLayout{Root: sized(false, []float64{1, 1},
			grp("a", "a"), grp("a", "a"))},
			func(l DockLayout) DockLayout { return l.Focus("a") }, "[a*]"},
		{"reset returns the default", base, func(DockLayout) DockLayout { return dockBase().Clone() },
			"H{0.50 [a* b], 0.50 V{0.50 [c*], 0.50 [d*]}}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := layoutText(c.l)
			got := layoutText(c.op(c.l))
			if got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
			if layoutText(c.l) != before {
				t.Errorf("the operation changed its argument")
			}
		})
	}
}

// TestDockLayoutOperationsKeepSizesSummingToOne checks the shares of every split after a
// change: they sum to 1, so a split never shows a gap or overflows.
func TestDockLayoutOperationsKeepSizesSummingToOne(t *testing.T) {
	l := dockBase()
	ops := []DockLayout{
		l.Move("a", "d", DockRight),
		l.Move("b", "", DockBottom),
		l.Move("c", "a", DockTop),
		l.Close("d"),
		l.Float("c", 0, 0, 100, 100),
	}
	for i, o := range ops {
		var walk func(n DockNode)
		walk = func(n DockNode) {
			if !n.isSplit() {
				return
			}
			sum := 0.0
			for _, s := range n.Sizes {
				sum += s
			}
			if len(n.Sizes) != len(n.Children) || sum < 0.999 || sum > 1.001 {
				t.Errorf("op %d: sizes %v for %d children do not sum to 1", i, n.Sizes, len(n.Children))
			}
			for _, c := range n.Children {
				walk(c)
			}
		}
		walk(o.Root)
	}
}

func TestDockLayoutJSONRoundTrip(t *testing.T) {
	l := dockBase().Float("a", 10, 20, 300, 200)
	data, err := json.Marshal(l)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back DockLayout
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(back, l) {
		t.Errorf("round trip changed the layout:\n got %s\nwant %s", layoutText(back), layoutText(l))
	}
}

// TestDockLayoutJSONTolerance reads a saved layout that names a panel the app no longer
// has, and one with fields the layout does not know. The known panels are kept, the
// unknown one is dropped, and the layout still normalizes.
func TestDockLayoutJSONTolerance(t *testing.T) {
	saved := `{"root":{"vertical":false,"sizes":[0.5,0.5],"children":[
		{"panels":["a","ghost"],"active":"ghost"},
		{"panels":["c"],"active":"c","future":true}]},
		"version":7}`
	var l DockLayout
	if err := json.Unmarshal([]byte(saved), &l); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := dockText(normalizeLayout(l, func(id string) bool { return id != "ghost" }).Root)
	if want := "H{0.50 [a*], 0.50 [c*]}"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestDockZoneHitTesting(t *testing.T) {
	cases := []struct {
		x, y float32
		want DockZone
	}{
		{5, 50, DockLeft}, {95, 50, DockRight}, {50, 5, DockTop}, {50, 95, DockBottom},
		{50, 50, DockCenter}, {20, 20, DockLeft}, {30, 50, DockCenter},
	}
	for _, c := range cases {
		if got := dockZoneAt(c.x, c.y, 100, 100); got != c.want {
			t.Errorf("dockZoneAt(%v, %v) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}
